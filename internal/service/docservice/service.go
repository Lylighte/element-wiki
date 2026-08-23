// Package docservice 承载文档树与版本域的业务规则（T1.5~T1.7）。
package docservice

import (
	"context"
	"fmt"
	"regexp"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

// Service 依赖全部经 store 接口注入，便于测试替身与未来 PG 实现。
type Service struct {
	docs    store.DocumentStore
	trees   store.TreeStore
	coms    store.CommitStore
	app     store.AppendCommitter
	drafts  store.DraftStore
	maxVers int64 // max_versions 默认值；运行时设置在 M6 接管
}

func New(docs store.DocumentStore, trees store.TreeStore,
	coms store.CommitStore, app store.AppendCommitter,
	drafts store.DraftStore, defaultMaxVersions int64) *Service {
	return &Service{docs: docs, trees: trees, coms: coms, app: app, drafts: drafts, maxVers: defaultMaxVersions}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateSlug(slug string) error {
	if slug == "" || len(slug) > 80 {
		return invalid("slug", "长度须为 1~80")
	}
	if !slugRe.MatchString(slug) {
		return invalid("slug", "仅允许小写字母、数字与中划线组合")
	}
	return nil
}

func validateTitle(title string) error {
	if title == "" {
		return invalid("title", "不能为空")
	}
	if len([]rune(title)) > 200 {
		return invalid("title", "长度超过 200 字符")
	}
	return nil
}

func aliveDoc(ctx context.Context, s *Service, id string) (*model.Document, error) {
	d, err := s.docs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !d.Alive() {
		return nil, store.ErrNotFound // 回收站文档对外不可见
	}
	return d, nil
}

// CreateDocument 创建树节点；父级必须存在且存活。
func (s *Service) CreateDocument(ctx context.Context, actor permission.Actor,
	parentID *string, slug, title string) (*model.Document, error) {
	if err := actor.Require(permission.DocCreate); err != nil {
		return nil, err
	}
	if parentID != nil && *parentID != "" {
		if _, err := aliveDoc(ctx, s, *parentID); err != nil {
			return nil, err
		}
	} else {
		parentID = nil
	}
	if err := validateSlug(slug); err != nil {
		return nil, err
	}
	if err := validateTitle(title); err != nil {
		return nil, err
	}

	now := nowMillis()
	d := &model.Document{
		ID: util.NewID(), ParentID: parentID, Slug: slug, Title: title,
		SortKey: 100, Visibility: model.VisibilityStandard,
		CreatedBy: actor.UserID(), UpdatedBy: actor.UserID(),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.docs.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// RenameDocument 修改 slug 与/或标题。
func (s *Service) RenameDocument(ctx context.Context, actor permission.Actor,
	id string, newSlug, newTitle *string) error {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return err
	}
	if _, err := aliveDoc(ctx, s, id); err != nil {
		return err
	}
	mut := model.DocumentMut{}
	if newSlug != nil {
		if *newSlug == "" {
			newSlug = nil // 仅改标题场景
		} else if err := validateSlug(*newSlug); err != nil {
			return err
		}
		mut.Slug = newSlug
	}
	if newTitle != nil {
		if err := validateTitle(*newTitle); err != nil {
			return err
		}
		mut.Title = newTitle
	}
	return s.docs.UpdateMeta(ctx, id, mut, actor.UserID(), nowMillis())
}

// SetVisibility 调整自身可见性档位（继承由读取侧解析）。
func (s *Service) SetVisibility(ctx context.Context, actor permission.Actor,
	id string, v model.Visibility) error {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return err
	}
	if !v.Valid() {
		return invalid("visibility", "取值非法")
	}
	if _, err := aliveDoc(ctx, s, id); err != nil {
		return err
	}
	return s.docs.UpdateMeta(ctx, id, model.DocumentMut{Visibility: &v}, actor.UserID(), nowMillis())
}

// MoveDocument 移动节点；禁止移入自身子树（含移动到自身）。
func (s *Service) MoveDocument(ctx context.Context, actor permission.Actor,
	id string, newParentID *string) error {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return err
	}
	if _, err := aliveDoc(ctx, s, id); err != nil {
		return err
	}
	if newParentID != nil && *newParentID != "" {
		if _, err := aliveDoc(ctx, s, *newParentID); err != nil {
			return err
		}
		sub, err := s.trees.SubtreeIDs(ctx, id)
		if err != nil {
			return err
		}
		for _, sid := range sub {
			if sid == *newParentID {
				return ErrSelfChild
			}
		}
	} else {
		newParentID = nil
	}
	return s.docs.Move(ctx, id, newParentID, actor.UserID(), nowMillis())
}

// ---- 草稿 ----

func (s *Service) SaveDraft(ctx context.Context, actor permission.Actor,
	docID, baseCommitID, content string) error {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return err
	}
	if _, err := aliveDoc(ctx, s, docID); err != nil {
		return err
	}
	return s.drafts.UpsertDraft(ctx, &model.Draft{
		DocumentID: docID, UserID: actor.UserID(),
		BaseCommitID: baseCommitID, Content: content, UpdatedAt: nowMillis(),
	})
}

func (s *Service) GetDraft(ctx context.Context, actor permission.Actor,
	docID string) (*model.Draft, error) {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return nil, err
	}
	if _, err := aliveDoc(ctx, s, docID); err != nil {
		return nil, err
	}
	return s.drafts.GetDraft(ctx, docID, actor.UserID())
}

func (s *Service) DeleteDraft(ctx context.Context, actor permission.Actor, docID string) error {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return err
	}
	return s.drafts.DeleteDraft(ctx, docID, actor.UserID())
}

// ---- 版本 ----

// CommitResult 是提交成功的返回：新版本 + 死链报告（RD-05）。
type CommitResult struct {
	Commit    *model.Commit
	DeadLinks []string
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// deadLinks 解析 [[目标]] 并逐个按 slug 全树匹配存活文档。
func (s *Service) deadLinks(ctx context.Context, content string) []string {
	found := map[string]bool{}
	matches := wikilinkRe.FindAllStringSubmatch(content, -1)
	var dead []string
	for _, m := range matches {
		target := m[1]
		if found[target] {
			continue
		}
		found[target] = true
		if _, err := s.docs.GetBySlug(ctx, nil, target, false); err != nil {
			dead = append(dead, target)
		}
	}
	return dead
}

// Commit 提交新版本。base 不等于当前 HEAD 时返回 VersionConflictError 且零写入。
func (s *Service) Commit(ctx context.Context, actor permission.Actor,
	docID, baseCommitID, content, message string) (*CommitResult, error) {
	if err := actor.Require(permission.DocUpdate); err != nil {
		return nil, err
	}
	d, err := aliveDoc(ctx, s, docID)
	if err != nil {
		return nil, err
	}
	if d.HeadCommitID != baseCommitID {
		return nil, &VersionConflictError{HeadCommitID: d.HeadCommitID}
	}

	hash := util.SHA256Hex(content)
	if err := s.coms.PutBlob(ctx, hash, content); err != nil {
		return nil, err
	}

	nextNo, err := s.nextCommitNo(ctx, docID)
	if err != nil {
		return nil, err
	}
	c := &model.Commit{
		ID: util.NewID(), DocumentID: docID, CommitNo: nextNo,
		BlobHash: hash, AuthorID: actor.UserID(), Message: message,
		CreatedAt: nowMillis(),
	}
	if nextNo > 1 {
		c.ParentCommitID = ptrStr(d.HeadCommitID)
	}
	if _, err := s.app.AppendCommit(ctx, c, s.maxVers); err != nil {
		return nil, err
	}
	return &CommitResult{Commit: c, DeadLinks: s.deadLinks(ctx, content)}, nil
}

// nextCommitNo 用 MAX(commit_no)+1：版本裁剪会制造序号缺口，COUNT+1 会撞号。
func (s *Service) nextCommitNo(ctx context.Context, docID string) (int64, error) {
	m, err := s.coms.MaxCommitNo(ctx, docID)
	if err != nil {
		return 0, err
	}
	return m + 1, nil
}

// Revert 以历史版本内容新建 commit（不改写历史，DM-04）。
func (s *Service) Revert(ctx context.Context, actor permission.Actor,
	docID, commitID string) (*CommitResult, error) {
	if err := actor.Require(permission.VersionRevert); err != nil {
		return nil, err
	}
	d, err := aliveDoc(ctx, s, docID)
	if err != nil {
		return nil, err
	}
	old, err := s.coms.GetCommit(ctx, docID, commitID)
	if err != nil {
		return nil, err
	}
	content, err := s.coms.GetBlob(ctx, old.BlobHash)
	if err != nil {
		return nil, err
	}
	res, err := s.commitLocked(ctx, actor, d, content,
		fmt.Sprintf("revert to #%d", old.CommitNo))
	if err != nil {
		return nil, err
	}
	return res, nil
}

// commitLocked 跳过 base 校验的内部提交（revert 专用）。
func (s *Service) commitLocked(ctx context.Context, actor permission.Actor,
	d *model.Document, content, message string) (*CommitResult, error) {
	hash := util.SHA256Hex(content)
	if err := s.coms.PutBlob(ctx, hash, content); err != nil {
		return nil, err
	}
	nextNo, err := s.nextCommitNo(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	c := &model.Commit{
		ID: util.NewID(), DocumentID: d.ID, CommitNo: nextNo,
		BlobHash: hash, AuthorID: actor.UserID(), Message: message,
		CreatedAt: nowMillis(),
	}
	if nextNo > 1 {
		fresh, err := s.docs.Get(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		c.ParentCommitID = ptrStr(fresh.HeadCommitID)
	}
	if _, err := s.app.AppendCommit(ctx, c, s.maxVers); err != nil {
		return nil, err
	}
	return &CommitResult{Commit: c, DeadLinks: s.deadLinks(ctx, content)}, nil
}

// ListCommits 版本历史（降序）。
func (s *Service) ListCommits(ctx context.Context, actor permission.Actor,
	docID string, limit int) ([]*model.Commit, error) {
	if err := actor.Require(permission.VersionRead); err != nil {
		return nil, err
	}
	if _, err := aliveDoc(ctx, s, docID); err != nil {
		return nil, err
	}
	return s.coms.ListCommits(ctx, docID, limit)
}

// HeadContent 返回 HEAD 正文；无任何版本时返回空串。
func (s *Service) HeadContent(ctx context.Context, actor permission.Actor,
	docID string) (string, *model.Commit, error) {
	if err := actor.Require(permission.VersionRead); err != nil {
		return "", nil, err
	}
	d, err := aliveDoc(ctx, s, docID)
	if err != nil {
		return "", nil, err
	}
	if d.HeadCommitID == "" {
		return "", nil, nil
	}
	head, err := s.coms.GetCommit(ctx, docID, d.HeadCommitID)
	if err != nil {
		return "", nil, err
	}
	content, err := s.coms.GetBlob(ctx, head.BlobHash)
	if err != nil {
		return "", nil, err
	}
	return content, head, nil
}

// Get 暴露受权限控制的单文档元数据读取。
func (s *Service) Get(ctx context.Context, actor permission.Actor, id string) (*model.Document, error) {
	if err := actor.Require(permission.DocRead); err != nil {
		return nil, err
	}
	return aliveDoc(ctx, s, id)
}
