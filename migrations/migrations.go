// Package migrations 提供嵌入式 SQL 迁移框架。
// 迁移文件按方言存放于 sqlite/ 与 postgres/ 目录，文件名格式 0001_name.sql，
// 按版本号升序应用；每个迁移在独立事务内执行并记录到 schema_migrations。
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

//go:embed sqlite/*.sql postgres/*.sql
var embedded embed.FS

// FS 返回指定方言的迁移文件集。
func FS(dialect string) (fs.FS, error) {
	switch dialect {
	case "sqlite", "postgres":
		return fs.Sub(embedded, dialect)
	default:
		return nil, fmt.Errorf("migrations: 未知方言 %q", dialect)
	}
}

type entry struct {
	version int
	name    string
	sql     string
}

func loadEntries(dialect string) ([]entry, error) {
	dir, err := FS(dialect)
	if err != nil {
		return nil, err
	}
	matches, err := fs.Glob(dir, "*.sql")
	if err != nil {
		return nil, err
	}
	slices.Sort(matches)

	out := make([]entry, 0, len(matches))
	for _, m := range matches {
		v, err := versionOf(m)
		if err != nil {
			return nil, err
		}
		data, err := fs.ReadFile(dir, m)
		if err != nil {
			return nil, fmt.Errorf("migrations: 读取 %s: %w", m, err)
		}
		out = append(out, entry{version: v, name: m, sql: string(data)})
	}
	return out, nil
}

func versionOf(filename string) (int, error) {
	prefix, found := strings.CutSuffix(filename, ".sql")
	if len(prefix) < 4 || !found {
		return 0, fmt.Errorf("migrations: 文件名非法 %q", filename)
	}
	v, err := strconv.Atoi(prefix[:4])
	if err != nil {
		return 0, fmt.Errorf("migrations: 版本前缀非法 %q", filename)
	}
	return v, nil
}

// Migrator 在指定数据库上执行迁移。DB 的驱动由调用方注册。
type Migrator struct {
	DB      *sql.DB
	Dialect string
}

const createVersionTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at BIGINT NOT NULL
)`

// Apply 将数据库推进到嵌入迁移的最新版本，重复调用幂等。
func (m *Migrator) Apply(ctx context.Context) error {
	if _, err := m.DB.ExecContext(ctx, createVersionTable); err != nil {
		return fmt.Errorf("migrations: 建 schema_migrations 失败: %w", err)
	}

	applied, err := m.appliedSet(ctx)
	if err != nil {
		return err
	}
	entries, err := loadEntries(m.Dialect)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if applied[e.version] {
			continue
		}
		if err := m.applyOne(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) applyOne(ctx context.Context, e entry) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrations: 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, e.sql); err != nil {
		return fmt.Errorf("migrations: 应用 %s 失败: %w", e.name, err)
	}
	now := timeNow()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		e.version, e.name, now); err != nil {
		return fmt.Errorf("migrations: 记录版本 %d 失败: %w", e.version, err)
	}
	return tx.Commit()
}

func (m *Migrator) appliedSet(ctx context.Context) (map[int]bool, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrations: 查询已应用版本失败: %w", err)
	}
	defer rows.Close()
	set := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrations: 扫描版本失败: %w", err)
		}
		set[v] = true
	}
	return set, rows.Err()
}

// Current 返回数据库当前版本；空库返回 0。
func (m *Migrator) Current(ctx context.Context) (int, error) {
	var v sql.NullInt64
	err := m.DB.QueryRowContext(ctx,
		`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("migrations: 读取当前版本失败: %w", err)
	}
	return int(v.Int64), nil
}

// Latest 返回嵌入迁移的最高版本。
func Latest(dialect string) (int, error) {
	entries, err := loadEntries(dialect)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}
	return entries[len(entries)-1].version, nil
}

// VerifyUpToDate 断言数据库版本与二进制内嵌迁移一致：
// 库版本高于内嵌（旧二进制跑新库）视为致命错误。
func (m *Migrator) VerifyUpToDate(ctx context.Context) error {
	latest, err := Latest(m.Dialect)
	if err != nil {
		return err
	}
	current, err := m.Current(ctx)
	if err != nil {
		return err
	}
	if current > latest {
		return fmt.Errorf("migrations: 数据库版本 %d 高于二进制内嵌版本 %d，禁止用旧程序启动新库", current, latest)
	}
	return nil
}
