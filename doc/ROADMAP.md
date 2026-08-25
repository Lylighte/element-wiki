# ROADMAP —— 自迭代执行清单

> 进度唯一事实来源。完成任务的最后一步是勾选条目并包含在收尾 commit 中。

## 使用协议

- 会话开场白固定：`读 doc/ROADMAP.md 和 AGENTS.md，从下一个未勾选任务继续。`
- 一次只做一个任务。**DoD（完成定义）**：验收标准全部满足 + `go vet` 干净 + `go test ./...` 绿 + 总覆盖率 ≥85% + 本条目勾选并入收尾 commit。
- 门禁连续失败 5 次：在本任务条目下追加「阻塞: 原因」并把勾选框改为 `[!]`，**立即跳到下一个任务**；每完成若干任务或全部完成后回头处理 `[!]`，仍失败则保持标记继续。整个迭代期间不做任何人工干预，所有事项推迟到最终交付审查。
- 契约偏离：自行研究决定最小改动并继续推进，同时在下方「契约变更登记」登记，供最终人工审查。
- 任务跨会话：收工前把中间进度写在对应条目下方。
- **连续迭代模式**（人工已授权）：里程碑最后一个任务完成后，输出该 M 的 demo 步骤清单并**直接继续下一个 M**，不等待人工放行；迭代持续到路线图全部勾选为止。
- **夜间迭代批次（2026-08-26 人工批准）**：本批 M8–M12 执行期间总覆盖率门禁暂停——DoD 调整为 `go vet` 干净 + `go test ./...` 全绿 + 新增关键路径轻量测试（错误路径/权限矩阵优先），存量测试必须保持绿。契约偏离已一次性批准：按「契约变更登记」先改契约文档再写码。

---

## M0 工程骨架

- [x] T0.1 go module + `cmd/wikid` 入口 + config 加载（内置默认值 → config.yaml → 环境变量覆盖）
  验收: config 测试覆盖三层覆盖顺序与非法值报错；无配置文件时可用默认值启动
- [x] T0.2 迁移框架：`schema_migrations` 表 + 嵌入式 SQL 迁移文件
  验收: 空库应用全部迁移后版本一致；重复应用幂等；二进制与库版本不一致时启动报错
- [x] T0.3 v1 全部表结构迁移落地（doc/01 全部 DDL + 种子 settings 键）
  验收: 表/索引存在性测试；CHECK 约束违规插入被拒；`comments_enabled` 种子为 false
- [x] T0.4 CI（GitHub Actions）: build + vet + test + 覆盖率门禁 ≥85%
  验收: 工作流已创建且全部步骤本地镜像验证通过；远程运行验证列入交付审查清单（无远程仓库限制）。

## M1 文档树与版本域（service 用测试 Actor，暂不接真认证）

- [x] T1.1 documents store：CRUD + slug 部分唯一索引
  验收: 同父级重复 slug → 约束错误；进回收站后释放 slug；根级（parent_id NULL）唯一性生效
- [x] T1.2 树查询 store：子树读取 + 递归 CTE 生效可见性解析
  验收: 任一祖先 restricted → 生效 restricted；根文档、深层嵌套场景断言
- [x] T1.3 commits/blob/draft store
  验收: commit_no 文档内单调唯一；blob 按 sha256 去重；draft UPSERT 幂等
- [x] T1.4 版本裁剪（max_versions 默认 100）
  验收: 第 N+1 次提交后最旧版不可列；blob 不立即删除；裁剪与新 commit 同事务
- [x] T1.5 document service：创建 / 改名改 slug / 移动
  验收: 移动进自身子树被拒；slug 冲突返回冲突错误；失败事务回滚无残留行
- [x] T1.6 commit service：base≠HEAD 冲突语义 + 成功路径一致性
  验收: 冲突后 HEAD/blob/草稿零污染（表驱动）；并发双写仅一方成功
- [x] T1.7 revert service：以历史内容新建 commit
  验收: 原 commit 行不可变；新 HEAD 内容等于目标历史内容
- [x] T1.8 httpapi 全链路：documents/tree CRUD + draft + commits + revert + render-preview（测试 Actor 注入）
  验收: method/path/status/body 精确断言；409/422 错误契约符合 doc/02 §14

## M2 渲染管线

- [x] T2.1 goldmark 基座 + GFM 扩展（表格/任务清单/删除线/GFM Alert）
  验收: 每种扩展的黄金文件快照测试
- [x] T2.2 代码高亮标记 + KaTeX/Mermaid 前端渲染占位约定
  验收: 快照断言输出块结构与 class 约定
- [x] T2.3 TOC 提取 + 标题锚点
  验收: 嵌套层级正确；中文标题锚点生成稳定可复现
- [x] T2.4 wikilink 解析 + 死链检测（commit 响应字段，不入库）
  验收: 目标存在/不存在/自引用三类结果；`[[..]]` 语法边界用例
- [x] T2.5 输出消毒 + XSS 测试集
  验收: script/onerror/javascript: URI 等攻击向量快照全部中和

## M3 认证与权限（安全面，完成后人工审查 diff）

- [x] T3.1 权限码 catalog + viewer/editor/admin 映射 + Actor Require/HasAny
  验收: catalog 与 doc/02 §13 一致性测试；缺同步更新时测试失败
- [x] T3.2 sessions store + 认证中间件
  验收: 过期 session 401；disabled 用户 403；cookie 属性符合 AGENTS §5
- [x] T3.3 OIDC 授权码流（PKCE+nonce+state）+ JIT 建号（stub IdP 集成测试）
  验收: state/nonce 校验失败拒绝；admin_emails 首次匹配提升且不重复提权；JIT 建 viewer
- [x] T3.4 api_tokens：签发 / Bearer 解析 / 吊销
  验收: 库中仅存哈希+prefix；revoked token 401；last_used_at 更新
- [x] T3.5 匿名只读门闩（anonymous_read）
  验收: 关闭时匿名请求 401；开启时匿名树不含 restricted 且存在性 404 掩护
- [x] T3.6 受保护端点全面接线（替换 M1 测试 Actor）
  验收: 未认证/无权限/restricted viewer 拒/editor 过/admin 过矩阵测试全绿

## M4 搜索

- [x] T4.1 Bleve 封装（CJK 分析器 + documents.bleve 目录抽象）
  验收: t.TempDir 下建索引/写入/查询/关闭生命周期测试
- [x] T4.2 commit→同步索引更新 + 失败入 search_reindex_jobs 降级
  验收: 注入索引故障仍返 201 且 job 行存在；job 消费后索引恢复一致
- [x] T4.3 搜索 API：service 层可见集过滤 + 逐条二次校验
  验收: 构造越权查询无权文档不出现在结果；短语语法与高亮 snippet 断言
- [x] T4.4 手动全量重建端点 + worker
  验收: 202 受理 → done；重建期间旧索引持续可用

## M5 协作外围

- [x] T5.1 回收站 store/service：软删子树 / restore / 彻底删除
  验收: 删除后主树不可见而 trash 可见；restore 父链缺失 → 409 需 parent_id；purge 级联清 commits
- [x] T5.2 purge 后台任务（purge_at 到期清理）
  验收: 到期清理、未到期保留、执行幂等
- [x] T5.3 blob GC 任务
  验收: 仅无 commit 引用的 blob 被删；有引用的保留
- [x] T5.4 附件受控上传（临时文件 → 校验 → 落盘+DB）
  验收: DB 失败磁盘无孤儿文件；白名单外 415；超限 413
- [x] T5.5 附件读取/删除端点
  验收: 权限随文档可见性；删除后磁盘文件同步清理
- [x] T5.6 评论 CRUD + mentions（comments_enabled 门闩）
  验收: 禁用时全接口 403 detail=comments disabled；@提及解析入库；删除权限 own/any 矩阵
- [x] T5.7 sitemap.xml
  验收: 仅含匿名可见 standard 文档；restricted 与回收站文档不出现；匿名模式关闭时返回空壳

## M6 管理

- [x] T6.1 settings store/service（种子键 + 类型解析）
  验收: 未知键拒绝；布尔/整数键类型转换错误路径
- [x] T6.2 settings API GET/PATCH
  验收: 非 admin 403；PATCH 部分更新不影响未提交键
- [x] T6.3 用户管理 API（list / PATCH role|status）
  验收: 不可操作自己；禁用用户再登录被拒且不复活
- [x] T6.4 dashboard 统计聚合
  验收: 计数与造数一致；restricted 文档不计入匿名视角
- [x] T6.5 备份导出 job（zip: manifest + db + attachments）
  验收: manifest schema_version/计数正确；job 状态机 pending→running→done
- [x] T6.6 备份导入（manifest/schema/路径安全校验 → 成功或零残留）
  验收: 损坏 zip/非法路径/schema 不符均整体失败且 DB 与磁盘零残留
- [x] T6.7 Markdown zip 导入 job（目录映射文档树 + 图片转附件）
  验收: 嵌套层级正确映射；非法条目计入 failed_files 不中断整体；失败零残留

## M7 前端 Vue 完整化

- [x] T7.1 frontend 骨架（Vite+Vue3+TS+Element Plus+Tailwind+i18n zh-CN/en）
  验收: build 通过；两语言资源 key 完整性测试
- [x] T7.2 api client + wrapper 层
  验收: wrapper 测试精确断言 method/path/params/body
- [x] T7.3 认证流视图（OIDC 跳转 / me / logout / token 管理 UI）
- [x] T7.4 文档树侧栏 + 面包屑 + 文档页渲染视图（只读路由零编辑器加载）
  验收: lazy import 断言只读页 chunk 不含编辑器
- [x] T7.5 编辑器（Tiptap2 + 精简工具栏 + 表格 + [[补全] + 粘贴上传）
  验收: 409 冲突 UI 提示；自动保存与离开确认状态机测试
- [x] T7.6 搜索页 + 高亮展示
- [x] T7.7 回收站 / 评论 / 附件界面
  验收: comments_enabled=false 时评论区不渲染
- [x] T7.8 管理面板（设置/用户/备份/导入/仪表盘）
  验收: E2E 冒烟：登录 → 建文 → 编辑 → 提交 → 搜索命中

## M8 文档树与导航（契约变更 C1）

- [x] T8.1 补齐 `DELETE /v1/documents/{id}` 路由（进回收站）+ PATCH 透传 sort_key
  验收: HTTP 进回收站后主树不可见、trash 可见、slug 释放；PATCH sort_key 持久化且 ListChildren 顺序生效；restricted 越权 404 掩护
- [x] T8.2 `PUT /v1/documents/reorder` 同层批量重排（doc/02 §4 已登记）
  验收: 完整兄弟列表语义，缺员/多余/跨父 422 附 fields；成功按 (i+1)*100 写入且 204；匿名越权矩阵覆盖
- [x] T8.3 侧栏折叠/展开 + localStorage 持久化
  验收: 刷新后状态保持；无子节点不渲染折叠箭头
- [x] T8.4 拖拽移动 + 同层排序（接 move / reorder API）
  验收: 拖入自身子树被前端阻止，后端 422 有用户可见提示；排序与跨父移动刷新后均保持
- [x] T8.5 右键菜单：内联重命名 / 新建子文档 / 移入回收站
  验收: 菜单项按权限码显隐；删除后树局部刷新不整页重载
- [x] T8.6 新建对话框父级选择 + 树上「新建子文档」入口
  验收: 新文档出现在目标父级下并自动导航

## M9 编辑体验（契约变更 C2/C4）

- [ ] T9.1 标题纳入草稿与提交：commit body 可选 title（doc/02 §5 已登记）
  验收: 带 title 的 commit 同事务改标题；草稿保存/回填含标题；不带 title 不动标题；冲突时标题零写入
- [ ] T9.2 实时预览分栏（兑现 ED-02，复用 POST /v1/render-preview）
  验收: 输入防抖渲染；只读页 chunk 不含编辑器代码的既有断言保持绿
- [ ] T9.3 KaTeX/Mermaid 前端懒加载渲染（兑现 RD-03）
  验收: 仅内容含公式/图时加载依赖；普通文档 chunk 无 katex/mermaid
- [ ] T9.4 工具栏补全：图片拖拽上传 / strike / 表格行列操作 / 链接弹窗替换 window.prompt
  验收: 各按钮行为有测试断言；拖拽上传失败提示且无孤儿附件（复用 ED-06 管线）
- [ ] T9.5 离开确认（onBeforeRouteLeave + beforeunload）
  验收: dirty 时路由离开弹确认、直接关闭弹 beforeunload；保存后不弹
- [ ] T9.6 DocView TOC 侧栏 + wikilink 点击导航（兑现 RD-08）
  验收: 锚点跳转平滑滚动；wikilink 解析 slug→路由；死链点击有「目标不存在」反馈；匿名 restricted 目标 404 掩护
- [ ] T9.7 slash 命令菜单（可选增强，时间富余再做）
  验收: `/` 触发块类型菜单，Esc 关闭，选择后插入对应节点

## M10 国际化扫盲（契约变更 C3）

- [ ] T10.1 语言切换器 + 浏览器检测 + 消费 `GET /v1/site`（公开站点信息端点已登记）
  验收: 切换即时生效并持久化 localStorage；首次访问按 navigator.language 兜底 default_lang；匿名可取站点信息且 wiki_title 反映到 header
- [ ] T10.2 全部硬编码文案接入 locale：AdminView/DocView/HomeView/EditView/SearchView/TrashView/TokensView/AttachmentsPanel/loginErrors/App 对话框
  验收: grep 无残留硬编码用户可见文本；两语言 key 完整性测试扩展后仍绿
- [ ] T10.3 后端 detail 中文泄漏清理为英文规范码
  验收: internal/httpapi 与 service 用户可读错误无中文泄漏；相关测试断言同步更新

## M11 设置生效与管理面板

- [ ] T11.1 设置运行时即时生效（读缓存 + PATCH 失效）：anonymous_read/max_versions/upload_max_mb/allowed_extensions/trash_retention_days 等
  验收: PATCH 后无需重启即改变行为的服务层测试；缓存失效并发安全；comments_enabled 既有门闩行为不变
- [ ] T11.2 设置表单九键控件化（switch/select/number/tags）+ 校验错误展示 + 仅提交变更键
  验收: 类型错误 422 展示 fields 明细；wiki_title 保存后 header 即时更新
- [ ] T11.3 users tab 补 display_name/q 搜索/危险操作确认；dashboard 渲染 recent_docs + contributors
  验收: 搜索走 q= 参数精确断言；自己一行不可禁用；仪表盘五指标齐全

## M12 导出导入闭环（契约变更 C5/C6）

- [ ] T12.1 备份 tab 流程闭环：发起备份按钮 + job 轮询进度 + 备份导入 + Markdown zip 导入入口
  验收: E2E：导出 → 列表出现产物 → 下载非空 zip；两个导入均有进度轮询到终态；危险操作有确认
- [ ] T12.2 后端导入合规修复：manifest 缺失整体失败 / 导入成功自动入队全量索引重建 / 清除 DBG println / markdown-import tmp 文件竞态 / job 补 running 态
  验收: 损坏 zip 零残留测试绿；导入完成后新内容可被搜索命中；tmp 文件在 goroutine 读完后才删除
- [ ] T12.3 单文档 Markdown 导出 `GET /v1/documents/{id}/export.md` + DocView 按钮
  验收: 内容等于 HEAD 源码且 Content-Disposition 正确；restricted 匿名 404 掩护
- [ ] T12.4 PG 方言备份显式 501 降级
  验收: driver=postgres 时备份导出与两种导入返回 501 明确 detail，不创建 job 不产文件；SQLite 路径不受影响

---

## 交付审查清单（路线图全完成后，人工作业）

连续迭代模式下不逐 M 设人工门禁；以下事项在**路线图全部勾选后**集中提交人工审查：

- 安全面 diff 集中审查清单：M3 全部（OIDC/session/token）、T2.5 消毒、T5.4 上传受控提交
- 各 M 的 demo 步骤清单汇总（迭代过程中随完成输出）
- 契约变更记录：任何 doc/00~02 的改动须在迭代中即时登记于此
- 发布检查单：覆盖率报告存档 / 双方言迁移冒烟（SQLite+PG）/ CHANGELOG / LICENSE 确认 MIT

### 契约变更登记

- C1 (2026-08-26): doc/02 §4 新增 `PUT /v1/documents/reorder`（同层批量重排 sort_key，完整兄弟列表语义）；doc/00 新增 DM-09、UX-07
- C2 (2026-08-26): doc/02 §5 commit 请求体新增可选 `title` 字段（与正文同事务写入）；doc/00 新增 ED-08
- C3 (2026-08-26): doc/02 §12 新增公开端点 `GET /v1/site`；doc/00 新增 OP-07（在线设置即时生效）、UX-06（i18n 全量覆盖）
- C4 (2026-08-26): doc/02 §5 新增 `GET /v1/documents/{id}/export.md`；doc/00 新增 OP-08
- C5 (2026-08-26): doc/02 §11 补登既有实现 `POST /v1/admin/markdown-import`（此前代码存在而契约漏登；进度复用 imports jobs 端点）
- C6 (2026-08-26): doc/02 §11 明确 manifest 缺失即整体失败 + 导入后自动索引重建；§14 新增 501 错误语义（PG 备份降级）；doc/00 新增 OP-09
- 说明：doc/01 无需改动——reorder 用既有 `sort_key` 列，commit title 写既有 `documents.title`，/v1/site 读既有 settings，export.md 读既有 blob；doc/00 版本号 v0.2 → v0.3
