package sqlite

import (
	"context"
	"testing"
)

func TestMaxCommitNoEmptyAndAfterAppends(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "maxno")

	if v, err := s.coms.(interface {
		MaxCommitNo(context.Context, string) (int64, error)
	}).MaxCommitNo(ctx, d.ID); err != nil || v != 0 {
		t.Fatalf("空文档 MaxCommitNo = %d,%v", v, err)
	}
	appendN(t, s, ctx, d.ID, 3, 0)
	v, err := s.coms.(interface {
		MaxCommitNo(context.Context, string) (int64, error)
	}).MaxCommitNo(ctx, d.ID)
	if err != nil || v != 3 {
		t.Fatalf("MaxCommitNo = %d,%v", v, err)
	}
}
