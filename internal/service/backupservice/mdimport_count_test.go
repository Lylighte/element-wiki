package backupservice

import (
	"context"
	"database/sql"
	"testing"

	docservice "element-wiki/internal/service/docservice"
)

// countDocs 统计存活文档数（测试辅助，走 service 只读视图）。
func countDocs(t *testing.T, docs *docservice.Service) int {
	t.Helper()
	return docs.CountAliveForTest(context.Background())
}

func countDocsViaDB(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
