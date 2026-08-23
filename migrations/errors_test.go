package migrations

import (
	"context"
	"strings"
	"testing"
)

// 直接驱动 applyOne 的失败分支（表驱动，命名区分差异点）。
func TestApplyOneErrorBranches(t *testing.T) {
	db := newTestDB(t)
	m := &Migrator{DB: db, Dialect: "sqlite"}
	ctx := context.Background()
	if err := m.Apply(ctx); err != nil {
		t.Fatal(err)
	}

	t.Run("非法 SQL 返回带文件名的错误", func(t *testing.T) {
		err := m.applyOne(ctx, entry{version: 50, name: "0050_bad.sql", sql: "CREATE TABLE !!!"})
		if err == nil || !strings.Contains(err.Error(), "0050_bad.sql") {
			t.Fatalf("错误应包含迁移名, got %v", err)
		}
	})

	t.Run("版本记录冲突返回明确错误", func(t *testing.T) {
		err := m.applyOne(ctx, entry{version: 1, name: "0001_dup.sql", sql: "SELECT 1"})
		if err == nil || !strings.Contains(err.Error(), "记录版本 1 失败") {
			t.Fatalf("主键冲突应报记录失败, got %v", err)
		}
	})
}

func TestLatestPostgresMirrorsSQLite(t *testing.T) {
	sq, err := Latest("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	pg, err := Latest("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if sq == 0 || sq != pg {
		t.Fatalf("两方言迁移集必须同步: sqlite=%d postgres=%d", sq, pg)
	}
}
