// Package store 提供数据库连接的方言注册与打开。
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Open 按方言建立数据库连接。
// postgres 适配器在对应里程碑接入 pgx 后启用。
func Open(dialect, url string) (*sql.DB, error) {
	switch dialect {
	case "sqlite":
		dsn := withSQLitePragmas(url)
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("store: 打开 sqlite 失败: %w", err)
		}
		return db, nil
	case "postgres":
		return nil, fmt.Errorf("store: postgres 适配器尚未实现")
	default:
		return nil, fmt.Errorf("store: 未知方言 %q", dialect)
	}
}

// withSQLitePragmas 为 sqlite DSN 追加外键与忙等设置（幂等，已有参数则续接）。
func withSQLitePragmas(url string) string {
	pragmas := "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if strings.Contains(url, "?") {
		return url + "&" + pragmas
	}
	return url + "?" + pragmas
}
