// T9.1 验收：commit 可选 title 与正文同事务写入；冲突零写入；非法标题 422。
package httpapi

import (
	"testing"
)

func TestCommitWithOptionalTitle(t *testing.T) {
	e := newEnv(t)
	id := createDoc(t, e, "title-c", "Title C", nil)

	// 不带 title 字段 → 标题不变
	resp, body := e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": "", "content": "body1", "message": "m"})
	mustStatus(t, resp.StatusCode, 201, body)
	head := body["commit"].(map[string]any)["id"].(string)

	_, body = e.do("GET", "/v1/documents/"+id, "editor", nil)
	if got := body["document"].(map[string]any)["title"]; got != "Title C" {
		t.Fatalf("无 title 提交改动了标题: %v", got)
	}

	// 带 title → 同事务更新
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": head, "content": "body2", "message": "m", "title": "Title C2"})
	mustStatus(t, resp.StatusCode, 201, body)
	_, body = e.do("GET", "/v1/documents/"+id, "editor", nil)
	if got := body["document"].(map[string]any)["title"]; got != "Title C2" {
		t.Fatalf("commit title 未生效: %v", got)
	}

	// 过期 base + title → 409 且标题零写入（冲突判定先行）
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": head, "content": "x", "message": "m", "title": "T3"})
	mustStatus(t, resp.StatusCode, 409, body)
	_, body = e.do("GET", "/v1/documents/"+id, "editor", nil)
	if got := body["document"].(map[string]any)["title"]; got != "Title C2" {
		t.Fatalf("冲突路径污染了标题: %v", got)
	}

	// 空 title → 422 fields.title
	head2 := body["document"].(map[string]any)["head_commit_id"].(string)
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": head2, "content": "y", "message": "m", "title": ""})
	mustStatus(t, resp.StatusCode, 422, body)
	if _, ok := body["fields"].(map[string]any)["title"]; !ok {
		t.Fatalf("fields 缺少 title: %v", body)
	}

	// viewer 提交 → 403
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "viewer",
		map[string]any{"base_commit_id": head2, "content": "z", "message": "m"})
	mustStatus(t, resp.StatusCode, 403, body)
}
