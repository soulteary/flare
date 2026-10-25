package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

// These tests use the same native Listener path as the daemon. The transport
// connects in memory, preserving fasthttp's wire parser, limits, and TLS while
// avoiding a listening socket or the net/http compatibility adaptor.
func nativeFiberClient(t *testing.T, app *fiber.App, secure bool) (*http.Client, string) {
	t.Helper()
	listener := fasthttputil.NewInmemoryListener()
	listener.SetLocalAddr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5005})
	var serverListener net.Listener = listener
	transport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return listener.Dial() },
	}
	scheme := "http"
	if secure {
		certificate, roots := nativeFiberCertificate(t)
		serverListener = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		scheme = "https"
	}
	finished := make(chan error, 1)
	go func() {
		finished <- app.Listener(serverListener, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, app.ShutdownWithContext(ctx))
		select {
		case err := <-finished:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("native Fiber listener did not stop")
		}
	})
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{
		Transport: transport, Jar: jar, Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, scheme + "://flare.example.test:5005"
}

func nativeFiberCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "flare.example.test"},
		DNSNames: []string{"flare.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	require.NoError(t, err)
	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, roots
}

func newNativeFiberApp(t *testing.T, login bool) *fiber.App {
	t.Helper()
	originalFlags := define.AppFlags
	originalTheme, originalColor := define.ThemeCurrent, define.ThemePrimaryColor
	t.Cleanup(func() {
		define.AppFlags = originalFlags
		define.ThemeCurrent, define.ThemePrimaryColor = originalTheme, originalColor
	})
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	flags := model.Flags{
		Port: 5005, DisableLoginMode: !login, EnableOfflineMode: true, EnableEditor: true,
		CookieName: "flare", CookieSecret: "native-fiber-cookie-signing-secret-at-least-32-bytes",
		User: "native-user", Pass: "native-password",
	}
	app, err := NewFiberApp(&flags)
	require.NoError(t, err)
	return app
}

func nativeFiberBody(t *testing.T, response *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return string(body)
}

func TestNativeFiberRoutesAndRequestLimits(t *testing.T) {
	app := newNativeFiberApp(t, false)
	readBufferSize := app.Config().ReadBufferSize
	require.Equal(t, 64<<10, readBufferSize, "the candidate must use a 64 KiB request buffer")
	client, base := nativeFiberClient(t, app, false)
	for _, test := range []struct {
		path   string
		status int
	}{
		{define.RegularPages.Home.Path, http.StatusOK},
		{define.RegularPages.Help.Path, http.StatusOK},
		{define.RegularPages.Applications.Path, http.StatusOK},
		{define.RegularPages.Bookmarks.Path, http.StatusOK},
		{define.RegularPages.Settings.Path, http.StatusFound},
		{define.RegularPages.Editor.Path, http.StatusOK},
		{define.MiscPages.HealthCheck.Path, http.StatusOK},
		{define.MiscPages.RedirHome.Path, http.StatusFound},
		{"/favicon.ico", http.StatusOK},
		{"/missing", http.StatusNotFound},
	} {
		response, err := client.Get(base + test.path)
		require.NoError(t, err)
		nativeFiberBody(t, response)
		assert.Equal(t, test.status, response.StatusCode, test.path)
	}

	for _, test := range []struct {
		name   string
		size   int
		status int
	}{
		{"headers within buffer", readBufferSize / 2, http.StatusOK},
		{"headers exceed buffer", readBufferSize + 1024, http.StatusRequestHeaderFieldsTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, base+define.MiscPages.HealthCheck.Path, nil)
			require.NoError(t, err)
			request.Header.Set("X-Large-Fixture", strings.Repeat("x", test.size))
			response, err := client.Do(request)
			require.NoError(t, err)
			nativeFiberBody(t, response)
			assert.Equal(t, test.status, response.StatusCode)
		})
	}

	for _, test := range []struct {
		name   string
		size   int
		status int
	}{
		{"bookmark URL within buffer", readBufferSize / 2, http.StatusFound},
		{"bookmark URL exceeds buffer", readBufferSize, http.StatusRequestHeaderFieldsTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			bookmarkURL := "https://example.test/" + strings.Repeat("x", test.size)
			require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "long URL", URL: bookmarkURL}}}))
			response, err := client.Get(base + define.MiscPages.RedirHelper.Path + "?go=" + data.Base64EncodeUrl(bookmarkURL))
			require.NoError(t, err)
			nativeFiberBody(t, response)
			assert.Equal(t, test.status, response.StatusCode)
			if test.status == http.StatusFound {
				assert.Equal(t, bookmarkURL, response.Header.Get("Location"))
			}
		})
	}
}

func TestNativeFiberTLSCookieLogin(t *testing.T) {
	app := newNativeFiberApp(t, true)
	client, base := nativeFiberClient(t, app, true)
	response, err := client.PostForm(base+define.MiscPages.Login.Path, url.Values{"username": {"native-user"}, "password": {"native-password"}})
	require.NoError(t, err)
	nativeFiberBody(t, response)
	require.Equal(t, http.StatusFound, response.StatusCode)
	cookies := response.Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].Secure, "the native TLS connection must enable Secure")
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	response, err = client.Get(base + define.SettingPages.Theme.Path)
	require.NoError(t, err)
	nativeFiberBody(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode, "the browser jar must return the Secure session over TLS")

	response, err = client.PostForm(base+define.MiscPages.Logout.Path, nil)
	require.NoError(t, err)
	nativeFiberBody(t, response)
	require.Equal(t, http.StatusFound, response.StatusCode)
	response, err = client.Get(base + define.SettingPages.Theme.Path)
	require.NoError(t, err)
	nativeFiberBody(t, response)
	assert.Equal(t, http.StatusFound, response.StatusCode, "logout must remove authentication on the native TLS path")
}
