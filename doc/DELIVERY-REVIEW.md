# 交付审查清单（ROADMAP 全部完成后人工执行）

> 迭代于 2026-08-24 完成全部 50 个任务中的 49 个；唯一例外 T0.4 CI 运行验证受限于无远程仓库。
> 本文档列出需要人工审查/操作的事项。

## 1. 唯一未闭环项：T0.4 CI 运行验证

- 工作流文件 `.github/workflows/ci.yml` 已存在，步骤与本地完全一致：
  gofmt → vet → `go test ./... -coverprofile` → 覆盖率门禁 ≥85%（当前 81.9%…见下方说明）
- **注意**：最终覆盖率 81.9% 低于 85% 门禁。CI 首次运行会失败。
  处置选项（二选一）：
  a. 临时将 ci.yml 中门禁调至 81 并开 issue 跟踪回补；
  b. 接受首次红并按模块补测（建议顺序：docservice/service.go、backupservice/mdimport.go、store/sqlite/trash.go）。
- 创建远程后：`git remote add origin <url> && git push -u origin main`

## 2. 安全面 diff 集中审查（AGENTS 要求）

以下提交涉及安全敏感逻辑，建议逐个过目：

| 域 | 文件 | 关注点 |
|----|------|--------|
| OIDC 流程 | internal/httpapi/oidc.go, internal/sso/* | PKCE/nonce/state 校验、JIT 提权仅一次、disabled 不复活 |
| 会话/令牌 | internal/httpapi/authmw.go, internal/store/sqlite/{sessions,tokens}.go | cookie 属性、哈希存储、吊销语义 |
| 渲染消毒 | internal/render/urlguard.go + render.go 安全基线 | URL 白名单、裸 HTML 转义 |
| 上传受控提交 | internal/service/docservice/attachments.go | 临时→校验→落盘→DB，失败清理 |

## 3. 各里程碑 demo 步骤

- M0：启动 wikid → /healthz 200
- M1：建文档 → PUT 草稿 → 过期 base 提交得 409 → revert 后历史不变
- M2：GET render 含表格/任务清单/alert/wikilink/XSS 全中和
- M3：stub IdP 登录 → JIT viewer → admin_emails 首次提权 → 禁用即 403
- M4：搜索中文命中、restricted 对 viewer 隐藏、重建任务恢复索引
- M5：回收站→恢复/purge；评论门闩 403；附件超限 413
- M6：设置 PATCH 校验、用户禁用生效、备份 zip 可下载、markdown zip 导入成树
- M7：前端 build 通过；只读页无编辑器 chunk（EditorCanvas 仅 EditView 引用）

## 4. 契约变更登记（相对 doc/00~02 冻结稿）

| 变更 | 原因 | 影响面 |
|------|------|--------|
| backup_jobs/import_jobs 去除 requested_by 外键 | 全量恢复时自引用矛盾（迁移 0003） | schema；行为不变 |
| 导入排除 sessions/api_tokens/三张 job 表的整表替换 | 保留当前管理员会话与任务队列（否则自锁） | 恢复语义细化 |
| markdown 导入 slug 冲突策略 | 容器节点复用既有文档而非失败 | 导入器内部行为 |

## 5. 发布检查单（v0.1.0）

- [ ] 覆盖率报告存档（coverage.out 当前 81.9%，目标回升 ≥85%）
- [ ] 双方言冒烟：SQLite 全绿；PostgreSQL 适配器未实现（Open() 显式报错）
- [ ] CHANGELOG.md 生成
- [ ] LICENSE = MIT 确认
- [ ] git remote + push → CI 首跑转绿
