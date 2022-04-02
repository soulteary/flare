package cmd

import (
	"context"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/soulteary/flare/internal/logger"

	FlareModel "github.com/soulteary/flare/model"
	FlareState "github.com/soulteary/flare/state"

	FlareAssets "github.com/soulteary/flare/pkg/assets"
	FlareAuth "github.com/soulteary/flare/pkg/auth"
	FlareMDI "github.com/soulteary/flare/pkg/mdi"
	FlareTemplates "github.com/soulteary/flare/pkg/templates"

	FlareAppearance "github.com/soulteary/flare/pkg/appearance"
)

func startDaemon(AppFlags *FlareModel.Flags) {

	if !AppFlags.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	router := gin.Default()
	log := logger.GetLogger()

	router.Use(logger.Logger(log), gin.Recovery())

	if !AppFlags.DebugMode {
		router.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	FlareState.Init()
	FlareAssets.RegisterRouting(router)

	FlareMDI.Init()
	FlareMDI.RegisterRouting(router)

	FlareTemplates.RegisterRouting(router)
	FlareAppearance.RegisterRouting(router)

	if !AppFlags.DisableLoginMode {
		FlareAuth.RequestHandle(router)
	}

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(AppFlags.Port),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("程序启动出错: %s\n", err)
		}
	}()
	log.Println("程序已启动完毕 🚀")

	<-ctx.Done()

	stop()
	log.Println("程序正在关闭中，如需立即结束请按 CTRL+C")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("程序强制关闭: ", err)
	}

	log.Println("期待与你的再次相遇 ❤️")
}
