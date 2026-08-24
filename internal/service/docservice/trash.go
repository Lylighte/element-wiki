// 回收站业务（T5.1）与维护任务编排（T5.2/T5.3）。
package docservice

import (
	"context"
	"errors"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

// ErrParentGone 恢复时父链已删除（映射 409，可带 parent_id 重试）。
var ErrParentGone = errors.New("docservice: 父级位于回收站")

type TrashMaintenanceStore interface {
	store.TrashStore
	store.MaintenanceStore
}

func (s *Service) trash() TrashMaintenanceStore { return s.trashStore }

var ErrTrashNotWired = errors.New("docservice: 回收站存储未注入")

func (s *Service) ensureTrash() error {
	if s.trashStore == nil {
		return ErrTrashNotWired
	}
	return nil
}

// SetTrashHooks 注入回收站存储与维护面。
func (s *Service) SetTrashHooks(t TrashMaintenanceStore) {
	s.trashStore = t
	s.maint = t
}

// removeIndexed 尽力把文档移出搜索索引；失败入 delete 任务降级。
func (s *Service) removeIndexed(ctx context.Context, ids []string) {
	if s.indexer == nil {
		return
	}
	for _, id := range ids {
		if err := s.indexer.DeleteDoc(ctx, id); err != nil {
			idCopy := id
			s.enqueueReindex(ctx, &idCopy, "delete")
		}
	}
}

// TrashDocument 软删除子树；同步移除索引并释放 slug。
func (s *Service) TrashDocument(ctx context.Context, actor permission.Actor, id string) error {
	if err := s.ensureTrash(); err != nil {
		return err
	}
	if err := actor.Require(permission.DocDelete); err != nil {
		return err
	}
	d, err := aliveDoc(ctx, s, id)
	if err != nil {
		return err
	}
	sub, err := s.trees.SubtreeIDs(ctx, d.ID)
	if err != nil {
		return err
	}
	now := nowMillis()
	purgeAt := now + int64(s.trashDays)*86400_000
	if err := s.trash().SoftDeleteSubtree(ctx, d.ID, actor.UserID(), now, purgeAt); err != nil {
		return err
	}
	s.removeIndexed(ctx, sub)
	return nil
}

// ListTrash 回收站条目。
func (s *Service) ListTrash(ctx context.Context, actor permission.Actor,
	limit int) ([]*model.Document, error) {
	if err := actor.Require(permission.DocDelete); err != nil {
		return nil, err
	}
	if err := s.ensureTrash(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.trash().ListTrash(ctx, limit)
}

// RestoreDocument 从回收站恢复子树。父链已删且未指定新父 → ErrParentGone。
func (s *Service) RestoreDocument(ctx context.Context, actor permission.Actor,
	id string, newParentID *string) error {
	if err := actor.Require(permission.DocRestore); err != nil {
		return err
	}
	if err := s.ensureTrash(); err != nil {
		return err
	}
	d, err := s.docs.Get(ctx, id)
	if err != nil {
		return err
	}
	if d.Alive() {
		return invalid("id", "该文档不在回收站")
	}

	parentGone, err := s.trash().HasDeletedAncestor(ctx, id)
	if err != nil {
		return err
	}
	if parentGone && newParentID == nil {
		return ErrParentGone
	}
	if newParentID != nil && *newParentID != "" {
		if _, err := aliveDoc(ctx, s, *newParentID); err != nil {
			return err
		}
		if err := s.docs.Move(ctx, d.ID, newParentID, actor.UserID(), nowMillis()); err != nil {
			return err
		}
	} else if parentGone {
		return ErrParentGone
	}
	if err := s.trash().RestoreSubtree(ctx, d.ID, actor.UserID(), nowMillis()); err != nil {
		return err
	}
	s.reindexSnapshot(ctx, d.ID)
	// 子树内容快照一并恢复
	sub, _ := s.trees.SubtreeIDs(ctx, d.ID)
	for _, sid := range sub {
		if sid != d.ID {
			s.reindexSnapshot(ctx, sid)
		}
	}
	return nil
}

// PurgeDocument 彻底清除子树（不可逆）。
func (s *Service) PurgeDocument(ctx context.Context, actor permission.Actor, id string) error {
	if err := s.ensureTrash(); err != nil {
		return err
	}
	if err := actor.Require(permission.DocDelete); err != nil {
		return err
	}
	d, err := s.docs.Get(ctx, id)
	if err != nil {
		return err
	}
	if d.Alive() {
		return invalid("id", "须先进入回收站再彻底删除")
	}
	sub, err := s.trees.SubtreeIDsOfTrashed(ctx, d.ID)
	if err != nil {
		return err
	}
	if err := s.trash().PurgeSubtree(ctx, d.ID); err != nil {
		return err
	}
	s.removeIndexed(ctx, sub)
	return nil
}

// SweepPurgeDue 清理到期条目，返回处理数（T5.2 后台任务入口）。
func (s *Service) SweepPurgeDue(ctx context.Context, now int64) (int, error) {
	if err := s.ensureTrash(); err != nil {
		return 0, err
	}
	ids, err := s.trash().DuePurgeIDs(ctx, now)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, id := range ids {
		sub, serr := s.trees.SubtreeIDsOfTrashed(ctx, id)
		if serr != nil {
			continue
		}
		if perr := s.trash().PurgeSubtree(ctx, id); perr != nil {
			continue
		}
		s.removeIndexed(ctx, sub)
		count++
	}
	return count, nil
}

// GCBlobs 清理无引用 blob，返回数量（T5.3）。
func (s *Service) GCBlobs(ctx context.Context) (int64, error) {
	if s.maint == nil {
		return 0, errors.New("docservice: maintenance store 未注入")
	}
	return s.maint.GCDereferencedBlobs(ctx)
}
