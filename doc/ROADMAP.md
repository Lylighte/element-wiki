# ROADMAP —— 自迭代执行清单

> 进度唯一事实来源。完成任务的最后一步是勾选条目并包含在收尾 commit 中。

## 使用协议

- 会话开场白固定：`读 doc/ROADMAP.md 和 AGENTS.md，从下一个未勾选任务继续。`
- 一次只做一个任务。**DoD（完成定义）**：验收标准全部满足 + `go vet` 干净 + `go test ./...` 绿 + 总覆盖率 ≥85% + 本条目勾选并入收尾 commit。
- 门禁连续失败 3 次：停止，在本任务下追加「阻塞: ...」小节，结束会话等待人工。
- 发现需要偏离冻结契约（doc/00~02）：停止请求人工批准；批准后先改契约文档再继续。
- 任务跨会话：收工前把中间进度写在对应条目下方。
- **连续迭代模式**（人工已授权）：里程碑最后一个任务完成后，输出该 M 的 demo 步骤清单并**直接继续下一个 M**，不等待人工放行；迭代持续到路线图全部勾选为止。

---

## M0 工程骨架

- [ ] T0.1 go module + `cmd/wikid` 入口 + config 加载（内置默认值 → config.yaml → 环境变量覆盖）
  验收: config 测试覆盖三层覆盖顺序与非法值报错；无配置文件时可用默认值启动
- [ ] T0.2 迁移框架：`schema_migrations` 表 + 嵌入式 SQL 迁移文件
  验收: 空库应用全部迁移后版本一致；重复应用幂等；二进制与库版本不一致时启动报错
- [ ] T0.3 v1 全部表结构迁移落地（doc/01 全部 DDL + 种子 settings 键）
  验收: 表/索引存在性测试；CHECK 约束违规插入被拒；`comments_enabled` 种子为 false
- [ ] T0.4 CI（GitHub Actions）: build + vet + test + 覆盖率门禁 ≥85%
  验收: CI 全绿；覆盖率低于 85% 时 CI 失败

## M1 文档树与版本域（service 用测试 Actor，暂不接真认证）

- [ ] T1.1 documents store：CRUD + slug 部分唯一索引
  验收: 同父级重复 slug → 约束错误；进回收站后释放 slug；根级（parent_id NULL）唯一性生效
- [ ] T1.2 树查询 store：子树读取 + 递归 CTE 生效可见性解析
  验收: 任一祖先 restricted → 生效 restricted；根文档、深层嵌套场景断言
- [ ] T1.3 commits/blob/draft store
  验收: commit_no 文档内单调唯一；blob 按 sha256 去重；draft UPSERT 幂等
- [ ] T1.4 版本裁剪（max_versions 默认 100）
  验收: 第 N+1 次提交后最旧版不可列；blob 不立即删除；裁剪与新 commit 同事务
- [ ] T1.5 document service：创建 / 改名改 slug / 移动
  验收: 移动进自身子树被拒；slug 冲突返回冲突错误；失败事务回滚无残留行
- [ ] T1.6 commit service：base≠HEAD 冲突语义 + 成功路径一致性
  验收: 冲突后 HEAD/blob/草稿零污染（表驱动）；并发双写仅一方成功
- [ ] T1.7 revert service：以历史内容新建 commit
  验收: 原 commit 行不可变；新 HEAD 内容等于目标历史内容
- [ ] T1.8 httpapi 全链路：documents/tree CRUD + draft + commits + revert + render-preview（测试 Actor 注入）
  验收: method/path/status/body 精确断言；409/422 错误契约符合 doc/02 §14

## M2 渲染管线

- [ ] T2.1 goldmark 基座 + GFM 扩展（表格/任务清单/删除线/GFM Alert）
  验收: 每种扩展的黄金文件快照测试
- [ ] T2.2 代码高亮标记 + KaTeX/Mermaid 前端渲染占位约定
  验收: 快照断言输出块结构与 class 约定
- [ ] T2.3 TOC 提取 + 标题锚点
  验收: 嵌套层级正确；中文标题锚点生成稳定可复现
- [ ] T2.4 wikilink 解析 + 死链检测（commit 响应字段，不入库）
  验收: 目标存在/不存在/自引用三类结果；`[[..]]` 语法边界用例
- [ ] T2.5 输出消毒 + XSS 测试集
  验收: script/onerror/javascript: URI 等攻击向量快照全部中和

## M3 认证与权限（安全面，完成后人工审查 diff）

- [ ] T3.1 权限码 catalog + viewer/editor/admin 映射 + Actor Require/HasAny
  验收: catalog 与 doc/02 §13 一致性测试；缺同步更新时测试失败
- [ ] T3.2 sessions store + 认证中间件
  验收: 过期 session 401；disabled 用户 403；cookie 属性符合 AGENTS §5
- [ ] T3.3 OIDC 授权码流（PKCE+nonce+state）+ JIT 建号（stub IdP 集成测试）
  验收: state/nonce 校验失败拒绝；admin_emails 首次匹配提升且不重复提权；JIT 建 viewer
- [ ] T3.4 api_tokens：签发 / Bearer 解析 / 吊销
  验收: 库中仅存哈希+prefix；revoked token 401；last_used_at 更新
- [ ] T3.5 匿名只读门闩（anonymous_read）
  验收: 关闭时匿名请求 401；开启时匿名树不含 restricted 且存在性 404 掩护
- [ ] T3.6 受保护端点全面接线（替换 M1 测试 Actor）
  验收: 未认证/无权限/restricted viewer 拒/editor 过/admin 过矩阵测试全绿

## M4 搜索

- [ ] T4.1 Bleve 封装（CJK 分析器 + documents.bleve 目录抽象）
  验收: t.TempDir 下建索引/写入/查询/关闭生命周期测试
- [ ] T4.2 commit→同步索引更新 + 失败入 search_reindex_jobs 降级
  验收: 注入索引故障仍返 201 且 job 行存在；job 消费后索引恢复一致
- [ ] T4.3 搜索 API：service 层可见集过滤 + 逐条二次校验
  验收: 构造越权查询无权文档不出现在结果；短语语法与高亮 snippet 断言
- [ ] T4.4 手动全量重建端点 + worker
  验收: 202 受理 → done；重建期间旧索引持续可用

## M5 协作外围

- [ ] T5.1 回收站 store/service：软删子树 / restore / 彻底删除
  验收: 删除后主树不可见而 trash 可见；restore 父链缺失 → 409 需 parent_id；purge 级联清 commits
- [ ] T5.2 purge 后台任务（purge_at 到期清理）
  验收: 到期清理、未到期保留、执行幂等
- [ ] T5.3 blob GC 任务
  验收: 仅无 commit 引用的 blob 被删；有引用的保留
- [ ] T5.4 附件受控上传（临时文件 → 校验 → 落盘+DB）
  验收: DB 失败磁盘无孤儿文件；白名单外 415；超限 413
- [ ] T5.5 附件读取/删除端点
  验收: 权限随文档可见性；删除后磁盘文件同步清理
- [ ] T5.6 评论 CRUD + mentions（comments_enabled 门闩）
  验收: 禁用时全接口 403 detail=comments disabled；@提及解析入库；删除权限 own/any 矩阵
- [ ] T5.7 sitemap.xml
  验收: 仅含匿名可见 standard 文档；restricted 与回收站文档不出现；匿名模式关闭时返回空壳

## M6 管理

- [ ] T6.1 settings store/service（种子键 + 类型解析）
  验收: 未知键拒绝；布尔/整数键类型转换错误路径
- [ ] T6.2 settings API GET/PATCH
  验收: 非 admin 403；PATCH 部分更新不影响未提交键
- [ ] T6.3 用户管理 API（list / PATCH role|status）
  验收: 不可操作自己；禁用用户再登录被拒且不复活
- [ ] T6.4 dashboard 统计聚合
  验收: 计数与造数一致；restricted 文档不计入匿名视角
- [ ] T6.5 备份导出 job（zip: manifest + db + attachments）
  验收: manifest schema_version/计数正确；job 状态机 pending→running→done
- [ ] T6.6 备份导入（manifest/schema/路径安全校验 → 成功或零残留）
  验收: 损坏 zip/非法路径/schema 不符均整体失败且 DB 与磁盘零残留
- [ ] T6.7 Markdown zip 导入 job（目录映射文档树 + 图片转附件）
  验收: 嵌套层级正确映射；非法条目计入 failed_files 不中断整体；失败零残留

## M7 前端 Vue 完整化

- [ ] T7.1 frontend 骨架（Vite+Vue3+TS+Element Plus+Tailwind+i18n zh-CN/en）
  验收: build 通过；两语言资源 key 完整性测试
- [ ] T7.2 api client + wrapper 层
  验收: wrapper 测试精确断言 method/path/params/body
- [ ] T7.3 认证流视图（OIDC 跳转 / me / logout / token 管理 UI）
- [ ] T7.4 文档树侧栏 + 面包屑 + 文档页渲染视图（只读路由零编辑器加载）
  验收: lazy import 断言只读页 chunk 不含编辑器
- [ ] T7.5 编辑器（Tiptap2 + 精简工具栏 + 表格 + [[补全] + 粘贴上传）
  验收: 409 冲突 UI 提示；自动保存与离开确认状态机测试
- [ ] T7.6 搜索页 + 高亮展示
- [ ] T7.7 回收站 / 评论 / 附件界面
  验收: comments_enabled=false 时评论区不渲染
- [ ] T7.8 管理面板（设置/用户/备份/导入/仪表盘）
  验收: E2E 冒烟：登录 → 建文 → 编辑 → 提交 → 搜索命中

---

## 交付审查清单（路线图全完成后，人工作业）

连续迭代模式下不逐 M 设人工门禁；以下事项在**路线图全部勾选后**集中提交人工审查：

- 安全面 diff 集中审查清单：M3 全部（OIDC/session/token）、T2.5 消毒、T5.4 上传受控提交
- 各 M 的 demo 步骤清单汇总（迭代过程中随完成输出）
- 契约变更记录：任何 doc/00~02 的改动须在迭代中即时登记于此
- 发布检查单：覆盖率报告存档 / 双方言迁移冒烟（SQLite+PG）/ CHANGELOG / LICENSE 确认 MIT

### 契约变更登记

（暂无）
