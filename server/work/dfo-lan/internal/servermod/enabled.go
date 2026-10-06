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
// 所以启用/禁用是**运行期门禁**：
//   - 所有已安装的 mod 都会被编译进二进制（安装时决定）；
//   - 启动时读一份清单，只让启用的 mod 真正注册钩子；
//   - 被禁用的 mod 代码在里面，但 Register() 里会先问 Enabled()，直接跳过。
//
// 代价：二进制里始终带着被禁用 mod 的代码。收益：勾选/取消**立即生效**，
// 不需要重编译、不需要重新安装。对"运营侧开关"这个用途，收益远大于代价。
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
	disabledSet     map[string]bool
	enabledLoaded   bool
	enabledLoadNote string
)

// LoadEnabledList 读入启用清单。path 为空时用 <modsDir>/enabled.json。
//
// 文件不存在 = 全部启用（不是错误）；文件损坏 = 全部启用并记一条说明，
// 因为"清单坏了"绝不能等价于"把所有 mod 关掉"——那会让玩家以为 mod 全丢了。
func LoadEnabledList(modsDir string) {
	enabledMu.Lock()
	defer enabledMu.Unlock()
	enabledPath = filepath.Join(modsDir, enabledFileName)
	disabledSet = map[string]bool{}
	enabledLoaded = true
	enabledLoadNote = ""

	b, err := os.ReadFile(enabledPath)
	if os.IsNotExist(err) {
		enabledLoadNote = "没有 " + enabledFileName + "（按全部启用处理）"
		return
	}
	if err != nil {
		enabledLoadNote = fmt.Sprintf("读 %s 失败（按全部启用处理）：%v", enabledFileName, err)
		return
	}
	var st enabledState
	if err := json.Unmarshal(b, &st); err != nil {
		enabledLoadNote = fmt.Sprintf("%s 解析失败（按全部启用处理）：%v", enabledFileName, err)
		return
	}
	for _, id := range st.Disabled {
		id = strings.TrimSpace(id)
		if id != "" {
			disabledSet[id] = true
		}
	}
}

// Enabled 报告某个 mod 现在是否启用。
//
// 没有加载过清单（例如单元测试直接调 mod 的 Register）时**默认启用**，
// 避免"测试里忘了加载清单导致 mod 静默不生效"。
//
// mod 的 Register() 应当在开头就判断它：
//
//	func Register() {
//	    if !servermod.Enabled(modID) { return }
//	    ...
//	}
func Enabled(modID string) bool {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return !disabledSet[modID]
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
