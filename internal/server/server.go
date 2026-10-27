package server

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/security"
)

func StartDaemon(AppFlags *model.Flags, stores ...*security.Store) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := logger.GetLogger()
	srv, err := newRouter(AppFlags, stores...)
	if err != nil {
		log.Error("路由初始化失败", "error", err.Error())
		os.Exit(1)
	}

	// #nosec G102 -- Container port publishing and LAN access require all interfaces; deployments control exposure.
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(AppFlags.Port))
	if err != nil {
		log.Error("程序启动出错：", slog.Any("error", err))
		os.Exit(1)
	}

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Error("程序启动出错：", slog.Any("error", err))
			os.Exit(1)
		}
	}()
	log.Info("程序已启动完毕 🚀")

	<-ctx.Done()

	stop()
	log.Info("程序正在关闭中，如需立即结束请按 CTRL+C")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("程序强制关闭：", slog.Any("error", err))
		os.Exit(1)
	}

	log.Info("期待与你的再次相遇 ❤️")
}
