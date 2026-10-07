package servermod

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// 本文件让 mod 能往**奖励管线**里追加规则脚本。
//
// # 为什么走奖励管线，而不是自己发道具
//
// 服务端已经有一条成熟的"事件 → Lua 规则 → 幂等发放"链路：
//
//	character.Service.Create  ──新角色提交成功──>  reward.Service.CharacterCreate
//	                                                    │
//	                                    内嵌脚本 on("character_create", ...)
//	                                                    │
//	                                    grant_item / send_mail(带附件) / grant_cera
//	                                                    │
//	                              CommitCharacterEvent（稳定幂等键）/ CommitSystemMail
//
// 这条链路自带：幂等键（重放不重复发）、事务、背包满转邮件、装备实例重建。
// mod 自己造一条发放路径，等于把这些全部重写一遍，还会引入第二套真源。
//
// 所以 mod 的"发东西"= **追加一条事件规则脚本**，复用既有的发放与存档语义。
//
// # 启用/禁用怎么生效
//
// 脚本在**奖励服务构造时**一次性加载（服务端启动阶段）。所以：
//   - 禁用的 mod 不让它的脚本进注册表（Register() 里先问 Enabled()）；
//   - 改启用状态后需要**重启服务端**（脚本已加载进 LState，无法热摘）。
// 这是明确的边界，写进启动段文档。

var (
	scriptMu sync.RWMutex
	// modScripts 收集各 mod 要注入的 Lua 脚本（已过滤掉被禁用的 mod）。
	modScripts []NamedScript
)

// NamedScript 是一份来自 mod 的规则脚本。
type NamedScript struct {
	// ModID 是来源 mod（写进日志与审计，便于归因）。
	ModID string
	// Name 是传给奖励管线的脚本名（= 文件名，*.lua）。
	Name string
	// Body 是脚本内容。
	Body []byte
}

// RegisterRewardScript 登记一份 mod 规则脚本。
//
// mod 在自己的 Register() 里调用（并且应当已经先判断 Enabled()）：
//
//	func Register() {
//	    if !servermod.Enabled(modID) { return }
//	    _ = servermod.RegisterRewardScript(modID, "my-rule.lua", body)
//	}
//
// 重名会被拒绝（不同 mod 用同一个文件名会互相覆盖，属于作者错误）。
func RegisterRewardScript(modID, name string, body []byte) error {
	if !validModID(modID) {
		return errInvalidModID(modID)
	}
	name = strings.TrimSpace(name)
	if name == "" || !strings.HasSuffix(strings.ToLower(name), ".lua") {
		return errBadScriptName(name)
	}
	if path.Base(name) != name || strings.Contains(name, "..") {
		return errBadScriptName(name)
	}
	if len(body) == 0 {
		return errEmptyScript(name)
	}
	scriptMu.Lock()
	defer scriptMu.Unlock()
	for _, s := range modScripts {
		if s.Name != name {
			continue
		}
		// 同一个 mod 重复登记同名脚本 = **幂等**（内容一致直接成功，内容不同才报错）。
		//
		// 为什么必须幂等：RegisterMods() 在进程里可能被调用多次（测试、将来的重载路径），
		// 而"重复登记"不是错误。但不同 mod 抢同一个文件名是实打实的冲突，必须拒绝。
		if s.ModID == modID {
			if string(s.Body) == string(body) {
				return nil
			}
			return errScriptContentChanged(name, modID)
		}
		return errDuplicateScript(name, s.ModID)
	}
	modScripts = append(modScripts, NamedScript{ModID: modID, Name: name, Body: append([]byte(nil), body...)})
	registeredIDs[modID] = true
	return nil
}

// RewardScripts 返回已登记的 mod 脚本（稳定顺序：按名字）。
func RewardScripts() []NamedScript {
	scriptMu.RLock()
	defer scriptMu.RUnlock()
	out := append([]NamedScript(nil), modScripts...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RewardScriptNames 返回已登记脚本的名字（诊断用）。
func RewardScriptNames() []string {
	out := make([]string, 0)
	for _, s := range RewardScripts() {
		out = append(out, s.ModID+":"+s.Name)
	}
	return out
}

// modScriptFS 是"mod 脚本 + 落盘补充脚本"的只读视图，实现 fs.FS。
//
// 两个来源都要支持：
//   - 内存：mod 通过 RegisterRewardScript 交进来的（编译进二进制的那份内容）；
//   - 磁盘：<服务端模块>/mods/scripts/*.lua —— 给不想重编译就想加规则的人留的口子，
//     也便于运营侧快速试一条规则。
//
// 磁盘上的文件优先于同名的内存脚本（便于就地覆盖调试）。
//
// **关键：内存脚本必须"实时读取注册表"，不能在构造时快照。**
// 踩过的坑：奖励服务是在 prepareRuntime（配置/存储准备）里构造的，而
// mods.RegisterMods() 在那之后才跑。构造时快照会捕获一张**空表**，
// 之后 mod 再登记也进不去 —— 规则静默失效，日志上却显示"已装载/已登记"。
// 所以 Open/ReadDir 每次都问 RewardScripts()。
type modScriptFS struct {
	dir string
}

// NewRewardScriptFS 构造 mod 脚本视图。dir 为空表示只看内存脚本。
//
// 它返回的是**视图**而不是快照：调用后新登记的 mod 脚本依然可见。
// 这一点是硬要求，见类型注释里的踩坑记录。
func NewRewardScriptFS(dir string) fs.FS {
	return &modScriptFS{dir: dir}
}

// ReadDir 实现 fs.ReadDirFS。
//
// **必须有它**：奖励管线是用 `fs.Glob(src, "*.lua")` 列脚本的，而 fs.Glob 依赖
// ReadDir 来枚举目录（不是靠 Open 碰运气）。只实现 Open 的话，glob 会**静默返回空**——
// mod 的规则脚本一份都读不到，且不报错。这是实测踩到的坑。
func (m *modScriptFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." && name != "" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	seen := map[string]bool{}
	var out []fs.DirEntry
	if m.dir != "" {
		if entries, err := os.ReadDir(m.dir); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".lua") {
					continue
				}
				if seen[e.Name()] {
					continue
				}
				seen[e.Name()] = true
				out = append(out, e)
			}
		}
	}
	// 实时查注册表（不用构造时的快照）。
	for _, s := range RewardScripts() {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		out = append(out, memDirEntry{name: s.Name, size: int64(len(s.Body))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// Stat 实现 fs.StatFS（fs.Glob 的某些路径会用到）。
func (m *modScriptFS) Stat(name string) (fs.FileInfo, error) {
	if name == "." || name == "" {
		return memFileInfo{name: ".", size: 0, dir: true}, nil
	}
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

// memDirEntry 是内存脚本的目录项。
type memDirEntry struct {
	name string
	size int64
}

func (e memDirEntry) Name() string               { return e.name }
func (e memDirEntry) IsDir() bool                { return false }
func (e memDirEntry) Type() fs.FileMode          { return 0 }
func (e memDirEntry) Info() (fs.FileInfo, error) { return memFileInfo{name: e.name, size: e.size}, nil }

// Open 实现 fs.FS。
func (m *modScriptFS) Open(name string) (fs.File, error) {
	if path.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".lua") {
		// 只有平铺的 *.lua 会被奖励管线用 fs.Glob 取到；其余一律当作不存在。
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if m.dir != "" {
		full := path.Join(m.dir, name)
		if f, err := os.Open(full); err == nil {
			return f, nil
		}
	}
	// 实时查注册表（不用构造时的快照）。
	for _, s := range RewardScripts() {
		if s.Name == name {
			return &memFile{name: name, data: s.Body}, nil
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// memFile 是内存里的一个只读文件，满足 fs.File 的最小实现。
type memFile struct {
	name   string
	data   []byte
	off    int64
	closed bool
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}
	return memFileInfo{name: f.name, size: int64(len(f.data))}, nil
}

func (f *memFile) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	if f.off >= int64(len(f.data)) {
		// 必须返回 io.EOF：fs.ReadFile 靠它判断读完了。
		// 返回 fs.ErrClosed 或 (0,nil) 都会让"读这个脚本"整个失败。
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += int64(n)
	return n, nil
}

func (f *memFile) Close() error {
	f.closed = true
	return nil
}

type memFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i memFileInfo) Name() string { return i.name }
func (i memFileInfo) Size() int64  { return i.size }
func (i memFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return i.dir }
func (i memFileInfo) Sys() any           { return nil }

// ---------------------------------------------------------------------------
// 错误构造（把"哪一步错了"写清楚，mod 作者不用猜）
// ---------------------------------------------------------------------------

func errInvalidModID(id string) error {
	return fmt.Errorf("servermod: 非法 mod id %q（只允许小写字母/数字/./-，≤64）", id)
}

func errBadScriptName(name string) error {
	return fmt.Errorf("servermod: 非法脚本名 %q（只允许平铺的 *.lua 文件名，不含目录与 ..）", name)
}

func errEmptyScript(name string) error {
	return fmt.Errorf("servermod: 脚本 %s 内容为空", name)
}

func errDuplicateScript(name, owner string) error {
	return fmt.Errorf("servermod: 脚本名 %s 已被 mod %s 占用（不同 mod 不能用同一个文件名）", name, owner)
}

// errNoScripts 供调用方判断"没有 mod 脚本"这一正常情况。
var errNoScripts = errors.New("servermod: 没有 mod 提供奖励脚本")

func errScriptContentChanged(name, modID string) error {
	return fmt.Errorf("servermod: mod %s 重复登记脚本 %s 且内容与已登记的不一致"+
		"（同一进程内同名脚本只能有一份内容）", modID, name)
}
