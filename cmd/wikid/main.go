// Command wikid 是 Element Wiki 服务入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"element-wiki/internal/bootstrap"
	"element-wiki/internal/config"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("wikid", flag.ContinueOnError)
	configFile := fs.String("configfile", "", "配置文件路径（默认 CONFIG_FILE 环境变量，其次 config.yaml）")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Error("配置加载失败", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx, cfg, logger); err != nil {
		logger.Error("服务退出", "err", err)
		return 1
	}
	logger.Info("element-wiki 已停止")
	fmt.Fprintln(os.Stderr) // 保持输出末尾整洁
	return 0
}
