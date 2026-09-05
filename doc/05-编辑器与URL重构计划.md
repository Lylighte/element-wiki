# 05 编辑器与 URL 重构计划

> 状态：已批准待执行（alpha 阶段，v1 契约可改）。
> 2026-08-29 审阅修订：补 wikilink/deadLinks 语义、slug 冲突策略定稿、提交 2/4 解耦、放弃退出清草稿、编辑器回归登记、sitemap 同步、覆盖率门禁核实（全库基线 79.3%，非 85%）。
> 2026-08-29 已按提交 1→2→3→4 执行完毕：`go vet` 干净、`go test ./...` 全绿（覆盖率 79.5%）、`npm test` 117 绿、`npm run build` 通过。
> 本计划在 04 严重问题收敛基础上，重构编辑器体验与文档 URL 模型。

## 背景

手动验收发现以下问题：

1. 目录边栏嵌套标题点击不跳转到对应节。
2. 编辑页无显式"不保存而退出"按钮。
3. 编辑器存在 Edit(富文本)/Source 两模式 + 独立预览按钮，冗余且富文本体验差。
4. slug 未体现在 URL 中，文档 URL 为 ULID 乱码（如 `/docs/01HXY…`）。

## 已确认决策

- 编辑器模型：**源码 + 预览分栏，弃用富文本（Tiptap）**。
- slug 与 URL：**路径式 slug URL**（`/docs/<祖先slug>/…/<slug>`）。
- slug 自动生成：未指定时 **拉丁净化 + 短 ID 回退**（`[a-z0-9-]+`，净化结果为空则 `doc-<ULID 前 8 位>`）。
- slug 冲突策略：自动生成冲突时**自增 `-2`、`-3`…（上限 20 次）**，仍冲突返回 409；显式指定 slug 冲突维持 409。自增探测是乐观路径，最终以 `(parent, slug)` 唯一索引兜底——并发撞车时一方 409，前端提示重试。
- wikilink 语义：`[[…]]` 目标为 **slug 路径**（`/` 分隔，单段即根级），解析规则与文档 URL 一致（从根逐段下钻）。现网前后端分裂：后端 `deadLinks` 只查根级（`internal/service/docservice/service.go:321`，注释「全树匹配」与实现不符），前端 `DocView.vue:86` 用 `findNodeBySlug` 全树匹配——本计划统一为同一路径语义。
- 提交 2/4 解耦：提交 2 的退出跳转按**路由名**（`{ name: 'doc', params: { id } }`）寻址，不硬编码路径字符串；提交 4 仅改路由定义与该处参数。
- 契约（doc/00~02）在 alpha 阶段可改，先改契约文档再写码。

---

## 提交 1 · 修复目录嵌套点击不跳转

### 根因
`frontend/src/components/doc/TocTree.vue` 嵌套的递归子组件未转发 `jump` 事件：

```vue
<TocTree v-if="n.children.length" :nodes="n.children" class="..." />
```

只有树根节点（顶层标题）点击能触发跳转；h2/h3/h4… 作为子节点时，`jump` 事件发到嵌套 TocTree 后无人监听，事件断在递归层。现有测试用单条 level-2 节点（在树中是根节点），未覆盖嵌套场景。

### 改动
- `TocTree.vue`：嵌套 `<TocTree>` 补 `@jump="$emit('jump', $event)"` 向上冒泡。
- `frontend/src/views/doc-toc.spec.ts`：新增嵌套结构点击测试（h1>h2，点 h2 也触发 jump）。

### 验收
- 顶层与嵌套标题点击均能跳转对应节。

---

## 提交 2 · 编辑页"不保存而退出"按钮

### 改动
- `frontend/src/views/EditView.vue`：标题栏新增"放弃修改退出"按钮。
  - 点击后**取消挂起的自动保存定时器**，调 `DELETE /v1/documents/{id}/draft`（幂等；失败仅提示不阻塞离开）清服务端草稿——否则已自动保存的草稿会在下次进入时复活，与「放弃」语义矛盾。
  - 再置 `leaveConfirmed = true`，`router.push({ name: 'doc', params: { id: props.id } })`（按路由名寻址，为提交 4 路径式改造留单一改动点）。
  - 不 flush 待存内容、不弹离开确认、不提交 commit；在途自动保存与删草稿的竞态由后端幂等兜底。
  - `commitAndExit` 现存的 `location.href` 整页刷新本次不动，随提交 4 一并改 `router.push` slug 路径。
- i18n 新增 `doc.discard`：
  - zh-CN：`放弃修改退出`
  - en：`Discard & exit`
- 两 locale 同步，key parity 测试保持绿。

### 测试
- `frontend/src/views/edit-leave.spec.ts` 补放弃退出用例：脏状态点按钮不弹离开确认、`DELETE draft` 被调用、按路由名跳转到文档页；「自动保存成功后立即放弃」路径草稿不复活。

### 验收
- 脏状态下点"放弃修改退出"直接离开，不保存、不弹确认。
- 放弃退出后服务端草稿被清除，再次进入不恢复旧草稿。
- 干净状态下行为一致。

---

## 提交 3 · 编辑器重构：源码 + 预览分栏（弃用富文本）

### 目标
- 源码编辑器始终展示原始 Markdown（硬性要求）。
- 消除 Edit/Source + 独立预览 三按钮冗余，源码成为唯一编辑模式（天然满足"默认进源码模式"）。
- 移除体验差的 Tiptap 富文本。

### 改动

#### `frontend/src/components/editor/EditorCanvas.vue`（重写）
- 移除 Tiptap/tiptap-markdown，改为 `<textarea>` 源码编辑器。
- 工具栏改为**向光标处插入 Markdown 片段**：H1/H2/H3、粗体、斜体、无序/有序列表、任务列表、引用、代码块、链接、图片、表格、分割线。
- 图片上传：按钮触发上传 → 插入 `![alt](url)`；保留拖拽粘贴上传插入 Markdown。
- `[[` wikilink 补全：监听输入弹浮层、选中后插入（如时间紧可先保留手写 `[[…]]`，补全后续迭代）。
- 对外接口不变：`props { initialMarkdown, docID, titles, uploadImage }`、`emit('change', markdown)`、`defineExpose({ getMarkdown })`；移除 `mode-change`。

#### `frontend/src/views/EditView.vue`
- 预览分栏**默认开启**：源码左 / 渲染右；保留"预览"开关按钮（默认开）。
- 移除 `editingSource` / `mode-change` 守卫：源码框为纯 textarea，不会被渲染内容替换；预览恢复防抖（500ms）服务端渲染。
- `commitAndExit` 沿用已同步的 `markdown`（change 事件已回写）。

#### 功能回归登记（随本提交显式接受，ROADMAP 登记 backfill）
- ED-04 表格可视化编辑（行列增删/移动/对齐）降级为**插入 Markdown 表格语法**；textarea 下不再提供可视化表格操作。
- ED-11 slash 命令菜单随 Tiptap 移除（00 手册标注"可选增强"）；同步删除 slash 相关测试与不再引用的 `editor.*` i18n key（两 locale 同步）。
- ED-07 `[[` 补全在 textarea 上以光标定位浮层重做；如本次不落地则最低保留手写 `[[…]]`，并在 ROADMAP 登记 backfill 条目。

#### 依赖清理
- `package.json` 移除 `@tiptap/*`、`tiptap-markdown` 及仅被 Tiptap 使用的类型依赖；`npm ci` 后重新锁定。
- **保留** `katex`、`mermaid`、`@types/katex`：RD-03 前端懒加载渲染与 `enhanceMarkdownExtras` 仍在使用，勿随 Tiptap 误删。

#### 测试
- 重写 `frontend/src/components/editor/editor-toolbar.spec.ts` 为源码编辑器用例：
  - 初始化 textarea 展示原始 Markdown。
  - 输入触发 `change` 且内容为源码。
  - 插入按钮（标题/列表/图片上传）在光标处插入对应 Markdown。
- `readonly-contract.spec.ts` 保持绿（EditorCanvas 仍懒加载、DocView 不含编辑器）。

### 验收
- 打开编辑页即源码模式，显示真实 Markdown。
- 源码输入不会被渲染内容替换。
- 预览分栏防抖更新；仅一个"预览"开关。
- 图片上传/拖拽插入正常。
- 只读页 chunk 不含编辑器代码。

---

## 提交 4 · 路径式 slug URL + slug 自动生成（契约变更）

### 契约变更（先改文档）

#### doc/00-需求手册.md
- DM-02 补充：文档公开 URL 为 `/docs/<祖先slug>/…/<slug>`；slug 父级内唯一；创建时 slug 可选，未提供按"拉丁净化+短 ID 回退"生成（冲突自增，见决策）。
- RD-05/RD-08 补充：wikilink `[[…]]` 目标为 slug 路径（`/` 分隔，单段即根级），后端死链检测与前端点击导航共用同一语义。

#### doc/02-后端API设计.md
- 新增 `GET /v1/documents/resolve?path=<slug路径>`：按路径解析文档，返回 `{ document, render }`；权限同 document.read；`path` 必填、逐段校验 slug 合法性；任一段不存在/不可见一律 404 掩护（含匿名）。
- §4 `POST /v1/documents` 请求体：`slug` 变为**可选**，缺省由服务端按标题生成（拉丁净化 + 短 ID 回退 + 冲突自增）；显式 slug 的校验与 409 语义不变。
- §5 死链报告：`deadLinks` 解析由根级单段 `GetBySlug` 改为 slug 路径逐段下钻（与 resolve 共用同一解析函数，纯存在性检查、无权限过滤，保持"读时派生报告"性质）；T2.4 三类结果快照用例同步更新。
- §12 sitemap：条目 URL 由 id 形态（`internal/httpapi/sitemap.go:50` 现输出 `/docs/<id>`）改为 slug 路径形态，与公开 URL 一致。
- 现有 `/v1/documents/{id}` 系列保留用于编辑/管理操作（按 id 寻址）。

#### doc/01-数据库表设计.md
- 无表结构变化（slug 列与 `(COALESCE(parent_id,''), slug)` 唯一索引已存在）。

### 后端
- `docservice.ResolveByPath(ctx, actor, path []string)`：从根逐段 `GetBySlug` 下钻，每段做生效可见性检查（service 层，AGENTS §3）；任一段不存在或不可见一律返回 404（不区分不存在与无权限）。
- `deadLinks` 改造：抽出与 ResolveByPath 共用的「路径下钻」解析函数（无 actor、纯存在性检查），替换现根级单段查询；单段目标行为与现状兼容（根级）。
- `httpapi`：新增 `GET /v1/documents/resolve`，绑定 `document.read`。
- 创建文档：slug 变为可选；为空时服务端按标题生成：
  - 拉丁/数字净化为 kebab（小写、非 `[a-z0-9-]` 剔除以空格/连字符折叠）；
  - 净化结果为空（纯 CJK 等）→ `doc-<ULID 前 8 位>`；
  - 生成结果与父级 slug 冲突 → 追加 `-2`、`-3`… **自增上限 20 次**，仍冲突返回 409（乐观探测 + 唯一索引兜底，并发撞车一方 409）。
- 改名：slug 仍可显式修改，校验与 409 不变。
- sitemap 生成改输出 slug 路径（需在 service 侧拼接祖先链或按树聚合后输出）。

### 前端
- 路由 `/docs/:id` → `/docs/:pathMatch(.*)*`，`/docs/:id/edit` → `/docs/:pathMatch(.*)*/edit`；`DocView` 先 `resolve?path=` 取得文档（含 id）再走既有渲染/评论/附件流程，`EditView` 取得 id 后走既有草稿/HEAD 流程。
- 树节点点击、面包屑、wikilink、深链接刷新、`router.push` 全部改 slug 路径；`EditView` 内部跳转（backToDoc RouterLink、save-exit 的 `location.href` 整页刷新、discard 按钮的路由名参数）统一改 slug 路径并消除整页刷新。
- `DocView` wikilink 点击：`findNodeBySlug` 改为按 slug 路径在可见树内匹配（单段 = 根级），与后端 deadLinks 同语义；DocView 登录 redirect 的 `/docs/${id}` 一并路径化。
- 创建对话框：slug 可选，留空时提交后由后端回显实际 slug（不在前端复制一套生成逻辑，AGENTS §1 单一事实来源）。
- 旧 `/docs/<id>` 深链接不再路由（alpha 阶段不要求兼容重定向）；落地前 `grep -rn "/docs/" frontend/src internal` 清点 id 形态出口（登录 redirect、sitemap、测试断言等）防漏改。

### 测试
- 后端：`ResolveByPath` 单元测试（根/嵌套/跳父、受限 404 掩护、匿名门闩）。
- 后端：resolve 端点 HTTP 测试（method/path/status/权限矩阵）。
- 后端：slug 自动生成测试（拉丁、纯 CJK 回退、冲突自增、自增上限 20 后 409）。
- 后端：slug 并发竞态——两个 actor 同时创建同标题根文档，断言最终 slug 不重复（唯一索引兜底，允许一方 409）。
- 后端：deadLinks 路径解析测试（单段根级兼容、多段嵌套命中、不存在死链；T2.4 三类结果快照更新）。
- 后端：sitemap 测试更新为 slug 路径断言。
- 前端：路由/导航测试改为路径式；doc-toc、面包屑、wikilink、EditView 加载、edit-leave、DocView/HomeView/side-tree-menu 的 `/docs/` 断言更新。
- 全量：`go test ./...`、`go vet ./...`、`npm test`、`npm run build`。

### 验收
- 创建文档后 URL 为可读 slug 路径（指定或自动生成）。
- 深链接直接打开/刷新按路径解析正常。
- 嵌套 slug 路径、改名后路径变化行为一致。
- 受限文档匿名访问仍 404 掩护。
- sitemap.xml 条目与公开 URL 形态一致。
- wikilink 死链判定与点击导航行为一致（同一路径语义）。

---

## 风险与备注

- 提交 3 为大重构：重写编辑器组件与测试；`[[` 补全/拖拽在 textarea 下需重新实现；ED-04/ED-07/ED-11 回归处置见提交 3「功能回归登记」，须在 ROADMAP 登记 backfill 或显式接受。
- 提交 4 为最大改动：后端 + 全前端路由 + 契约；建议 4 与 slug 自动生成拆开便于回滚。
- 提交 2 已按路由名寻址与提交 4 解耦；提交 4 落地时以 `grep -rn "/docs/" frontend/src internal` 清点 id 形态出口，防漏改。
- slug 自增探测存在并发竞态，最终一致性依赖 `(parent, slug)` 唯一索引；409 文案须引导用户重试或改 slug。
- 既有 14 个 npm 依赖漏洞（11 moderate / 2 high / 1 critical）不在本计划范围，另行治理。
- 本计划完成后，回填 ROADMAP 对应条目并把进度记入本文件与 04 计划。

## 收尾门禁

- `go test ./...`、`go vet ./...` 全绿。
- 总覆盖率不下降：基线 79.3% → 实测 79.5%（`go test ./... -coverprofile` + `go tool cover -func` total）；本项目全库覆盖率从未达到 85%（M8–M12 夜间批次豁免期积累，M13 未完成），≥85% 为全库遗留门禁，随 ROADMAP 门禁恢复统一治理，不在本计划单独背负。
- `npm test -- --run` 全绿、无未处理异常。
- `npm run build` 通过。
- `doc/ROADMAP.md`、`doc/04-严重问题修复计划.md` 状态与本计划一致。
- 契约变更登记：本计划涉及的 doc/00、doc/02 改动同步登记到 ROADMAP「契约变更登记」（登记为 C7；若提交 4 拆分则按拆分粒度分条登记）。
