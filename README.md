# Element Wiki

Element Wiki 是一个自托管的团队/个人知识库：Go 后端、Vue 3 前端、SQLite 默认存储、Bleve 全文搜索，并且只通过一个 OIDC Provider 进行登录。

## 特性

- 文档树、Markdown 编辑、版本提交、草稿和历史版本
- 文档可见性、权限码和管理员后台
- 中文全文搜索
- 附件、评论、回收站和备份/导入任务
- OIDC 授权码流登录，包含 PKCE、state、nonce 和 JIT 建号
- SQLite 零外部依赖起步，也支持 PostgreSQL

## 快速开始

要求：Go 1.26.7 或更高版本、Node.js 18 或更高版本。

### 1. 准备配置

在项目根目录执行：

```bash
cp config.yaml.example config.yaml
```

`config.yaml` 与 `go.mod` 位于同一级目录。它不会被提交到 Git；其中的 `oidc.client_secret` 只能填写本地或部署环境的真实密钥。

默认示例将 OIDC 保持为关闭状态，便于先检查页面和 API。要启用登录，至少修改：

```yaml
oidc:
  enabled: true
  issuer: https://sso.example.com
  client_id: element-wiki
  client_secret: your-client-secret
  redirect_uri: https://wiki.example.com/v1/auth/oidc/callback
```

在 OIDC Provider 中注册完全相同的 `redirect_uri`。项目只使用这一组 OIDC 配置，不提供本地密码登录，也不提供多 Provider 选择。

开发模式下，OIDC 回调由后端 `8080` 接收，登录成功后通过 `server.frontend_url` 返回前端 `5175`。生产环境将它改为实际的前端 HTTPS 地址。浏览器访问使用 `localhost` 还是 `127.0.0.1` 时，`frontend_url` 也必须使用相同主机名，保证 session cookie 生效。

### 2. 启动后端

```bash
go run ./cmd/wikid
```

默认地址为 `http://127.0.0.1:8080`。后端启动时会自动创建数据库迁移、搜索索引、附件和备份目录。

检查服务：

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/v1/auth/oidc/status
```

本地 HTTP 开发时，配置中的 `server.secure_cookies` 必须为 `false`；生产 HTTPS 应设为 `true`。

也可以指定配置文件：

```bash
go run ./cmd/wikid -configfile /path/to/config.yaml
```

配置读取顺序为：内置默认值、`config.yaml`、环境变量。也可以通过 `CONFIG_FILE` 指定配置路径。

### 3. 启动前端开发服务器

另开一个终端：

```bash
cd frontend
npm ci
npm run dev
```

开发前端地址为 `http://127.0.0.1:5175`。Vite 会将 `/v1` 和 `/healthz` 请求代理到后端 `http://127.0.0.1:8080`。

生产构建：

```bash
cd frontend
npm run build
npm run preview
```

当前后端提供 API，前端开发服务器负责页面资源；两者需要分别启动。

## OIDC 配置

完整模板见 [`config.yaml.example`](config.yaml.example)。登录流程使用单一 `oidc.issuer`：

1. 用户访问前端登录页。
2. 后端跳转到配置的 OIDC Provider。
3. Provider 回调 `/v1/auth/oidc/callback`。
4. 后端按 `(issuer, subject)` 查找或创建用户，并建立 session。

首次登录邮箱命中 `oidc.admin_emails` 时提升为管理员。OIDC Provider 必须允许配置的 `redirect_uri`，并返回 `openid`、`profile`、`email` 所需声明。

查看 OIDC 是否启用：

```bash
curl http://127.0.0.1:8080/v1/auth/oidc/status
```

应返回类似：

```json
{"enabled":true,"provider_name":"Example SSO"}
```

## 常用环境变量

环境变量会覆盖 YAML 配置，适合容器或部署环境：

```bash
WIKI_SERVER_HTTP_ADDR=0.0.0.0:8080
WIKI_SERVER_SECURE_COOKIES=true
WIKI_SERVER_FRONTEND_URL=https://wiki.example.com
WIKI_DATABASE_DRIVER=sqlite
WIKI_DATABASE_URL=data/element-wiki.db
WIKI_STORAGE_DIR=storage
WIKI_SEARCH_INDEX_DIR=storage/search/documents.bleve
WIKI_OIDC_ENABLED=true
WIKI_OIDC_ISSUER=https://sso.example.com
WIKI_OIDC_CLIENT_ID=element-wiki
WIKI_OIDC_CLIENT_SECRET=...
WIKI_OIDC_ADMIN_EMAILS=admin@example.com,owner@example.com
WIKI_WIKI_ANONYMOUS_READ=false
WIKI_WIKI_COMMENTS_ENABLED=false
```

涉及密钥的环境变量不要写入 shell 历史、日志或仓库文件。

## 验证与测试

后端：

```bash
go test ./...
go vet ./...
```

前端：

```bash
cd frontend
npm test -- --run
npm run build
```

## 目录说明

```text
cmd/wikid/             后端入口
internal/httpapi/      HTTP 路由、认证包装和响应
internal/service/      业务规则和权限判断
internal/store/        存储接口及 SQLite 实现
internal/search/       Bleve 搜索索引与重建 worker
internal/render/       Markdown 渲染
migrations/             SQLite/PostgreSQL 数据库迁移
frontend/src/api/       前端 API wrapper
frontend/src/views/     前端页面
frontend/src/components/前端组件
doc/                    需求、数据库、API 和路线图
```

运行期数据默认位于 `data/` 和 `storage/`，不要把真实数据库、附件、搜索索引或备份提交到 Git。

## 常见问题

### 页面空白

确认后端和前端都已启动，并检查浏览器 Network 面板中的 `/v1/site`、`/v1/users/me` 和 `/v1/documents/tree` 请求。修改 Vue 代码后重启 `npm run dev` 或强制刷新页面。

### 登录按钮不可用

检查：

```bash
curl http://127.0.0.1:8080/v1/auth/oidc/status
```

如果 `enabled` 为 `false`，检查是否从包含 `config.yaml` 的项目根目录启动，以及 `oidc.enabled`、`issuer`、`client_id` 是否填写正确。

### 回调失败

确认 Provider 中登记的回调地址与 `oidc.redirect_uri` 完全一致，包括协议、端口、路径和尾部斜杠。生产环境还要确认 `secure_cookies: true` 与 HTTPS 配套。

### API 返回 401

这是未登录或 session cookie 未发送的结果。开发环境使用 HTTP 时将 `server.secure_cookies` 设为 `false`；生产环境不要关闭安全 Cookie，而应使用 HTTPS。

## 文档

```text
doc/00-需求手册.md      需求基线
doc/01-数据库表设计.md   SQLite/PostgreSQL schema
doc/02-后端API设计.md    /v1 REST 契约与权限码目录
doc/03-页面导航改造计划.md 页面导航问题、实施顺序与验收标准
```

## License

MIT
