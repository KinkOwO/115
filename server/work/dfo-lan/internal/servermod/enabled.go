package servermod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 本文件实现**运行时启用/禁用**：mod 管理器（启动器内的那一块）通过它
// 在不动编译产物的前提下勾选/取消某个 mod。
//
// # 为什么不做成"编译期裁剪"
//
// 服务端 mod 是编译进二进制的（B 路线），如果启用/禁用要重编译，那每次勾选
// 都等于重建服务端——玩家在管理器里点一下要等一次 go build，体验不可接受。
//
// 所以启用/禁用是**运行期门禁**，门禁分两处，代价不同：
//   - 所有已安装的 mod 都会被编译进二进制（安装时决定，改这一层要重编译）；
//   - Register() 一律登记钩子，**派发**时才问"这个 mod 在不在跑"（Live()）——
//     于是勾选/取消既不重编译，也不用等下一次启动；
//   - 清单按文件指纹惰性重读（RefreshEnabledList），管理器在服务端运行中改
//     enabled.json，下一次 Reconcile 就把状态迁移过去。
//
// 代价：二进制里始终带着被禁用 mod 的代码。收益：勾选/取消**立即生效**。
// 对"运营侧开关"这个用途，收益远大于代价。
//
// 仍需重启的方向（Reconcile 会在 note 里点名，不静默）：
//   - 把一个 mod **打开**，而它的 server.boot 副作用没有配 server.shutdown；
//   - mod 提供奖励规则脚本（脚本要编译进 LState，只在启动时装载）。
//
// # 与 modkit 的关系
//
// modkit（启动器仓）负责装/卸；本文件负责"装了之后开不开"。
// 两边共用同一份文件：<服务端模块>/mods/enabled.json。

// enabledFileName 是启用清单的文件名（与各 mod 源码同处 mods/ 目录）。
const enabledFileName = "enabled.json"

// EnabledFileSchema 是启用清单的结构版本。
const EnabledFileSchema = 1

// enabledState 是 enabled.json 的结构。
//
// 语义刻意用**禁用名单**而不是启用名单：
//   - 新装的 mod 默认**启用**（符合直觉：装了就是想用）；
//   - 只有被显式取消勾选的才进 disabled；
//   - 这样清单文件缺失、损坏、或新增了 mod，都不会出现"莫名其妙全都不生效"。
type enabledState struct {
	Schema   int      `json:"schema"`
	Disabled []string `json:"disabled"`
	// UpdatedAt 与 UpdatedBy 供管理器写回时留痕（谁在什么时候动的）。
	UpdatedAt string `json:"updatedAt,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
}

var (
	enabledMu       sync.RWMutex
	enabledPath     string
	enabledDir      string
	disabledSet     map[string]bool
	enabledLoaded   bool
	enabledLoadNote string
	// enabledStamp 是清单文件上次的指纹，RefreshEnabledList 用它判断要不要重读。
	enabledStamp fileStamp
)

// fileStamp 是"文件变没变"的判据（存在性 + 大小 + mtime）。
type fileStamp struct {
	exists bool
	size   int64
	mod    time.Time
}

func stampOf(path string) fileStamp {
	st, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{exists: true, size: st.Size(), mod: st.ModTime()}
}

// sameAs 报告磁盘上的清单与上次读到的那份是不是同一份。
func (s fileStamp) sameAs(path string) bool {
	cur := stampOf(path)
	if s.exists != cur.exists {
		return false
	}
	if !s.exists {
		return true
	}
	return s.size == cur.size && s.mod.Equal(cur.mod)
}

// LoadEnabledList 读入启用清单。path 为空时用 <modsDir>/enabled.json。
//
// 文件不存在 = 全部启用（不是错误）；文件损坏 = 全部启用并记一条说明，
// 因为"清单坏了"绝不能等价于"把所有 mod 关掉"——那会让玩家以为 mod 全丢了。
func LoadEnabledList(modsDir string) {
	enabledMu.Lock()
	defer enabledMu.Unlock()
	enabledDir = modsDir
	enabledPath = filepath.Join(modsDir, enabledFileName)
	disabledSet, enabledLoadNote = readDisabledSet(enabledPath)
	enabledLoaded = true
	enabledStamp = stampOf(enabledPath)
}

// EnabledModsDir 返回当前清单所在的 mods 目录（状态文件与声明核对都按它扫）。
func EnabledModsDir() string {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return enabledDir
}

// readDisabledSet 读一份清单，返回禁用集合与一条需要告示的问题（没有则空串）。
// 缺失与损坏都退回"全部启用"，口径与 LoadEnabledList 一致。
func readDisabledSet(path string) (map[string]bool, string) {
	set := map[string]bool{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return set, "没有 " + enabledFileName + "（按全部启用处理）"
	}
	if err != nil {
		return set, fmt.Sprintf("读 %s 失败（按全部启用处理）：%v", enabledFileName, err)
	}
	var st enabledState
	if err := json.Unmarshal(b, &st); err != nil {
		return set, fmt.Sprintf("%s 解析失败（按全部启用处理）：%v", enabledFileName, err)
	}
	for _, id := range st.Disabled {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
	return set, ""
}

// RefreshEnabledList 在服务端运行中重读启用清单（mod 管理器改过 enabled.json）。
//
// 只在文件指纹变化时才真的读盘解析，所以调用方可以放心按秒轮询。
// 返回 changed 表示禁用名单与内存里的不一致（这时才需要做状态迁移）。
//
// 它不改任何钩子注册，只换这份名单：谁真的开/关由 host.go 的 live 集合与
// Reconcile 决定，因此这里不能回调 mod 的钩子（会拿着锁重入）。
func RefreshEnabledList() (changed bool, err error) {
	enabledMu.Lock()
	defer enabledMu.Unlock()
	if !enabledLoaded || enabledPath == "" {
		return false, nil
	}
	if enabledStamp.sameAs(enabledPath) {
		return false, nil
	}
	set, note := readDisabledSet(enabledPath)
	enabledStamp = stampOf(enabledPath)
	if note != "" {
		enabledLoadNote = note
	}
	if sameSet(set, disabledSet) {
		return false, nil
	}
	disabledSet = set
	return true, nil
}

// sameSet 比较两个禁用名单是否等价（只看键，不看顺序）。
func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// Enabled 报告某个 mod 现在是否启用（清单里被显式取消勾选才算禁用）。
//
// # 它不是"这个 mod 现在在不在跑"
//
// 运行中通过 Reconcile 打开一个没有 cleanup 的 mod 会被拒绝，那时它**已启用但仍未生效**。
// 所以派发钩子、写 mods-status.json、判"要不要重启"都用 Live()，不要用本函数。
// 本函数只表达清单意图，适合在 mod 的 Register() 里决定"这份脚本要不要装载"。
//
// 没有加载过清单（例如单元测试直接调 mod 的 Register）时**默认启用**，
// 避免"测试里忘了加载清单导致 mod 静默不生效"。
func Enabled(modID string) bool {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return !disabledSet[modID]
}

// Live 报告某个 mod 现在是否真的在派发钩子（由 RegisterMods 初始化、Reconcile 迁移）。
func Live(modID string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return live[modID]
}

// EnabledInfo 返回启用清单的加载状态，供启动日志与 `mod help` 展示。
func EnabledInfo() (path string, disabled []string, note string) {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	out := make([]string, 0, len(disabledSet))
	for id := range disabledSet {
		out = append(out, id)
	}
	sort.Strings(out)
	return enabledPath, out, enabledLoadNote
}

// DisabledSnapshot 返回当前禁用名单（稳定排序）。
func DisabledSnapshot() []string {
	_, out, _ := EnabledInfo()
	return out
}

// SetModEnabled 写回启用清单（管理器勾选/取消勾选用）。
//
// 它是唯一写这个文件的地方：读-改-写 + 原子落盘，避免管理器与服务端并发写坏。
func SetModEnabled(modsDir, modID string, enabled bool, updatedBy string) error {
	modID = strings.TrimSpace(modID)
	if !validModID(modID) {
		return fmt.Errorf("非法 mod id：%q", modID)
	}
	path := filepath.Join(modsDir, enabledFileName)

	// 读现有清单（不存在视为空）
	st := enabledState{Schema: EnabledFileSchema}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &st); err != nil {
			return fmt.Errorf("%s 已存在但无法解析，拒绝覆盖（请先人工修好）：%w", enabledFileName, err)
		}
	}
	set := map[string]bool{}
	for _, id := range st.Disabled {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = true
		}
	}
	if enabled {
		delete(set, modID)
	} else {
		set[modID] = true
	}
	st.Schema = EnabledFileSchema
	st.Disabled = st.Disabled[:0]
	for id := range set {
		st.Disabled = append(st.Disabled, id)
	}
	sort.Strings(st.Disabled)
	st.UpdatedAt = nowRFC3339()
	st.UpdatedBy = updatedBy

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// nowRFC3339 用 UTC 记时间，避免跨时区排查时对不上。
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
