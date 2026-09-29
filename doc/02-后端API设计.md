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
| GET | /v1/users/me/preferences | 登录 | 当前用户偏好；无显式值时返回 null |
| PUT | /v1/users/me/preferences | 登录 | 保存 `language` 与 `theme`，覆盖字段全量提交 |
| GET | /v1/users/{user_id}/page | 公开/登录 | 个人页；匿名读取受 `anonymous_read` 控制，登录读取要求启用个人页；本人和管理员可查看待审版本状态 |
| PUT | /v1/users/me/page | 登录 | 新建或更新本人 Markdown 个人页；管理员也可管理本人页 |
| PUT | /v1/admin/users/{user_id}/page | user.manage | 管理员代用户编辑个人页 |
| DELETE | /v1/users/me/page | 登录 | 删除本人页面及其修订 |
| POST | /v1/users/{user_id}/page/revisions/{revision_id}/approve | review.manage | 审核通过；提交者不能审核自己的内容 |
| POST | /v1/users/{user_id}/page/revisions/{revision_id}/reject | review.manage | 退回并记录理由 |
| GET | /v1/admin/reviews/user-pages | review.manage | 待审个人页列表 |
| GET | /v1/admin/reviews/comments | review.manage | 待审评论列表 |
| POST | /v1/admin/reviews/comments/{id}/approve | review.manage | 发布评论 |
| POST | /v1/admin/reviews/comments/{id}/reject | review.manage | 退回评论并记录理由 |
| GET | /v1/users/{user_id}/page/revisions | 公开/登录 | 已发布个人页历史；本人/审核员另可见待审与退回修订 |
| GET | /v1/users/me/page/revisions | 登录 | 本人个人页全部修订及审核理由 |
| POST | /v1/documents/{id}/commits | document.update | 开启文档审核时返回 202 和 submission_id，正文仍不发布 |
| GET | /v1/admin/reviews/documents | review.manage | 待审文档修订 |
| POST | /v1/admin/reviews/documents/{id}/approve | review.manage | 基线仍匹配时发布修订；过期返回 409；不能自审 |
| POST | /v1/admin/reviews/documents/{id}/reject | review.manage | 退回并保留理由 |
| POST | /v1/reports | report.create | 登录用户举报文档、评论或个人页；待审重复举报受唯一约束 |
| GET | /v1/admin/reviews/reports | review.manage | 待处理举报，含服务端消毒渲染的文档/个人页摘要或评论文本 |
| POST | /v1/admin/reviews/reports/{id}/resolve | review.manage | 标记已处理并写处理意见 |
| POST | /v1/admin/reviews/reports/{id}/dismiss | review.manage | 标记不予处理并写处理意见 |
| GET | /v1/admin/moderation/hidden | review.manage | 已下架内容队列 |
| POST | /v1/admin/moderation/{content_type}/{content_id}/unpublish | review.manage | 下架文档、评论或个人页，保存先前发布状态并记录理由 |
| POST | /v1/admin/moderation/{content_type}/{content_id}/restore | review.manage | 恢复原发布状态并记录理由 |

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

`POST /v1/tokens` 请求 `{"name": "ci-script", "expires_in_days": 90}`。`expires_in_days` 仅允许 30、90、365 天；新令牌默认 90 天。响应明文 token 仅此一次：

```json
{ "id": "01J8ZT...", "name": "ci-script", "prefix": "ew_abc12", "token": "ew_abc12...", "expires_at": 1756000000000 }
```

列表返回 `expires_at`；到期 bearer token 与吊销 token 一样按未认证处理。旧 token 的 `expires_at=null` 保持有效直至主动吊销。

### 3.1 用户偏好

`PUT /v1/users/me/preferences` 请求 `{"language":"zh-CN","theme":"system"}`。语言枚举为 `zh-CN|en`，主题枚举为 `light|dark|system`。未保存偏好响应字段为 null；登录后前端使用用户偏好覆盖本地缓存，匿名用户继续使用本地缓存。邮箱、显示名和角色仍来自 OIDC/管理员，不能通过此端点修改。

### 3.2 个人页与审核

个人页地址为 `/users/{user_id}`，不进入文档树、不支持子页。匿名可见范围遵循 `anonymous_read`；个人页全局开关 `user_pages_enabled` 默认关闭。更新接口只接受 Markdown 源码并限制为 20,000 个 Unicode 字符，使用与文档相同的安全渲染规则。

`user_pages_review_required=true` 时，新建内容和更新内容进入 pending；既有 published 版本继续公开，待审内容仅本人和审核员可见。审核通过将 pending 修订提升为 published；退回保留理由给作者查看。审核者不得审批自己提交的修订。关闭审核后作者提交直接发布。管理后台的统一审核队列还处理文档、评论和举报，操作记录包含对象、修订、动作、操作者、时间与理由；紧急下架不删除历史修订。

`document_review_required=true` 时，普通编辑者的 commit 返回 `202 Accepted`，修订单独进入待审队列，不覆盖已发布 HEAD；审核时再次校验提交基线，HEAD 已变化则 `409`，防止旧稿覆盖新版本。仅 review.manage 可处理，提交者不可自审。举报要求登录，理由 5–2000 字；同一用户对同一内容最多有一条待审举报。举报结案需要管理员填写处理意见。

部署预设 `internal|public_readonly|public_contributions` 只向管理员页面填充建议开关，不会自动保存；应用后各开关仍可单独编辑。内置建议：内部知识库为匿名/评论/用户页/审核全关；公开只读开启匿名阅读，关闭评论和用户页；公开投稿开启匿名阅读与用户页，建议开启文档/评论/个人页审核。

## 4. 文档树

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | /v1/documents/tree | 见下 | 侧栏全量树，按可见性过滤 |
| POST | /v1/documents | document.create | 创建节点（文档或目录性文档）；slug 可选，缺省按标题自动生成 |
| GET | /v1/documents/{id} | document.read | 元数据 + 生效可见性 + HEAD 摘要 |
| GET | /v1/documents/resolve?path= | document.read | 按 slug 路径解析文档（见下） |
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

### 4.1 创建文档

`POST /v1/documents` 请求体 `{"parent_id": null, "slug": "get-started", "title": "入门指南"}`：

- `slug` **可选**：缺省由服务端按标题自动生成——拉丁/数字净化（小写、非 `[a-z0-9-]` 剔除并按空格/连字符折叠为 kebab-case）；净化结果为空（纯 CJK 等）则 `doc-<ULID 前 8 位>`；生成结果与父级内既有 slug 冲突则追加 `-2`、`-3`…（上限 20 次），仍冲突返回 409。
- `slug` 显式指定：校验规则不变（`[a-z0-9-]+`，长度 1-80），冲突返回 409。
- 创建响应携带实际落库的 `slug`，前端据此回显。

### 4.2 按路径解析

`GET /v1/documents/resolve?path=<slug路径>`：`path` 为 `/` 分隔的 slug 路径（如 `guide/setup`），从根逐段解析；`path` 必填、逐段校验 slug 合法性；任一段不存在或对当前 actor 不可见一律返回 404（不区分不存在与无权限，含匿名模式）。响应：

```json
{ "document": { "id": "01J8ZD...", "parent_id": null, "title": "入门指南", "slug": "setup", "visibility": "standard" } }
```

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

成功响应携带保存期死链报告（不入库，读时解析）。死链判定：`[[目标]]` 目标按 slug 路径解析（`/` 分隔，单段即根级），与 `GET /v1/documents/resolve` 同一路径下钻语义，仅做存活存在性检查（无权限过滤）；任一段不存在即判定死链。

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
| POST | /v1/trash/{id}/restore | document.restore | 恢复子树到「已恢复」容器（M18/C9），204；无请求体语义 |
| DELETE | /v1/trash/{id} | document.delete | 彻底清除子树（commits 级联、blob 待 GC），204 |

恢复落位（M18/C9）：恢复一律落到根级「已恢复」容器（`slug=restored`、title「已恢复」，普通文档属性，visibility=restricted——恢复内容对 viewer/匿名不可见（404 掩护），移出容器后按新父级生效）。容器缺失（被改名/回收）时惰性重建；不检查祖先链，父链被彻底删除亦可恢复。容器内 slug 冲突（如同名文档曾被恢复）→ 原地自增 `-2/-3…`（上限 20，仍冲突 409）——回收站行不参与部分唯一索引，可安全改写。子树内部结构随恢复保留；恢复后 `purge_at` 清空。

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
| POST | /v1/admin/site/icon | settings.manage | multipart `file`，上传 PNG/ICO/WebP 图标；大小受 `upload_max_mb` 限制，返回公开图标 URL |
| GET | /v1/admin/users | user.list | 支持 `q=` 过滤 email/name |
| PATCH | /v1/admin/users/{user_id} | user.manage | `{role?}` 或 `{status?}`；不可操作自己 |
| GET | /v1/admin/dashboard | dashboard.read | 文档总数/最近更新/活跃贡献者 |
| POST | /v1/admin/search/rebuild | search.rebuild | 202 `{job_id}`（全量重建任务） |

PATCH 设置采用逐键校验、任一失败整体拒绝（零写入）；成功后新值对运行时**即时生效**（服务层每次读取设置而非启动期快照），无需重启。

站点品牌设置键：`site_icon_url`（空值使用默认图标；HTTP(S) 外链由浏览器直接加载；本地上传由 `POST /v1/admin/site/icon` 保存并写入本站公开 URL）、`theme_preset`（首版仅 `blue`）、浅色与深色分别配置的 `theme_{light,dark}_{primary,accent,focus}`（`#RRGGBB`）、`article_footer_markdown`、`sidebar_footer_markdown`。站点图标上传格式限 PNG/ICO/WebP，大小上限实时复用 `upload_max_mb`；文件存入附件根目录 `_site-icons/` 并随附件备份。Markdown 附加文案复用安全渲染规则，原始 HTML 不渲染，链接仅 HTTP(S)。

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

导入规则（OP-04，M17 隔离根语义）：全部内容导入到**全新隔离根** `import-<短ID>`（title 取 zip 文件名），与站点既有文档零交集；zip 内目录即文档树，`README.md`（取排序首个变体）为其所在目录容器的正文；slug 取文件名净化（拉丁/数字，结果为空或含非法字符——如纯 CJK——传空由服务端按标题自动生成）；zip 自身 slug 冲突（含大小写变体）一律**计失败，绝不覆盖**任何既有内容；图片等非 md 文件提取为同 stem 文档（回退目录容器）的附件，**正文相对路径引用不重写**（渲染为死链，附件本体可经附件面板取用；引用重写为 backfill）。全失败（0 成功）→ 隔离根自动入回收站（零残留）；部分失败保留已导入部分并由 job 计数。空 zip/全部条目非法 → job 失败。

备份 zip 结构：`manifest.json`（schema_version、创建时间、计数）+ `db.sqlite3` + `attachments/`。导入前校验 manifest 与目标库 schema_version 兼容性；**manifest 缺失即整体失败**（不允许无 manifest 导入）。导入成功后必须自动入队一次全量搜索索引重建，保证 Bleve 与恢复后数据一致。

`driver=postgres` 时备份导出与两种导入均暂不支持：受理前直接返回 501 `{"detail": "backup not supported for postgres"}`，不创建 job、不产生文件。

## 12. 公共路由

```text
GET /healthz        探活，公开
GET /sitemap.xml    匿名可访问；仅收录匿名模式下可见的 standard 文档，URL 为 slug 路径形态
GET /v1/site        公开站点信息，登录与否均可访问；附加文案以安全渲染后的 HTML 返回
GET /v1/site/icon/{filename} 公开读取已上传站点图标，仅接受生成的文件名
```

`GET /v1/site` 响应（值来自运行时设置，供前端首屏决定 UI 形态、语言兜底和日期展示；`article_footer_html` 与 `sidebar_footer_html` 由服务端安全 Markdown 渲染器生成）：

```json
{ "title": "Element Wiki", "default_lang": "zh-CN", "timezone": "Asia/Shanghai", "anonymous_read": true, "comments_enabled": true, "user_pages_enabled": false, "user_pages_review_required": false, "document_review_required": false, "comment_review_required": false, "deployment_preset": "internal", "site_icon_url": "", "theme_preset": "blue", "theme_light_primary": "#2563EB", "theme_light_accent": "#DBEAFE", "theme_light_focus": "#2563EB", "theme_dark_primary": "#60A5FA", "theme_dark_accent": "#1E3A5F", "theme_dark_focus": "#93C5FD", "article_footer_html": "", "sidebar_footer_html": "" }
```

`timezone` 为全站日期展示使用的 IANA 时区。管理员在线修改优先于配置文件；数据库中尚未被管理员修改的时区种子值采用配置文件默认值。时间戳仍以 Unix 毫秒存储和传输。

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
| user.page.manage.own | 管理本人用户页 | ✓ | ✓ | ✓ |
| review.manage | 处理审核队列与举报 | ✗ | ✗ | ✓ |
| report.create | 举报内容 | ✓ | ✓ | ✓ |
| report.create | 举报内容 | ✓ | ✓ | ✓ |
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
| 409 | 版本冲突（base≠HEAD）、slug 重复、恢复 slug 自增耗尽 |
| 413 | 上传超过 upload_max_mb |
| 415 | 扩展名/MIME 不在白名单 |
| 422 | 字段校验失败（含移动进自身子树、非法 slug 等），附 fields 明细 |
| 202 | 异步 job 受理（备份/导入/索引重建） |
| 501 | 功能对该部署形态不可用（如 postgres 驱动下的备份导出/导入） |

业务错误禁止用 500 掩盖；500 仅保留给未预期故障并触发告警日志。
