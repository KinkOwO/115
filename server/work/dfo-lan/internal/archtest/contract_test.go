package archtest

// 依赖边守卫（Architecture Contract Guard）
//
// 规则真源：server/work/dfo-lan/docs/architecture-contract.md
//
// 本测试把契约 §2 的依赖规则变成可执行的断言：
//   R1 纯基础设施不得 import 任何 L3 领域；
//   R2 L3 领域不得 import internal/storage；
//   R4 L3 领域之间默认禁止互相 import；
//   R6 L3 领域不得 import L4 组合/工具层。
//
// 允许清单 = 契约 §7 的例外清单，只减不增。任何**新增**的反向依赖都会让本测试失败；
// 消除一条例外后必须同步删除对应允许条目（否则下面的"过期条目"检查会失败）。

import (
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const importPrefix = "dfolan/"

// L0/L1 基础设施：不承载业务语义，禁止 import 领域（R1）。
var infraPackages = map[string]bool{
	"internal/game/wire":     true,
	"internal/game/protocol": true,
	"internal/catalog":       true,
	"internal/catalog/pvf":   true,
	"internal/derivedcache":  true,
	"internal/savecontract":  true,
}

// L2 持久化：领域不得直接依赖（R2）。
const persistencePackage = "internal/storage"

// L3 业务领域。
var domainPackages = map[string]bool{
	"internal/character":   true,
	"internal/inventory":   true,
	"internal/loot":        true,
	"internal/quest":       true,
	"internal/dungeon":     true,
	"internal/world":       true,
	"internal/cashshop":    true,
	"internal/progression": true,
	"internal/adventure":   true,
	"internal/legion":      true,
	"internal/npcpresence": true,
	"internal/profileskin": true,
	"internal/rosterbg":    true,
}

// L4 组合与工具层：领域不得反向依赖（R6）。
var layer4Packages = map[string]bool{
	"internal/gamedata":       true,
	"internal/managementdata": true,
	"internal/admin":          true,
	"internal/channelrefresh": true,
}

// allowedEdges 是契约 §7 例外清单，只减不增。键为 import 方，值为被允许的例外目标。
var allowedEdges = map[string]map[string]bool{
	// R2 + R4 例外：领域 -> storage / 领域 -> 领域
	"internal/character": {
		"internal/storage":     true, // R2
		"internal/adventure":   true, // R4
		"internal/dungeon":     true, // R4
		"internal/inventory":   true, // R4
		"internal/progression": true, // R4
	},
	"internal/inventory": {
		"internal/storage": true, // R2
	},
	"internal/loot": {
		"internal/storage":   true, // R2
		"internal/adventure": true, // R4
		"internal/cashshop":  true, // R4
		"internal/dungeon":   true, // R4
		"internal/inventory": true, // R4
	},
	"internal/quest": {
		"internal/storage":     true, // R2
		"internal/character":   true, // R4
		"internal/dungeon":     true, // R4
		"internal/inventory":   true, // R4
		"internal/progression": true, // R4
	},
	"internal/world": {
		"internal/storage": true, // R2
	},
	"internal/cashshop": {
		"internal/storage":   true, // R2
		"internal/inventory": true, // R4
	},
	"internal/legion": {
		"internal/dungeon": true, // R4
	},
}

func isAllowed(from, to string) bool {
	return allowedEdges[from][to]
}

// loadInternalEdges 返回 internal 下各包的模块内 import（不含 _test.go）。
func loadInternalEdges(t *testing.T) map[string][]string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	moduleRoot := filepath.Dir(filepath.Dir(wd)) // <module>/internal/archtest -> <module>
	root := filepath.Join(moduleRoot, "internal")

	edges := map[string][]string{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		pkg, berr := build.ImportDir(path, 0)
		if berr != nil {
			return nil // 目录没有可构建的 Go 文件（或不参与构建）
		}
		rel, rerr := filepath.Rel(moduleRoot, path)
		if rerr != nil {
			return rerr
		}
		from := filepath.ToSlash(rel)
		if from == "internal/archtest" {
			return nil
		}
		var deps []string
		for _, imp := range pkg.Imports {
			if !strings.HasPrefix(imp, importPrefix+"internal/") {
				continue
			}
			deps = append(deps, strings.TrimPrefix(imp, importPrefix))
		}
		sort.Strings(deps)
		edges[from] = deps
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal: %v", err)
	}
	return edges
}

func TestDependencyContract(t *testing.T) {
	edges := loadInternalEdges(t)

	var violations []string
	seenAllowed := map[string]bool{}

	for from, deps := range edges {
		for _, to := range deps {
			if from == to {
				continue
			}
			var rule string
			switch {
			case infraPackages[from] && domainPackages[to]:
				rule = "R1 基础设施不得依赖领域"
			case domainPackages[from] && to == persistencePackage:
				rule = "R2 领域不得依赖 storage"
			case domainPackages[from] && domainPackages[to]:
				rule = "R4 领域间协作必须显式"
			case domainPackages[from] && layer4Packages[to]:
				rule = "R6 领域不得依赖组合/工具层"
			default:
				continue
			}
			if isAllowed(from, to) {
				seenAllowed[from+"|"+to] = true
				continue
			}
			violations = append(violations, rule+"："+from+" -> "+to)
		}
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("依赖违约（未登记在契约 §7）：%s", v)
	}

	// 只减不增：允许清单里的每条边都应仍然存在；修复后必须删掉对应条目。
	var stale []string
	for from, tos := range allowedEdges {
		for to := range tos {
			if !seenAllowed[from+"|"+to] {
				stale = append(stale, from+" -> "+to)
			}
		}
	}
	sort.Strings(stale)
	for _, s := range stale {
		t.Errorf("契约 §7 例外清单已过期（该边已不存在，请删除该例外）：%s", s)
	}
}
