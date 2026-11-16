// Package themefetch downloads a theme from an address the administrator
// gives: one HTTPS request to a public host, with a bound on what is read.
// Nothing is fetched unless it is asked for, and nothing is fetched again by
// itself.
package themefetch

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Reasons a theme is not downloaded.
var (
	// ErrAddress reports an address that is not an https address of a
	// host, or one that carries a user name or a password.
	ErrAddress = errors.New("the address is not an https address of a host")
	// ErrName reports an address whose last part is not the name of a theme
	// file or of an archive: <name>.yml or <name>.zip.
	ErrName = errors.New("the address does not end in the name of a theme file or an archive")
	// ErrDestination reports a host that is not reached at a public address:
	// this machine, a private network, or an address that is reserved.
	ErrDestination = errors.New("the host is not at a public address")
	// ErrRedirect reports an answer that leads on too often, or to an
	// address that is not accepted.
	ErrRedirect = errors.New("the address leads on to one that is not followed")
	// ErrStatus reports an answer that is not the file.
	ErrStatus = errors.New("the host did not answer with the file")
	// ErrSize reports a file that is larger than a theme of its kind can be.
	ErrSize = errors.New("the file is larger than a theme can be")
	// ErrFetch reports a request that did not come to an answer.
	ErrFetch = errors.New("the file could not be fetched")
)

const (
	// maximumRedirects is how often an answer can lead on to another
	// address.
	maximumRedirects = 5
	// The time one download can take as a whole, and the time to connect
	// and to the first answer.
	downloadTimeout = 30 * time.Second
	stepTimeout     = 10 * time.Second
	// userAgent names the client without the release or the instance.
	userAgent = "flare-theme-install"
)

// Download is a theme as it was fetched.
type Download struct {
	// Name is the name of the theme: the last part of the address without
	// its extension.
	Name string
	// Archive reports a ZIP file; the download is a theme file otherwise.
	Archive bool
	// Raw is the content, and SHA256 its SHA-256 in hexadecimal digits.
	Raw    []byte
	SHA256 string
}

// Fetcher downloads themes. The zero value is the one for use: it reaches
// public addresses only and trusts the certificates the system trusts.
type Fetcher struct {
	// Allow reports whether a connection to an address is made. It is
	// PublicAddress unless it is set, which is for tests.
	Allow func(netip.Addr) bool
	// RootCAs are the certificate authorities that are trusted, those of
	// the system unless it is set, which is for tests.
	RootCAs *x509.CertPool
}

// Default is the fetcher the application uses.
var Default = &Fetcher{}

// reserved lists the address ranges that are not public beyond those the
// methods of an address know: shared address space, protocol assignments,
// documentation, benchmarking, what was class E, and the prefixes that hold
// an IPv4 address inside an IPv6 address, through which a private address
// could be named.
var reserved = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

// PublicAddress reports whether an address is one of the public internet:
// not this machine, not a private or a link-local network, not a multicast
// address and not a reserved one.
func PublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsInterfaceLocalMulticast() {
		return false
	}
	for _, prefix := range reserved {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

// ParseAddress reads an address a theme is fetched from: an https address
// of a host, without a user name, whose path ends in <name>.yml or
// <name>.zip. It returns the address, the name and whether it is an archive.
// Whether the name is one a theme can have is for the caller.
func ParseAddress(address string) (*url.URL, string, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, "", false, ErrAddress
	}
	parsed.Fragment, parsed.RawFragment = "", ""
	file := parsed.Path[strings.LastIndex(parsed.Path, "/")+1:]
	if name, isArchive := strings.CutSuffix(file, ".zip"); isArchive && name != "" {
		return parsed, name, true, nil
	}
	if name, isFile := strings.CutSuffix(file, ".yml"); isFile && name != "" {
		return parsed, name, false, nil
	}
	return nil, "", false, ErrName
}

// Fetch downloads the theme at an address, reading up to limit bytes of a
// theme file and up to archiveLimit bytes of an archive. The connection is
// made to a public address only, also where the answer leads on to another
// address, and no proxy is used: the address that is checked is the one
// that is connected to.
func (f *Fetcher) Fetch(ctx context.Context, address string, limit, archiveLimit int64) (Download, error) {
	target, name, archive, err := ParseAddress(address)
	if err != nil {
		return Download{}, err
	}
	if archive {
		limit = archiveLimit
	}
	allow := f.Allow
	if allow == nil {
		allow = PublicAddress
	}
	dialer := &net.Dialer{
		Timeout: stepTimeout,
		// The address is checked where the connection is made, after the
		// name is resolved, so a name cannot stand for another address
		// between the check and the connection.
		Control: func(_, remote string, _ syscall.RawConn) error {
			to, parseErr := netip.ParseAddrPort(remote)
			if parseErr != nil || !allow(to.Addr()) {
				return ErrDestination
			}
			return nil
		},
	}
	client := &http.Client{
		Timeout: downloadTimeout,
		Transport: &http.Transport{
			DialContext:            dialer.DialContext,
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.RootCAs},
			TLSHandshakeTimeout:    stepTimeout,
			ResponseHeaderTimeout:  stepTimeout,
			MaxResponseHeaderBytes: 64 << 10,
			DisableKeepAlives:      true,
			ForceAttemptHTTP2:      true,
		},
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) > maximumRedirects || next.URL.Scheme != "https" || next.URL.User != nil {
				return ErrRedirect
			}
			return nil
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Download{}, ErrAddress
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "*/*")
	response, err := client.Do(request)
	if err != nil {
		for _, reason := range []error{ErrDestination, ErrRedirect} {
			if errors.Is(err, reason) {
				return Download{}, reason
			}
		}
		return Download{}, ErrFetch
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Download{}, ErrStatus
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	switch {
	case int64(len(raw)) > limit:
		return Download{}, ErrSize
	case err != nil:
		return Download{}, ErrFetch
	}
	digest := sha256.Sum256(raw)
	return Download{Name: name, Archive: archive, Raw: raw, SHA256: hex.EncodeToString(digest[:])}, nil
}
