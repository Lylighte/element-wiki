// 评论业务（T5.6）：线性列表 + @提及解析；全局门闩在路由层。
package docservice

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

// SetCommentStore 注入评论存储与用户查找面。
func (s *Service) SetCommentStore(cs store.CommentStore, users store.UserStore) {
	s.comments = cs
	s.userLookup = users
}

var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`)

// ErrCommentsDisabled 由路由层依据设置短路，service 不重复判断。
var ErrCommentsDisabled = errors.New("comments disabled")

// AddComment 写入评论并解析 @email 提及。
func (s *Service) AddComment(ctx context.Context, actor permission.Actor,
	docID, content string) (*model.Comment, error) {
	if err := actor.Require(permission.CommentCreate); err != nil {
		return nil, err
	}
	if s.comments == nil {
		return nil, errors.New("docservice: comment store 未配置")
	}
	d, err := aliveDoc(ctx, s, docID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureReadable(ctx, actor, d.ID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" || len([]rune(content)) > 8000 {
		return nil, invalid("content", "length must be 1-8000")
	}

	c := &model.Comment{
		ID: util.NewID(), DocumentID: docID, AuthorID: actor.UserID(),
		Content: content, CreatedAt: nowMillis(),
	}
	var mentionIDs []string
	for _, email := range mentionRe.FindAllStringSubmatch(content, -1) {
		if u, uerr := s.userLookup.FindUserByEmail(ctx, email[1]); uerr == nil {
			mentionIDs = append(mentionIDs, u.ID)
		}
	}
	if err := s.comments.CreateComment(ctx, c, dedupe(mentionIDs)); err != nil {
		return nil, err
	}
	c.Mentions = dedupe(mentionIDs)
	return c, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ListComments 升序分页读取。
func (s *Service) ListComments(ctx context.Context, actor permission.Actor,
	docID string, limit int) ([]*model.Comment, error) {
	if err := actor.Require(permission.CommentRead); err != nil {
		return nil, err
	}
	d, err := aliveDoc(ctx, s, docID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureReadable(ctx, actor, d.ID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	list, err := s.comments.ListComments(ctx, docID, limit)
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		mids, _ := s.comments.MentionIDsOf(ctx, c.ID)
		c.Mentions = mids
	}
	return list, nil
}

// DeleteComment：作者本人或 CommentDeleteAny。
func (s *Service) DeleteComment(ctx context.Context, actor permission.Actor, id string) error {
	c, err := s.comments.GetComment(ctx, id)
	if err != nil {
		return err
	}
	if !actor.Has(permission.CommentDeleteAny) &&
		!(actor.Has(permission.CommentDeleteOwn) && c.AuthorID == actor.UserID()) {
		return permission.ErrDenied
	}
	return s.comments.DeleteComment(ctx, id)
}
