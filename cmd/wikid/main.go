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
	"element-wiki/internal/database"
	"element-wiki/internal/httpapi"
	"element-wiki/internal/service/docservice"
	"element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func main() {
	os.Exit(run(os.Args[1:], context.Background()))
}

func run(args []string, parent context.Context) int {
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

	// 启动即推进数据库 schema，并拒绝旧二进制跑新库。
	db, err := database.Open(cfg.Database.Driver, cfg.Database.URL)
	if err != nil {
		logger.Error("数据库打开失败", "err", err)
		return 1
	}
	defer db.Close()
	m := &migrations.Migrator{DB: db, Dialect: cfg.Database.Driver}
	if err := m.Apply(context.Background()); err != nil {
		logger.Error("数据库迁移失败", "err", err)
		return 1
	}
	if err := m.VerifyUpToDate(context.Background()); err != nil {
		logger.Error("数据库版本校验失败", "err", err)
		return 1
	}

	// 组装路由依赖：认证在 M3 接入前使用默认全拒 Actor。
	impl := sqlite.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, int64(cfg.Wiki.MaxVersions))
	deps := httpapi.Deps{Docs: svc, Trees: impl}

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx, cfg, logger, httpapi.NewRouter(deps)); err != nil {
		logger.Error("服务退出", "err", err)
		return 1
	}
	logger.Info("element-wiki 已停止")
	fmt.Fprintln(os.Stderr) // 保持输出末尾整洁
	return 0
}
