package weather

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	provider "github.com/soulteary/funny-china-weather"
	"github.com/stretchr/testify/require"
)

func resetDirectoryBudget(t *testing.T) {
	t.Helper()
	directoryBudget.Lock()
	start, count := directoryBudget.start, directoryBudget.count
	directoryBudget.start = time.Now()
	directoryBudget.count = 0
	directoryBudget.Unlock()
	t.Cleanup(func() {
		directoryBudget.Lock()
		directoryBudget.start, directoryBudget.count = start, count
		directoryBudget.Unlock()
	})
}

func TestWeatherDirectoryFailuresAndPicker(t *testing.T) {
	app, _ := weatherApp(t)
	resetDirectoryBudget(t)
	for _, tc := range []struct {
		keyword string
		err     error
		status  int
		code    string
	}{
		{"", nil, 400, "invalid_keyword"}, {strings.Repeat("a", 65), nil, 400, "invalid_keyword"}, {"city", context.DeadlineExceeded, 504, "timeout"}, {"city", errors.New("provider down"), 502, "upstream"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			fakeLocations(t, nil, tc.err)
			r, err := app.Test(httptest.NewRequest("GET", "/settings/weather/locations?q="+tc.keyword, nil))
			require.NoError(t, err)
			defer r.Body.Close()
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, tc.status, r.StatusCode)
			require.Contains(t, string(raw), tc.code)
		})
	}
	r, err := app.Test(httptest.NewRequest("GET", "/settings/weather/picker.js", nil))
	require.NoError(t, err)
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.NoError(t, r.Body.Close())
	require.Equal(t, 200, r.StatusCode)
	require.Equal(t, pickerJS, string(raw))
	directoryBudget.Lock()
	directoryBudget.count = 120
	directoryBudget.Unlock()
	for _, path := range []string{"/settings/weather/locations?q=city", "/settings/weather/detect"} {
		r, err = app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		require.Equal(t, 429, r.StatusCode)
		require.NoError(t, r.Body.Close())
	}
}

func TestWeatherDirectoryConcurrencyLimit(t *testing.T) {
	resetDirectoryBudget(t)
	for i := 0; i < cap(directorySlots); i++ {
		require.NoError(t, acquireDirectory())
	}
	defer func() {
		for i := 0; i < cap(directorySlots); i++ {
			<-directorySlots
		}
	}()
	require.ErrorIs(t, acquireDirectory(), errDirectoryBusy)
}

func TestWeatherDetectionRejectsProviderFailureAndAmbiguity(t *testing.T) {
	app, _ := weatherApp(t)
	resetDirectoryBudget(t)
	original := locateProvider
	t.Cleanup(func() { locateProvider = original })
	for _, tc := range []struct {
		name      string
		locateErr error
		values    []provider.Location
	}{
		{name: "upstream unavailable", locateErr: errors.New("no network")},
		{name: "ambiguous result", values: []provider.Location{{ID: "1"}, {ID: "2"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			locateProvider = func(context.Context) (string, error) { return "city", tc.locateErr }
			fakeLocations(t, tc.values, nil)
			r, err := app.Test(httptest.NewRequest("GET", "/settings/weather/detect", nil))
			require.NoError(t, err)
			defer r.Body.Close()
			require.Equal(t, 502, r.StatusCode)
		})
	}
	fakeLocations(t, nil, errors.New("lookup failed"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := resolveLegacy(ctx, "北京, 北京")
	require.Error(t, err)
	_, err = resolveLegacy(ctx, "北京市")
	require.Error(t, err)
}
