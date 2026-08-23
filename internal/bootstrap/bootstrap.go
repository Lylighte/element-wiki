// Package bootstrap 组装可运行的 HTTP 服务，独立于 main 以便测试。
package bootstrap

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"element-wiki/internal/config"
	"element-wiki/internal/httpapi"
)

// Run 监听配置地址并阻塞服务，直到 ctx 取消（优雅关闭）或监听失败。
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger,
	router http.Handler) error {
	ln, err := net.Listen("tcp", cfg.Server.HTTPAddr)
	if err != nil {
		return err
	}
	return Serve(ctx, ln, logger, router)
}

// Serve 在已就绪的 listener 上提供服务；ctx 取消时优雅关闭。
func Serve(ctx context.Context, ln net.Listener, logger *slog.Logger, router http.Handler) error {
	if router == nil {
		router = httpapi.NewRouter(httpapi.Deps{})
	}
	srv := &http.Server{Handler: router}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	logger.Info("element-wiki 已启动", "addr", ln.Addr().String())

	select {
	case <-ctx.Done():
		logger.Info("收到退出信号，开始优雅关闭")
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}
