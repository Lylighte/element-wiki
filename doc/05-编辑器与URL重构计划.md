# 05 编辑器与 URL 重构计划

> 状态：已批准待执行（alpha 阶段，v1 契约可改）。
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
  - 点击后置 `leaveConfirmed = true`，`router.push('/docs/:id')`。
  - 不 flush 草稿、不弹离开确认、不提交 commit。
- i18n 新增 `doc.discard`：
  - zh-CN：`放弃修改退出`
  - en：`Discard & exit`
- 两 locale 同步，key parity 测试保持绿。

### 验收
- 脏状态下点"放弃修改退出"直接离开，不保存、不弹确认。
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

#### 依赖清理
- `package.json` 移除 `@tiptap/*`、`tiptap-markdown` 及不再使用的类型依赖；`npm ci` 后重新锁定。

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
- DM-02 补充：文档公开 URL 为 `/docs/<祖先slug>/…/<slug>`；slug 父级内唯一；创建时 slug 可选，未提供按"拉丁净化+短 ID 回退"生成。

#### doc/02-后端API设计.md
- 新增 `GET /v1/documents/resolve?path=<slug路径>`：按路径解析文档，返回 `{ document, render }`；权限同 document.read；受限文档对匿名 404 掩护。
- 现有 `/v1/documents/{id}` 系列保留用于编辑/管理操作（按 id 寻址）。

#### doc/01-数据库表设计.md
- 无表结构变化（slug 列与 `(COALESCE(parent_id,''), slug)` 唯一索引已存在）。

### 后端
- `docservice.ResolveByPath(ctx, actor, path []string)`：从根逐段 `GetBySlug` 下钻；任一段不可见返回 404（不区分不存在与无权限）。
- `httpapi`：新增 `GET /v1/documents/resolve`，绑定 `document.read`。
- 创建文档：slug 变为可选；为空时服务端按标题生成：
  - 拉丁/数字净化为 kebab（小写、非 `[a-z0-9-]` 剔除以空格/连字符折叠）；
  - 净化结果为空（纯 CJK 等）→ `doc-<ULID 前 8 位>`；
  - 生成结果与父级 slug 冲突 → 追加 `-2`、`-3`… 直至唯一（或返回 409 由前端提示）。
- 改名：slug 仍可显式修改，校验不变。

### 前端
- 路由 `/docs/:id` → `/docs/:pathMatch(.*)*`；`DocView`/`EditView` 改用 `resolve?path=` 加载。
- 树节点点击、面包屑、wikilink、深链接刷新、`router.push` 全部改 slug 路径。
- 创建对话框：slug 可选，留空时前端预览自动生成值或提交后由后端回显。
- 旧 `/docs/<id>` 深链接不再路由（alpha 阶段不要求兼容重定向）。

### 测试
- 后端：`ResolveByPath` 单元测试（根/嵌套/跳父、slug 冲突、受限 404 掩护、匿名门闩）。
- 后端：resolve 端点 HTTP 测试（method/path/status/权限矩阵）。
- 后端：slug 自动生成测试（拉丁、纯 CJK 回退、冲突自增）。
- 前端：路由/导航测试改为路径式；doc-toc、面包屑、wikilink、EditView 加载测试更新。
- 全量：`go test ./...`、`go vet ./...`、`npm test`、`npm run build`。

### 验收
- 创建文档后 URL 为可读 slug 路径（指定或自动生成）。
- 深链接直接打开/刷新按路径解析正常。
- 嵌套 slug 路径、改名后路径变化行为一致。
- 受限文档匿名访问仍 404 掩护。

---

## 风险与备注

- 提交 3 为大重构：重写编辑器组件与测试；`[[` 补全/拖拽在 textarea 下需重新实现。
- 提交 4 为最大改动：后端 + 全前端路由 + 契约；建议 4 与 slug 自动生成拆开便于回滚。
- 既有 14 个 npm 依赖漏洞（11 moderate / 2 high / 1 critical）不在本计划范围，另行治理。
- 本计划完成后，回填 ROADMAP 对应条目并把进度记入本文件与 04 计划。

## 收尾门禁

- `go test ./...`、`go vet ./...` 全绿。
- `npm test -- --run` 全绿、无未处理异常。
- `npm run build` 通过。
- `doc/ROADMAP.md`、`doc/04-严重问题修复计划.md` 状态与本计划一致。
- 契约变更登记：本计划涉及的 doc/00、doc/02 改动同步登记到 ROADMAP「契约变更登记」。
