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

var slugReLocal = slugReCompiled

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

// run 两阶段：抽取校验 → 写入（全失败时回收已建根）。
func (m *MarkdownImporter) run(ctx context.Context, jobID string,
	actor permission.Actor, zipPath string) (total, imported, failed int64, err error) {

	zr, zerr := zip.OpenReader(zipPath)
	if zerr != nil {
		return 0, 0, 0, zerr
	}
	defer zr.Close()

	var entries []zipEntry
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := filepath.ToSlash(f.Name)
		if strings.Contains(clean, "..") || path.IsAbs(clean) ||
			strings.ContainsAny(clean, "\\:") || strings.HasPrefix(clean, "/") {
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

	created := map[string]string{}   // dirPath 或 fileKey -> docID
	lastMDDoc := map[string]string{} // 目录 -> 最近一次创建的 md 文档 ID
	var createdRoots []string

	titleOf := func(base, content string) string {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
		}
		return base
	}

	ensureDirNode := func(dir string) (string, error) {
		if dir == "." || dir == "" {
			return "", nil
		}
		if id, ok := created[dir]; ok {
			return id, nil
		}
		parts := strings.Split(dir, "/")
		parent := ""
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
			var pid *string
			if parent != "" {
				pp := parent
				pid = &pp
			}
			nd, cerr := m.Svc.CreateDocument(ctx, actor, pid, sanitizeSlugL(part), sanitizeSlugL(part))
			if cerr != nil && !isConflictL(cerr) {
				return "", cerr
			}
			if nd == nil { // 冲突：复用既有同名节点
				existing, ferr := m.Svc.FindBySlug(ctx, actor, pidOf(parent), sanitizeSlugL(part))
				if ferr != nil {
					return "", ferr
				}
				nd = existing
			}
			created[cur] = nd.ID
			if parent == "" {
				createdRoots = append(createdRoots, nd.ID)
			}
			parent = nd.ID
		}
		return created[dir], nil
	}

	for _, e := range entries {
		if e.kind != kindMD {
			continue
		}
		dir := path.Dir(e.name)
		base := strings.TrimSuffix(path.Base(e.name), path.Ext(e.name))
		content := string(e.data)

		chainDir := dir
		isReadme := strings.EqualFold(path.Base(e.name), "readme.md")
		if isReadme && dir != "." {
			chainDir = path.Dir(dir)
		}

		parentID, derr := ensureDirNode(chainDir)
		if derr != nil {
			failed++
			m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
			m.rollbackRoots(ctx, createdRoots)
			m.Jobs.FinishImport(ctx, jobID, true, derr.Error())
			return total, imported, failed, nil
		}

		var pid *string
		if parentID != "" {
			pid = &parentID
		}

		slugBase := base
		if isReadme && dir != "." && dir != "/" {
			slugBase = path.Base(dir)
		}
		slug := sanitizeSlugL(slugBase)
		title := titleOf(slugBase, content)

		doc, cerr := m.Svc.CreateDocument(ctx, actor, pid, slug, title)
		switch {
		case cerr != nil && !isConflictL(cerr):
			failed++
		case cerr != nil:
			// slug 冲突：README 场景把正文提交到既有节点；普通文件计失败
			existing, ferr := m.Svc.FindBySlug(ctx, actor, pid, slug)
			if ferr != nil {
				failed++
				break
			}
			m.Svc.Commit(ctx, actor, existing.ID, "", content, "import(update)")
			imported++
			created[dir+"/"+base] = existing.ID
			if isReadme {
				created[dir] = existing.ID
			}
		default:
			m.Svc.Commit(ctx, actor, doc.ID, "", content, "import")
			imported++
			created[dir+"/"+base] = doc.ID
			if isReadme {
				created[dir] = doc.ID
			}
		}
		m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
	}

	for _, e := range entries {
		if e.kind == kindMD {
			continue
		}
		dir := path.Dir(e.name)
		docID := created[strings.TrimSuffix(e.name, path.Ext(e.name))]
		if docID == "" {
			docID = lastMDDoc[dir]
		}
		if docID == "" {
			docID = created[dir]
		}
		if docID == "" {
			failed++
			m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
			continue
		}
		if _, uerr := m.Svc.UploadAttachment(ctx, actor, docID,
			path.Base(e.name), bytes.NewReader(e.data)); uerr != nil {
			failed++
		} else {
			imported++
		}
		m.Jobs.UpdateImportProgress(ctx, jobID, total, imported, failed)
	}

	if imported == 0 && failed > 0 {
		m.rollbackRoots(ctx, createdRoots)
		m.Jobs.FinishImport(ctx, jobID, true, "全部条目导入失败")
		return total, imported, failed, nil
	}
	m.Jobs.FinishImport(ctx, jobID, false, "")
	return total, imported, failed, nil
}

func (m *MarkdownImporter) rollbackRoots(ctx context.Context, roots []string) {
	admin := m.actorWith(roots, permission.Admin)
	for _, r := range roots {
		_ = m.Svc.TrashDocument(ctx, admin, r)
	}
}

func (m *MarkdownImporter) actorWith(_ []string, role permission.Role) permission.Actor {
	_ = role
	return permission.NewActor("system-import", permission.CodesFor(permission.Admin))
}

func isConflictL(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "conflict")
}

func sanitizeSlugL(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == ' ' || r >= 0x80:
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "doc"
	}
	return out
}

var _ = model.JobDone
var _ = util.NewID

func pidOf(parent string) *string {
	if parent == "" {
		return nil
	}
	p := parent
	return &p
}
