package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	adminservice "element-wiki/internal/service/adminservice"
	authservice "element-wiki/internal/service/authservice"
	backupservice "element-wiki/internal/service/backupservice"
	"element-wiki/internal/service/docservice"
	searchservice "element-wiki/internal/service/searchservice"
	userpageservice "element-wiki/internal/service/userpageservice"

	store "element-wiki/internal/database"
	"element-wiki/internal/render"
)

// Deps 是路由层全部依赖。
type Deps struct {
	Docs          *docservice.Service
	Trees         store_Tree
	ActorFor      func(r *http.Request) permission.Actor // 测试注入；中间件注入的上下文身份优先
	Render        func(src string) (*render.Result, error)
	Auth          *authservice.Service
	OIDC          *OIDCDeps
	SecureCookies bool
	Jobs          store.SearchJobStore
	Imports       store.ImportJobStore

	// Search 可选；nil 时不挂载搜索路由。
	Search *searchservice.Service

	Backups         *backupservice.Service
	MarkdownImports *backupservice.MarkdownImporter

	// 协作与附件开关/配置（main 注入）。
	Admin             *adminservice.Service
	UserPages         *userpageservice.Service
	Reports           store.ContentReportStore
	ContentModeration store.ContentModerationStore

	CommentsEnabled bool
	AttachmentsOn   bool
	AttachDir       string
	UploadMaxBytes  int64

	// DBDriver：postgres 时备份导出/导入显式 501 降级（C6/T12.4）。
	DBDriver string

	// 站点公开信息默认值（config 注入）；Admin 可用在线设置覆盖（C3）。
	SiteDefaults SiteInfo
}

// SiteInfo 是 GET /v1/site 的公开载荷。
type SiteInfo struct {
	Title                   string `json:"title"`
	DefaultLang             string `json:"default_lang"`
	Timezone                string `json:"timezone"`
	AnonymousRead           bool   `json:"anonymous_read"`
	CommentsEnabled         bool   `json:"comments_enabled"`
	UserPagesEnabled        bool   `json:"user_pages_enabled"`
	UserPagesReviewRequired bool   `json:"user_pages_review_required"`
	DocumentReviewRequired  bool   `json:"document_review_required"`
	CommentReviewRequired   bool   `json:"comment_review_required"`
	DeploymentPreset        string `json:"deployment_preset"`
	SiteIconURL             string `json:"site_icon_url"`
	ThemePreset             string `json:"theme_preset"`
	ThemeLightPrimary       string `json:"theme_light_primary"`
	ThemeLightAccent        string `json:"theme_light_accent"`
	ThemeLightFocus         string `json:"theme_light_focus"`
	ThemeDarkPrimary        string `json:"theme_dark_primary"`
	ThemeDarkAccent         string `json:"theme_dark_accent"`
	ThemeDarkFocus          string `json:"theme_dark_focus"`
	ArticleFooterHTML       string `json:"article_footer_html"`
	SidebarFooterHTML       string `json:"sidebar_footer_html"`
}

// handleSite 公开站点信息（契约 §12/C3）：config 默认值 + 在线设置覆盖。
func (d *Deps) handleSite(w http.ResponseWriter, r *http.Request) {
	site := d.SiteDefaults
	if d.Admin != nil {
		if m := d.Admin.PublicSiteValues(r.Context()); m != nil {
			if v, ok := m["wiki_title"]; ok && v != "" {
				site.Title = v
			}
			if v, ok := m["default_lang"]; ok && (v == "zh-CN" || v == "en") {
				site.DefaultLang = v
			}
			if v, ok := m["timezone"]; ok && v != "" {
				site.Timezone = v
			}
			if v, err := strconv.ParseBool(m["anonymous_read"]); err == nil && m["anonymous_read"] != "" {
				site.AnonymousRead = v
			}
			if v, err := strconv.ParseBool(m["comments_enabled"]); err == nil && m["comments_enabled"] != "" {
				site.CommentsEnabled = v
			}
			if v, err := strconv.ParseBool(m["user_pages_enabled"]); err == nil && m["user_pages_enabled"] != "" {
				site.UserPagesEnabled = v
			}
			if v, err := strconv.ParseBool(m["user_pages_review_required"]); err == nil && m["user_pages_review_required"] != "" {
				site.UserPagesReviewRequired = v
			}
			if v, err := strconv.ParseBool(m["document_review_required"]); err == nil && m["document_review_required"] != "" {
				site.DocumentReviewRequired = v
			}
			if v, err := strconv.ParseBool(m["comment_review_required"]); err == nil && m["comment_review_required"] != "" {
				site.CommentReviewRequired = v
			}
			if v, ok := m["deployment_preset"]; ok {
				site.DeploymentPreset = v
			}
			if v, ok := m["site_icon_url"]; ok {
				site.SiteIconURL = v
			}
			if v, ok := m["theme_preset"]; ok && v != "" {
				site.ThemePreset = v
			}
			if v, ok := m["theme_light_primary"]; ok && v != "" {
				site.ThemeLightPrimary = v
			}
			if v, ok := m["theme_light_accent"]; ok && v != "" {
				site.ThemeLightAccent = v
			}
			if v, ok := m["theme_light_focus"]; ok && v != "" {
				site.ThemeLightFocus = v
			}
			if v, ok := m["theme_dark_primary"]; ok && v != "" {
				site.ThemeDarkPrimary = v
			}
			if v, ok := m["theme_dark_accent"]; ok && v != "" {
				site.ThemeDarkAccent = v
			}
			if v, ok := m["theme_dark_focus"]; ok && v != "" {
				site.ThemeDarkFocus = v
			}
			if v, ok := m["article_footer_markdown"]; ok && v != "" {
				if rendered, err := d.Render(v); err == nil {
					site.ArticleFooterHTML = safeSiteFooterHTML(rendered.HTML)
				}
			}
			if v, ok := m["sidebar_footer_markdown"]; ok && v != "" {
				if rendered, err := d.Render(v); err == nil {
					site.SidebarFooterHTML = safeSiteFooterHTML(rendered.HTML)
				}
			}
		}
	}
	if site.ThemePreset == "" {
		site.ThemePreset = "blue"
	}
	if site.DeploymentPreset == "" {
		site.DeploymentPreset = "internal"
	}
	if site.ThemeLightPrimary == "" {
		site.ThemeLightPrimary = "#2563EB"
	}
	if site.ThemeLightAccent == "" {
		site.ThemeLightAccent = "#DBEAFE"
	}
	if site.ThemeLightFocus == "" {
		site.ThemeLightFocus = "#2563EB"
	}
	if site.ThemeDarkPrimary == "" {
		site.ThemeDarkPrimary = "#60A5FA"
	}
	if site.ThemeDarkAccent == "" {
		site.ThemeDarkAccent = "#1E3A5F"
	}
	if site.ThemeDarkFocus == "" {
		site.ThemeDarkFocus = "#93C5FD"
	}
	writeJSON(w, http.StatusOK, site)
}

// CookieCfg 供认证处理器写 cookie。
func (d *Deps) CookieCfg() cookieCfg { return cookieCfg{d.SecureCookies} }

// store_Tree 仅取树查询所需接口，避免依赖整个 store 包。
type store_Tree interface {
	ListChildren(ctx context.Context, parentID *string) ([]*model.Document, error)
	EffectiveVisibility(ctx context.Context, docID string) (model.Visibility, error)
}

func (d *Deps) actor(r *http.Request) permission.Actor {
	if a := ActorFrom(r); a != nil {
		return a // 中间件注入（真实认证）
	}
	if d.ActorFor != nil {
		return d.ActorFor(r) // 测试覆盖
	}
	return permission.Anonymous(false)
}

// NewRouter 组装全站路由；deps.Docs 为 nil 时仅挂载 healthz。
// Auth 非空时整体包裹认证中间件。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		deps.handleSitemap(w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if deps.Docs == nil {
		return mux
	}
	if deps.Render == nil {
		deps.Render = render.Render
	}
	dp := &deps

	mux.HandleFunc("GET /v1/documents/tree", func(w http.ResponseWriter, r *http.Request) {
		dp.handleTree(w, r)
	})
	mux.HandleFunc("GET /v1/documents/resolve", func(w http.ResponseWriter, r *http.Request) {
		dp.handleResolve(w, r)
	})
	mux.HandleFunc("POST /v1/documents", func(w http.ResponseWriter, r *http.Request) {
		dp.handleCreate(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handleGet(w, r)
	})
	mux.HandleFunc("PATCH /v1/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handlePatch(w, r)
	})
	mux.HandleFunc("DELETE /v1/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handleDeleteDocument(w, r)
	})
	mux.HandleFunc("PUT /v1/documents/reorder", func(w http.ResponseWriter, r *http.Request) {
		dp.handleReorder(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/render", func(w http.ResponseWriter, r *http.Request) {
		dp.handleRender(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/export.md", func(w http.ResponseWriter, r *http.Request) {
		dp.handleExportMarkdown(w, r)
	})
	mux.HandleFunc("PUT /v1/documents/{id}/draft", func(w http.ResponseWriter, r *http.Request) {
		dp.handlePutDraft(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/draft", func(w http.ResponseWriter, r *http.Request) {
		dp.handleGetDraft(w, r)
	})
	mux.HandleFunc("DELETE /v1/documents/{id}/draft", func(w http.ResponseWriter, r *http.Request) {
		dp.handleDeleteDraft(w, r)
	})
	mux.HandleFunc("POST /v1/documents/{id}/commits", func(w http.ResponseWriter, r *http.Request) {
		dp.handleCommit(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/commits", func(w http.ResponseWriter, r *http.Request) {
		dp.handleListCommits(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/commits/{commit_id}/content", func(w http.ResponseWriter, r *http.Request) {
		dp.handleCommitContent(w, r)
	})
	mux.HandleFunc("POST /v1/documents/{id}/revert", func(w http.ResponseWriter, r *http.Request) {
		dp.handleRevert(w, r)
	})
	mux.HandleFunc("POST /v1/render-preview", func(w http.ResponseWriter, r *http.Request) {
		dp.handlePreview(w, r)
	})

	if deps.Search != nil {
		mux.HandleFunc("GET /v1/search", func(w http.ResponseWriter, r *http.Request) {
			dp.handleSearch(w, r)
		})
		mux.HandleFunc("POST /v1/admin/search/rebuild", func(w http.ResponseWriter, r *http.Request) {
			dp.handleRebuildRequest(w, r)
		})
		mux.HandleFunc("GET /v1/admin/search/rebuild/{job_id}", func(w http.ResponseWriter, r *http.Request) {
			dp.handleRebuildStatus(w, r)
		})
	}

	mux.HandleFunc("GET /v1/trash", func(w http.ResponseWriter, r *http.Request) {
		dp.handleListTrash(w, r)
	})
	mux.HandleFunc("POST /v1/trash/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		dp.handleRestoreTrash(w, r)
	})
	mux.HandleFunc("DELETE /v1/trash/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handlePurgeTrash(w, r)
	})

	mux.HandleFunc("POST /v1/documents/{id}/comments", func(w http.ResponseWriter, r *http.Request) {
		dp.handleAddComment(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/comments", func(w http.ResponseWriter, r *http.Request) {
		dp.handleListComments(w, r)
	})
	mux.HandleFunc("DELETE /v1/comments/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handleDeleteComment(w, r)
	})

	mux.HandleFunc("POST /v1/documents/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		dp.handleUploadAttachment(w, r)
	})
	mux.HandleFunc("GET /v1/documents/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		dp.handleListAttachments(w, r)
	})
	mux.HandleFunc("GET /v1/attachments/{id}/raw", func(w http.ResponseWriter, r *http.Request) {
		dp.handleRawAttachment(w, r)
	})
	mux.HandleFunc("DELETE /v1/attachments/{id}", func(w http.ResponseWriter, r *http.Request) {
		dp.handleDeleteAttachment(w, r)
	})

	if deps.AttachmentsOn {
		_ = deps.AttachDir
	}

	if deps.Backups != nil && deps.Jobs != nil {
		mux.HandleFunc("POST /v1/admin/backups", func(w http.ResponseWriter, r *http.Request) {
			dp.handleStartBackup(w, r)
		})
		mux.HandleFunc("GET /v1/admin/backups/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			dp.handleBackupJobStatus(w, r)
		})
		mux.HandleFunc("GET /v1/admin/backups/files", func(w http.ResponseWriter, r *http.Request) {
			dp.handleListBackupFiles(w, r)
		})
		mux.HandleFunc("GET /v1/admin/backups/files/{name}/download", func(w http.ResponseWriter, r *http.Request) {
			dp.handleDownloadBackup(w, r)
		})
		mux.HandleFunc("DELETE /v1/admin/backups/files/{name}", func(w http.ResponseWriter, r *http.Request) {
			dp.handleDeleteBackupFile(w, r)
		})
	}
	if deps.Backups != nil && deps.Jobs != nil {
		mux.HandleFunc("POST /v1/admin/imports", func(w http.ResponseWriter, r *http.Request) {
			dp.handleImportBackup(w, r)
		})
		mux.HandleFunc("POST /v1/admin/markdown-import", func(w http.ResponseWriter, r *http.Request) {
			dp.handleStartMarkdownImport(w, r)
		})
		mux.HandleFunc("GET /v1/admin/imports/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			dp.handleImportJobStatus(w, r)
		})
	}

	mux.HandleFunc("GET /v1/site", func(w http.ResponseWriter, r *http.Request) {
		dp.handleSite(w, r)
	})
	mux.HandleFunc("GET /v1/site/icon/{filename}", func(w http.ResponseWriter, r *http.Request) {
		dp.handleSiteIcon(w, r)
	})
	if deps.Auth != nil {
		if deps.Reports != nil {
			mux.HandleFunc("POST /v1/reports", dp.handleCreateReport)
		}
		if deps.ContentModeration != nil {
			mux.HandleFunc("GET /v1/admin/moderation/hidden", dp.handleListHiddenContent)
			mux.HandleFunc("POST /v1/admin/moderation/{content_type}/{content_id}/unpublish", dp.handleUnpublishContent)
			mux.HandleFunc("POST /v1/admin/moderation/{content_type}/{content_id}/restore", dp.handleRestoreContent)
		}
		mux.HandleFunc("GET /v1/auth/oidc/status", func(w http.ResponseWriter, r *http.Request) {
			dp.handleOIDCStatus(w, r)
		})
		mux.HandleFunc("GET /v1/auth/oidc/login", func(w http.ResponseWriter, r *http.Request) {
			dp.handleOIDCLogin(w, r)
		})
		mux.HandleFunc("GET /v1/auth/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
			dp.handleOIDCCallback(w, r)
		})
		mux.HandleFunc("POST /v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
			dp.handleLogout(w, r)
		})
		mux.HandleFunc("GET /v1/users/me", func(w http.ResponseWriter, r *http.Request) {
			dp.handleMe(w, r)
		})
		mux.HandleFunc("GET /v1/users/me/preferences", dp.handleGetMyPreferences)
		mux.HandleFunc("PUT /v1/users/me/preferences", dp.handleSetMyPreferences)
		if deps.UserPages != nil {
			mux.HandleFunc("GET /v1/users/me/page", dp.handleGetMyPage)
			mux.HandleFunc("GET /v1/users/me/page/revisions", dp.handleGetMyPageRevisions)
			mux.HandleFunc("PUT /v1/users/me/page", dp.handleSaveMyPage)
			mux.HandleFunc("DELETE /v1/users/me/page", dp.handleDeleteMyPage)
			mux.HandleFunc("GET /v1/users/{user_id}/page", dp.handleGetUserPage)
			mux.HandleFunc("GET /v1/users/{user_id}/page/revisions", dp.handleGetUserPageRevisions)
			mux.HandleFunc("PUT /v1/admin/users/{user_id}/page", dp.handleAdminSaveUserPage)
			mux.HandleFunc("POST /v1/users/{user_id}/page/revisions/{revision_id}/approve", dp.handleApproveUserPage)
			mux.HandleFunc("POST /v1/users/{user_id}/page/revisions/{revision_id}/reject", dp.handleRejectUserPage)
			mux.HandleFunc("GET /v1/admin/reviews/user-pages", dp.handleListPendingUserPages)
		}
		mux.HandleFunc("GET /v1/admin/reviews/comments", dp.handleListPendingComments)
		mux.HandleFunc("POST /v1/admin/reviews/comments/{id}/approve", dp.handleApproveComment)
		mux.HandleFunc("POST /v1/admin/reviews/comments/{id}/reject", dp.handleRejectComment)
		mux.HandleFunc("GET /v1/admin/reviews/documents", dp.handleListPendingDocuments)
		mux.HandleFunc("POST /v1/admin/reviews/documents/{id}/approve", dp.handleApproveDocumentSubmission)
		mux.HandleFunc("POST /v1/admin/reviews/documents/{id}/reject", dp.handleRejectDocumentSubmission)
		if deps.Reports != nil {
			mux.HandleFunc("GET /v1/admin/reviews/reports", dp.handleListReports)
			mux.HandleFunc("POST /v1/admin/reviews/reports/{id}/resolve", dp.handleResolveReport)
			mux.HandleFunc("POST /v1/admin/reviews/reports/{id}/dismiss", dp.handleDismissReport)
		}

		mux.HandleFunc("GET /v1/admin/settings", func(w http.ResponseWriter, r *http.Request) {
			dp.handleGetSettings(w, r)
		})
		mux.HandleFunc("PATCH /v1/admin/settings", func(w http.ResponseWriter, r *http.Request) {
			dp.handlePatchSettings(w, r)
		})
		mux.HandleFunc("POST /v1/admin/site/icon", func(w http.ResponseWriter, r *http.Request) {
			dp.handleUploadSiteIcon(w, r)
		})
		mux.HandleFunc("GET /v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
			dp.handleListUsers(w, r)
		})
		mux.HandleFunc("PATCH /v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
			dp.handlePatchUser(w, r)
		})
		mux.HandleFunc("GET /v1/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
			dp.handleDashboard(w, r)
		})

		mux.HandleFunc("GET /v1/tokens", func(w http.ResponseWriter, r *http.Request) {
			dp.handleListTokens(w, r)
		})
		mux.HandleFunc("POST /v1/tokens", func(w http.ResponseWriter, r *http.Request) {
			dp.handleCreateToken(w, r)
		})
		mux.HandleFunc("DELETE /v1/tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
			dp.handleDeleteToken(w, r)
		})
	}
	var handler http.Handler = mux
	// ActorFor 是测试注入通道：存在时跳过真实认证中间件，避免双重身份语义。
	if deps.Auth != nil && deps.ActorFor == nil {
		handler = authMiddleware(deps.Auth, mux)
	}
	return handler
}

func pathID(r *http.Request) string { return r.PathValue("id") }

type treeNode struct {
	ID         string     `json:"id"`
	ParentID   *string    `json:"parent_id"`
	Title      string     `json:"title"`
	Slug       string     `json:"slug"`
	SortKey    int64      `json:"sort_key"`
	Restricted bool       `json:"restricted"`
	Children   []treeNode `json:"children"`
}

func (d *Deps) buildTree(ctx context.Context, actor permission.Actor,
	parent *string, restrictedInherited bool) []treeNode {
	kids, err := d.Docs.ListChildrenForTree(ctx, actor, parent)
	if err != nil {
		slog.Error("tree 构建失败", "err", err)
		return []treeNode{}
	}
	out := make([]treeNode, 0, len(kids))
	for _, k := range kids {
		restricted := restrictedInherited || k.Visibility == model.VisibilityRestricted
		out = append(out, treeNode{
			ID: k.ID, ParentID: k.ParentID, Title: k.Title, Slug: k.Slug,
			SortKey: k.SortKey, Restricted: restricted,
			Children: d.buildTree(ctx, actor, &k.ID, restricted),
		})
	}
	return out
}

func documentView(v *model.Document) map[string]any {
	return map[string]any{
		"id": v.ID, "parent_id": v.ParentID, "slug": v.Slug, "title": v.Title,
		"sort_key": v.SortKey, "visibility": string(v.Visibility),
		"head_commit_id": v.HeadCommitID,
		"created_at":     v.CreatedAt, "updated_at": v.UpdatedAt,
	}
}

func (d *Deps) handleTree(w http.ResponseWriter, r *http.Request) {
	if err := d.actor(r).Require(permission.DocRead); err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": d.buildTree(r.Context(), d.actor(r), nil, false)})
}

func (d *Deps) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID *string `json:"parent_id"`
		Slug     string  `json:"slug"`
		Title    string  `json:"title"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := d.Docs.CreateDocument(r.Context(), d.actor(r), req.ParentID, req.Slug, req.Title)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"document": documentView(doc)})
}

// handleResolve 按 slug 路径解析文档（契约 §4.2）：返回 document + 渲染结果，
// 前端只读页一次请求即可。任一段不存在/不可见一律 404 掩护。
func (d *Deps) handleResolve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		mapServiceErr(w, store.ErrNotFound)
		return
	}
	doc, err := d.Docs.ResolveByPath(r.Context(), d.actor(r), strings.Split(path, "/"))
	if mapServiceErr(w, err) {
		return
	}
	vis, verr := d.Trees.EffectiveVisibility(r.Context(), doc.ID)
	if mapServiceErr(w, verr) {
		return
	}
	body, _, err := d.Docs.HeadContent(r.Context(), d.actor(r), doc.ID)
	if mapServiceErr(w, err) {
		return
	}
	res, rerr := d.Render(body)
	if rerr != nil {
		slog.Error("渲染失败", "err", rerr)
		writeErr(w, http.StatusInternalServerError, "render error")
		return
	}
	view := documentView(doc)
	view["effective_visibility"] = string(vis)
	writeJSON(w, http.StatusOK, map[string]any{
		"document": view,
		"render":   map[string]any{"html": res.HTML, "title": doc.Title, "toc": res.TOC},
	})
}

func (d *Deps) handleGet(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	doc, err := d.Docs.Get(r.Context(), d.actor(r), id)
	if mapServiceErr(w, err) {
		return
	}
	vis, verr := d.Trees.EffectiveVisibility(r.Context(), id)
	if mapServiceErr(w, verr) {
		return
	}
	view := documentView(doc)
	view["effective_visibility"] = string(vis)
	writeJSON(w, http.StatusOK, map[string]any{"document": view})
}

// handlePatch 支持部分更新；出现 parent_id 键即执行移动（null 表示移回根）。
func (d *Deps) handlePatch(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if !decodeJSON(w, r, &raw) {
		return
	}
	id := pathID(r)
	ctx := r.Context()
	act := d.actor(r)

	if pv, ok := raw["parent_id"]; ok {
		var pid *string
		if string(pv) != "null" {
			if err := json.Unmarshal(pv, &pid); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid parent_id")
				return
			}
		}
		if err := d.Docs.MoveDocument(ctx, act, id, pid); mapServiceErr(w, err) {
			return
		}
	}

	var title, slug *string
	var vis *model.Visibility
	var sortKey *int64
	if v, ok := raw["title"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			title = &s
		}
	}
	if v, ok := raw["slug"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			slug = &s
		}
	}
	if v, ok := raw["visibility"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			mv := model.Visibility(s)
			vis = &mv
		}
	}
	if v, ok := raw["sort_key"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			sortKey = &n
		}
	}
	switch {
	case title != nil && slug != nil:
		if err := d.Docs.RenameDocument(ctx, act, id, slug, title); mapServiceErr(w, err) {
			return
		}
	case title != nil:
		if err := d.Docs.RenameDocument(ctx, act, id, nil, title); mapServiceErr(w, err) {
			return
		}
	case slug != nil:
		if err := d.Docs.RenameDocument(ctx, act, id, slug, nil); mapServiceErr(w, err) {
			return
		}
	}
	if vis != nil {
		if err := d.Docs.SetVisibility(ctx, act, id, *vis); mapServiceErr(w, err) {
			return
		}
	}
	if sortKey != nil {
		if err := d.Docs.SetSortKey(ctx, act, id, *sortKey); mapServiceErr(w, err) {
			return
		}
	}

	doc, err := d.Docs.Get(ctx, act, id)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": documentView(doc)})
}

// handleDeleteDocument 进回收站（契约 §4：软删含子树），204 无内容。
func (d *Deps) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	if err := d.Docs.TrashDocument(r.Context(), d.actor(r), pathID(r)); mapServiceErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleReorder 同层批量重排（契约 §4 C1）：完整兄弟列表语义，204 无内容。
func (d *Deps) handleReorder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID    *string  `json:"parent_id"`
		DocumentIDs []string `json:"document_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := d.Docs.ReorderSiblings(r.Context(), d.actor(r), req.ParentID, req.DocumentIDs); mapServiceErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- 渲染 ----

func (d *Deps) handleRender(w http.ResponseWriter, r *http.Request) {
	doc, err := d.Docs.Get(r.Context(), d.actor(r), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	body, _, err := d.Docs.HeadContent(r.Context(), d.actor(r), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	res, rerr := d.Render(body)
	if rerr != nil {
		slog.Error("渲染失败", "err", rerr)
		writeErr(w, http.StatusInternalServerError, "render error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"html": res.HTML, "title": doc.Title, "toc": res.TOC,
	})
}

// handleExportMarkdown 下载 HEAD 源码（C4）：document.read + 可见性校验。
func (d *Deps) handleExportMarkdown(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	doc, err := d.Docs.Get(r.Context(), d.actor(r), id)
	if mapServiceErr(w, err) {
		return
	}
	body, _, err := d.Docs.HeadContent(r.Context(), d.actor(r), id)
	if mapServiceErr(w, err) {
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", doc.Slug+".md"))
	w.Write([]byte(body))
}

// handlePreview 实时预览渲染。
func (d *Deps) handlePreview(w http.ResponseWriter, r *http.Request) {
	if err := d.actor(r).Require(permission.DocUpdate); err != nil {
		mapServiceErr(w, err)
		return
	}
	var req struct {
		Markdown string `json:"markdown"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, rerr := d.Render(req.Markdown)
	if rerr != nil {
		writeErr(w, http.StatusInternalServerError, "render error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": res.HTML})
}
