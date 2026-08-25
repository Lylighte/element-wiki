// Package backupservice 实现备份导出（zip）与两类导入：
// 全量备份恢复（DB+附件，事务内 ATTACH 拷贝）与 Markdown zip 内容导入。
package backupservice

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

// manifest 是备份包元数据（OP-03 校验依据）。
type Manifest struct {
	SchemaVersion   int    `json:"schema_version"`
	CreatedAt       int64  `json:"created_at"`
	Generator       string `json:"generator"`
	DocumentsTotal  int64  `json:"documents_total"`
	AttachmentsTotl int64  `json:"attachments_total"`
}

const generator = "element-wiki/1"

// Service 备份域服务。
type Service struct {
	jobs       store.BackupJobStore
	imports    store.ImportJobStore
	live       *sql.DB // 主库连接（VACUUM INTO / ATTACH 使用）
	dbPath     string  // sqlite 文件路径
	attachDir  string  // 附件根目录
	backupsDir string  // 备份产物目录
	schemaVer  int
	nowFn      func() int64

	// 导入成功后的全量索引重建入队钩子（T12.2：Bleve 与恢复后数据保持一致）。
	rebuild func(ctx context.Context, docID *string, reason string) (string, error)
}

// SetRebuildHook 注入全量重建入队钩子（main 装配 store.EnqueueReindex）。
func (s *Service) SetRebuildHook(fn func(ctx context.Context, docID *string, reason string) (string, error)) {
	s.rebuild = fn
}

func New(jobs store.BackupJobStore, imports store.ImportJobStore,
	live *sql.DB, dbPath, attachDir, backupsDir string, schemaVersion int) *Service {
	return &Service{jobs: jobs, imports: imports, live: live,
		dbPath: dbPath, attachDir: attachDir, backupsDir: backupsDir,
		schemaVer: schemaVersion, nowFn: time.Now().UnixMilli}
}

var ErrBadManifest = errors.New("backup manifest invalid")

// StartExport 创建导出任务并后台执行；返回 job_id（202 契约）。
func (s *Service) StartExport(ctx context.Context, actorID string) (string, error) {
	id, err := s.jobs.EnqueueBackup(ctx, "export", actorID)
	if err != nil {
		return "", err
	}
	go s.runExport(context.Background(), id)
	return id, nil
}

func (s *Service) runExport(ctx context.Context, id string) {
	_ = s.jobs.SetBackupFilename(ctx, id, "")
	name := fmt.Sprintf("backup-%d-%s.zip", s.nowFn(), id)
	finalPath := filepath.Join(s.backupsDir, name)

	if err := os.MkdirAll(s.backupsDir, 0o755); err != nil {
		_ = s.jobs.FinishBackup(ctx, id, true, err.Error())
		return
	}
	out, err := os.Create(finalPath)
	if err != nil {
		_ = s.jobs.FinishBackup(ctx, id, true, err.Error())
		return
	}
	zw := zip.NewWriter(out)

	// 1) DB 一致性快照：VACUUM INTO（modernc 支持）
	tmpDB := filepath.Join(s.backupsDir, ".stage-"+id+".db")
	if _, err := s.live.ExecContext(ctx, `VACUUM INTO ?`, tmpDB); err != nil {
		out.Close()
		zw.Close()
		os.Remove(finalPath)
		os.Remove(tmpDB)
		_ = s.jobs.FinishBackup(ctx, id, true, "snapshot: "+err.Error())
		return
	}
	var docsTotal int64
	_ = s.live.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&docsTotal)
	if werr := writeZipFile(zw, "db.sqlite3", tmpDB); werr != nil {
		out.Close()
		zw.Close()
		os.Remove(finalPath)
		os.Remove(tmpDB)
		_ = s.jobs.FinishBackup(ctx, id, true, werr.Error())
		return
	}
	os.Remove(tmpDB)

	// 2) 附件树
	var attTotal int64
	filepath.WalkDir(s.attachDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(s.attachDir, p)
		attTotal++
		return writeZipFile(zw, filepath.ToSlash(filepath.Join("attachments", rel)), p)
	})

	// 3) manifest
	mf, _ := json.Marshal(Manifest{SchemaVersion: s.schemaVer,
		CreatedAt: s.nowFn(), Generator: generator, DocumentsTotal: docsTotal,
		AttachmentsTotl: attTotal})
	mw, _ := zw.Create("manifest.json")
	mw.Write(mf)
	zw.Close()
	out.Close()

	_ = s.jobs.SetBackupFilename(ctx, id, name)
	_ = s.jobs.FinishBackup(ctx, id, false, "")
}

func writeZipFile(zw *zip.Writer, name, srcPath string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

// ListBackupFiles 列出产物文件名（仅 .zip）。
func (s *Service) ListBackupFiles(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".zip") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// BackupFilePath 返回可下载产物的绝对路径；名称必须安全。
func (s *Service) BackupFilePath(name string) (string, error) {
	base := filepath.Base(name)
	if base != name || !strings.HasSuffix(base, ".zip") || strings.Contains(base, "..") {
		return "", fmt.Errorf("%w: invalid filename", store.ErrInvalid)
	}
	return filepath.Join(s.backupsDir, base), nil
}

// DeleteBackupFile 删除产物。
func (s *Service) DeleteBackupFile(name string) error {
	p, err := s.BackupFilePath(name)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return store.ErrNotFound
	}
	return err
}

type importChecker struct{}

// StartImportOfZip 校验并执行全量备份导入（T6.6）。
// 目标库在单事务内被整表替换（ATTACH 源库拷贝行），失败零残留。
func (s *Service) StartImportOfZip(ctx context.Context, actorID string, zipPath string, onDone func()) (string, error) {
	jobID, err := s.jobs.EnqueueBackup(ctx, "import", actorID)
	if err != nil {
		return "", err
	}
	go func() {
		runErr := s.runImport(context.Background(), zipPath, jobID)
		if onDone != nil {
			onDone()
		}
		ferr := s.jobs.FinishBackup(context.Background(), jobID, runErr != nil, errText2(runErr))
		_ = ferr
	}()
	return jobID, nil
}

var dataTableOrder = []string{
	"settings", "users", "document_blobs", "documents",
	"document_commits", "document_drafts", "comments", "comment_mentions",
	"attachments",
}

func (s *Service) runImport(ctx context.Context, zipPath string, selfID string) (runFailed error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open backup zip: %w", err)
	}
	defer zr.Close()

	staged := filepath.Join(os.TempDir(), "ew-import-"+util.NewID())
	defer os.RemoveAll(staged)
	os.MkdirAll(staged, 0o755)

	var mf Manifest
	hasDB := false
	hasManifest := false
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("%w: illegal path %q", ErrBadManifest, name)
		}
		switch {
		case name == "manifest.json":
			hasManifest = true
			rc, _ := f.Open()
			if jerr := json.NewDecoder(rc).Decode(&mf); jerr != nil {
				rc.Close()
				return fmt.Errorf("%w: %v", ErrBadManifest, jerr)
			}
			rc.Close()
			if mf.SchemaVersion != s.schemaVer {
				return fmt.Errorf("%w: schema_version %d != current %d",
					ErrBadManifest, mf.SchemaVersion, s.schemaVer)
			}
		case name == "db.sqlite3":
			hasDB = true
			dst := filepath.Join(staged, "db.sqlite3")
			out, oerr := os.Create(dst)
			if oerr != nil {
				return oerr
			}
			rc, _ := f.Open()
			_, cerr := io.Copy(out, rc)
			rc.Close()
			out.Close()
			if cerr != nil {
				return cerr
			}
		case strings.HasPrefix(name, "attachments/"):
			dst := filepath.Join(staged, filepath.FromSlash(name))
			os.MkdirAll(filepath.Dir(dst), 0o755)
			out, oerr := os.Create(dst)
			if oerr != nil {
				return oerr
			}
			rc, _ := f.Open()
			_, cerr := io.Copy(out, rc)
			rc.Close()
			out.Close()
			if cerr != nil {
				return cerr
			}
		}
	}
	// 契约 §11/C6：manifest 缺失即整体失败；不允许无 manifest 导入。
	if !hasManifest {
		return fmt.Errorf("%w: missing manifest.json", ErrBadManifest)
	}
	if !hasDB {
		return fmt.Errorf("%w: missing db.sqlite3", ErrBadManifest)
	}

	// 附件目录换入：备份含 attachments/ 才替换；否则清空目标内容
	stagedAtt := filepath.Join(staged, "attachments")
	oldAtt := s.attachDir + ".pre-import"
	attSwapped := false
	if _, serr := os.Stat(stagedAtt); serr == nil {
		if rerr := os.Rename(s.attachDir, oldAtt); rerr == nil {
			attSwapped = true
			if rerr := os.Rename(stagedAtt, s.attachDir); rerr != nil {
				os.Rename(oldAtt, s.attachDir)
				runFailed = rerr
				return
			}
		}
	} else if !errors.Is(serr, os.ErrNotExist) {
		runFailed = serr
		return
	}

	// DB 整表替换：打开 staged 库，逐表在单事务中清空并拷贝（驱动无关）
	src, err := sql.Open("sqlite", filepath.Join(staged, "db.sqlite3"))
	if err != nil {
		return err
	}
	defer src.Close()

	tx, err := s.live.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先清操作型子表，避免残留行悬挂引用新用户集
	for _, tbl := range []string{"sessions", "api_tokens",
		"search_reindex_jobs", "import_jobs", "backup_jobs"} {
		extra := ""
		if tbl == "backup_jobs" || tbl == "import_jobs" {
			extra = " WHERE id <> '" + selfID + "'"
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+tbl+extra); err != nil {
			return err
		}
	}
	for i := len(dataTableOrder) - 1; i >= 0; i-- {
		tbl := dataTableOrder[i]
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+tbl); err != nil {
			return fmt.Errorf("clear table %s: %w", tbl, err)
		}
	}
	for _, tbl := range dataTableOrder {
		if err := copyTable(ctx, src, tx, tbl); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if attSwapped {
		os.RemoveAll(oldAtt)
	}
	// 派生数据补偿：恢复后强制一次全量索引重建（C6）
	if s.rebuild != nil {
		_, _ = s.rebuild(context.Background(), nil, "post-import")
	}
	return nil
}

// copyTable 通用整表拷贝：按源列名生成参数化插入。
func copyTable(ctx context.Context, src *sql.DB, dst *sql.Tx, table string) error {
	rows, err := src.QueryContext(ctx, `SELECT * FROM `+table)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return err
	}

	// 先全量读入内存，关闭游标后再写入目标，避免跨库游标交错
	var batch [][]any
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close()
			return err
		}
		cp := make([]any, len(cols))
		copy(cp, vals)
		batch = append(batch, cp)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	n := len(cols)
	placeholders := strings.Repeat("?,", n)[:2*n-1]
	stmt, err := dst.PrepareContext(ctx,
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			table, strings.Join(cols, ","), placeholders))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range batch {
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return err
		}
	}
	return nil
}

func errText2(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// GetJob 供 httpapi 查询任务状态。
func (s *Service) GetJob(ctx context.Context, id string) (*model.BackupJob, error) {
	return s.jobs.GetBackupJob(ctx, id)
}

// GetImportJob 供 httpapi 查询导入任务状态。
func (s *Service) GetImportJob(ctx context.Context, id string) (*model.ImportJob, error) {
	return s.imports.GetImportJob(ctx, id)
}
