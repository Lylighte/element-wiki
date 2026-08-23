package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"element-wiki/internal/config"
	"element-wiki/internal/httpapi"
)

func TestServeHealthzAndGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Serve(ctx, ln, slog.Default()) }()

	url := "http://" + ln.Addr().String() + "/healthz"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("请求 healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("healthz body = %v", body)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("优雅关闭应返回 nil, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后未在超时内退出")
	}
}

func TestNewRouterHealthzDirect(t *testing.T) {
	srv := httptest.NewServer(httpapi.NewRouter())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("body = %v", body)
	}
}

func TestRunServesOnConfiguredAddr(t *testing.T) {
	// 先占用一个端口再释放，拿到大概率可复用的端口号。
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	cfg := config.Defaults()
	cfg.Server.HTTPAddr = addr

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, slog.Default()) }()

	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		resp, err = http.Get("http://" + addr + "/healthz")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("服务未在超时内就绪: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer resp.Body.Close()
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("healthz body = %v", body)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("优雅关闭应返回 nil, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后未退出")
	}
}

func TestRunListenFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() // 保持占用

	cfg := config.Defaults()
	cfg.Server.HTTPAddr = ln.Addr().String()

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), cfg, slog.Default()) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("端口被占必须报错")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("监听失败未返回")
	}
}
