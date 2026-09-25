# Element Wiki

**更轻的 MediaWiki**：承载 wiki 的精神——页面互链、版本历史、权限可见性、多用户协作——但以纯 Markdown 为唯一内容格式。Go API 以单二进制 + SQLite 运行；Vue 3 前端需另行构建和提供静态资源。

## 定位与约束

红线（不可妥协）：

- **纯 Markdown**：内容源永远是纯 Markdown 文本；编辑器为源码 + 预览分栏，不做富文本/所见即所得。
- **仅 OIDC**：登录只走外部 OIDC Provider（授权码流 + PKCE + nonce），无本地密码体系。

设计目标：

- 轻部署：Go API 以单二进制 + 单 SQLite 文件起步，前端静态资源由 Vite 或 Nginx 提供；PostgreSQL 为规划中的可选后端（适配器尚未实现）。
- 现代 UI：Vue 3 + Element Plus + Tailwind CSS。
- 内容互操作：`[[wikilink]]` 与 `[[目标|别名]]`（slug 路径语义）、GFM、KaTeX、Mermaid，与纯 Markdown 生态互通。

## 特性

- 文档树、Markdown 源码编辑（源码 + 预览分栏）、版本提交、草稿和历史版本
- slug 路径 URL（`/docs/<祖先slug>/…/<slug>`）与 `[[wikilink]]` 互链、死链报告
- 文档可见性（沿树继承）、权限码和管理员后台
- 中文全文搜索（Bleve）
- 附件、评论、回收站和备份/导入任务
- OIDC 授权码流登录，包含 PKCE、state、nonce 和 JIT 建号
- i18n（zh-CN / en）、运行时设置即时生效、个人 API Token

## 尚未实现

- PostgreSQL 适配器：配置接受 `postgres`，但连接时显式报错
- 后端覆盖率门禁 90%（NF-01）尚未达成：当前约 80%，回补顺序见 [doc/DELIVERY-REVIEW.md](doc/DELIVERY-REVIEW.md)
- 真实环境验收：OIDC 真登录、Nginx 深链接刷新与完整编辑链路
- 富文本编辑器：设计上不做（红线）

## 快速开始

要求：Go 1.26.7 或更高版本、Node.js 20.19+/22.12+（与 element-skin 对齐的 `engines` 约束）。

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

生产环境由 Nginx 提供 `frontend/dist` 静态资源，后端监听 `127.0.0.1:8080`。构建完成后将 `frontend/dist` 部署到 `/var/www/element-wiki`，并参考 [`deploy/nginx.conf`](deploy/nginx.conf) 配置：`/v1/` 和 `/healthz` 反向代理到后端，其余页面路径回退到 `index.html`，以支持文档、搜索和管理页面深链接刷新。生产环境应启用 HTTPS，并将 `server.frontend_url` 设置为同一 HTTPS 地址。

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
npm run test:e2e
npm run test:e2e:auth
```

`npm run test:e2e` 会自行启动前端并模拟公开 API；`npm run test:e2e:auth` 会额外启动测试专用 OIDC Provider 和隔离后端，实际走通登录、建文、编辑、搜索与回收站。详情见 [本地 OIDC 验收策略](doc/09-本地OIDC验收策略.md)。首次使用 Playwright 需要执行 `npx playwright install chromium`。

## 目录说明

```text
cmd/wikid/             后端入口
internal/httpapi/      HTTP 路由、认证包装和响应
internal/service/      业务规则和权限判断
internal/store/        存储接口及 SQLite 实现
internal/search/       Bleve 搜索索引与重建 worker
internal/render/       Markdown 渲染
migrations/            SQLite/PostgreSQL 数据库迁移
frontend/src/api/      前端 API wrapper
frontend/src/views/    前端页面
frontend/src/components/ 前端组件
doc/                   需求、数据库、API 和路线图
```

运行期数据默认位于 `data/` 和 `storage/`，不要把真实数据库、附件、搜索索引或备份提交到 Git。

## 文档

```text
doc/00-需求手册.md        需求基线（v0.4，定位与约束见 §0）
doc/01-数据库表设计.md     SQLite/PostgreSQL schema
doc/02-后端API设计.md      /v1 REST 契约与权限码目录
doc/06-回收站子树标注与确认弹窗计划.md 待执行的前端改进计划
doc/DELIVERY-REVIEW.md    交付审查清单（待人工执行）
doc/ROADMAP.md            迭代路线与进度（唯一事实来源）
doc/archive/              已归档的过程计划（03 页面导航 / 04 严重问题修复 / 05 编辑器与 URL 重构）
```

## License

MIT
