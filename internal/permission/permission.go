// Package permission 实现权限码目录与 Actor（doc/02 §13 为权威契约）。
package permission

import (
	"errors"
	"maps"
	"slices"
)

// 权限码常量。新增必须同步四处：此处、角色映射、catalog 测试、前端 permissions。
const (
	DocRead           = "document.read"
	DocReadRestricted = "document.read.restricted"
	DocCreate         = "document.create"
	DocUpdate         = "document.update"
	DocDelete         = "document.delete"
	DocRestore        = "document.restore"

	VersionRead   = "version.read"
	VersionRevert = "version.revert"

	AttachmentRead   = "attachment.read"
	AttachmentUpload = "attachment.upload"
	AttachmentDelete = "attachment.delete"

	CommentRead      = "comment.read"
	CommentCreate    = "comment.create"
	CommentDeleteOwn = "comment.delete.own"
	CommentDeleteAny = "comment.delete.any"

	UserList       = "user.list"
	UserManage     = "user.manage"
	SettingsManage = "settings.manage"
	DashboardRead  = "dashboard.read"
	BackupManage   = "backup.manage"
	ImportRun      = "import.run"
	SearchRebuild  = "search.rebuild"
	TokenManageOwn = "token.manage.own"
)

// AllCodes 是目录全集，测试用它保证角色映射不越界、不遗漏。保持排序以便 diff 审查。
var AllCodes = []string{
	AttachmentDelete, AttachmentRead, AttachmentUpload,
	BackupManage, CommentCreate, CommentDeleteAny, CommentDeleteOwn, CommentRead,
	DashboardRead, DocCreate, DocDelete, DocRead, DocReadRestricted, DocRestore, DocUpdate,
	ImportRun, SearchRebuild, SettingsManage, TokenManageOwn,
	UserList, UserManage, VersionRead, VersionRevert,
}

// Role 三档内置模板（PM-04）：仅作为指派模板，业务代码禁止按角色名分支。
type Role string

const (
	Viewer Role = "viewer"
	Editor Role = "editor"
	Admin  Role = "admin"
)

func (r Role) Valid() bool { return r == Viewer || r == Editor || r == Admin }

// roleExtras 是各角色相对 viewer 的增量，Admin 拥有全目录。
var roleExtras = map[Role][]string{
	Viewer: {},
	Editor: {DocReadRestricted, DocCreate, DocUpdate, DocDelete, DocRestore, VersionRevert,
		AttachmentUpload, AttachmentDelete},
	Admin: {CommentDeleteAny, UserList, UserManage, SettingsManage, DashboardRead,
		BackupManage, ImportRun, SearchRebuild},
}

// base 是所有登录角色的公共集。
var base = []string{DocRead, VersionRead, AttachmentRead,
	CommentRead, CommentCreate, CommentDeleteOwn, TokenManageOwn}

// CodesFor 返回角色的权限码集合副本。
func CodesFor(r Role) []string {
	set := map[string]bool{}
	for _, c := range base {
		set[c] = true
	}
	for _, c := range roleExtras[r] {
		set[c] = true
	}
	if r == Admin {
		for _, c := range AllCodes {
			set[c] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

var ErrDenied = errors.New("permission denied")

// Actor 是已认证操作者的权限视图。
type Actor interface {
	UserID() string
	Has(code string) bool
	HasAny(codes ...string) bool
	// Require 任一缺失即返回 ErrDenied。
	Require(codes ...string) error
}

type actor struct {
	id    string
	codes map[string]bool
}

// NewActor 构造登录 Actor；codes 来自 CodesFor 或用户自定义集合。
func NewActor(id string, codes []string) Actor {
	a := &actor{id: id, codes: make(map[string]bool, len(codes))}
	for _, c := range codes {
		a.codes[c] = true
	}
	return a
}

// Anonymous 构造匿名 Actor；仅当站点开启匿名只读时持有读集。
func Anonymous(readEnabled bool) Actor {
	if !readEnabled {
		return NewActor("", nil)
	}
	return NewActor("", []string{DocRead, VersionRead, AttachmentRead, CommentRead})
}

func (a *actor) UserID() string       { return a.id }
func (a *actor) Has(code string) bool { return a.codes[code] }
func (a *actor) HasAny(codes ...string) bool {
	for _, c := range codes {
		if a.codes[c] {
			return true
		}
	}
	return false
}
func (a *actor) Require(codes ...string) error {
	for _, c := range codes {
		if !a.codes[c] {
			return ErrDenied
		}
	}
	return nil
}
