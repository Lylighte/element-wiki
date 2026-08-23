# 编码规范

> 本文件是 Element Wiki 的长期工程规范，opencode 在本项目根目录自动加载。
> `doc/00-需求手册.md`、`doc/01-数据库表设计.md`、`doc/02-后端API设计.md` 为**冻结契约**：实现必须与其一致；任何偏离（加表/加列/改端点语义/加权限码）必须停下请求人工批准，批准后先修改契约文档再修改代码。
> 进度唯一事实来源是 `doc/ROADMAP.md`，完成任务的最后一步是勾选对应条目。

## 1. 通用原则

- 代码结构层次清晰、高内聚、低耦合。
- 不为同一功能保留多套实现；删除旧路径后再让新路径成为唯一事实来源。
- 业务数据必须结构化建模，不用 JSON 字符串逃避表设计。
- Handler 不写复杂业务流程；业务规则、权限判断、跨存储一致性放在 service 层。
- 数据库层不得做权限判断，不返回 HTTP 语义错误。
- 错误路径必须验证不会污染 SQLite、PostgreSQL、Bleve 或文件系统。
- 重构以减少真实复杂度、重复逻辑或职责混乱为目标，不做无行为收益的风格性搬运。

## 2. 技术栈与目录

技术栈：

```text
后端: Go / net/http / modernc.org/sqlite(默认) / pgx(PostgreSQL 可选) / Bleve
前端: Vue 3 / TypeScript / Vite / Element Plus / Tailwind CSS
```

后端目录职责：

```text
cmd/wikid/            入口
internal/httpapi/     HTTP 请求解析、认证包装、响应
internal/service/     业务规则、权限判断、跨存储协调
internal/store/       存储接口定义 + sqlite / pg 两个实现
internal/search/      Bleve 封装与重建 worker
internal/permission/  权限 catalog、actor
internal/render/      Markdown 渲染管线
internal/model        跨层模型
internal/util         通用工具
migrations/           嵌入式 SQL 迁移（按方言分目录）
```

前端目录职责：

```text
src/api                 API wrapper
src/api/__tests__       API wrapper 测试
src/components/ui       通用 UI 封装
src/components/common   通用业务组件
src/components/doc      文档业务组件
src/components/admin    管理后台组件
src/composables         可复用状态和流程
src/permissions         页面权限配置
src/views               页面级视图
```

前端格式：`<script setup lang="ts">`；不使用分号；单引号；`printWidth` 100；前后端字段一律 snake_case。
组件不得直接创建 axios 实例，必须走 `src/api/client.ts`；页面访问控制使用权限码，不使用角色名；大型编辑器等重依赖按场景懒加载，只读页面不得加载编辑器代码。

## 3. 分层规范

Handler：

- 解析 path、query、JSON、multipart。
- 调用 Auth 包装获得 actor，调用 service，返回 JSON 或文件流。
- 不直接编排多个数据库写入，不直接做业务权限判断。

Service：

- 所有受保护方法必须接收 `permission.Actor`。
- 先读取业务资源，再做权限判断；不可见资源返回 404（不区分不存在与无权限）。
- 负责 DB、Bleve、文件系统之间的一致性；跨存储写入必须有明确提交顺序和失败补偿策略。
- 外部副作用失败不得静默吞掉，除非该副作用是派生数据（如搜索索引）且降级行为有测试覆盖。

Store：

- 方法第一个参数为 `context.Context`。
- 事务使用 `Begin`、`defer tx.Rollback(ctx)`、成功后 `tx.Commit(ctx)`。
- 返回存储语义错误，不返回 HTTP 语义错误。
- **双方言约束**：SQLite 与 PostgreSQL 共用同一套表结构（doc/01），SQL 只使用双方言共同支持的特性（部分索引、表达式索引、递归 CTE 允许）；方言差异（如 UPSERT 语法）在各 store 实现内部隔离。
- 时间统一 Unix 毫秒 BIGINT；主键 TEXT ULID。

## 4. 权限编码规范

权限码格式 `resource.action.scope`，权威目录见 `doc/02-后端API设计.md` §13。

新增权限必须同时更新四处：permission catalog、内置角色映射、catalog 测试、前端 `src/permissions` 配置。

禁止：

```go
if actor.Role == "admin" {}
```

允许：

```go
actor.Require("document.update")
actor.HasAny("comment.delete.own", "comment.delete.any")
```

前端可以展示角色标签，但访问控制必须基于权限码或后端返回的资源访问级别。

## 5. 认证规范

- 登录方式仅有 OIDC（授权码流 + PKCE + nonce）。**永远不要引入本地密码体系**，出现任何密码哈希依赖即视为违约。
- JIT 建号：`(issuer, subject)` 不存在则创建 viewer。
- `oidc.admin_emails` 引导仅在首次邮箱匹配登录时提升 admin，此后不再自动提权。
- `status='disabled'` 的账号在认证中间件直接拒绝；禁用账号重新登录不得复活为新账号。
- 明文 API token 仅在签发响应中出现一次，库中只存 SHA-256 与前 8 位 prefix。
- session cookie 属性遵循配置：HttpOnly 恒开，Secure 由 `secure_cookies` 控制。

## 6. 文件存储规范

- 附件上传先写临时文件，校验通过后进入受控提交流程；最终目录按文档 id 隔离。
- 数据库写入失败必须清理本次写入的临时文件和最终文件。
- 失败路径测试必须断言磁盘上没有本次操作留下的孤儿文件。

## 7. API 规范

- 站点 API 在 `/v1` 下；JSON 请求 `application/json`；上传 multipart/form-data；成功无内容返回 204。
- 业务错误返回明确状态码，不用 500 掩盖；错误结构 `{"detail": "..."}"`，校验失败附 `fields` 明细。
- 评论全局门闩：`comments_enabled=false`（默认值）时所有评论端点返回 403 `{"detail": "comments disabled"}`。
- 响应结构必须稳定；测试精确断言 method、path、query、body、status 和关键字段。

## 8. 搜索与备份规范

- Bleve 为派生数据，事实来源是数据库；索引目录 `storage/search/documents.bleve/`，测试一律用 `t.TempDir()`。
- commit 成功后同步更新索引；更新失败写入 `search_reindex_jobs` 并照常返回 201（派生数据降级，必须有测试覆盖）。
- 搜索权限过滤在 service 层完成，结果返回前逐条二次校验。
- 备份导出为 zip（manifest + db + attachments）；导入前必须校验 manifest、schema_version、路径安全，任何失败不得留下半导入数据、半写文件或错误索引任务。
- 备份/导入/索引重建均为异步 job：受理 202，状态可查询。

## 9. 测试规范

- 后端总覆盖率 ≥85%，以 `go test ./... -coverprofile=coverage.out` 与 `go tool cover -func` 的 total 为准；coverage profile 是临时产物，不得提交。
- store 测试使用临时 SQLite 文件库；PostgreSQL 适配器里程碑启用前不需要 PG 测试库，启用后通过 `TEST_DATABASE_URL` 指向库名含 `test` 的隔离库。
- OIDC 集成测试使用本地 stub IdP，不访问外网。
- 文件存储、Bleve 索引、备份目录必须使用 `t.TempDir()`。
- 不允许弱测试；能断言的字段必须断言。表驱动测试每个 case 明确命名并断言差异点。
- 重要错误路径必须断言操作前后事实来源没有被污染。
- 必须覆盖的矩阵：未认证、无权限、restricted 文档 viewer 拒绝、editor 通过、admin 通过、错误路径无污染。

前端测试：

- API wrapper 测试精确断言 method、path、params、body。
- 编辑页测试自动保存、提交、409 冲突提示、离开确认。
- `comments_enabled=false` 时评论区不渲染。

## 10. 提交规范

Commit message 使用英文，一次提交一个意图：

```text
feat: add document commit service
fix: release slug after moving doc to trash
test: cover version conflict zero-pollution path
docs: extend permission catalog
```

任务完成的勾选变更（ROADMAP）包含在该任务的收尾 commit 中。

## 11. 禁忌

- 不在组件中直接调用 axios。
- 不在 handler 中写复杂业务。
- 不在 store 中判断权限。
- 不把结构化业务数据塞进 JSON 字符串。
- 不通过角色名判断管理员。
- 不让搜索先查全库再在前端过滤权限。
- 不把 Bleve 当作权限系统。
- 不在备份/导入失败后留下半成品。
- 不用低覆盖率集成测试冒充精准单元测试。
- 不引入本地密码体系。
- 不绕过 `comments_enabled` 门闩。
- 匿名模式下不得泄漏 restricted 文档的存在性（一律 404 掩护）。
- 不修改冻结契约，除非当前任务已获人工批准。
