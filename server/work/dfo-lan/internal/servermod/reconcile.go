package servermod

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 本文件把"清单说该开/该关"落成"进程里真的在跑/真的停了"，并把结果写成一份
// 外部可读的运行期实况文件。
//
// # 为什么需要状态迁移
//
// enabled.json 只是**意图**。派发钩子看的是 live 集合（host.go），两者会不一致，
// 而且不一致是设计出来的，不是 bug：
//   - 关掉一个 mod 立刻不再派发它的钩子，但它 boot 时改过的进程级状态（模式策略、
//     注册过的内容、后台 goroutine）只有它自己知道怎么撤 —— 所以有 server.shutdown
//     才谈得上"干净地关"，没有就记成脏（dirtyBoot）；
//   - 脏状态下再打开会把同一份副作用改两遍，所以宿主**拒绝**热开并写明"需重启"，
//     而不是假装切换成功；
//   - 只提供奖励规则脚本的方向也不能热开：脚本要在奖励服务构造时编译进那个唯一的
//     LState（见 scripts.go），运行时补不进去。
//
// 这三条缺口都必须在日志与实况文件里点名。项目里最难查的失败一直是"看起来做了，
// 其实没生效"（server/AGENTS §6 的教训），所以宁可多写一行说明，也不留静默。
//
// # 为什么还要一份实况文件
//
// mod 管理器页面上的「生效」列原先靠两件事推断：拿 modID 在服务端 exe 里全文搜字符串、
// 比 mtime。前者会误报（重名、残留字符串），后者只能证明"不比源码旧"，证明不了
// "这份 exe 出自这份源码"。本进程启动时把**实况**（登记了什么、哪些在跑、哪些被拒、
//  exe 自身的 size/mtime/sha256）落到 runtime/mods-status.json，界面就有了对账依据：
// 注册表说有 / 二进制里有 / 进程里真在跑，是三个不同的事实，分开显示。

// StatusFileName 是运行期实况文件的文件名（落在 <服务端模块>/runtime/ 下，不入库）。
const StatusFileName = "mods-status.json"

// StatusFileSchema 是实况文件的结构版本。
const StatusFileSchema = 1

// transitionMu 串行化状态迁移：boot / shutdown 钩子绝不能并发跑，
// mod 作者写它们时按"启动期一次、收尾期一次"的心智模型，不会预期重入。
var transitionMu sync.Mutex

// TransitionAction 是迁移结果的取值。
const (
	// ActionEnabled 是"运行中打开成功"。
	ActionEnabled = "enabled"
	// ActionDisabled 是"运行中关闭成功"。
	ActionDisabled = "disabled"
	// ActionRefused 是"这次切换被拒绝"，原因写在 Note 里。
	ActionRefused = "refused"
)

// Transition 是一次运行期开关迁移的结果。
type Transition struct {
	ModID  string `json:"modId"`
	Action string `json:"action"`
	// Note 只在"有缺口"时非空：没撤销的 boot 副作用、没装载的脚本、boot 失败。
	Note string `json:"note,omitempty"`
	// RequiresRestart 表示这次切换只完成了一半，要重启才与清单一致。
	RequiresRestart bool   `json:"requiresRestart,omitempty"`
	At              string `json:"at"`
}

func (t Transition) String() string {
	if t.Note == "" {
		return fmt.Sprintf("mod %s → %s", t.ModID, t.Action)
	}
	return fmt.Sprintf("mod %s → %s（%s）", t.ModID, t.Action, t.Note)
}

// Reconcile 重读启用清单，并把 live 集合迁过去。清单没变时不跑任何钩子、不写任何文件。
//
// 调用方按秒轮询即可（RefreshEnabledList 只在文件指纹变化时才真的读盘解析）。
func Reconcile(version string) ([]Transition, error) {
	changed, err := RefreshEnabledList()
	if err != nil || !changed {
		return nil, err
	}
	out, applyErr := ApplyDesiredState(version)
	return out, applyErr
}

// ApplyDesiredState 无条件把 live 集合对齐到清单意图（不看文件有没有变）。
//
// 单独导出是为了测试与"管理器主动要求重算"的场景；启动路径上第一个调用者是 Reconcile。
func ApplyDesiredState(version string) ([]Transition, error) {
	transitionMu.Lock()
	defer transitionMu.Unlock()

	declared := declaredHooksByMod(EnabledModsDir())
	var out []Transition
	for _, id := range candidateIDs(declared) {
		want := Enabled(id)
		now := Live(id)
		reg := IsRegistered(id)

		// 本进程里根本没有它的钩子：要么这份二进制没编进去，要么它只提供数据/脚本。
		// 运行期变不出代码 ⇒ 不迁移，只在实况文件里标 requiresRebuild（见 BuildStatus）。
		if !reg {
			continue
		}
		if want == now {
			continue
		}

		var t Transition
		if now {
			t = disableMod(id, version)
		} else {
			t = enableMod(id, declared[id], version)
		}
		setNote(id, t.Note)
		log.Printf("servermod: 热开关 %s", t)
		out = append(out, t)
	}
	return out, nil
}

// disableMod 关掉一个 mod：先停止派发（这是玩家要的"关了"），再请它撤销 boot 副作用。
//
// 顺序不能反：钩子收尾期间如果还在接收客户端报文，mod 面对的就是一个正在被拆的状态。
func disableMod(modID, version string) Transition {
	setLive(modID, false)
	t := Transition{ModID: modID, Action: ActionDisabled, At: nowRFC3339()}
	switch {
	case HasShutdown(modID):
		if err := runShutdownHook(modID, &ShutdownContext{Reason: ReasonDisabled, Version: version}); err != nil {
			markDirty(modID, true)
			t.RequiresRestart = true
			t.Note = fmt.Sprintf("收尾钩子失败：%v（boot 改过的状态可能仍留在进程里，重启后干净）", err)
		}
	case HasBoot(modID):
		markDirty(modID, true)
		t.RequiresRestart = true
		t.Note = "该 mod 没有注册 server.shutdown，boot 改过的进程级状态没有撤销（重启后干净）"
	}
	return t
}

// enableMod 打开一个 mod。三种可能被拒绝，都写清楚原因：
// 脏状态（上次没能撤销）、boot 钩子自检失败、脚本方向本来就只能等重启。
func enableMod(modID string, hooks []string, version string) Transition {
	t := Transition{ModID: modID, Action: ActionEnabled, At: nowRFC3339()}
	if IsDirty(modID) {
		t.Action = ActionRefused
		t.RequiresRestart = true
		t.Note = "停用时的 boot 副作用没有撤销（见上一条说明），现在打开会把同一份状态改两遍；请重启服务端"
		return t
	}
	if HasBoot(modID) {
		if err := runBootHook(modID, &BootContext{Version: version}); err != nil {
			// 运行期打开失败只关这一个 mod，不能像启动期那样拒绝整个服务端。
			t.Action = ActionRefused
			t.Note = fmt.Sprintf("热开时 boot 钩子自检失败：%v（保持关闭，其它 mod 与服务端不受影响）", err)
			return t
		}
	}
	setLive(modID, true)
	markDirty(modID, false)
	// 声明了 reward.script 却一份都没装载 = 启动时它是关着的，脚本没进 LState。
	if contains(hooks, HookRewardScript) && len(RewardScriptsByMod(modID)) == 0 {
		t.RequiresRestart = true
		t.Note = "钩子已恢复，但它提供的奖励规则脚本要到重启服务端后才会装载（脚本编译进 LState 的时机只在启动）"
	}
	return t
}

// ---------------------------------------------------------------------------
// live / dirty / note 的存取（都由 host.go 的 mu 保护）
// ---------------------------------------------------------------------------

// IsRegistered 报告本进程里有没有这个 mod 登记的钩子。
//
// 它是"装了但没重新编译服务端"的唯一硬判据：mod.json 在磁盘上，而这里没有它的钩子。
func IsRegistered(modID string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return registeredIDs[modID]
}

// LiveStates 返回每个 mod 的 live 快照（稳定：调用方自己排序）。
func LiveStates() map[string]bool {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]bool, len(live))
	for k, v := range live {
		out[k] = v
	}
	return out
}

// liveModIDs 返回当前在跑的 mod id（稳定排序）。
func liveModIDs() []string {
	mu.RLock()
	out := make([]string, 0, len(live))
	for id, on := range live {
		if on {
			out = append(out, id)
		}
	}
	mu.RUnlock()
	sort.Strings(out)
	return out
}

// RegisteredIDs 返回本进程登记过钩子的 mod id（稳定排序）。
func RegisteredIDs() []string {
	mu.RLock()
	out := make([]string, 0, len(registeredIDs))
	for id := range registeredIDs {
		out = append(out, id)
	}
	mu.RUnlock()
	sort.Strings(out)
	return out
}

func setLive(modID string, on bool) {
	mu.Lock()
	defer mu.Unlock()
	if live == nil {
		live = map[string]bool{}
	}
	live[modID] = on
}

func IsDirty(modID string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return dirtyBoot[modID]
}

func markDirty(modID string, dirty bool) {
	mu.Lock()
	defer mu.Unlock()
	if dirtyBoot == nil {
		dirtyBoot = map[string]bool{}
	}
	if dirty {
		dirtyBoot[modID] = true
		return
	}
	delete(dirtyBoot, modID)
}

func setNote(modID, note string) {
	mu.Lock()
	defer mu.Unlock()
	if lastNote == nil {
		lastNote = map[string]string{}
	}
	if note == "" {
		delete(lastNote, modID)
		return
	}
	lastNote[modID] = note
}

// LastNote 返回某个 mod 最近一次迁移留下的说明。
func LastNote(modID string) string {
	mu.RLock()
	defer mu.RUnlock()
	return lastNote[modID]
}

// candidateIDs 汇总需要判定的 mod：登记过钩子的、清单里被点名的、磁盘上有 mod.json 的。
//
// 三类并集才够：只被禁用而从未登记的（没重编译）也要进实况文件，否则界面看不到它。
func candidateIDs(declared map[string][]string) []string {
	set := map[string]bool{}
	for _, id := range RegisteredIDs() {
		set[id] = true
	}
	for _, id := range DisabledSnapshot() {
		set[id] = true
	}
	for id := range declared {
		set[id] = true
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// declaredHooksByMod 扫 mods 目录，返回 mod id → 它声明的钩子点。
//
// 读不到目录只返回空表：实况文件不能因为一次扫描失败就不写。
func declaredHooksByMod(modsDir string) map[string][]string {
	out := map[string][]string{}
	if strings.TrimSpace(modsDir) == "" {
		return out
	}
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id, hooks, ok, _ := declaredHooks(filepath.Join(modsDir, e.Name()))
		if !ok {
			continue
		}
		if id == "" {
			id = e.Name()
		}
		out[id] = hooks
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 运行期实况文件
// ---------------------------------------------------------------------------

// SelfInfo 是"正在跑的这份服务端程序"的身份，由 main 在启动时算好交进来。
//
// 带 exe 的 sha256 是有意的：界面原先只能比 mtime，证明不了"这份 exe 出自这份源码"；
// 现在拿实况文件里的哈希与磁盘上的程序对照，就知道**运行中的进程是不是旧的那一份**。
type SelfInfo struct {
	ServerVersion  string `json:"serverVersion"`
	ExePath        string `json:"exePath,omitempty"`
	ExeSizeBytes   int64  `json:"exeSizeBytes,omitempty"`
	ExeModifiedAt  string `json:"exeModifiedAt,omitempty"`
	ExeSHA256      string `json:"exeSha256,omitempty"`
	ExeHashSkipped string `json:"exeHashSkippedReason,omitempty"`
	PID            int    `json:"pid"`
	StartedAt      string `json:"startedAt"`
}

// ModStatus 是一个 mod 在本进程里的实况。
type ModStatus struct {
	ID string `json:"id"`
	// Declared 取自磁盘上的 mod.json，Registered 取自本进程的钩子登记表。
	// 两者不一致就是"装了没重编译"或"注册了没声明"，直接显示出来，不用界面猜。
	Declared   []string `json:"declared,omitempty"`
	Registered []string `json:"registered,omitempty"`
	Scripts    []string `json:"scripts,omitempty"`
	Enabled    bool     `json:"enabled"`
	Live       bool     `json:"live"`
	// HasCleanup 报告这个 mod 有没有登记 server.shutdown（boot 的逆操作）。
	// 界面靠它决定该说什么：**没有收尾钩子的 mod 一被关掉就打不回来**（dirtyBoot），
	// 这时候"请重启服务端"比"已停用"有用得多。
	HasCleanup bool `json:"hasCleanup,omitempty"`
	DirtyBoot  bool `json:"dirtyBoot,omitempty"`
	// RequiresRebuild 只在"磁盘上有带钩子的 mod.json、本进程却一个钩子都没有"时为真。
	// 它是界面唯一能说出口的"重编译服务端"判据；只提供脚本/文件的 mod 不会被它冤枉
	// （那类 mod 压根没有钩子可登记，见 BuildStatus 里的分支）。
	RequiresRebuild bool   `json:"requiresRebuild,omitempty"`
	RequiresRestart bool   `json:"requiresRestart,omitempty"`
	Note            string `json:"note,omitempty"`
}

// RuntimeStatus 是 mods-status.json 的全部结构。
type RuntimeStatus struct {
	Schema    int      `json:"schema"`
	WrittenAt string   `json:"writtenAt"`
	Self      SelfInfo `json:"self"`
	// EnabledFile 与清单加载说明：文件坏了按"全部启用"处理这件事必须可见,不能只进日志。
	EnabledFile string      `json:"enabledFile,omitempty"`
	EnabledNote string      `json:"enabledNote,omitempty"`
	Disabled    []string    `json:"disabled,omitempty"`
	Mods        []ModStatus `json:"mods"`
	// DeclarationProblems 是声明与注册的差集（与启动期那条 fail-closed 核对同源）。
	DeclarationProblems []string `json:"declarationProblems,omitempty"`
	Summary             string   `json:"summary"`
}

// BuildStatus 汇总当前实况，不读盘以外的信息。
func BuildStatus(self SelfInfo) RuntimeStatus {
	modsDir := EnabledModsDir()
	declared := declaredHooksByMod(modsDir)
	path, disabled, note := EnabledInfo()

	st := RuntimeStatus{
		Schema:      StatusFileSchema,
		WrittenAt:   nowRFC3339(),
		Self:        self,
		EnabledFile: path,
		EnabledNote: note,
		Disabled:    disabled,
		Summary:     Description(),
	}
	if problems, err := CheckDeclarations(modsDir); err == nil {
		for _, p := range problems {
			st.DeclarationProblems = append(st.DeclarationProblems, p.String())
		}
	}
	for _, id := range candidateIDs(declared) {
		ms := ModStatus{
			ID:         id,
			Declared:   declared[id],
			Registered: RegisteredHooks(id),
			Scripts:    RewardScriptsByMod(id),
			Enabled:    Enabled(id),
			Live:       Live(id),
			HasCleanup: HasShutdown(id),
			DirtyBoot:  IsDirty(id),
			Note:       LastNote(id),
		}
		// "本进程没有它的钩子"有两种截然不同的原因，混为一谈就会冤枉人：
		// 	① 磁盘上有带钩子的 mod.json ⇒ 这个 mod 要编进 Go 二进制，而它没被编进去
		// 	   → 唯一的 requiresRebuild 判据；
		// 	② 磁盘上只有禁用名单里点了一下、没有 mod.json ⇒ 它是**只提供规则脚本/文件**
		// 	   的 mod（modkit 不给这类 mod 落清单，见 mods/README.md），本来就不参与编译，
		// 	   热开关也就没有可门禁的东西。把它标成"要重编译"是假告警。
		if !IsRegistered(id) {
			switch {
			case len(declared[id]) > 0:
				ms.RequiresRebuild = true
				if ms.Note == "" {
					// 服务端分不出这两种原因，但结论一样：**现在打开它不会有任何效果**。
					// 旧写法（Register() 里 `if !Enabled() { return }`）也归到这里——
					// 那种 mod 被停用时一个钩子都不登记，热开关对它无效，必须去掉守卫。
					ms.Note = "本进程没有登记它的任何钩子：要么这份二进制没编进它，" +
						"要么它的 Register() 仍按 Enabled() 提前返回（去掉那个守卫开关才是热的）"
				}
			case contains(disabled, id):
				if ms.Note == "" {
					ms.Note = "本进程没有它的钩子，磁盘上也没有带钩子的 mod.json：" +
						"这是只提供规则脚本/文件的 mod，勾选只影响装卸，热开关不覆盖它"
				}
			}
		}
		if ms.Note == "" && IsDirty(id) {
			ms.Note = "上次停用没能撤销 boot 副作用，现在打开会被拒绝（需重启）"
		}
		ms.RequiresRestart = ms.RequiresRebuild || ms.DirtyBoot
		st.Mods = append(st.Mods, ms)
	}
	if st.Mods == nil {
		st.Mods = []ModStatus{}
	}
	return st
}

// WriteRuntimeStatus 原子落一份实况文件。path 为空表示不写（例如没配 mods 目录）。
//
// 调用时机：启动装配完成后一次、每次状态迁移后一次，外加一条心跳（见 cmd/wireprobe
// 的 startModLive）。界面按 writtenAt + pid 判断这是不是当前进程产出的，
// 因此心跳的意义不是"刷新数据"，而是"证明进程还活着"。
func WriteRuntimeStatus(path string, self SelfInfo) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	b, err := json.MarshalIndent(BuildStatus(self), "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// StatusPath 给出实况文件的默认位置：<模块根>/runtime/mods-status.json。
func StatusPath(moduleDir string) string {
	return filepath.Join(moduleDir, "runtime", StatusFileName)
}

// ExeSHA256 算一份程序指纹。超过上限时跳过并说明原因——实况文件不该让启动去等一次大哈希。
func ExeSHA256(path string, limit int64) (sum string, skipped string, size int64, modTime string) {
	st, err := os.Stat(path)
	if err != nil {
		return "", "读不到程序文件：" + err.Error(), 0, ""
	}
	size = st.Size()
	modTime = st.ModTime().UTC().Format(time.RFC3339)
	if limit > 0 && size > limit {
		return "", fmt.Sprintf("程序 %d 字节超过指纹上限 %d，跳过哈希", size, limit), size, modTime
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "打开程序文件失败：" + err.Error(), size, modTime
	}
	defer f.Close()
	sum, err = sha256Of(f)
	if err != nil {
		return "", "算哈希失败：" + err.Error(), size, modTime
	}
	return sum, "", size, modTime
}

// sha256Of 算一个可读流的 SHA-256（十六进制小写）。
func sha256Of(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
