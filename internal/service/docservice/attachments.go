// 附件受控上传与读取（T5.4/T5.5）。
package docservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

// AttachmentStore 注入面（含文件根目录与限制）。
func (s *Service) SetAttachmentStore(st store.AttachmentStore, dir, allowedExtensions string, maxMB int) {
	s.att = st
	s.attachDir = dir
	s.allowedExt = strings.Split(allowedExtensions, ",")
	for i := range s.allowedExt {
		s.allowedExt[i] = strings.ToLower(strings.TrimSpace(s.allowedExt[i]))
	}
	s.maxBytes = int64(maxMB) * 1024 * 1024
}

var ErrTooLarge = errors.New("file exceeds size limit")
var ErrBadType = errors.New("extension not allowed")

// UploadAttachment 受控提交：临时文件 → 校验 → 落盘 → DB；
// 任一步失败不留孤儿文件（AGENTS §6）。
func (s *Service) UploadAttachment(ctx context.Context, actor permission.Actor,
	docID, filename string, src io.Reader) (*model.Attachment, error) {
	if err := actor.Require(permission.AttachmentUpload); err != nil {
		return nil, err
	}
	if _, err := aliveDoc(ctx, s, docID); err != nil {
		return nil, err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if !s.extAllowed(ext) {
		return nil, fmt.Errorf("%w: .%s", ErrBadType, ext)
	}
	safe := sanitizeFilename(filepath.Base(filename))

	// 1) 临时文件
	tmp, err := os.CreateTemp(s.attachDir, "upload-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	hashW := sha256.New()
	limitR := io.LimitReader(src, s.maxBytes+1)
	size, err := io.Copy(io.MultiWriter(tmp, hashW), limitR)
	if err != nil {
		cleanup()
		return nil, err
	}
	if size > s.maxBytes {
		cleanup()
		return nil, fmt.Errorf("%w: %d > %d", ErrTooLarge, size, s.maxBytes)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return nil, err
	}

	// 2) 落定最终路径
	finalRel := filepath.Join(docID, fmt.Sprintf("%d-%s", timeNowUnix(), safe))
	finalAbs := filepath.Join(s.attachDir, finalRel)
	if err := os.MkdirAll(filepath.Dir(finalAbs), 0o755); err != nil {
		os.Remove(tmpName)
		return nil, err
	}
	if err := os.Rename(tmpName, finalAbs); err != nil {
		os.Remove(tmpName)
		return nil, err
	}

	// 3) DB 写入；失败回滚文件系统
	a := &model.Attachment{
		ID: util.NewID(), DocumentID: docID, Filename: safe,
		StoragePath: finalRel,
		MimeType:    mime.TypeByExtension("." + ext),
		Size:        size,
		SHA256:      hex.EncodeToString(hashW.Sum(nil)),
		UploadedBy:  actor.UserID(), CreatedAt: timeNowUnix(),
	}
	if err := s.att.CreateAttachment(ctx, a); err != nil {
		os.Remove(finalAbs)
		return nil, err
	}
	return a, nil
}

func (s *Service) extAllowed(ext string) bool {
	for _, a := range s.allowedExt {
		if a == ext {
			return true
		}
	}
	return false
}

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '\'', '"':
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name
}

// ListAttachments / OpenAttachment / DeleteAttachment。
func (s *Service) ListAttachments(ctx context.Context, actor permission.Actor,
	docID string) ([]*model.Attachment, error) {
	if err := actor.Require(permission.AttachmentRead); err != nil {
		return nil, err
	}
	if _, err := aliveDoc(ctx, s, docID); err != nil {
		return nil, err
	}
	return s.att.ListAttachments(ctx, docID)
}

// OpenAttachment 返回元数据并校验文档可读性；调用方负责打开物理文件。
func (s *Service) GetAttachment(ctx context.Context, actor permission.Actor,
	id string) (*model.Attachment, error) {
	a, err := s.att.GetAttachment(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(permission.AttachmentRead); err != nil {
		return nil, err
	}
	d, err := aliveDoc(ctx, s, a.DocumentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureReadable(ctx, actor, d.ID); err != nil {
		return nil, err
	}
	return a, nil
}

// DeleteAttachment 先删行后删文件；行不存在返回 NotFound。
func (s *Service) DeleteAttachment(ctx context.Context, actor permission.Actor, id string) error {
	a, err := s.att.GetAttachment(ctx, id)
	if err != nil {
		return err
	}
	if err := actor.Require(permission.AttachmentDelete); err != nil {
		return err
	}
	if _, err := aliveDoc(ctx, s, a.DocumentID); err != nil && !IsNotFound(err) {
		return err
	}
	if err := s.att.DeleteAttachment(ctx, id); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.attachDir, a.StoragePath)) // 尽力清理
	return nil
}

func timeNowUnix() int64 { return nowMillis() / 1000 }
