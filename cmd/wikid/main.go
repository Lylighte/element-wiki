// Command wikid 是 Element Wiki 服务入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"element-wiki/internal/bootstrap"
	"element-wiki/internal/config"
	"element-wiki/internal/database"
	"element-wiki/internal/httpapi"
	"element-wiki/internal/permission"
	"element-wiki/internal/search"
	adminservice "element-wiki/internal/service/adminservice"
	authsvc "element-wiki/internal/service/authservice"
	backupservice "element-wiki/internal/service/backupservice"
	docservice "element-wiki/internal/service/docservice"
	searchservice "element-wiki/internal/service/searchservice"
	"element-wiki/internal/sso"
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

	// 首启自举：确保运行期目录存在（SQLite 文件、索引、附件、备份）
	for _, dir := range []string{
		filepath.Dir(cfg.Database.URL),
		cfg.Storage.SearchIndexDir,
		cfg.Storage.AttachmentsDir,
		filepath.Join(cfg.Storage.Dir, "backups"),
	} {
		if dir == "" || dir == "." {
			continue
		}
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			logger.Error("目录创建失败", "dir", dir, "err", mkErr)
			return 1
		}
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

	// 组装路由依赖。
	impl := sqlite.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, int64(cfg.Wiki.MaxVersions))
	auth := authsvc.New(impl, impl, impl, cfg.OIDC.Issuer, cfg.OIDC.AdminEmails, cfg.Wiki.AnonymousRead)

	var oidcDeps *httpapi.OIDCDeps
	if cfg.OIDC.Enabled {
		redirect := cfg.OIDC.RedirectURI // 已由 Validate 保证为绝对地址
		oidcDeps = &httpapi.OIDCDeps{
			Enabled: true, ProviderName: cfg.OIDC.ProviderName,
			RedirectURI: redirect, Scopes: cfg.OIDC.Scopes,
			Client: sso.NewClient(cfg.OIDC.Issuer, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret),
		}
	}

	searchIdx, serr := search.Open(cfg.Storage.SearchIndexDir)
	if serr != nil {
		logger.Error("搜索索引打开失败", "err", serr)
		return 1
	}
	defer searchIdx.Close()
	ssvc := searchservice.New(searchIdx, impl, impl)

	svc.SetSearchHooks(searchIdx, impl)
	svc.SetCommentStore(impl, impl)
	svc.SetAttachmentStore(impl, cfg.Storage.AttachmentsDir,
		cfg.Wiki.AllowedExtensions, cfg.Wiki.UploadMaxMB)

	jobs := search.RebuildDeps{Jobs: impl, Docs: impl, Coms: impl, Index: searchIdx, Log: logger}
	schemaVerLatest, _ := migrations.Latest(cfg.Database.Driver)
	backups := backupservice.New(impl, impl, db, cfg.Database.URL,
		cfg.Storage.AttachmentsDir, filepath.Join(cfg.Storage.Dir, "backups"), schemaVerLatest)
	mdImports := backupservice.NewMarkdownImporter(impl, svc, func(id string) permission.Actor {
		return permission.NewActor(id, permission.CodesFor(permission.Admin))
	})
	admin := adminservice.New(impl, impl, impl)

	deps := httpapi.Deps{
		Docs: svc, Trees: impl, Auth: auth, Admin: admin,
		OIDC: oidcDeps, SecureCookies: cfg.Server.SecureCookies,
		Search: ssvc, Jobs: impl, Imports: impl,
		Backups: backups, MarkdownImports: mdImports,
		CommentsEnabled: cfg.Wiki.CommentsEnabled,
		AttachmentsOn:   true, AttachDir: cfg.Storage.AttachmentsDir,
		UploadMaxBytes: int64(cfg.Wiki.UploadMaxMB) * 1024 * 1024,
	}

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go search.RunRebuildWorker(ctx, jobs, 5*time.Second)

	if err := bootstrap.Run(ctx, cfg, logger, httpapi.NewRouter(deps)); err != nil {
		logger.Error("服务退出", "err", err)
		return 1
	}
	logger.Info("element-wiki 已停止")
	fmt.Fprintln(os.Stderr) // 保持输出末尾整洁
	return 0
}
