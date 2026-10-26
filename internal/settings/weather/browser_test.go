package weather

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/resources/templates"
	provider "github.com/soulteary/funny-china-weather"
	"github.com/stretchr/testify/require"
)

func TestWeatherBrowser(t *testing.T) {
	if os.Getenv("FLARE_WEATHER_BROWSER") != "1" {
		t.Skip("opt-in browser test; requires tests/clock Playwright installation")
	}
	runWeatherBrowser(t, false)
}

func runWeatherBrowser(t *testing.T, live bool) {
	scriptName := "browser.cjs"
	if live {
		scriptName = "live-browser.cjs"
	}
	script, err := filepath.Abs("../../../tests/weather/" + scriptName)
	require.NoError(t, err)
	weatherApp(t)
	define.AppFlags.DebugMode = false
	if theme := os.Getenv("FLARE_WEATHER_THEME"); theme != "" {
		require.True(t, data.UpdateThemeName(theme))
	}
	define.Init()
	view, err := templates.NewRenderer()
	require.NoError(t, err)
	app := fiber.New(fiber.Config{Views: view})
	RegisterRouting(app)
	original := searchProvider
	originalIP := locateProvider
	t.Cleanup(func() { searchProvider = original; locateProvider = originalIP })
	if !live {
		locateProvider = func(context.Context) (string, error) { return "北京, 北京", nil }
		searchProvider = func(ctx context.Context, keyword string) ([]provider.Location, error) {
			switch keyword {
			case "朝阳":
				return []provider.Location{{ID: "1", Province: "北京", City: "北京", County: "朝阳"}, {ID: "2", Province: "辽宁", City: "朝阳"}}, nil
			case "津市":
				return []provider.Location{{ID: "3", Province: "湖南", City: "常德", County: "津市"}}, nil
			case "北京":
				return []provider.Location{{ID: "4", Province: "北京", City: "北京"}}, nil
			case "失败":
				return nil, errors.New("fixture unavailable")
			case "慢查询":
				time.Sleep(600 * time.Millisecond)
				return []provider.Location{{ID: "5", Province: "旧", City: "旧"}}, nil
			default:
				return nil, nil
			}
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() { stopped <- app.Listener(listener) }()
	defer func() { require.NoError(t, app.Shutdown()); require.NoError(t, <-stopped) }()
	command := exec.Command("node", script)
	command.Env = append(os.Environ(), "FLARE_WEATHER_URL=http://"+listener.Addr().String())
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	t.Log(string(output))
}
