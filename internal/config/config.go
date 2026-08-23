package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是全站配置的唯一结构，加载顺序：内置默认值 -> config.yaml -> 环境变量。
type Config struct {
	Server   Server   `yaml:"server"`
	Database Database `yaml:"database"`
	Storage  Storage  `yaml:"storage"`
	OIDC     OIDC     `yaml:"oidc"`
	Wiki     Wiki     `yaml:"wiki"`
}

type Server struct {
	HTTPAddr      string `yaml:"http_addr"`
	SecureCookies bool   `yaml:"secure_cookies"`
}

type Database struct {
	Driver string `yaml:"driver"` // sqlite | postgres
	URL    string `yaml:"url"`    // postgres DSN；sqlite 为文件路径
}

type Storage struct {
	Dir            string `yaml:"dir"`
	SearchIndexDir string `yaml:"search_index_dir"`
	AttachmentsDir string `yaml:"attachments_dir"`
}

type OIDC struct {
	Enabled      bool     `yaml:"enabled"`
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURI  string   `yaml:"redirect_uri"`
	ProviderName string   `yaml:"provider_name"`
	Scopes       []string `yaml:"scopes"`
	AdminEmails  []string `yaml:"admin_emails"`
}

type Wiki struct {
	Title              string `yaml:"title"`
	BasePath           string `yaml:"base_path"`
	AnonymousRead      bool   `yaml:"anonymous_read"`
	CommentsEnabled    bool   `yaml:"comments_enabled"`
	MaxVersions        int    `yaml:"max_versions"`
	UploadMaxMB        int    `yaml:"upload_max_mb"`
	AllowedExtensions  string `yaml:"allowed_extensions"`
	Timezone           string `yaml:"timezone"`
	DefaultLang        string `yaml:"default_lang"`
	TrashRetentionDays int    `yaml:"trash_retention_days"`
}

// Defaults 返回内置默认值。
func Defaults() *Config {
	return &Config{
		Server: Server{
			HTTPAddr:      "127.0.0.1:8080",
			SecureCookies: true,
		},
		Database: Database{
			Driver: "sqlite",
			URL:    "data/element-wiki.db",
		},
		Storage: Storage{
			Dir:            "storage",
			SearchIndexDir: "storage/search/documents.bleve",
			AttachmentsDir: "storage/attachments",
		},
		OIDC: OIDC{
			Scopes: []string{"openid", "profile", "email"},
		},
		Wiki: Wiki{
			Title:              "Element Wiki",
			MaxVersions:        100,
			UploadMaxMB:        20,
			AllowedExtensions:  "png,jpg,jpeg,gif,webp,svg,txt,log,csv,md,zip,pdf,docx,xlsx,pptx,mp4",
			Timezone:           "UTC",
			DefaultLang:        "zh-CN",
			TrashRetentionDays: 30,
		},
	}
}

// Load 按三层顺序加载配置：默认值 -> YAML 文件 -> 环境变量，最后校验。
// configFile 为空时依次尝试 CONFIG_FILE 环境变量与默认路径 config.yaml；
// 文件不存在不视为错误（允许纯默认值启动）。
func Load(configFile string) (*Config, error) {
	cfg := Defaults()

	if configFile == "" {
		configFile = os.Getenv("CONFIG_FILE")
	}
	if configFile == "" {
		configFile = "config.yaml"
	}
	if data, err := os.ReadFile(configFile); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: 解析 %s 失败: %w", configFile, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: 读取 %s 失败: %w", configFile, err)
	}

	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// envVar 将配置字段名映射为环境变量名。
type envBinding struct {
	key string
	set func(cfg *Config, raw string) error
}

var envBindings = []envBinding{
	{"WIKI_SERVER_HTTP_ADDR", func(c *Config, v string) error { c.Server.HTTPAddr = v; return nil }},
	{"WIKI_SERVER_SECURE_COOKIES", func(c *Config, v string) error { b, err := parseBool(v); c.Server.SecureCookies = b; return err }},
	{"WIKI_DATABASE_DRIVER", func(c *Config, v string) error { c.Database.Driver = v; return nil }},
	{"WIKI_DATABASE_URL", func(c *Config, v string) error { c.Database.URL = v; return nil }},
	{"WIKI_STORAGE_DIR", func(c *Config, v string) error { c.Storage.Dir = v; return nil }},
	{"WIKI_SEARCH_INDEX_DIR", func(c *Config, v string) error { c.Storage.SearchIndexDir = v; return nil }},
	{"WIKI_OIDC_ENABLED", func(c *Config, v string) error { b, err := parseBool(v); c.OIDC.Enabled = b; return err }},
	{"WIKI_OIDC_ISSUER", func(c *Config, v string) error { c.OIDC.Issuer = v; return nil }},
	{"WIKI_OIDC_CLIENT_ID", func(c *Config, v string) error { c.OIDC.ClientID = v; return nil }},
	{"WIKI_OIDC_CLIENT_SECRET", func(c *Config, v string) error { c.OIDC.ClientSecret = v; return nil }},
	{"WIKI_OIDC_ADMIN_EMAILS", func(c *Config, v string) error { c.OIDC.AdminEmails = splitList(v); return nil }},
	{"WIKI_WIKI_TITLE", func(c *Config, v string) error { c.Wiki.Title = v; return nil }},
	{"WIKI_WIKI_BASE_PATH", func(c *Config, v string) error { c.Wiki.BasePath = v; return nil }},
	{"WIKI_WIKI_ANONYMOUS_READ", func(c *Config, v string) error { b, err := parseBool(v); c.Wiki.AnonymousRead = b; return err }},
	{"WIKI_WIKI_COMMENTS_ENABLED", func(c *Config, v string) error { b, err := parseBool(v); c.Wiki.CommentsEnabled = b; return err }},
	{"WIKI_WIKI_MAX_VERSIONS", func(c *Config, v string) error { n, err := strconv.Atoi(v); c.Wiki.MaxVersions = n; return err }},
	{"WIKI_WIKI_UPLOAD_MAX_MB", func(c *Config, v string) error { n, err := strconv.Atoi(v); c.Wiki.UploadMaxMB = n; return err }},
	{"WIKI_WIKI_TIMEZONE", func(c *Config, v string) error { c.Wiki.Timezone = v; return nil }},
	{"WIKI_WIKI_TRASH_RETENTION_DAYS", func(c *Config, v string) error { n, err := strconv.Atoi(v); c.Wiki.TrashRetentionDays = n; return err }},
}

func applyEnv(cfg *Config) error {
	for _, b := range envBindings {
		raw, ok := os.LookupEnv(b.key)
		if !ok || raw == "" {
			continue
		}
		if err := b.set(cfg, raw); err != nil {
			return fmt.Errorf("config: 环境变量 %s 非法: %w", b.key, err)
		}
	}
	return nil
}

// Validate 校验派生一致性与取值范围，错误信息必须能定位字段。
func Validate(cfg *Config) error {
	switch cfg.Database.Driver {
	case "sqlite":
	case "postgres":
		if cfg.Database.URL == "" {
			return errors.New("config: database.url 不能为空 (driver=postgres)")
		}
	default:
		return fmt.Errorf("config: database.driver 非法: %q (仅支持 sqlite|postgres)", cfg.Database.Driver)
	}
	if cfg.Wiki.MaxVersions < 1 {
		return fmt.Errorf("config: wiki.max_versions 必须 >= 1, 当前 %d", cfg.Wiki.MaxVersions)
	}
	if cfg.Wiki.UploadMaxMB < 1 {
		return fmt.Errorf("config: wiki.upload_max_mb 必须 >= 1, 当前 %d", cfg.Wiki.UploadMaxMB)
	}
	if cfg.Wiki.TrashRetentionDays < 1 {
		return fmt.Errorf("config: wiki.trash_retention_days 必须 >= 1, 当前 %d", cfg.Wiki.TrashRetentionDays)
	}
	bp := cfg.Wiki.BasePath
	if bp != "" && (!strings.HasPrefix(bp, "/") || strings.HasSuffix(bp, "/")) {
		return fmt.Errorf("config: wiki.base_path 必须以 / 开头且不以 / 结尾, 当前 %q", bp)
	}
	if cfg.OIDC.Enabled {
		if strings.TrimSpace(cfg.OIDC.Issuer) == "" {
			return errors.New("config: oidc.enabled=true 时 oidc.issuer 不能为空")
		}
		if strings.TrimSpace(cfg.OIDC.ClientID) == "" {
			return errors.New("config: oidc.enabled=true 时 oidc.client_id 不能为空")
		}
	}
	if _, err := time.LoadLocation(cfg.Wiki.Timezone); err != nil {
		return fmt.Errorf("config: wiki.timezone 非法: %w", err)
	}
	return nil
}

func parseBool(raw string) (bool, error) {
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("需要布尔值: %w", err)
	}
	return b, nil
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
