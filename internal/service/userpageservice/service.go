package userpageservice

import (
	"context"
	"errors"
	"strconv"
	"strings"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/util"
)

var (
	ErrDisabled   = errors.New("user pages disabled")
	ErrSelfReview = errors.New("cannot review own submission")
	ErrInvalid    = errors.New("invalid user page")
)

type Settings interface {
	StrSetting(context.Context, string, string) string
}

type Service struct {
	pages    store.UserPageStore
	users    store.UserStore
	settings Settings
	now      func() int64
}

func New(pages store.UserPageStore, users store.UserStore, settings Settings) *Service {
	return &Service{pages: pages, users: users, settings: settings, now: util.NowMillis}
}

func (s *Service) enabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	v, err := strconv.ParseBool(s.settings.StrSetting(ctx, "user_pages_enabled", "false"))
	return err == nil && v
}
func (s *Service) reviewRequired(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	v, err := strconv.ParseBool(s.settings.StrSetting(ctx, "user_pages_review_required", "false"))
	return err == nil && v
}

func (s *Service) Get(ctx context.Context, actor permission.Actor, userID string) (*model.UserPage, error) {
	if !s.enabled(ctx) && !actor.Has(permission.UserManage) {
		return nil, ErrDisabled
	}
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	page, err := s.pages.GetUserPage(ctx, userID)
	if store.IsNotFound(err) && (actor.UserID() == userID || actor.Has(permission.UserManage)) {
		page = &model.UserPage{UserID: userID}
	} else if err != nil {
		return nil, err
	}
	if actor.UserID() != userID && !actor.Has(permission.ReviewManage) {
		page.Pending = nil
	}
	page.DisplayName = user.DisplayName
	return page, nil
}

func (s *Service) Save(ctx context.Context, actor permission.Actor, userID, content string) (*model.UserPage, error) {
	if !s.enabled(ctx) {
		return nil, ErrDisabled
	}
	if actor.UserID() == userID {
		if err := actor.Require(permission.UserPageManageOwn); err != nil {
			return nil, err
		}
	} else if err := actor.Require(permission.UserManage); err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" || len([]rune(content)) > 20000 {
		return nil, ErrInvalid
	}
	if _, err := s.users.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	requireReview := s.reviewRequired(ctx) && !actor.Has(permission.ReviewManage)
	r := &model.UserPageRevision{ID: util.NewID(), UserID: userID, Content: content, CreatedBy: actor.UserID(), CreatedAt: s.now()}
	if err := s.pages.SubmitUserPageRevision(ctx, r, requireReview); err != nil {
		return nil, err
	}
	return s.Get(ctx, actor, userID)
}

func (s *Service) Review(ctx context.Context, actor permission.Actor, userID, revisionID, action, reason string) error {
	if err := actor.Require(permission.ReviewManage); err != nil {
		return err
	}
	if action != "approve" && action != "reject" {
		return ErrInvalid
	}
	page, err := s.pages.GetUserPage(ctx, userID)
	if err != nil {
		return err
	}
	if page.Pending == nil || page.Pending.ID != revisionID {
		return store.ErrNotFound
	}
	if page.Pending.CreatedBy == actor.UserID() {
		return ErrSelfReview
	}
	if len([]rune(reason)) > 2000 || action == "reject" && strings.TrimSpace(reason) == "" {
		return ErrInvalid
	}
	return s.pages.ReviewUserPageRevision(ctx, userID, revisionID, actor.UserID(), action, reason, s.now())
}

func (s *Service) Delete(ctx context.Context, actor permission.Actor, userID string) error {
	if actor.UserID() == userID {
		if err := actor.Require(permission.UserPageManageOwn); err != nil {
			return err
		}
	} else if err := actor.Require(permission.UserManage); err != nil {
		return err
	}
	return s.pages.DeleteUserPage(ctx, userID)
}

func (s *Service) Pending(ctx context.Context, actor permission.Actor, limit int) ([]*model.UserPageRevision, error) {
	if err := actor.Require(permission.ReviewManage); err != nil {
		return nil, err
	}
	return s.pages.ListPendingUserPageRevisions(ctx, limit)
}

func (s *Service) History(ctx context.Context, actor permission.Actor, userID string, limit int) ([]*model.UserPageRevision, error) {
	if !s.enabled(ctx) && !actor.Has(permission.UserManage) {
		return nil, ErrDisabled
	}
	if _, err := s.users.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	publicViewer := actor.UserID() != userID && !actor.Has(permission.ReviewManage)
	if publicViewer {
		page, err := s.pages.GetUserPage(ctx, userID)
		if err != nil {
			return nil, err
		}
		if page.Published == nil {
			return []*model.UserPageRevision{}, nil
		}
	}
	revisions, err := s.pages.ListUserPageRevisions(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	if publicViewer {
		visible := revisions[:0]
		for _, rev := range revisions {
			if rev.Status == "published" {
				visible = append(visible, rev)
			}
		}
		revisions = visible
	}
	return revisions, nil
}
