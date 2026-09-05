package backupservice

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	docservice "element-wiki/internal/service/docservice"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

var slugReCompiled = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type zipEntry struct {
	name string // slash 路径
	data []byte
	kind entryKind
}

type entryKind int

const (
	kindMD entryKind = iota
	kindAsset
)

// MarkdownImporter 依赖文档域服务完成实际写入。
type MarkdownImporter struct {
	Jobs  store.ImportJobStore
	Svc   *docservice.Service
	actor func(id string) permission.Actor
	nowFn func() int64
}

// NewMarkdownImporter 构造；actor 工厂用于以请求者身份写入。
func NewMarkdownImporter(jobs store.ImportJobStore,
	svc *docservice.Service, actorOf func(userID string) permission.Actor) *MarkdownImporter {
	if actorOf == nil {
		actorOf = func(id string) permission.Actor {
			return permission.NewActor(id, permission.CodesFor(permission.Editor))
		}
	}
	return &MarkdownImporter{Jobs: jobs, Svc: svc, actor: actorOf,
		nowFn: util.NowMillis}
}

// StartMarkdownImport 异步执行导入，返回 job_id（202 契约）。
// onDone 在 goroutine 读取完 zipPath 之后回调（供调用方清理临时文件，T12.2）。
func (m *MarkdownImporter) StartMarkdownImport(ctx context.Context,
	actorID, zipPath string, onDone func()) (string, error) {
	id, err := m.Jobs.EnqueueImport(ctx, actorID)
	if err != nil {
		return "", err
	}
	go func() {
		total, imp, fail, rerr := m.run(context.Background(), id, m.actor(actorID), zipPath)
		m.Jobs.UpdateImportProgress(context.Background(), id, total, imp, fail)
		m.Jobs.FinishImport(context.Background(), id, rerr != nil, errStr(rerr))
		if onDone != nil {
			onDone()
		}
	}()
	return id, nil
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// run 隔离根导入（M17/T17.1）：全部条目落在 import-<短ID> 根容器下，
// 与站点既有文档零交集；slug 冲突一律计失败（绝不覆盖既有内容）。
// 全部失败（0 成功）→ trash 隔离根，零残留；部分失败保留并由 job 计数。
// 图片等媒体的正文相对引用不重写（契约 §11 backfill）：文件本体仍提取为附件。
func (m *MarkdownImporter) run(ctx context.Context, jobID string,
	actor permission.Actor, zipPath string) (total, imported, failed int64, err error) {

	zr, zerr := zip.OpenReader(zipPath)
	if zerr != nil {
		return 0, 0, 0, zerr
	}
	defer zr.Close()

	// 抽取：反斜杠宽容归一（Windows 打包器）→ 穿越拦截
	var entries []zipEntry
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := strings.ReplaceAll(filepath.ToSlash(f.Name), "\\", "/")
		if strings.Contains(clean, "..") || path.IsAbs(clean) ||
			strings.ContainsAny(clean, ":") || strings.HasPrefix(clean, "/") {
			failed++
			continue
		}
		rc, oerr := f.Open()
		if oerr != nil {
			failed++
			continue
		}
		var buf bytes.Buffer
		_, cerr := io.Copy(&buf, rc)
		rc.Close()
		if cerr != nil {
			failed++
			continue
		}
		kind := kindAsset
		if strings.EqualFold(path.Ext(clean), ".md") {
			kind = kindMD
		}
		entries = append(entries, zipEntry{name: clean, data: buf.Bytes(), kind: kind})
	}
	// md 文件优先于资产，确保目录容器与正文先落库
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].kind != entries[j].kind {
			return entries[i].kind == kindMD
		}
		return entries[i].name < entries[j].name
	})
	total = int64(len(entries))
	m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)

	if total == 0 {
		m.Jobs.FinishImport(ctx, jobID, true, "zip 为空或全部条目非法")
		return total, imported, failed, nil
	}

	// 隔离根：slug 显式 import-<短ID>（天然唯一），title 取 zip 文件名便于识别
	rootTitle := strings.TrimSuffix(filepath.Base(zipPath), ".zip")
	if strings.TrimSpace(rootTitle) == "" {
		rootTitle = "导入"
	}
	root, rerr := m.Svc.CreateDocument(ctx, actor, nil,
		"import-"+strings.ToLower(util.NewID()[:8]), rootTitle)
	if rerr != nil {
		m.Jobs.FinishImport(ctx, jobID, true, "隔离根创建失败: "+rerr.Error())
		return total, imported, failed, nil
	}

	created := map[string]string{} // 隔离根内相对路径（目录 或 stem）→ docID
	created["."] = root.ID
	readmeDone := map[string]bool{}

	titleOf := func(base, content string) string {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
		}
		return base
	}

	rollback := func(reason string) {
		_ = m.Svc.TrashDocument(ctx, m.systemActor(), root.ID)
		m.Jobs.FinishImport(ctx, jobID, true, reason)
	}

	ensureDirNode := func(dir string) (string, error) {
		if dir == "." || dir == "" {
			return root.ID, nil
		}
		if id, ok := created[dir]; ok {
			return id, nil
		}
		parts := strings.Split(dir, "/")
		parent := root.ID
		cur := ""
		for _, part := range parts {
			if cur == "" {
				cur = part
			} else {
				cur += "/" + part
			}
			if id, ok := created[cur]; ok {
				parent = id
				continue
			}
			// CJK 等不合格目录名传空 slug：由后端按标题自动生成（doc-<短ID> 回退）
			nd, cerr := m.Svc.CreateDocument(ctx, actor, &parent, sanitizeSlugL(part), part)
			if cerr != nil && !isConflictL(cerr) {
				return "", cerr
			}
			if nd == nil { // 防御：隔离根内 zip 自身大小写变体撞名 → 复用既有容器
				existing, ferr := m.Svc.FindBySlug(ctx, actor, &parent, sanitizeSlugL(part))
				if ferr != nil {
					return "", ferr
				}
				nd = existing
			}
			created[cur] = nd.ID
			parent = nd.ID
		}
		return created[dir], nil
	}

	// —— 正文（md 优先落库）——
	for _, e := range entries {
		if e.kind != kindMD {
			continue
		}
		dir := path.Dir(e.name)
		base := strings.TrimSuffix(path.Base(e.name), path.Ext(e.name))
		stemKey := strings.TrimSuffix(e.name, path.Ext(e.name))
		content := string(e.data)
		isReadme := strings.EqualFold(path.Base(e.name), "readme.md")

		containerID, derr := ensureDirNode(dir)
		if derr != nil {
			rollback(derr.Error())
			return total, imported, failed, nil
		}

		switch {
		case isReadme:
			// README → 所在目录容器的正文（显式提交，不依赖冲突路径）；变体只取首个
			if readmeDone[dir] {
				failed++
			} else if _, cerr := m.Svc.Commit(ctx, actor, containerID, "", content, "import"); cerr != nil {
				failed++
			} else {
				imported++
				readmeDone[dir] = true
				created[stemKey] = containerID
			}
		default:
			// 普通文档：slug 冲突一律计失败，绝不覆盖（T17.1 语义反转）
			slug := sanitizeSlugL(base)
			title := titleOf(base, content)
			doc, cerr := m.Svc.CreateDocument(ctx, actor, &containerID, slug, title)
			if cerr != nil {
				failed++
			} else if _, cerr := m.Svc.Commit(ctx, actor, doc.ID, "", content, "import"); cerr != nil {
				failed++
			} else {
				imported++
				created[stemKey] = doc.ID
			}
		}
		m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
	}

	// —— 媒体/附件：同 stem 文档优先，回退目录容器 ——
	for _, e := range entries {
		if e.kind == kindMD {
			continue
		}
		dir := path.Dir(e.name)
		stemKey := strings.TrimSuffix(e.name, path.Ext(e.name))
		targetID := created[stemKey]
		if targetID == "" {
			cid, derr := ensureDirNode(dir)
			if derr != nil {
				rollback(derr.Error())
				return total, imported, failed, nil
			}
			targetID = cid
		}
		if _, uerr := m.Svc.UploadAttachment(ctx, actor, targetID,
			path.Base(e.name), bytes.NewReader(e.data)); uerr != nil {
			failed++
		} else {
			imported++
		}
		m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
	}

	if imported == 0 && failed > 0 {
		rollback("全部条目导入失败")
		return total, imported, failed, nil
	}
	m.Jobs.FinishImport(ctx, jobID, false, "")
	return total, imported, failed, nil
}

// systemActor 回滚专用：清理不受请求者权限波动影响。
func (m *MarkdownImporter) systemActor() permission.Actor {
	return permission.NewActor("system-import", permission.CodesFor(permission.Admin))
}

func isConflictL(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "conflict")
}

// sanitizeSlugL 文件名/目录名 → 合法 slug；拉丁/数字保留、其余折叠为 '-'；
// 结果为空或含非法字符（如 CJK 未被折叠前）返回 ""，由调用方传空 slug 走后端自动生成。
func sanitizeSlugL(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if !slugReCompiled.MatchString(out) {
		return ""
	}
	return out
}

var _ = model.JobDone
