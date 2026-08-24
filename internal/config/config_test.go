// Package config_test 验证三层加载顺序与校验规则（T0.1 验收）。
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsUsableWithoutFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // 目录内无 config.yaml
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("默认值应可直接启动: %v", err)
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("HTTPAddr 默认值 = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("Driver 默认值 = %q", cfg.Database.Driver)
	}
	if cfg.Wiki.CommentsEnabled {
		t.Error("comments_enabled 默认必须为 false")
	}
	if cfg.Wiki.MaxVersions != 100 || cfg.Wiki.TrashRetentionDays != 30 {
		t.Errorf("MaxVersions/TrashRetention 默认 = %d/%d", cfg.Wiki.MaxVersions, cfg.Wiki.TrashRetentionDays)
	}
}

func TestYAMLOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	yamlFile := filepath.Join(dir, "config.yaml")
	content := `
server:
  http_addr: 0.0.0.0:9000
wiki:
  title: My Wiki
  max_versions: 50
`
	if err := os.WriteFile(yamlFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != "0.0.0.0:9000" {
		t.Errorf("yaml 覆盖 HTTPAddr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Wiki.Title != "My Wiki" || cfg.Wiki.MaxVersions != 50 {
		t.Errorf("yaml 覆盖 Title/MaxVersions = %q/%d", cfg.Wiki.Title, cfg.Wiki.MaxVersions)
	}
	if !cfg.Server.SecureCookies || cfg.Wiki.CommentsEnabled {
		t.Error("未覆盖字段必须保留默认值")
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	yamlFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte("wiki:\n  title: FromYAML\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WIKI_WIKI_TITLE", "FromENV")
	t.Setenv("WIKI_WIKI_MAX_VERSIONS", "7")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Wiki.Title != "FromENV" {
		t.Errorf("环境变量优先级应高于 yaml, Title = %q", cfg.Wiki.Title)
	}
	if cfg.Wiki.MaxVersions != 7 {
		t.Errorf("env MaxVersions = %d", cfg.Wiki.MaxVersions)
	}
}

func TestExplicitConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	custom := filepath.Join(dir, "other.yaml")
	if err := os.WriteFile(custom, []byte("server:\n  http_addr: :7777\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(custom)
	if err != nil {
		t.Fatalf("Load(custom): %v", err)
	}
	if cfg.Server.HTTPAddr != ":7777" {
		t.Errorf("显式文件未生效: %q", cfg.Server.HTTPAddr)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"max_versions 越界", "wiki:\n  max_versions: 0\n", "max_versions"},
		{"driver 非法", "database:\n  driver: oracle\n", "database.driver"},
		{"postgres 缺 URL", "database:\n  driver: postgres\n  url: \"\"\n", "database.url"},
		{"oidc 缺 issuer", "oidc:\n  enabled: true\n  client_id: abc\n", "oidc.issuer"},
		{"oidc 回调非绝对地址", "oidc:\n  enabled: true\n  issuer: https://i\n  client_id: c\n  redirect_uri: /cb\n", "oidc.redirect_uri"},
		{"base_path 形态非法", "wiki:\n  base_path: wiki/\n", "base_path"},
		{"timezone 非法", "wiki:\n  timezone: Mars/Olympus\n", "timezone"},
		{"trash_retention 越界", "wiki:\n  trash_retention_days: -3\n", "trash_retention_days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load("")
			if err == nil {
				t.Fatalf("期望报错含 %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("错误 %q 未定位到字段 %q", err, tc.wantErr)
			}
		})
	}
}

func TestEnvInvalidValue(t *testing.T) {
	t.Setenv("WIKI_SERVER_SECURE_COOKIES", "not-a-bool")
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "WIKI_SERVER_SECURE_COOKIES") {
		t.Fatalf("错误应指明环境变量名, got %v", err)
	}
}

func TestUnreadableFileFails(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(bad, []byte("{{{ 不是 yaml"), 0o644)
	if _, err := Load(bad); err == nil {
		t.Fatal("损坏的 yaml 必须报错")
	}
}

func TestAllEnvBindingsAccepted(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	valid := map[string]string{
		"WIKI_SERVER_HTTP_ADDR":          ":9999",
		"WIKI_SERVER_SECURE_COOKIES":     "false",
		"WIKI_DATABASE_DRIVER":           "sqlite",
		"WIKI_DATABASE_URL":              "data/x.db",
		"WIKI_STORAGE_DIR":               "s",
		"WIKI_SEARCH_INDEX_DIR":          "s/idx.bleve",
		"WIKI_OIDC_ENABLED":              "false",
		"WIKI_OIDC_ISSUER":               "https://idp.example.com",
		"WIKI_OIDC_CLIENT_ID":            "cid",
		"WIKI_OIDC_CLIENT_SECRET":        "sec",
		"WIKI_OIDC_ADMIN_EMAILS":         "a@x.com, b@x.com ,",
		"WIKI_WIKI_TITLE":                "T",
		"WIKI_WIKI_BASE_PATH":            "/wiki",
		"WIKI_WIKI_ANONYMOUS_READ":       "true",
		"WIKI_WIKI_COMMENTS_ENABLED":     "true",
		"WIKI_WIKI_MAX_VERSIONS":         "12",
		"WIKI_WIKI_UPLOAD_MAX_MB":        "5",
		"WIKI_WIKI_TIMEZONE":             "Asia/Shanghai",
		"WIKI_WIKI_TRASH_RETENTION_DAYS": "7",
	}
	for k, v := range valid {
		t.Setenv(k, v)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("全量合法环境变量应通过: %v", err)
	}
	if cfg.Server.HTTPAddr != ":9999" || cfg.Wiki.Title != "T" || cfg.Wiki.BasePath != "/wiki" {
		t.Errorf("绑定未生效: %+v", cfg)
	}
	if len(cfg.OIDC.AdminEmails) != 2 {
		t.Errorf("AdminEmails 列表解析 = %v", cfg.OIDC.AdminEmails)
	}
	if cfg.Wiki.AnonymousRead != true || cfg.Wiki.CommentsEnabled != true || !cfg.OIDC.Enabled == false && cfg.OIDC.Enabled {
		t.Errorf("布尔绑定异常: %+v", cfg.Wiki)
	}
}

func TestValidateOIDCEnabledHappyPath(t *testing.T) {
	cfg := Defaults()
	cfg.OIDC.Enabled = true
	cfg.OIDC.Issuer = "https://idp.example.com"
	cfg.OIDC.ClientID = "cid"
	cfg.OIDC.RedirectURI = "https://app.example.com/v1/auth/oidc/callback"
	if err := Validate(cfg); err != nil {
		t.Fatalf("oidc 完整配置应通过: %v", err)
	}
}
