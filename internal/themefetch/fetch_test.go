package themefetch

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicAddress(t *testing.T) {
	for address, public := range map[string]bool{
		"93.184.216.34": true, "8.8.8.8": true, "2606:4700:4700::1111": true, "1.1.1.1": true,
		// This machine, private networks and what is local to a link.
		"127.0.0.1": false, "127.255.255.254": false, "::1": false, "0.0.0.0": false, "::": false, "0.1.2.3": false,
		"10.0.0.1": false, "172.16.0.1": false, "172.31.255.255": false, "192.168.1.1": false, "fd00::1": false, "fc00::1": false,
		"169.254.169.254": false, "fe80::1": false,
		// Shared, reserved, documentation and multicast addresses.
		"100.64.0.1": false, "100.127.255.255": false, "192.0.0.1": false, "192.0.2.1": false, "198.18.0.1": false, "198.19.255.255": false,
		"198.51.100.1": false, "203.0.113.1": false, "240.0.0.1": false, "255.255.255.255": false, "224.0.0.1": false, "ff02::1": false, "ff01::1": false,
		"2001:db8::1": false,
		// A private address written inside an IPv6 address.
		"::ffff:127.0.0.1": false, "::ffff:10.0.0.1": false, "::ffff:8.8.8.8": true, "::ffff:100.64.0.1": false, "::ffff:198.18.0.1": false, "::ffff:192.0.2.1": false, "64:ff9b::7f00:1": false, "64:ff9b:1::a00:1": false, "2002:7f00:1::1": false, "2001::1": false,
		// Just outside the ranges.
		"100.63.255.255": true, "100.128.0.0": true, "172.32.0.1": true, "172.15.255.255": true, "198.17.255.255": true, "198.20.0.0": true, "239.255.255.255": false, "223.255.255.255": true,
	} {
		require.Equal(t, public, PublicAddress(netip.MustParseAddr(address)), address)
	}
	require.False(t, PublicAddress(netip.Addr{}))
}

func TestParseAddress(t *testing.T) {
	for address, want := range map[string]struct {
		name    string
		archive bool
		shown   string
	}{
		"https://example.test/zelda.zip":                          {"zelda", true, "https://example.test/zelda.zip"},
		"https://example.test/themes/zelda.yml":                   {"zelda", false, "https://example.test/themes/zelda.yml"},
		"  https://example.test:8443/a/b/night-2.zip?token=1#top": {"night-2", true, "https://example.test:8443/a/b/night-2.zip?token=1"},
		"https://example.test/My%20Theme.zip":                     {"My Theme", true, "https://example.test/My%20Theme.zip"},
		"https://[2606:4700::1]/zelda.yml":                        {"zelda", false, "https://[2606:4700::1]/zelda.yml"},
	} {
		parsed, name, archive, err := ParseAddress(address)
		require.NoError(t, err, address)
		require.Equal(t, want.name, name, address)
		require.Equal(t, want.archive, archive, address)
		require.Equal(t, want.shown, parsed.String(), address)
	}
	for address, reason := range map[string]error{
		"":                                      ErrAddress,
		"zelda.zip":                             ErrAddress,
		"http://example.test/zelda.zip":         ErrAddress,
		"ftp://example.test/zelda.zip":          ErrAddress,
		"file:///themes/zelda.zip":              ErrAddress,
		"https:///zelda.zip":                    ErrAddress,
		"https:zelda.zip":                       ErrAddress,
		"https://user@example.test/zelda.zip":   ErrAddress,
		"https://u:p@example.test/zelda.zip":    ErrAddress,
		"https://exa mple.test/zelda.zip":       ErrAddress,
		"//example.test/zelda.zip":              ErrAddress,
		"HTTPS://example.test/zelda.zip\x00":    ErrAddress,
		"https://example.test/":                 ErrName,
		"https://example.test":                  ErrName,
		"https://example.test/zelda":            ErrName,
		"https://example.test/zelda.tar.gz":     ErrName,
		"https://example.test/.zip":             ErrName,
		"https://example.test/.yml":             ErrName,
		"https://example.test/zelda.zip/":       ErrName,
		"https://example.test/zelda.ZIP":        ErrName,
		"https://example.test/get?file=a.zip":   ErrName,
		"https://example.test/zelda.yml.backup": ErrName,
	} {
		_, _, _, err := ParseAddress(address)
		require.ErrorIs(t, err, reason, address)
	}
}

// serve starts a server with a certificate of its own and returns a fetcher
// that trusts it and reaches this machine.
func serve(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Fetcher) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	return server, &Fetcher{Allow: func(netip.Addr) bool { return true }, RootCAs: pool, AnyHost: true}
}

func TestFetch(t *testing.T) {
	content := "contract: 1\n"
	var asked atomic.Value
	server, fetcher := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked.Store(r.Header.Clone())
		switch r.URL.Path {
		case "/themes/zelda.yml", "/zelda.zip":
			_, err := w.Write([]byte(content))
			require.NoError(t, err)
		case "/moved.yml":
			http.Redirect(w, r, "/themes/zelda.yml", http.StatusFound)
		case "/around.yml":
			http.Redirect(w, r, "/around.yml", http.StatusFound)
		case "/down.yml":
			http.Redirect(w, r, "http://"+r.Host+"/themes/zelda.yml", http.StatusFound)
		case "/named.yml":
			http.Redirect(w, r, "https://user@"+r.Host+"/themes/zelda.yml", http.StatusFound)
		case "/large.yml":
			_, err := w.Write([]byte(strings.Repeat("a", 65)))
			require.NoError(t, err)
		case "/exact.yml":
			_, err := w.Write([]byte(strings.Repeat("a", 64)))
			require.NoError(t, err)
		case "/between.zip":
			_, err := w.Write([]byte(strings.Repeat("a", 100)))
			require.NoError(t, err)
		case "/large.zip":
			_, err := w.Write([]byte(strings.Repeat("a", 129)))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	})
	fetch := func(path string) (Download, error) {
		return fetcher.Fetch(context.Background(), server.URL+path, 64, 128)
	}
	digest := sha256.Sum256([]byte(content))

	got, err := fetch("/themes/zelda.yml")
	require.NoError(t, err)
	require.Equal(t, Download{Name: "zelda", Raw: []byte(content), SHA256: hex.EncodeToString(digest[:])}, got)
	// The request says nothing about the instance or the user.
	headers, _ := asked.Load().(http.Header)
	require.Equal(t, "flare-theme-install", headers.Get("User-Agent"))
	for _, header := range []string{"Cookie", "Authorization", "Referer", "Origin"} {
		require.Empty(t, headers.Get(header), header)
	}

	// Space around an address that was pasted is not part of it.
	got, err = fetcher.Fetch(context.Background(), "  "+server.URL+"/themes/zelda.yml\n", 64, 128)
	require.NoError(t, err)
	require.Equal(t, content, string(got.Raw))

	got, err = fetch("/zelda.zip")
	require.NoError(t, err)
	require.True(t, got.Archive)
	require.Equal(t, "zelda", got.Name)

	// An answer that leads on is followed within the same rules, and the
	// name is that of the address that was given.
	got, err = fetch("/moved.yml")
	require.NoError(t, err)
	require.Equal(t, "moved", got.Name)
	require.Equal(t, content, string(got.Raw))

	for path, reason := range map[string]error{
		"/around.yml": ErrRedirect, "/down.yml": ErrRedirect, "/named.yml": ErrRedirect,
		"/absent.yml": ErrStatus, "/large.yml": ErrSize, "/large.zip": ErrSize,
	} {
		_, err = fetch(path)
		require.ErrorIs(t, err, reason, path)
	}
	// Each kind has its own size.
	got, err = fetch("/exact.yml")
	require.NoError(t, err)
	require.Len(t, got.Raw, 64)
	got, err = fetch("/between.zip")
	require.NoError(t, err)
	require.Len(t, got.Raw, 100)

	_, err = fetcher.Fetch(context.Background(), "http://"+server.Listener.Addr().String()+"/zelda.yml", 64, 128)
	require.ErrorIs(t, err, ErrAddress)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = fetcher.Fetch(cancelled, server.URL+"/themes/zelda.yml", 64, 128)
	require.ErrorIs(t, err, ErrFetch)
}

// A connection is made to a public address only: the server of this test is
// on this machine and is not reached, whatever certificate is trusted, and
// neither is one that a public address leads on to.
func TestFetchReachesPublicAddressesOnly(t *testing.T) {
	var reached atomic.Int32
	server, trusting := serve(t, func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		if r.URL.Path == "/inward.yml" {
			http.Redirect(w, r, "/zelda.yml", http.StatusFound)
		}
	})
	port := server.URL[strings.LastIndex(server.URL, ":")+1:]
	strict := &Fetcher{RootCAs: trusting.RootCAs, AnyHost: true}
	for _, host := range []string{"127.0.0.1", "localhost", "[::1]"} {
		_, err := strict.Fetch(context.Background(), "https://"+host+":"+port+"/zelda.yml", 64, 128)
		require.ErrorIs(t, err, ErrDestination, host)
	}
	// The fetcher of the application does not take a host by its number
	// or by a name of one label in the first place.
	for _, host := range []string{"127.0.0.1", "localhost", "[::1]", "2130706433", "0x7f.0.0.1", "127.1"} {
		_, err := Default.Fetch(context.Background(), "https://"+host+":"+port+"/zelda.yml", 64, 128)
		require.ErrorIs(t, err, ErrAddress, host)
	}
	require.Zero(t, reached.Load())
	var err error

	// The address is checked for every connection: the first is allowed
	// and the one it leads on to is not.
	var connections atomic.Int32
	once := &Fetcher{RootCAs: trusting.RootCAs, AnyHost: true, Allow: func(netip.Addr) bool { return connections.Add(1) == 1 }}
	_, err = once.Fetch(context.Background(), server.URL+"/inward.yml", 64, 128)
	require.ErrorIs(t, err, ErrDestination)
	require.EqualValues(t, 1, reached.Load())
	require.EqualValues(t, 2, connections.Load())

	// A certificate that is not trusted is not accepted.
	open := &Fetcher{Allow: func(netip.Addr) bool { return true }, AnyHost: true}
	_, err = open.Fetch(context.Background(), server.URL+"/zelda.yml", 64, 128)
	require.ErrorIs(t, err, ErrFetch)
}

// The form of an address the application fetches: https, a host by a name
// of several labels, and nothing that is read differently by different
// readers of an address.
func TestFetchTakesHostsByName(t *testing.T) {
	for address, accepted := range map[string]bool{
		"https://example.org/zelda.zip":                       true,
		"https://themes.example.co.uk:8443/a/b/zelda.yml?x=1": true,
		"HTTPS://Example.ORG/zelda.zip":                       true,
		"https://xn--bcher-kva.example/zelda.zip":             true,
		"https://a-b.example.org/zelda.zip#top":               true,
		"https://127.0.0.1/zelda.zip":                         false,
		"https://[::1]/zelda.zip":                             false,
		"https://[2606:4700::1]/zelda.zip":                    false,
		"https://2130706433/zelda.zip":                        false,
		"https://0x7f000001/zelda.zip":                        false,
		"https://127.1/zelda.zip":                             false,
		"https://1.2.3.4.5/zelda.zip":                         false,
		"https://localhost/zelda.zip":                         false,
		"https://intranet/zelda.zip":                          false,
		"https://example.org./zelda.zip":                      false,
		"https://.example.org/zelda.zip":                      false,
		"https://-a.example.org/zelda.zip":                    false,
		"https://a_b.example.org/zelda.zip":                   false,
		"https://example.org":                                 false,
		"https://user@example.org/zelda.zip":                  false,
		"https://example.org@127.0.0.1/zelda.zip":             false,
		"https://example.org\\@127.0.0.1/zelda.zip":           false,
		"https://example.org:port/zelda.zip":                  false,
		"https://example.org/zel da.zip":                      false,
		"https://example.org/zelda.zip\nHost: other":          false,
		"http://example.org/zelda.zip":                        false,
		"//example.org/zelda.zip":                             false,
	} {
		require.Equal(t, accepted, namedHost.MatchString(address), address)
		if !accepted {
			_, err := Default.Fetch(context.Background(), address, 64, 128)
			require.ErrorIs(t, err, ErrAddress, address)
		}
	}
}
