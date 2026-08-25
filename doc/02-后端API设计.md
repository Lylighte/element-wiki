# 后端 API 设计

> Base path `/v1`；格式约定沿用 LabProject doc/03。权限列使用权限码，`登录` 表示任意 active 用户。

## 1. 通用约定

认证（二选一，解析为同一 `permission.Actor`）：

```text
Cookie: access_token=<session token>
Authorization: Bearer <api token>
```

JSON 响应 `Content-Type: application/json; charset=utf-8`；错误结构：

```json
{ "detail": "permission denied" }
```

校验类错误附带字段明细：

```json
{ "detail": "validation failed", "fields": { "slug": "仅允许 a-z0-9-" } }
```

时间统一 Unix 毫秒。cursor 分页响应：

```json
{ "items": [], "has_next": false, "next_cursor": null, "page_size": 50 }
```

资源对当前 actor 不可见时返回 404（不区分不存在与无权限）。slug 规则：`[a-z0-9-]+`。

## 2. 认证与会话

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/auth/oidc/status | 公开 | `{enabled, provider_name}` |
| GET | /v1/auth/oidc/login | 公开 | 302 至 IdP；写 state/nonce/PKCE 短效 cookie |
| GET | /v1/auth/oidc/callback | 公开 | 校验后 JIT 建号、发 session cookie、302 回跳 |
| POST | /v1/auth/logout | 登录 | 删除 session，204 |
| GET | /v1/users/me | 登录 | 当前用户 + 权限码列表 |

`GET /v1/users/me` 响应：

```json
{
  "user": {
    "id": "01J8ZK...",
    "email": "dev@example.com",
    "display_name": "Dev",
    "role": "editor",
    "status": "active"
  },
  "permissions": ["document.read", "document.update", "..."]
}
```

JIT 规则（PM-02/03）：`(issuer, subject)` 不存在则建 viewer；email 命中配置 `admin_emails` 则提升 admin（仅首次匹配时）。`status=disabled` 的既有账号登录直接拒绝，不复活。

## 3. 个人 Token（API-03）

| Method | Path | 权限 |
|--------|------|------|
| GET | /v1/tokens | 登录（own） |
| POST | /v1/tokens | 登录（own） |
| DELETE | /v1/tokens/{token_id} | 登录（own） |

`POST /v1/tokens` 请求 `{"name": "ci-script"}`，响应明文 token 仅此一次：

```json
{ "id": "01J8ZT...", "name": "ci-script", "prefix": "ew_abc12", "token": "ew_abc12..." }
```

## 4. 文档树

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/documents/tree | 见下 | 侧栏全量树，按可见性过滤 |
| POST | /v1/documents | document.create | 创建节点（文档或目录性文档） |
| GET | /v1/documents/{id} | document.read | 元数据 + 生效可见性 + HEAD 摘要 |
| PATCH | /v1/documents/{id} | document.update | title/slug/sort_key/visibility/parent_id（移动） |
| DELETE | /v1/documents/{id} | document.delete | 进回收站（含子树） |
| PUT | /v1/documents/reorder | document.update | 同层兄弟批量重排 sort_key |

`GET /v1/documents/tree` 响应节点：

```json
{
  "id": "01J8ZD...",
  "parent_id": null,
  "title": "入门指南",
  "slug": "get-started",
  "sort_key": 100,
  "restricted": false,
  "children": []
}
```

匿名模式开启时，匿名 actor 获得 standard 文档的只读树；关闭则 401。

移动约束：目标 parent 存活且未删除；不允许移入自己的子树（service 校验，违规 422）。

批量重排：`PUT /v1/documents/reorder`，请求体 `{"parent_id": null, "document_ids": ["01J8ZA..."]}`（`parent_id: null` 表示根级）。`document_ids` 必须为该父级下**全部存活兄弟的完整有序列表**，缺员、多余或跨父均 422 附 `fields` 明细；成功按下标写 `sort_key = (i+1)*100`，返回 204。actor 需对列表内全部文档持 `document.update`。

## 5. 草稿与版本

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| PUT | /v1/documents/{id}/draft | document.update | 自动保存 UPSERT |
| GET | /v1/documents/{id}/draft | document.update | 无草稿返回 `{"draft": null}` |
| DELETE | /v1/documents/{id}/draft | document.update | 放弃草稿，204 |
| POST | /v1/documents/{id}/commits | document.update | 提交新版本 |
| GET | /v1/documents/{id}/commits | version.read | 历史 cursor 分页 |
| GET | /v1/documents/{id}/commits/{commit_id}/content | version.read | 该版 Markdown 源码 |
| POST | /v1/documents/{id}/revert | version.revert | 回滚 = 以历史内容新建 commit |
| GET | /v1/documents/{id}/render | document.read | 服务端渲染 HTML + TOC |
| POST | /v1/render-preview | document.update | 编辑器实时预览渲染 |
| GET | /v1/documents/{id}/export.md | document.read | 当前 HEAD 的 Markdown 源码下载，`Content-Disposition: filename="{slug}.md"` |

提交请求与冲突：

```json
// POST /v1/documents/{id}/commits
{ "base_commit_id": "01J8ZC...", "content": "# 标题\n...", "message": "fix typo", "title": "入门指南" }
```

`title` 可选：出现时与正文在**同一事务**内校验并更新 `documents.title`（校验规则同创建），缺省不动标题；冲突判定仍以 `base_commit_id` 先行，409 时不写标题。

```json
// base_commit_id != 当前 HEAD → 409 Conflict
{ "detail": "version conflict", "head_commit_id": "01J8ZQ...", "base_commit_id": "01J8ZC..." }
```

成功响应携带保存期死链报告（不入库，读时解析）：

```json
{
  "commit": { "id": "01J8ZR...", "commit_no": 42, "created_at": 1756000000000 },
  "dead_links": [{ "target": "[[old-note]]", "reason": "not found" }]
}
```

提交副作用顺序（AGENTS §9）：事务落 commits/blob/HEAD/裁剪旧版 → 同步更新 Bleve → 索引失败则插入 `search_reindex_jobs` 并照常返回 201（派生数据降级）。

## 6. 回收站（DM-08）

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/trash | document.delete | 已删文档 cursor 分页 |
| POST | /v1/trash/{id}/restore | document.restore | 恢复子树；父链已删除时需 body 指定新 parent_id，否则 409 |
| DELETE | /v1/trash/{id} | document.delete | 彻底清除子树（commits 级联、blob 待 GC），204 |

后台任务按 `purge_at` 自动彻底清除。

## 7. 搜索（SE）

```text
GET /v1/search?q=goldmark+"exact phrase"&cursor=
```

响应：

```json
{
  "items": [
    {
      "document_id": "01J8ZD...",
      "title": "渲染管线",
      "snippet": "...基于 <mark>goldmark</mark> 的扩展...",
      "score": 3.41,
      "updated_at": 1756000000000
    }
 ],
  "has_next": false, "next_cursor": null, "page_size": 20
}
```

语法一期仅关键词 + `"精确短语"`。service 层先按 actor 可见文档集过滤查询，返回前逐条二次校验（AGENTS 禁忌：不得查全库再前端过滤）。

## 8. 附件

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/documents/{id}/attachments | attachment.read | 列表 |
| POST | /v1/documents/{id}/attachments | attachment.upload | multipart 单文件；白名单外 415、超限 413 |
| GET | /v1/attachments/{id}/raw | attachment.read | 文件流，Content-Disposition 原名 |
| DELETE | /v1/attachments/{id} | attachment.delete | 204 |

上传失败路径必须断言磁盘无孤儿文件（AGENTS §7）。

## 9. 评论

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/documents/{id}/comments | comment.read | 按 created_at 升序分页 |
| POST | /v1/documents/{id}/comments | comment.create | Markdown 内容；@提及写入 mention 表 |
| DELETE | /v1/comments/{id} | comment.delete.own / .any | 作者本人或 admin |

评论条目响应包含作者快照（display_name）与提及用户 id 列表。

**全局开关**：设置 `comments_enabled` 默认 `false`。禁用期间本节全部接口返回 403 `{"detail": "comments disabled"}`，前端据此整体隐藏评论区；开启后行为不变。

## 10. 管理

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/admin/settings | settings.manage | 全部设置键值 |
| PATCH | /v1/admin/settings | settings.manage | 部分更新 |
| GET | /v1/admin/users | user.list | 支持 `q=` 过滤 email/name |
| PATCH | /v1/admin/users/{user_id} | user.manage | `{role?}` 或 `{status?}`；不可操作自己 |
| GET | /v1/admin/dashboard | dashboard.read | 文档总数/最近更新/活跃贡献者 |
| POST | /v1/admin/search/rebuild | search.rebuild | 202 `{job_id}`（全量重建任务） |

PATCH 设置采用逐键校验、任一失败整体拒绝（零写入）；成功后新值对运行时**即时生效**（服务层每次读取设置而非启动期快照），无需重启。

## 11. 备份与导入（异步 job 模式）

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| POST | /v1/admin/backups | backup.manage | 发起导出，202 `{job_id}` |
| GET | /v1/admin/backups/jobs/{job_id} | backup.manage | job 状态 |
| GET | /v1/admin/backups/files | backup.manage | 备份产物列表 |
| GET | /v1/admin/backups/files/{filename}/download | backup.manage | 文件流 |
| DELETE | /v1/admin/backups/files/{filename} | backup.manage | 删除产物 |
| POST | /v1/admin/imports | import.run | multipart zip，202 `{job_id}` |
| GET | /v1/admin/imports/jobs/{job_id} | import.run | 进度：total/imported/failed |
| POST | /v1/admin/markdown-import | import.run | multipart zip（Markdown 目录包），202 `{job_id}`；进度查询复用上一行 imports jobs 端点 |

导入规则（OP-04）：zip 内目录即文档树，`README.md` 与其余 `.md` 一律按路径生成 slug 链；同名图片等文件作为对应文档附件。manifest 缺失/schema 不符/路径穿越 → 整体失败且零残留。

备份 zip 结构：`manifest.json`（schema_version、创建时间、计数）+ `db.sqlite3` + `attachments/`。导入前校验 manifest 与目标库 schema_version 兼容性；**manifest 缺失即整体失败**（不允许无 manifest 导入）。导入成功后必须自动入队一次全量搜索索引重建，保证 Bleve 与恢复后数据一致。

`driver=postgres` 时备份导出与两种导入均暂不支持：受理前直接返回 501 `{"detail": "backup not supported for postgres"}`，不创建 job、不产生文件。

## 12. 公共路由

```text
GET /healthz        探活，公开
GET /sitemap.xml    匿名可访问；仅收录匿名模式下可见的 standard 文档
GET /v1/site        公开站点信息，登录与否均可访问
```

`GET /v1/site` 响应（值来自运行时设置，供前端首屏决定 UI 形态与语言兜底）：

```json
{ "title": "Element Wiki", "default_lang": "zh-CN", "anonymous_read": true, "comments_enabled": true }
```

## 13. 权限码目录（PM-04）

新增权限必须同步更新：本目录、内置角色映射、catalog 测试、前端页面权限配置。

| 权限码 | 含义 | viewer | editor | admin |
|--------|------|:------:|:------:|:-----:|
| document.read | 读 standard 文档 | ✓ | ✓ | ✓ |
| document.read.restricted | 读 restricted 文档 | ✗ | ✓ | ✓ |
| document.create | 新建文档 | ✗ | ✓ | ✓ |
| document.update | 编辑/移动/改可见性/存草稿/提交 | ✗ | ✓ | ✓ |
| document.delete | 移入回收站/彻底删除 | ✗ | ✓ | ✓ |
| document.restore | 从回收站恢复 | ✗ | ✓ | ✓ |
| version.read | 读历史 | ✓ | ✓ | ✓ |
| version.revert | 回滚 | ✗ | ✓ | ✓ |
| attachment.read | 读附件 | ✓ | ✓ | ✓ |
| attachment.upload | 上传附件 | ✗ | ✓ | ✓ |
| attachment.delete | 删除附件 | ✗ | ✓ | ✓ |
| comment.read | 读评论 | ✓ | ✓ | ✓ |
| comment.create | 发评论 | ✓ | ✓ | ✓ |
| comment.delete.own | 删自己评论 | ✓ | ✓ | ✓ |
| comment.delete.any | 删任意评论 | ✗ | ✗ | ✓ |
| user.list / user.manage | 用户管理 | ✗ | ✗ | ✓ |
| settings.manage | 站点设置 | ✗ | ✗ | ✓ |
| dashboard.read | 仪表盘 | ✗ | ✗ | ✓ |
| backup.manage / import.run | 备份与导入 | ✗ | ✗ | ✓ |
| search.rebuild | 手动重建索引 | ✗ | ✗ | ✓ |
| token.manage.own | 管理个人 Token | ✓ | ✓ | ✓ |

匿名 actor（匿名模式开启时）仅持有 `document.read / version.read / attachment.read / comment.read` 且作用于 standard 文档。

禁止写法（AGENTS §4）：`if role == "admin"`；必须 `actor.Require("settings.manage")`。

## 14. 错误语义

| 状态码 | 场景 |
|--------|------|
| 401 | 未认证（匿名模式关闭时的所有请求） |
| 403 | 已认证但无对应权限码 / disabled 账号 |
| 404 | 资源不存在**或**对当前 actor 不可见 |
| 409 | 版本冲突（base≠HEAD）、slug 重复、回收站恢复父链缺失 |
| 413 | 上传超过 upload_max_mb |
| 415 | 扩展名/MIME 不在白名单 |
| 422 | 字段校验失败（含移动进自身子树、非法 slug 等），附 fields 明细 |
| 202 | 异步 job 受理（备份/导入/索引重建） |
| 501 | 功能对该部署形态不可用（如 postgres 驱动下的备份导出/导入） |

业务错误禁止用 500 掩盖；500 仅保留给未预期故障并触发告警日志。
