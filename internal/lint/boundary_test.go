// 08 计划阶段 4：分层边界门禁（测试即门禁）。
// 规则：SQL 与 database/sql 只允许出现在 internal/database 与 migrations；
// service/httpapi 的非测试代码禁止触碰存储层实现细节。
package boundary

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 本测试位于 internal/lint/，仓库根为其上级的上级
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func walkGoFiles(t *testing.T, root string, fn func(path string)) {
	t.Helper()
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", "frontend", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			fn(p)
		}
		return nil
	})
}

func isTestFile(p string) bool { return strings.HasSuffix(p, "_test.go") }

// TestSQLConfinedToDatabaseLayer：非测试 .go 中，database/sql 与 SQL 执行调用
// 只允许出现在 internal/database 与 migrations 包。
func TestSQLConfinedToDatabaseLayer(t *testing.T) {
	root := repoRoot(t)
	sqlImport := regexp.MustCompile(`"database/sql"`)
	sqlExec := regexp.MustCompile(`ExecContext|QueryContext|QueryRowContext|PrepareContext`)
	allowed := func(p string) bool {
		return strings.Contains(p, filepath.Join("internal", "database")) ||
			strings.Contains(p, filepath.Join("migrations"))
	}
	var violations []string
	walkGoFiles(t, root, func(p string) {
		if isTestFile(p) || allowed(p) {
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if sqlImport.MatchString(src) {
			violations = append(violations, p+" imports database/sql")
		}
		if sqlExec.MatchString(src) {
			violations = append(violations, p+" executes raw SQL")
		}
	})
	for _, v := range violations {
		t.Errorf("分层边界违规: %s（SQL 只允许在 internal/database 与 migrations）", v)
	}
}

// TestStorePackageAbolished：internal/store 已收拢进 internal/database，不得回潮。
func TestStorePackageAbolished(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "store")); err == nil {
		t.Error("internal/store 应已删除（08 计划阶段 3）")
	}
	// 拼接构造避免匹配到本文件自身
	storeRef := regexp.MustCompile(`element-wiki/internal/` + "store")
	walkGoFiles(t, root, func(p string) {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if storeRef.Match(b) {
			t.Errorf("%s 仍引用已废除的 internal/store", p)
		}
	})
}

// TestServiceNarrowInterfaces：service 层不得直接依赖 sqlite 实现包
// （依赖必须经 internal/database 的接口契约注入；测试文件豁免）。
func TestServiceNarrowInterfaces(t *testing.T) {
	root := repoRoot(t)
	sqliteImpl := regexp.MustCompile(`element-wiki/internal/database/sqlite`)
	var violations []string
	walkGoFiles(t, filepath.Join(root, "internal", "service"), func(p string) {
		if isTestFile(p) {
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if sqliteImpl.Match(b) {
			violations = append(violations, p)
		}
	})
	for _, v := range violations {
		t.Errorf("service 层直接依赖 sqlite 实现（应只依赖 internal/database 接口）: %s", v)
	}
}
