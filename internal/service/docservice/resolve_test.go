// 05 计划提交 4 验收：slug 自动生成、ResolveByPath 路径解析、deadLinks 路径语义、并发竞态。
package docservice

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

func isSlugFieldErr(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve) && ve.Field == "slug"
}

func TestSlugAutoGeneration(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, err := svc.CreateDocument(ctx, act, nil, "", "My   Great-Guide_!")
	if err != nil {
		t.Fatalf("拉丁净化创建: %v", err)
	}
	if d.Slug != "my-great-guide" {
		t.Errorf("拉丁净化 slug = %q, want my-great-guide", d.Slug)
	}

	cjk, err := svc.CreateDocument(ctx, act, nil, "", "入门指南")
	if err != nil {
		t.Fatalf("纯 CJK 创建: %v", err)
	}
	if !strings.HasPrefix(cjk.Slug, "doc-") || len(cjk.Slug) != 12 {
		t.Errorf("纯 CJK 回退 slug = %q, want doc-<ULID8>", cjk.Slug)
	}

	dup, err := svc.CreateDocument(ctx, act, nil, "", "My Great Guide")
	if err != nil {
		t.Fatalf("冲突自增创建: %v", err)
	}
	if dup.Slug != "my-great-guide-2" {
		t.Errorf("冲突自增 slug = %q, want my-great-guide-2", dup.Slug)
	}
	dup3, err := svc.CreateDocument(ctx, act, nil, "", "My Great Guide")
	if err != nil {
		t.Fatal(err)
	}
	if dup3.Slug != "my-great-guide-3" {
		t.Errorf("二次冲突自增 slug = %q, want my-great-guide-3", dup3.Slug)
	}

	if _, err := svc.CreateDocument(ctx, act, nil, "Bad Slug", "X"); !isSlugFieldErr(err) {
		t.Errorf("显式非法 slug 应校验失败: %v", err)
	}
}

func TestSlugAutoGenerationConflictCap(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	for i := 0; i < 21; i++ {
		slug := "cap-slug"
		if i > 0 {
			slug = fmt.Sprintf("cap-slug-%d", i+1)
		}
		if _, err := svc.CreateDocument(ctx, act, nil, slug, "x"); err != nil {
			t.Fatalf("占位 %d: %v", i, err)
		}
	}
	if _, err := svc.CreateDocument(ctx, act, nil, "", "Cap Slug"); !errors.Is(err, store.ErrConflict) {
		t.Errorf("自增上限后应 409 冲突, got %v", err)
	}
}

func TestResolveByPath(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	root, _ := svc.CreateDocument(ctx, act, nil, "guide", "Guide")
	child, _ := svc.CreateDocument(ctx, act, &root.ID, "setup", "Setup")
	grand, _ := svc.CreateDocument(ctx, act, &child.ID, "install", "Install")

	if _, err := svc.ResolveByPath(ctx, act, []string{"guide"}); err != nil {
		t.Errorf("根路径解析: %v", err)
	}
	d, err := svc.ResolveByPath(ctx, act, []string{"guide", "setup", "install"})
	if err != nil || d.ID != grand.ID {
		t.Errorf("嵌套路径解析 = %+v, %v", d, err)
	}
	if _, err := svc.ResolveByPath(ctx, act, []string{"guide", "nope"}); !IsNotFound(err) {
		t.Errorf("不存在路径应 404: %v", err)
	}
	if _, err := svc.ResolveByPath(ctx, act, []string{}); !IsNotFound(err) {
		t.Errorf("空路径应 404: %v", err)
	}
	if _, err := svc.ResolveByPath(ctx, act, []string{"guide", "setup", "install", "extra"}); !IsNotFound(err) {
		t.Errorf("超深路径应 404: %v", err)
	}
}

func TestResolveByPathVisibilityMasking(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	sec, _ := svc.CreateDocument(ctx, act, nil, "secret", "Secret")
	if err := svc.SetVisibility(ctx, act, sec.ID, model.VisibilityRestricted); err != nil {
		t.Fatal(err)
	}
	secChild, _ := svc.CreateDocument(ctx, act, &sec.ID, "sub", "Sub")

	if _, err := svc.ResolveByPath(ctx, viewer(), []string{"secret"}); !IsNotFound(err) {
		t.Errorf("restricted 对 viewer 应 404 掩护: %v", err)
	}
	if _, err := svc.ResolveByPath(ctx, viewer(), []string{"secret", "sub"}); !IsNotFound(err) {
		t.Errorf("restricted 祖先下子文档对 viewer 应 404: %v", err)
	}
	if _, err := svc.ResolveByPath(ctx, act, []string{"secret", "sub"}); err != nil {
		t.Errorf("editor 应可解析 restricted 子树: %v", err)
	}
	anon := permission.Anonymous(true)
	if _, err := svc.ResolveByPath(ctx, anon, []string{"secret"}); !IsNotFound(err) {
		t.Errorf("匿名 restricted 应 404: %v", err)
	}
	_ = secChild
}

func TestDeadLinksNestedPath(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	root, _ := svc.CreateDocument(ctx, act, nil, "guide", "G")
	svc.CreateDocument(ctx, act, &root.ID, "setup", "S")

	d, _ := svc.CreateDocument(ctx, act, nil, "main", "M")
	content := "见 [[setup]] 与 [[guide/setup]] 与 [[guide/missing]] 与 [[ghost]]\n"
	res, err := svc.Commit(ctx, act, d.ID, "", content, "init")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(res.DeadLinks, "guide/setup") {
		t.Errorf("多段命中不应判死链: %v", res.DeadLinks)
	}
	for _, want := range []string{"setup", "guide/missing", "ghost"} {
		if !slices.Contains(res.DeadLinks, want) {
			t.Errorf("应包含死链 %q: %v", want, res.DeadLinks)
		}
	}
}

func TestSlugConcurrentSameTitle(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	const n = 8
	var wg sync.WaitGroup
	slugs := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d, err := svc.CreateDocument(ctx, act, nil, "", "Concurrent Doc")
			if err == nil {
				slugs[i] = d.Slug
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	ok := 0
	for i, e := range errs {
		switch {
		case e == nil:
			if seen[slugs[i]] {
				t.Fatalf("并发下 slug 重复: %s", slugs[i])
			}
			seen[slugs[i]] = true
			ok++
		case !errors.Is(e, store.ErrConflict):
			t.Errorf("并发失败应 409 冲突, got %v", e)
		}
	}
	if ok == 0 {
		t.Fatal("并发下无任何成功创建")
	}
}
