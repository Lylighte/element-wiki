// 回收站业务（T5.1）与维护任务编排（T5.2/T5.3）。
package docservice

import (
	"context"
	"errors"
	"fmt"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

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
	retention := s.trashDays
	if s.settingsSrc != nil {
		retention = s.settingsSrc.IntSetting(ctx, "trash_retention_days", s.trashDays)
	}
	now := nowMillis()
	purgeAt := now + retention*86400_000
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

// 恢复容器（M18/T18.1）：根级普通文档，visibility=restricted——
// 恢复内容对 viewer/匿名不可见（404 掩护），管理员移出容器后按新父级生效。
// 容器可被移动/改名/回收（普通文档语义）；缺失时下次恢复惰性重建。
const (
	restoredRootSlug  = "restored"
	restoredRootTitle = "已恢复"
	restoredSlugTries = 20
)

// ensureRestoredRoot 复用存活根级 slug=restored 的容器；缺失则创建并设 restricted。
func (s *Service) ensureRestoredRoot(ctx context.Context, actor permission.Actor) (*model.Document, error) {
	existing, err := s.docs.GetBySlug(ctx, nil, restoredRootSlug, false)
	if err == nil {
		return existing, nil
	}
	if !IsNotFound(err) {
		return nil, err
	}
	root, err := s.CreateDocument(ctx, actor, nil, restoredRootSlug, restoredRootTitle)
	if err != nil {
		return nil, err
	}
	if err := s.SetVisibility(ctx, actor, root.ID, model.VisibilityRestricted); err != nil {
		return nil, err
	}
	return root, nil
}

// slugTakenUnder 判断容器下（存活行）是否已有该 slug。
func (s *Service) slugTakenUnder(ctx context.Context, parentID, slug string) (bool, error) {
	_, err := s.docs.GetBySlug(ctx, &parentID, slug, false)
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RestoreDocument 从回收站恢复子树到「已恢复」容器（M18/T18.1）：
// 不检查祖先链（父链被 purge/改名/移动均无影响）；子树内部结构随 RestoreSubtree 保留。
// 容器内 slug 冲突 → 原地自增 -2/-3…（上限 20，仍冲突返回 ErrConflict）——
// 回收站行不参与 (parent, slug) 部分唯一索引，可安全改写。
// 恢复后 purge_at 清空并重挂索引（派生数据）。
func (s *Service) RestoreDocument(ctx context.Context, actor permission.Actor, id string) error {
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
		return invalid("id", "document is not in trash")
	}

	root, err := s.ensureRestoredRoot(ctx, actor)
	if err != nil {
		return err
	}

	if taken, terr := s.slugTakenUnder(ctx, root.ID, d.Slug); terr != nil {
		return terr
	} else if taken {
		next, ok, nerr := func() (string, bool, error) {
			for n := 2; n <= restoredSlugTries+1; n++ {
				cand := fmt.Sprintf("%s-%d", d.Slug, n)
				if len(cand) > 80 {
					continue
				}
				t, terr := s.slugTakenUnder(ctx, root.ID, cand)
				if terr != nil {
					return "", false, terr
				}
				if !t {
					return cand, true, nil
				}
			}
			return "", false, nil
		}()
		if nerr != nil {
			return nerr
		}
		if !ok {
			return store.ErrConflict
		}
		if err := s.trash().UpdateTrashedSlug(ctx, id, next); err != nil {
			return err
		}
	}

	if d.ParentID == nil || *d.ParentID != root.ID {
		if err := s.docs.Move(ctx, id, &root.ID, actor.UserID(), nowMillis()); err != nil {
			return err
		}
	}
	if err := s.trash().RestoreSubtree(ctx, id, actor.UserID(), nowMillis()); err != nil {
		return err
	}
	s.reindexSnapshot(ctx, id)
	// 子树内容快照一并恢复
	sub, _ := s.trees.SubtreeIDs(ctx, id)
	for _, sid := range sub {
		if sid != id {
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
		return invalid("id", "move to trash before purging")
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
