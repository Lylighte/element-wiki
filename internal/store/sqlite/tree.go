package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

// SubtreeIDs 递归收集存活子树（含自身）。
func (s *DB) SubtreeIDs(ctx context.Context, rootID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
WITH RECURSIVE sub(id) AS (
    SELECT id FROM documents WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT d.id FROM documents d JOIN sub ON d.parent_id = sub.id
     WHERE d.deleted_at IS NULL
)
SELECT id FROM sub`, rootID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	if len(out) == 0 {
		return nil, store.ErrNotFound // 根不存在或已入回收站
	}
	return out, nil
}

// EffectiveVisibility 沿祖先链解析：任一祖先 restricted 即 restricted。
func (s *DB) EffectiveVisibility(ctx context.Context, docID string) (model.Visibility, error) {
	// 先确认目标存在（区分 404 与普通 standard）
	if _, err := s.Get(ctx, docID); err != nil {
		return "", err
	}
	var vis sql.NullString
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE anc(id, parent_id, visibility) AS (
    SELECT id, parent_id, visibility FROM documents WHERE id = ?
    UNION ALL
    SELECT d.id, d.parent_id, d.visibility
      FROM documents d JOIN anc a ON d.id = a.parent_id
)
SELECT CASE WHEN MAX(CASE WHEN visibility='restricted' THEN 1 ELSE 0 END) = 1
            THEN 'restricted' ELSE 'standard' END
FROM anc`, docID).Scan(&vis)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.VisibilityStandard, nil // 空集不可能（自查询含自身），防御性默认
		}
		return "", fmt.Errorf("sqlite: 解析生效可见性失败: %w", err)
	}
	return model.Visibility(vis.String), nil
}
