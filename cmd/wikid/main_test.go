package main

import (
	"net/http"
	"time"

	"context"
	"element-wiki/internal/database"
	"element-wiki/migrations"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// run 的失败路径可测；成功路径会阻塞在 Serve 上，由 bootstrap 包覆盖。
func TestRunInvalidConfigExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("wiki:\n  max_versions: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-configfile", bad}, context.Background()); code != 1 {
		t.Fatalf("非法配置应退出码 1, got %d", code)
	}
}

// 完整成功路径：迁移 → 监听 → healthz 可用 → 取消后以 0 退出。
func TestRunHappyStartupServesAndStops(t *testing.T) {
	dir := t.TempDir()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	yamlOK := filepath.Join(dir, "ok.yaml")
	content := "server:\n  http_addr: " + addr + "\ndatabase:\n  url: " +
		filepath.Join(dir, "ok.db") + "\n"
	if err := os.WriteFile(yamlOK, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run([]string{"-configfile", yamlOK}, ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("服务未就绪: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("优雅退出应返回 0, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后未退出")
	}

	// 迁移已落库：直接打开验证 schema_migrations 存在且为最新
	db, err := database.Open("sqlite", filepath.Join(dir, "ok.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.VerifyUpToDate(context.Background()); err != nil {
		t.Fatalf("启动后库应为最新版本: %v", err)
	}
}

// 用已占用的真实端口制造确定性监听失败，验证错误传播到退出码。
func TestRunListenFailureExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	yamlBusy := filepath.Join(dir, "busy.yaml")
	content := "server:\n  http_addr: " + ln.Addr().String() + "\ndatabase:\n  url: " + filepath.Join(t.TempDir(), "t.db") + "\n"
	if err := os.WriteFile(yamlBusy, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-configfile", yamlBusy}, context.Background()); code != 1 {
		t.Fatalf("监听失败应退出码 1, got %d", code)
	}
}

func TestRunBrokenYAMLExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "x.yaml")
	// "{{{" 开头无法作为合法 YAML 解析
	os.WriteFile(bad, []byte("{{{ not yaml"), 0o644)
	if code := run([]string{"-configfile", bad}, context.Background()); code != 1 {
		t.Fatalf("损坏 yaml 应退出码 1, got %d", code)
	}
}
