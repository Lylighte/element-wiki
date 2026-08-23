// Package database 提供方言驱动的连接工厂；不依赖任何仓库层类型。
package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Open 按方言建立连接。postgres 在对应里程碑接入 pgx 后启用。
func Open(dialect, url string) (*sql.DB, error) {
	switch dialect {
	case "sqlite":
		db, err := sql.Open("sqlite", withSQLitePragmas(url))
		if err != nil {
			return nil, fmt.Errorf("database: 打开 sqlite 失败: %w", err)
		}
		return db, nil
	case "postgres":
		return nil, fmt.Errorf("database: postgres 适配器尚未实现")
	default:
		return nil, fmt.Errorf("database: 未知方言 %q", dialect)
	}
}

// withSQLitePragmas 为 sqlite DSN 追加外键与忙等设置（幂等）。
func withSQLitePragmas(url string) string {
	pragmas := "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if strings.Contains(url, "?") {
		return url + "&" + pragmas
	}
	return url + "?" + pragmas
}
