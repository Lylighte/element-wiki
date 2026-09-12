// T6.4 存储层验收：仪表盘聚合与用户列表。
package sqlite

import (
	"context"
	"testing"
)

func TestDashboardStatsAndUserList(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
		VALUES ('u2','i','u2','','Two','viewer','active',2)`); err != nil {
		t.Fatal(err)
	}
	mk := func(id, slug string) {
		res, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
			VALUES ('` + id + `',NULL,'` + slug + `','t','u1','u1',5,6)`)
		if err != nil {
			t.Fatalf("mk %s: %v", id, err)
		}
		_ = res
	}
	mk("dA", "alpha")
	mk("dB", "beta")

	stats, err := s.DashboardStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DocumentsTotal != 2 || len(stats.RecentDocs) != 2 {
		t.Fatalf("聚合异常: %+v", stats)
	}
	list, err := s.ListUsers(ctx, "tw", 10)
	if err != nil || len(list) != 1 || list[0].ID != "u2" {
		t.Errorf("q 过滤 = %+v %v", list, err)
	}
	all, _ := s.ListUsers(ctx, "", 10)
	if len(all) < 2 {
		t.Errorf("全量列表 = %d", len(all))
	}
}
