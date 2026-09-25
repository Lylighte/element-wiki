# 本地 OIDC 验收策略

Element Wiki 不提供本地密码账号。自动化验收按故障边界分两层，均不在生产 API 增加测试登录入口。

## 1. 浏览器 UI 冒烟

`cd frontend && npm run test:e2e` 启动 Vite。`tests/navigation.spec.ts` 用 Playwright 网络拦截为公开页面提供固定 API 响应，验证路由与基本页面加载。组件级 Vitest 用 mock API 覆盖弹窗、失败反馈、权限入口等交互。这一层不声称覆盖真实认证或数据库。

## 2. 真实 OIDC 浏览器闭环

`cd frontend && npm run test:e2e:auth` 自动启动：

1. `tests/support/test-idp.mjs`：仅测试环境使用的本地 OIDC Provider，签发短效测试 ID Token，检查授权码一次性消费及 PKCE challenge。
2. `go run ./cmd/wikid`：连接本次运行在 `/tmp/element-wiki-e2e-*` 下的新 SQLite 文件、搜索索引与附件目录。
3. Vite：`/v1` 代理到隔离后端。
4. Playwright：浏览器点击 SSO 登录，经过真实后端回调、JIT 建号、session cookie，然后建文、编辑、阅读、搜索、移入回收站、取消与确认彻底删除；第二个浏览器身份验证 viewer 的权限入口，再由管理员在用户面板提升为 editor，验证文档树管理入口。

测试 IdP 提供 `admin@e2e.local` 与 `viewer@e2e.local` 两个虚拟身份。前者仅由测试配置的 `admin_emails` 提权，后者经 JIT 成为 viewer；测试密钥固定且只用于本地测试。正式配置与运行时数据不被读取或修改。测试 Provider 不编译进 `wikid`，也没有本地密码或认证绕过端点。

后端 `internal/httpapi/oidc_flow_test.go` 已单独覆盖 state、nonce、PKCE、JIT viewer、首次管理员引导和禁用账号等矩阵。浏览器闭环补的是前后端接线与用户实际路径。

## 仍需人工验收

部署环境的真实 Provider、反向代理深链接刷新、Cookie 的 HTTPS 属性和生产回跳地址，必须在实际域名下验收（ROADMAP T13.2）。本地测试 IdP 无法代替该步骤。
