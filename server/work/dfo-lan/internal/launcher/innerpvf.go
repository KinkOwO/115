package launcher

// 本文件是 docs/go-launch-migration-plan.md 的 Stage 4：把内层 PVF（PVF 直读模式的输入）
// 的**生成**从 Python 搬进 Go，替掉 scripts/ensure_inner_pvf.py（四态门禁）与
// scripts/prepare_inner_pvf.py（真正的剥壳）。
//
// 先说清“生成”是什么：内层归档**不是**重新打包出来的 PVF，而是客户端 `Script.pvf` 去掉
// 外层保护的同一份字节 —— 每 0xA00000(10 MiB) 一块，前 0x2800(10 KiB) 用 AES-256-CBC
// 加密，密钥由 `DFO.exe` 内嵌的 RSA 私钥解开 `sk.dat` 得到。所以这一步既不需要
// internal/catalog/pvf 的解析/重打包能力，也不需要任何第三方库；剥壳本身在
// internal/catalog/pvf 的 UnwrapOuter 里（它同时拥有内层格式的定义与 iNfO 密钥流）。
// 本文件负责门禁、轮换、清单与原子发布。
//
// 四态门禁（与 ensure_inner_pvf.py、以及启动器侧 115us-dfolauncher/internal/pvfprep
// 同语义；原因串逐字对齐，供启动输出逐字节比对）：
//
//	盘上 inner | manifest    | 客户端三件套指纹 | 动作
//	无         | —           | —                | 生成
//	有         | 无/不可解析/格式不符 | —        | 重建（无法证明它对应哪个客户端）
//	有         | 有          | 相等             | 复用
//	有         | 有          | 不等             | 重建
//
// 两路比较：先比 (size, mtime_ns) 三元组（毫秒级），全等才复用；只有三元组变了才做
// 760 MB 级的 sha256 全量比对，免得每次开游戏都白读一遍客户端。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dfolan/internal/catalog/pvf"
)

const (
	// innerFormat 是清单的 format 字段：prepare_inner_pvf.py 写它，两个门禁都只认它。
	// 它与 internal/catalog/pvf 的 FormatDFO20260901 是同一个字符串（内层归档格式名）。
	innerFormat = "dfo_20260901_inner"

	// innerDecoder 是清单的 decoder 字段。Python 版写的是解密链的模块名，逐字保留：
	// 它记录的是“这是谁解出来的”，不是可执行声明，改了就不再逐字节一致。
	innerDecoder = "pvf_archive.wrapper_keys/aes"

	// innerTempPattern 是生成期间的临时产物名：脚本用 pvf-prepare-*.partial，
	// 发布靠硬链接（只在目标不存在时成功），失败时临时文件被删掉。
	innerTempPattern = "pvf-prepare-*.partial"
)

// innerClientInputs 是内层归档由什么决定：DFO.exe 提供密钥、sk.dat 是密文、
// Script.pvf 是被剥壳的对象。任意一件变化都会得到不同的内层归档，所以门禁的键是三者。
var innerClientInputs = []string{"DFO.exe", "sk.dat", "Script.pvf"}

// InnerPVFOptions 是一次“准备内层 PVF”的输入。
//
// 路径都允许留空：留空时按启动器的既有约定推导（client 取 launcher.local.json 的
// client_dir，产物落在 server/work/client-build/）。
type InnerPVFOptions struct {
	Root      string // 仓库根（.cmd 入口 cd 到的那个目录）
	ClientDir string // 客户端目录（只读输入源）
	Inner     string // 内层归档的落地路径
	Manifest  string // 清单的落地路径
	// Force 无视门禁强制重建（--force）。门禁的用途是省下 760 MB 的重算，
	// 而不是禁止重算；怀疑产物被改坏时就需要它。
	Force bool
	// DryRun 只报告判定结果，一个字节都不写（新子命令的 --dry-run）。
	DryRun bool
	// Log 收生成过程的两行提示（可空）。文案与 Python 版逐字相同。
	Log func(format string, args ...any)
}

// InnerPVFResult 是一次判定的结果。
type InnerPVFResult struct {
	// NeedsBuild 是门禁的判定：为 true 表示盘上的产物不可信/已过期（DryRun 时即“本该重建”）。
	NeedsBuild bool
	Reason     string
	// Generated 表示这次真的生成了（DryRun 与复用都是 false）。
	Generated bool
	Elapsed   time.Duration

	Inner    string
	Manifest string
	// Rotated 是重建前被轮换走的旧件（无则空）。旧件只是改名，不删除，留一条回滚路。
	Rotated string
	// Size / SHA256 / Segments / Keys 是产物的身份与形状，供调用方打印与比对
	// （Segments 是 10 MiB 分段数，Keys 是 sk.dat 解出的段密钥数）。
	Size     int64
	SHA256   string
	Segments int
	Keys     int
}

// PrepareInnerPVF 是 ensure_inner_pvf.py 的 Go 版：四态门禁 + 需要时现场生成。
//
// 返回的错误是“无法生成”的硬边界（客户端三件套不齐、DFO.exe 不是可读的 PE、密钥
// 不匹配、磁盘写失败…）；调用方决定是否降级（launch 那条路只打 WARNING，与 Python 的
// _ensure_inner_pvf 一样：内层归档只影响直读模式，玩家仍可显式 --json-mode 回退）。
func PrepareInnerPVF(opts InnerPVFOptions) (InnerPVFResult, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return InnerPVFResult{}, fmt.Errorf("解析仓库根失败：%w", err)
	}
	serverRoot := filepath.Join(root, "server")

	client := opts.ClientDir
	if client == "" {
		if client, err = settingsClientDir(serverRoot); err != nil {
			return InnerPVFResult{}, err
		}
	}
	inner := opts.Inner
	if inner == "" {
		inner = InnerPVFPath(root)
	}
	manifestPath := opts.Manifest
	if manifestPath == "" {
		manifestPath = InnerPVFManifestPath(root)
	}
	client, inner, manifestPath = filepath.Clean(client), filepath.Clean(inner), filepath.Clean(manifestPath)

	result := InnerPVFResult{Inner: inner, Manifest: manifestPath}

	// 1. 三件套不齐直接拒绝（Python 的 ensure() 第一步）。这条是硬边界：没有输入就没有产物，
	//    不做任何兜底 —— 尤其是不能拿一份来源不明的旧归档顶上。
	if missing := innerClientMissing(client); len(missing) > 0 {
		return result, fmt.Errorf("客户端缺少 %s，无法生成内层 PVF（需 DFO.exe + sk.dat + Script.pvf 三件套）",
			strings.Join(missing, "、"))
	}

	// 2. 落点检查，与脚本的 check_destination 同义：客户端目录是只读输入源，产物不许写进去；
	//    归档与清单也不能是同一个文件。
	if inner == manifestPath {
		return result, fmt.Errorf("内层归档与清单的落盘位置不能相同：%s", inner)
	}
	if pathWithin(client, inner) || pathWithin(client, manifestPath) {
		return result, fmt.Errorf("输出不能落在只读的客户端目录内（%s）", client)
	}

	// 3. 门禁（只读）。
	needsBuild, reason := decideInnerPVF(client, inner, manifestPath)
	if opts.Force {
		needsBuild, reason = true, "指定了强制重建"
	}
	result.NeedsBuild, result.Reason = needsBuild, reason
	if !needsBuild || opts.DryRun {
		return result, nil
	}

	// 4. 生成。目录按需创建（Python 要求父目录已存在并抛 FileNotFoundError；启动器自己
	//    建更省事，且这里的路径本就是它自己的构建目录）。
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		return result, fmt.Errorf("创建内层 PVF 输出目录失败：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return result, fmt.Errorf("创建清单输出目录失败：%w", err)
	}

	// 生成期间客户端不许变：先记状态，生成完再比一次（脚本 L40/L67-68）。
	before, err := clientStates(client)
	if err != nil {
		return result, fmt.Errorf("无法读取客户端文件状态：%w", err)
	}

	// 重建前先把旧件轮换走：脚本用 os.link 做原子发布，前提是目标不存在；又因为硬链接
	// 不覆盖，脚本连 --force 都没有 —— 轮换是调用方的职责。旧件只改名不删除。
	result.Rotated = rotateStale(inner, manifestPath)
	if result.Rotated != "" {
		logf(opts, "内层 PVF 需要重建（%s）；旧件已备份为 %s", reason, filepath.Base(result.Rotated))
	}
	logf(opts, "正在从客户端 Script.pvf 生成内层 PVF（约 760 MB，请稍候）…")

	start := time.Now()
	stats, err := buildInnerPVF(client, inner, manifestPath, before)
	if err != nil {
		return result, err
	}
	result.Elapsed = time.Since(start)
	result.Generated = true
	result.Size, result.SHA256 = stats.Size, stats.InnerSHA256
	result.Segments, result.Keys = stats.Segments, stats.Keys
	return result, nil
}

// unwrapOuterTo 是剥壳入口。做成包级变量只是为了单测能替换它：真身
// （internal/catalog/pvf.UnwrapOuterTo）需要一份 760 MB 级的真实客户端，而轮换、
// 发布、清单、cache 这些编排逻辑应当能在没有客户端的机器上被测到。
var unwrapOuterTo = pvf.UnwrapOuterTo

// buildInnerPVF 是 prepare() 的生成段：临时文件 → 剥壳 → 复核 → 硬链接发布 → 写清单。
//
// 与脚本的差别只有两处（都是“失败时更干净”，成功产物逐字节相同）：
//   - 脚本先把旧件轮换掉再生成，这里同样是调用方先轮换（顺序不变）；
//   - 脚本发布后才做 inner 哈希复核并在不符时抛错（清单已落盘），这里复核在发布之前，
//     不符就不发布。
func buildInnerPVF(clientDir, inner, manifestPath string, before []innerCacheEntry) (pvf.OuterUnwrap, error) {
	source := filepath.Join(clientDir, "Script.pvf")

	// 临时产物与目标同目录：发布用的是硬链接，跨卷会失败。
	temp, err := os.CreateTemp(filepath.Dir(inner), innerTempPattern)
	if err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("创建临时产物失败：%w", err)
	}
	tempPath := temp.Name()
	// 发布是硬链接，临时文件的名字始终可以删（成功与失败都一样）。
	defer os.Remove(tempPath)

	stats, err := unwrapOuterTo(clientDir, source, temp)
	if closeErr := temp.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("关闭临时产物失败：%w", closeErr)
	}
	if err != nil {
		return pvf.OuterUnwrap{}, err
	}

	// 客户端在生成期间被改动过 ⇒ 作废（脚本 L67-68 的“retry from a stable source”）。
	after, err := clientStates(clientDir)
	if err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("无法读取客户端文件状态：%w", err)
	}
	if !sameClientStates(after, before) {
		return pvf.OuterUnwrap{}, fmt.Errorf("客户端文件在生成过程中发生了变化，请从稳定的来源重试")
	}

	// 复核：盘上那份的哈希必须等于剥壳时算出的哈希（脚本 L184 的同一件事）。
	published, err := fileSHA256(tempPath)
	if err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("复核内层 PVF 失败：%w", err)
	}
	if published != stats.InnerSHA256 {
		return pvf.OuterUnwrap{}, fmt.Errorf("生成的内层 PVF 哈希不符：期望 %s，实际 %s",
			shortHash(stats.InnerSHA256), shortHash(published))
	}

	// 剥壳不改变长度：清单里 outer.size 与 inner.size 都是产物大小（脚本也是同一个 size）。
	size := stats.Size
	manifest := innerManifest{
		Format:    innerFormat,
		Decoder:   innerDecoder,
		ClientExe: innerFingerprint{Path: filepath.Join(clientDir, "DFO.exe")},
		SkDat:     innerFingerprint{Path: filepath.Join(clientDir, "sk.dat")},
		Outer:     innerFingerprint{Path: source, Size: &size, SHA256: stats.OuterSHA256},
		Inner:     innerFingerprint{Path: inner, Size: &size, SHA256: stats.InnerSHA256},
		Cache:     after,
	}
	if err := fillFingerprint(&manifest.ClientExe); err != nil {
		return pvf.OuterUnwrap{}, err
	}
	if err := fillFingerprint(&manifest.SkDat); err != nil {
		return pvf.OuterUnwrap{}, err
	}
	body, err := renderInnerManifest(manifest)
	if err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("渲染清单失败：%w", err)
	}

	// 硬链接发布：目标已存在时失败，等价于脚本的 os.link（并发下也不会覆盖别人刚发布的产物）。
	if err := os.Link(tempPath, inner); err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("发布内层 PVF 失败（目标是否已存在？）：%w", err)
	}
	if err := saveInnerManifest(manifestPath, body); err != nil {
		return pvf.OuterUnwrap{}, fmt.Errorf("写清单失败：%w", err)
	}
	return stats, nil
}

// ensureInnerPVFForLaunch 是 launch_local.py 的 _ensure_inner_pvf：把生成包成“不阻断”。
//
// 返回的 Message 就是启动器要打印的那一行，与 Python 逐字相同：
//
//	复用     内层 PVF 无需重建：<reason>
//	生成     内层 PVF 已生成（耗时 %.1fs）
//	失败     WARNING: 内层 PVF 未就绪：<err>
//
// generate=false（--dry-run，Go 独有的开关）时只报告结论、不写盘。
func ensureInnerPVFForLaunch(client, inner, manifestPath string, generate bool) InnerPVFStatus {
	status := InnerPVFStatus{Path: inner, Manifest: manifestPath, Checked: true}

	result, err := PrepareInnerPVF(InnerPVFOptions{
		ClientDir: client,
		Inner:     inner,
		Manifest:  manifestPath,
		DryRun:    !generate,
		Log:       func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
	})
	// 生成失败不阻断启动：内层归档只影响直读模式，玩家还能显式 --json-mode 回退；
	// 失败原因如实报出来比吞掉更有用（Python 的 WARNING 就是这个口径）。
	if err != nil {
		status.NeedsBuild = true
		status.Reason = err.Error()
		status.Message = "WARNING: 内层 PVF 未就绪：" + err.Error()
		return status
	}
	status.NeedsBuild, status.Reason = result.NeedsBuild, result.Reason
	switch {
	case result.Generated:
		// 生成之后就没东西要重建了：NeedsBuild 描述的是“这一轮结束后盘上是否还需要生成”。
		status.NeedsBuild = false
		status.Message = fmt.Sprintf("内层 PVF 已生成（耗时 %.1fs）", result.Elapsed.Seconds())
	case !result.NeedsBuild:
		status.Message = "内层 PVF 无需重建：" + result.Reason
	default:
		// --dry-run：launch_local.py 没有这个开关，所以这句话没有 Python 对应物。
		status.Message = "内层 PVF 需要重建：" + result.Reason + "（--dry-run 只报告不生成）"
	}
	return status
}

// decideInnerPVF 镜像 ensure_inner_pvf.decide：纯只读判定，原因串与顺序一致。
//
// 廉价路径在前：三元组全等就直接复用，绝不为了“确认没变”去读 760 MB。
func decideInnerPVF(client, inner, manifestPath string) (bool, string) {
	return decideInnerManifest(client, inner, loadInnerManifest(manifestPath))
}

// decideInnerManifest 是 decide() 的本体，对应 Python 里那个能直接接字典的重载形态
// （ensure_inner_pvf.decide(client, inner, manifest)），也便于单测直接喂内存里的清单。
func decideInnerManifest(client, inner string, manifest *innerManifest) (bool, string) {
	if !regularFile(inner) {
		return true, "内层 PVF 不存在"
	}
	if manifest == nil {
		return true, "缺少或无法解析清单，旧件不可信"
	}
	if manifest.Format != innerFormat {
		// loadInnerManifest 已经拒绝了外来格式，这里再判一次是为了与 decide() 的顺序一致，
		// 也让“直接拿着清单结构调进来”的调用方同样安全。
		// `%r`：Python 用 repr，单引号；launch 的 Stage 1 阶段这里印的是 %q，两处都已不可达。
		return true, fmt.Sprintf("清单格式不符（'%s'）", manifest.Format)
	}

	// 先确认**盘上 inner 本身**与清单相符，与客户端是否变化无关：即便三件套没动，
	// 归档被截断/替换也必须重建，否则会拿着半截归档去起服。
	// 只比 size 不比 sha256：760 MB 的全量哈希是秒级开销，不能做成每次启动的常态。
	if manifest.Inner.Size != nil {
		info, err := os.Stat(inner)
		if err != nil {
			return true, "内层 PVF 无法读取"
		}
		if info.Size() != *manifest.Inner.Size {
			return true, fmt.Sprintf("内层 PVF 大小不符（盘上 %d，清单 %d）", info.Size(), *manifest.Inner.Size)
		}
	}

	// 快路径：三元组全等 ⇒ 直接复用。
	if len(manifest.Cache) == len(innerClientInputs) {
		states, err := clientStates(client)
		if err != nil {
			// Python 在 stat 抛 OSError 时走的就是这一句。
			return true, "无法读取客户端文件状态，按需重建"
		}
		if sameClientStates(states, manifest.Cache) {
			return false, "客户端与内层 PVF 均未变化，复用现有产物"
		}
	}

	// 慢路径：三元组变了（或快路径不可用），做全量 sha256 比对。
	exe, err := fileSHA256(filepath.Join(client, "DFO.exe"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	sk, err := fileSHA256(filepath.Join(client, "sk.dat"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	script, err := fileSHA256(filepath.Join(client, "Script.pvf"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	if exe == manifest.ClientExe.SHA256 && sk == manifest.SkDat.SHA256 && script == manifest.Outer.SHA256 {
		return false, "客户端指纹与清单一致，复用现有产物"
	}
	return true, "客户端 DFO.exe/sk.dat/Script.pvf 已变化"
}

// innerManifest 是清单文件的完整结构，键顺序与 prepare_inner_pvf.py 的写入顺序逐字对齐：
// format、decoder、client_exe、sk_dat、outer、inner、cache。Go 的 encoding/json 按字段
// 顺序输出，顺序错了就不再逐字节一致；同样的结构（含 cache）也被启动器侧
// 115us-dfolauncher/internal/pvfprep 使用，三处必须一致。
type innerManifest struct {
	Format    string            `json:"format"`
	Decoder   string            `json:"decoder"`
	ClientExe innerFingerprint  `json:"client_exe"`
	SkDat     innerFingerprint  `json:"sk_dat"`
	Outer     innerFingerprint  `json:"outer"`
	Inner     innerFingerprint  `json:"inner"`
	Cache     []innerCacheEntry `json:"cache"`
}

// innerFingerprint 是一个文件的指纹。Size 用指针是因为“清单里没写 size”必须能与
// “写了 0”区分开：Python 的 `recorded.get("size") not in (None, got)` 对缺失视为未知，
// 也就是不拦。写成值类型会让缺失变成 0 并误判成“大小不符”。
type innerFingerprint struct {
	Path   string `json:"path"`
	Size   *int64 `json:"size"`
	SHA256 string `json:"sha256"`
}

// innerCacheEntry 是 client_states() 的一条记录：一个输入文件的 (size, mtime_ns)。
type innerCacheEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	MtimeNS int64  `json:"mtime_ns"`
}

// fillFingerprint 把 Path 指向的文件补全成完整指纹（path + size + sha256）。
func fillFingerprint(fingerprint *innerFingerprint) error {
	info, err := os.Stat(fingerprint.Path)
	if err != nil {
		return fmt.Errorf("无法读取 %s 的状态：%w", fingerprint.Path, err)
	}
	sum, err := fileSHA256(fingerprint.Path)
	if err != nil {
		return fmt.Errorf("无法读取 %s 的指纹：%w", fingerprint.Path, err)
	}
	size := info.Size()
	fingerprint.Size = &size
	fingerprint.SHA256 = sum
	return nil
}

// renderInnerManifest 渲染清单字节，口径与 Python 的
// `json.dumps(manifest, ensure_ascii=False, indent=2) + "\n"` 相同：
// 两空格缩进、UTF-8 原样（不转义非 ASCII）、结尾一个换行。
func renderInnerManifest(manifest innerManifest) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	// ensure_ascii=False 的等价物：Go 默认会把 < > & 转义成 \u003c 之类，
	// Python 只在 indent 下转义控制字符与引号/反斜杠，所以必须关掉 HTML 转义。
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// saveInnerManifest 原子写清单（先写同目录临时文件再改名）。
func saveInnerManifest(path string, body []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// loadInnerManifest 读清单；文件不存在、无法解析、格式不符都返回 nil
// （调用方据此判“无法证明来源”并重建）。Python 的 load_manifest 也是这个口径，
// 所以“清单格式不符”在门禁里表现为“缺少或无法解析清单”。
func loadInnerManifest(path string) *innerManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// Python 用纯 utf-8 读（不认 BOM），带 BOM 的清单在那边同样解析失败。
	var manifest innerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	if manifest.Format != innerFormat {
		return nil
	}
	return &manifest
}

// clientStates 是廉价的比较键：只 stat，不读内容。
func clientStates(client string) ([]innerCacheEntry, error) {
	states := make([]innerCacheEntry, 0, len(innerClientInputs))
	for _, name := range innerClientInputs {
		info, err := os.Stat(filepath.Join(client, name))
		if err != nil {
			return nil, err
		}
		states = append(states, innerCacheEntry{
			Name:    name,
			Size:    info.Size(),
			MtimeNS: info.ModTime().UnixNano(),
		})
	}
	return states, nil
}

// sameClientStates 逐项按序比较，等价于 Python 的列表相等。
func sameClientStates(current, recorded []innerCacheEntry) bool {
	if len(current) != len(recorded) {
		return false
	}
	for index := range current {
		if current[index] != recorded[index] {
			return false
		}
	}
	return true
}

// fileSHA256 是指纹里存的那个哈希（分块读，不整文件载入内存）。
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// innerClientMissing 返回三件套里缺失的文件名（空切片表示齐全）。
func innerClientMissing(client string) []string {
	var missing []string
	for _, name := range innerClientInputs {
		if !regularFile(filepath.Join(client, name)) {
			missing = append(missing, name)
		}
	}
	return missing
}

// rotateStale 把已存在的产物改名成带时间戳的备份，返回第一个被轮换的路径（无则空串）。
//
// 为什么不删除：760 MB 的产物，生成可能失败；万一失败，玩家还能用回滚的旧件。
// 为什么必须轮换：脚本遇已存在输出会拒绝，且它的原子发布（os.link）依赖目标不存在。
// 命名规则与 ensure_inner_pvf.rotate_stale 一致：`<path>.stale-<stamp>`，
// 同秒重名时退成 `<path>.stale-<stamp>-1`、`-2`…
func rotateStale(paths ...string) string {
	stamp := time.Now().Format("20060102-150405")
	first := ""
	for _, path := range paths {
		if !regularFile(path) {
			continue
		}
		target := path + ".stale-" + stamp
		for index := 1; pathExists(target); index++ {
			target = fmt.Sprintf("%s.stale-%s-%d", path, stamp, index)
		}
		if err := os.Rename(path, target); err != nil {
			continue
		}
		if first == "" {
			first = target
		}
	}
	return first
}

// pathExists 是 Python 的 Path.exists()：文件、目录、符号链接都算存在。
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// pathWithin 报告 child 是否落在 dir 里（含 dir 本身），用于“产物不许写进客户端目录”。
func pathWithin(dir, child string) bool {
	relative, err := filepath.Rel(dir, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

// settingsClientDir 取 launcher.local.json 的 client_dir（相对路径按 server\ 解析），
// 与 launch 的默认客户端目录同源。
func settingsClientDir(serverRoot string) (string, error) {
	settings, err := loadLaunchSettings(filepath.Join(serverRoot, "launcher.local.json"))
	if err != nil {
		return "", err
	}
	if settings.ClientDir == nil || *settings.ClientDir == "" {
		return "", fmt.Errorf("launcher.local.json must set client_dir")
	}
	return resolveUnder(serverRoot, *settings.ClientDir), nil
}

// logf 调用可选的日志回调。
func logf(opts InnerPVFOptions, format string, args ...any) {
	if opts.Log != nil {
		opts.Log(format, args...)
	}
}

// shortHash 把哈希截短到 12 位给报错用（与启动器侧 pvfprep 的 same wording 一致）。
func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "…"
}

// 路径推导 ------------------------------------------------------------------
// 这两个函数是启动器与服务端两条链共同的约定：产物落在 server/work/client-build/
// （.gitignore 忽略整目录），客户端目录是只读输入源，所以不能把归档写在客户端里。

// InnerPVFPath 返回内层归档的默认落地路径。
func InnerPVFPath(root string) string {
	return filepath.Join(root, "server", "work", "client-build", "Script.inner.pvf")
}

// InnerPVFManifestPath 返回清单的默认落地路径（与内层归档同目录）。
func InnerPVFManifestPath(root string) string {
	return filepath.Join(root, "server", "work", "client-build", "Script.inner.manifest.json")
}
