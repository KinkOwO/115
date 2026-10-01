package savecontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// 身份口径守卫（2026-10-01，事后于 4171f55 的批量替换误伤）。
//
// `savecontract` 把身份分成三级（见包注释）：L1 存档身份 = `Identity()` / `SaveIdentity()`，
// L3 运行时校验 = 内层哈希 `Checksum`。两类值长得很像（都是 64 位 hex），于是**批量替换**
// 很容易把 L3 的比较改成契约身份 —— 4171f55 就这样改坏了两处（`loot/session.go` 的
// `Death()` 与 `loot/pickup.go` 的货币检查），后果是奥德赛副本每只小怪死亡结算直接报错、
// 清完怪不开门；同一次提交还漏了两处 L1 写入侧（`unseal` 的 version 实参、
// `quest/odyssey_mainline.go` 的 `ClearQuests`）。
//
// 本测试用 AST 扫源码，钉住两条规则：
//
//	规则 1（真哈希对象不许与契约身份比）：`X.Source != …SaveIdentity()`，其中 X 是**存放内层
//	        真哈希的目录/会话对象**（见 runtimeHashSources）。这类比较（L3）必须用 `.Checksum`，
//	        4171f55 正是把 `s.Currency.Source` 那两处改成了 SaveIdentity() 而恒不相等。
//	        只认这一份「已确认存真哈希」的名单：其余 `.Source`（回执 receipt/out/plan 字段、
//	        quest.Index 这类构造时就写契约身份的令牌）都是 L1，不在此列，避免误报。
//	规则 2（L1 不许用真哈希）：身份参数函数（写 DB version / 回执身份）**不得**收到 `.Checksum`。
//	规则 3（不许写恒等式）：`…SaveIdentity() ==/!= …Identity()` 两侧恒等（`SaveIdentity()` 是
//	        常量函数），这种条件永远为真/假、等于不写。命中即失败 —— 想要 L3 校验就比 `.Checksum`。
//
// 新增同类写法会立刻让本测试失败 —— 要么改对，要么在名单里写明理由后登记。
var (
	// runtimeHashSources 是「.Source 里存的是内层真哈希」的接收者（点号连接的字段路径）。
	// 事故面就在这里：拿它跟 SaveIdentity() 比一定不相等。
	runtimeHashSources = []string{
		"Currency.Source", // loot.Session/Service 的 OdysseyCurrency（ImportOdysseyCurrency 赋 a.Snapshot().Checksum）
		"Odyssey.Source",  // catalog.OdysseyGrowth（ImportOdysseyGrowth 赋 a.Snapshot().Checksum）
	}

	// identityParameterFuncs 的「版本 / 身份」实参必须是契约身份（L1）。
	identityParameterFuncs = map[string]bool{
		"CommitCharacterEvent":        true,
		"CommitAccountMaterialEvent":  true,
		"CommitCharacterPremiumEvent": true,
		"CommitQuestReward":           true,
		"CompleteQuestObjective":      true,
		"RecordQuestMapClear":         true,
		"MarkMeetNPCQuest":            true,
		"ClearQuests":                 true,
		"ClearActQuests":              true,
		// 会话方法：第 2 个实参是 unseal 的 version（写 character_events.config_version）。
		"unsealRandomOption": true,
	}
)

func render(fset *token.FileSet, node ast.Node) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, node); err != nil {
		return ""
	}
	return b.String()
}

func isSaveIdentityCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok && fun.Sel.Name == "SaveIdentity" {
		return true
	}
	return false
}

func isChecksumSelector(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Checksum"
}

// isIdentityCall 判断 `Identity()`（本包内）或 `savecontract.Identity()`（跨包）调用。
func isIdentityCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == "Identity"
	case *ast.SelectorExpr:
		return fun.Sel.Name == "Identity"
	}
	return false
}

func endsWithSourceSelector(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Source"
}

// runtimeHashSourceKey 把 `s.Currency.Source` / `session.Currency.Source` / `s.Odyssey.Source`
// 这类表达式压成 `Currency.Source` 形式的键，用于与 runtimeHashSources 名单比对。
func runtimeHashSourceKey(e ast.Expr) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Source" {
		return ""
	}
	var parts []string
	cur := sel.X
	for {
		switch v := cur.(type) {
		case *ast.SelectorExpr:
			parts = append([]string{v.Sel.Name}, parts...)
			cur = v.X
			continue
		case *ast.Ident:
			parts = append([]string{v.Name}, parts...)
		}
		break
	}
	if len(parts) == 0 {
		return ""
	}
	// 去掉接收者本身（s / session / Service 变量名），只看字段路径。
	if len(parts) > 1 {
		parts = parts[1:]
	}
	return strings.Join(parts, ".") + ".Source"
}

// sourceFiles 收集仓库里需要扫描的 Go 源文件（跳过测试与运行时产物目录）。
func sourceFiles(t *testing.T, roots ...string) []string {
	t.Helper()
	var out []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "update-backup", "update-cache", "runtime", "pgdata", "vendor":
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			out = append(out, path)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	sort.Strings(out)
	return out
}

func TestIdentityUsageRules(t *testing.T) {
	files := sourceFiles(t, filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd"))
	if len(files) == 0 {
		t.Fatal("no Go source files found; the guard would silently pass")
	}
	var l3, l1, tautology []string
	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		rel, _ := filepath.Rel(filepath.Join("..", ".."), path)
		rel = filepath.ToSlash(rel)
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BinaryExpr:
				if node.Op != token.NEQ && node.Op != token.EQL {
					return true
				}
				// 规则 3：`X.SaveIdentity() ==/!= Y.Identity()` —— 两侧恒等，条件永不生效。
				if (isSaveIdentityCall(node.X) && isIdentityCall(node.Y)) ||
					(isSaveIdentityCall(node.Y) && isIdentityCall(node.X)) {
					tautology = append(tautology, fmt.Sprintf("%s: %s  —— 两侧恒等（SaveIdentity() 就是 Identity()），该条件永远不生效",
						rel, render(fset, node)))
					return true
				}
				if !isSaveIdentityCall(node.X) && !isSaveIdentityCall(node.Y) {
					return true
				}
				for _, side := range []ast.Expr{node.X, node.Y} {
					key := runtimeHashSourceKey(side)
					if key == "" || !slices.Contains(runtimeHashSources, key) {
						continue
					}
					l3 = append(l3, fmt.Sprintf("%s: %s  —— %s 里存的是内层真哈希，L3 比较必须用 .Checksum",
						rel, render(fset, node), key))
				}
			case *ast.CallExpr:
				name := ""
				switch fun := node.Fun.(type) {
				case *ast.Ident:
					name = fun.Name
				case *ast.SelectorExpr:
					name = fun.Sel.Name
				}
				if !identityParameterFuncs[name] {
					return true
				}
				for _, arg := range node.Args {
					if isChecksumSelector(arg) {
						l1 = append(l1, fmt.Sprintf("%s: %s(… %s …)  —— L1 身份参数必须用契约身份（SaveIdentity()/Identity()/role.ConfigVersion）",
							rel, name, render(fset, arg)))
					}
				}
			}
			return true
		})
	}
	if len(l3) != 0 {
		t.Errorf("发现 %d 处「以契约身份做 L3 比较」：\n  %s\n该对象的 Source 存的是内层真哈希，比较必须用 .Checksum。",
			len(l3), strings.Join(l3, "\n  "))
	}
	if len(l1) != 0 {
		t.Errorf("发现 %d 处「L1 身份参数收到内层哈希」：\n  %s\n这些函数的 version 会写进数据库/回执并按契约身份读回，传 Checksum 会恒不匹配。",
			len(l1), strings.Join(l1, "\n  "))
	}
	if len(tautology) != 0 {
		t.Errorf("发现 %d 处「恒真/恒假的身份比较」：\n  %s\nSaveIdentity() 是常量函数，与 Identity() 比较等于没写；要校验目录来源请比 .Checksum。",
			len(tautology), strings.Join(tautology, "\n  "))
	}
}
