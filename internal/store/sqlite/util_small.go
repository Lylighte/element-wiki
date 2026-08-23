package sqlite

import (
	"database/sql"
	"time"

	"element-wiki/internal/util"
)

func newIDForJobs() string { return util.NewID() }

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ = time.Now

func errNoRowsWrap() error { return sql.ErrNoRows }
