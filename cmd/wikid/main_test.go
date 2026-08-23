package main

import (
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
	if code := run([]string{"-configfile", bad}); code != 1 {
		t.Fatalf("非法配置应退出码 1, got %d", code)
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
	content := "server:\n  http_addr: " + ln.Addr().String() + "\n"
	if err := os.WriteFile(yamlBusy, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-configfile", yamlBusy}); code != 1 {
		t.Fatalf("监听失败应退出码 1, got %d", code)
	}
}

func TestRunBrokenYAMLExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "x.yaml")
	// "{{{" 开头无法作为合法 YAML 解析
	os.WriteFile(bad, []byte("{{{ not yaml"), 0o644)
	if code := run([]string{"-configfile", bad}); code != 1 {
		t.Fatalf("损坏 yaml 应退出码 1, got %d", code)
	}
}
