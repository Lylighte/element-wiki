// T5.x 存储层验收：软删子树/恢复/物理清除/到期枚举/blob GC。
package sqlite

import (
	"context"
	"testing"
)

func seedTreeForTrash(t *testing.T, s *DB) (rootID, childID string) {
	t.Helper()
	ctx := context.Background()
	u := userRow("u1")
	if err := s.CreateUser(ctx, &u); err != nil {
		t.Fatal(err)
	}
	rootID, childID = "tr-root", "tr-child"
	for _, d := range []struct{ id, parent string }{
		{rootID, ""}, {childID, rootID},
	} {
		p := "NULL"
		if d.parent != "" {
			p = "'" + d.parent + "'"
		}
		if _, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
			VALUES ('` + d.id + `',` + p + `,'s-` + d.id + `','t','u1','u1',1,1)`); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func TestSoftDeleteRestoreAndPurge(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	rootID, _ := seedTreeForTrash(t, s)

	if err := s.SoftDeleteSubtree(ctx, rootID, "u1", 100, 200); err != nil {
		t.Fatal(err)
	}
	gone, err := s.HasDeletedAncestor(ctx, rootID)
	if err != nil || gone {
		t.Errorf("根自身不算祖先: %v %v", gone, err)
	}
	trashList, _ := s.ListTrash(ctx, 100)
	if len(trashList) != 2 {
		t.Fatalf("回收站应含两行: %d", len(trashList))
	}

	if err := s.RestoreSubtree(ctx, rootID, "u1", 300); err != nil {
		t.Fatal(err)
	}
	gone, _ = s.HasDeletedAncestor(ctx, rootID)
	if gone {
		t.Error("恢复后不应再有删除标记")
	}
	trashList, _ = s.ListTrash(ctx, 100)
	if len(trashList) != 0 {
		t.Errorf("恢复后回收站应空: %d", len(trashList))
	}
}

func TestPurgeRequiresTrashedAndCascades(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	rootID, _ := seedTreeForTrash(t, s)
	if err := s.PurgeSubtree(ctx, rootID); err == nil {
		t.Fatal("存活文档不允许直接 purge")
	}
	s.SoftDeleteSubtree(ctx, rootID, "u1", 1, 2)
	if err := s.PurgeSubtree(ctx, rootID); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id IN ('tr-root','tr-child')`).Scan(&n)
	if n != 0 {
		t.Errorf("purge 后仍有行: %d", n)
	}
}

func TestDuePurgeIDsOnlyRootsPastDeadline(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	rootID, _ := seedTreeForTrash(t, s)
	s.SoftDeleteSubtree(ctx, rootID, "u1", 1000, 5000) // 未来到期

	due, _ := s.DuePurgeIDs(ctx, 3000)
	if len(due) != 0 {
		t.Errorf("未到期不应出现: %v", due)
	}
	due, _ = s.DuePurgeIDs(ctx, 6000)
	if len(due) != 1 || due[0] != rootID {
		t.Errorf("到期限应只含根: %v", due)
	}
}

func TestGCDereferencedBlobs(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	if _, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
		VALUES ('d1',NULL,'doc','t','u1','u1',1,1)`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO document_blobs VALUES ('kept','x',1,1)`,
		`INSERT INTO document_blobs VALUES ('orphan','y',1,1)`,
		`INSERT INTO document_commits (id,document_id,commit_no,blob_hash,author_id,created_at)
		 VALUES ('c1','d1',1,'kept','u1',1)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	n, err := s.GCDereferencedBlobs(ctx)
	if err != nil || n < 1 {
		t.Fatalf("gc 回收数 = %d,%v", n, err)
	}
	var v string
	if err := s.db.QueryRow(`SELECT hash FROM document_blobs WHERE hash='kept'`).Scan(&v); err != nil || v != "kept" {
		t.Errorf("被引用 blob 不应被删: %v %q", err, v)
	}
	if err := s.db.QueryRow(`SELECT hash FROM document_blobs WHERE hash='orphan'`).Scan(&v); err == nil {
		t.Error("孤儿 blob 应被删除")
	}
}

func TestSubtreeIDsOfTrashed(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	rootID, childID := seedTreeForTrash(t, s)
	s.SoftDeleteSubtree(ctx, rootID, "u1", 1, 2)

	ids, err := s.SubtreeIDsOfTrashed(ctx, rootID)
	if err != nil {
		t.Fatal(err)
	}
	foundRoot, foundChild := false, false
	for _, id := range ids {
		if id == rootID {
			foundRoot = true
		}
		if id == childID {
			foundChild = true
		}
	}
	if !foundRoot || !foundChild {
		t.Errorf("回收站子树应含根与子: %v", ids)
	}
	if _, err := s.SubtreeIDsOfTrashed(ctx, "ghost"); err == nil {
		t.Error("幽灵根应报错")
	}
}
