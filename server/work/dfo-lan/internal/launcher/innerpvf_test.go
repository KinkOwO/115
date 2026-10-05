package launcher

// Stage 4 的验收面：四态门禁的每个分支、指纹、清单渲染风格、--dry-run 不写盘、
// 轮换保留回滚件、以及 launch 那几句话的措辞。
//
// 剥壳真身（internal/catalog/pvf）有自己的合成 PE 往返测试；这里用 unwrapOuterTo 这个
// 接缝替换掉它，专心覆盖编排（临时文件、硬链接发布、清单、cache、轮换）。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// stubUnwrap 替换剥壳入口：写出确定的字节并返回与之一致的统计量，
// 这样 buildInnerPVF 的复核、清单与 cache 全都能真跑。
func stubUnwrap(t *testing.T, body []byte) {
	t.Helper()
	previous := unwrapOuterTo
	unwrapOuterTo = func(clientDir, source string, out io.Writer) (pvf.OuterUnwrap, error) {
		if _, err := out.Write(body); err != nil {
			return pvf.OuterUnwrap{}, err
		}
		sum := sha256.Sum256(body)
		return pvf.OuterUnwrap{
			Size:        int64(len(body)),
			OuterSHA256: hex.EncodeToString(sum[:]),
			InnerSHA256: hex.EncodeToString(sum[:]),
			Segments:    1,
			Keys:        1,
		}, nil
	}
	t.Cleanup(func() { unwrapOuterTo = previous })
}

// buildInnerTree 造一棵“准备内层 PVF”用的最小树：客户端三件套 + launcher.local.json
// （client_dir 写相对路径，用来验证默认值解析），产物目录故意先不存在。
func buildInnerTree(t *testing.T) (root, client string) {
	t.Helper()
	root = t.TempDir()
	client = filepath.Join(root, "client")
	if err := os.MkdirAll(client, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(client, "DFO.exe"), "exe-bytes")
	write(filepath.Join(client, "sk.dat"), "sk-bytes")
	write(filepath.Join(client, "Script.pvf"), "script-bytes")
	write(filepath.Join(root, "server", "launcher.local.json"), `{"client_dir":"../client"}`)
	return root, client
}

// gateFixture 是喂给内存门禁的一份“与客户端一致”的清单。
func gateFixture(t *testing.T, client, inner string) *innerManifest {
	t.Helper()
	info, err := os.Stat(inner)
	if err != nil {
		t.Fatal(err)
	}
	states := mustStates(t, client)
	size := info.Size()
	return &innerManifest{
		Format:    innerFormat,
		Decoder:   innerDecoder,
		ClientExe: innerFingerprint{SHA256: mustHash(t, filepath.Join(client, "DFO.exe"))},
		SkDat:     innerFingerprint{SHA256: mustHash(t, filepath.Join(client, "sk.dat"))},
		Outer:     innerFingerprint{SHA256: mustHash(t, filepath.Join(client, "Script.pvf"))},
		Inner:     innerFingerprint{Size: &size},
		Cache:     states,
	}
}

func mustHash(t *testing.T, path string) string {
	t.Helper()
	sum, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func mustStates(t *testing.T, client string) []innerCacheEntry {
	t.Helper()
	states, err := clientStates(client)
	if err != nil {
		t.Fatal(err)
	}
	return states
}

// 归档缺失：与清单是否可信无关，第一句就是它。
func TestDecideInnerManifestMissingArchive(t *testing.T) {
	root, client := buildInnerTree(t)
	inner := writeInnerArchive(t, root, "inner-archive")
	manifest := gateFixture(t, client, inner)
	if needsBuild, reason := decideInnerManifest(client, filepath.Join(root, "gone.pvf"), manifest); !needsBuild ||
		reason != "内层 PVF 不存在" {
		t.Errorf("missing archive → (%v, %q)", needsBuild, reason)
	}
}

// 四态门禁的每个分支：一致（快/慢两路）、不一致、格式不符、大小不符、清单缺失、
// 读不到客户端状态，外加“清单没写 size 就是未知”的 Python 语义。
func TestDecideInnerManifestBranches(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(fixture *innerManifest) *innerManifest
		missing string // 从客户端删掉的文件
		build   bool
		reason  string
	}{
		{name: "复用（cache 快路径命中）", build: false, reason: "客户端与内层 PVF 均未变化，复用现有产物"},
		{
			name: "复用（cache 失配但指纹一致）",
			mutate: func(fixture *innerManifest) *innerManifest {
				stale := append([]innerCacheEntry(nil), fixture.Cache...)
				stale[1].MtimeNS++
				fixture.Cache = stale
				return fixture
			},
			build: false, reason: "客户端指纹与清单一致，复用现有产物",
		},
		{
			name: "重建（客户端三件套已变化）",
			mutate: func(fixture *innerManifest) *innerManifest {
				// 快路径也要失配，否则门禁走不到指纹比对那一层。
				fixture.Cache[0].MtimeNS++
				fixture.ClientExe.SHA256 = strings.Repeat("0", 64)
				return fixture
			},
			build: true, reason: "客户端 DFO.exe/sk.dat/Script.pvf 已变化",
		},
		{
			name: "重建（清单格式不符：防御分支）",
			mutate: func(fixture *innerManifest) *innerManifest {
				fixture.Format = "something-else"
				return fixture
			},
			build: true, reason: "清单格式不符（'something-else'）",
		},
		{
			name: "重建（盘上归档大小不符）",
			mutate: func(fixture *innerManifest) *innerManifest {
				other := int64(12345)
				fixture.Inner.Size = &other
				return fixture
			},
			build: true, reason: "内层 PVF 大小不符",
		},
		{
			name: "复用（清单没写 size ⇒ 未知，不拦）",
			mutate: func(fixture *innerManifest) *innerManifest {
				fixture.Inner.Size = nil
				return fixture
			},
			build: false, reason: "客户端与内层 PVF 均未变化，复用现有产物",
		},
		{
			name:   "重建（清单缺失）",
			mutate: func(fixture *innerManifest) *innerManifest { return nil },
			build:  true, reason: "缺少或无法解析清单，旧件不可信",
		},
		{
			name:    "重建（客户端读不到状态）",
			missing: "sk.dat",
			build:   true, reason: "无法读取客户端文件状态，按需重建",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, client := buildInnerTree(t)
			inner := writeInnerArchive(t, root, "inner-archive")
			fixture := gateFixture(t, client, inner)
			if testCase.missing != "" {
				if err := os.Remove(filepath.Join(client, testCase.missing)); err != nil {
					t.Fatal(err)
				}
			}
			if testCase.mutate != nil {
				fixture = testCase.mutate(fixture)
			}
			needsBuild, reason := decideInnerManifest(client, inner, fixture)
			if needsBuild != testCase.build {
				t.Errorf("needs build = %v, want %v（reason %q）", needsBuild, testCase.build, reason)
			}
			if !strings.Contains(reason, testCase.reason) {
				t.Errorf("reason = %q, want it to contain %q", reason, testCase.reason)
			}
		})
	}
}

// 走文件的入口与走内存的入口必须给出同一个结论：清单文件格式不符时，
// 读盘那一侧表现为“缺少或无法解析清单，旧件不可信”（与 Python 的 load_manifest 相同）。
func TestLoadInnerManifestRejectsForeignFormat(t *testing.T) {
	root, _ := buildInnerTree(t)
	manifestPath := InnerPVFManifestPath(root)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{"format":"something-else"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if manifest := loadInnerManifest(manifestPath); manifest != nil {
		t.Errorf("外来格式的清单必须是 nil，got %+v", manifest)
	}
	if manifest := loadInnerManifest(filepath.Join(root, "nope.json")); manifest != nil {
		t.Errorf("缺失的清单必须是 nil，got %+v", manifest)
	}
	if err := os.WriteFile(manifestPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if manifest := loadInnerManifest(manifestPath); manifest != nil {
		t.Errorf("坏 JSON 必须是 nil，got %+v", manifest)
	}
}

func writeInnerArchive(t *testing.T, root, body string) string {
	t.Helper()
	inner := InnerPVFPath(root)
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return inner
}

// 指纹就是 Python fingerprint() 的那三个字段：path、size、sha256。
func TestFillFingerprintMatchesPythonShape(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.bin")
	body := []byte("fingerprint me")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprint := innerFingerprint{Path: path}
	if err := fillFingerprint(&fingerprint); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if fingerprint.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("sha256 = %s", fingerprint.SHA256)
	}
	if fingerprint.Size == nil || *fingerprint.Size != int64(len(body)) {
		t.Errorf("size = %v, want %d", fingerprint.Size, len(body))
	}
	if fingerprint.Path != path {
		t.Errorf("path = %s", fingerprint.Path)
	}

	// 缺文件必须报错，不能写出一个 size=0 的假指纹。
	missing := innerFingerprint{Path: filepath.Join(root, "nope.bin")}
	if err := fillFingerprint(&missing); err == nil {
		t.Error("缺文件时应报错")
	}
}

// 清单渲染必须与 Python 的 json.dumps(ensure_ascii=False, indent=2) + "\n" 同口径：
// 键顺序、两空格缩进、非 ASCII 原样（不转义）、结尾一个换行。
func TestRenderInnerManifestStyle(t *testing.T) {
	size := int64(7)
	manifest := innerManifest{
		Format:    innerFormat,
		Decoder:   innerDecoder,
		ClientExe: innerFingerprint{Path: `C:\客户端\DFO.exe`, Size: &size, SHA256: "aa"},
		SkDat:     innerFingerprint{Path: `C:\客户端\sk.dat`, Size: &size, SHA256: "bb"},
		Outer:     innerFingerprint{Path: `C:\客户端\Script.pvf`, Size: &size, SHA256: "cc"},
		Inner:     innerFingerprint{Path: `C:\build\Script.inner.pvf`, Size: &size, SHA256: "dd"},
		Cache:     []innerCacheEntry{{Name: "DFO.exe", Size: 7, MtimeNS: 8}},
	}
	body, err := renderInnerManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "format": "dfo_20260901_inner",
  "decoder": "pvf_archive.wrapper_keys/aes",
  "client_exe": {
    "path": "C:\\客户端\\DFO.exe",
    "size": 7,
    "sha256": "aa"
  },
  "sk_dat": {
    "path": "C:\\客户端\\sk.dat",
    "size": 7,
    "sha256": "bb"
  },
  "outer": {
    "path": "C:\\客户端\\Script.pvf",
    "size": 7,
    "sha256": "cc"
  },
  "inner": {
    "path": "C:\\build\\Script.inner.pvf",
    "size": 7,
    "sha256": "dd"
  },
  "cache": [
    {
      "name": "DFO.exe",
      "size": 7,
      "mtime_ns": 8
    }
  ]
}
`
	if string(body) != want {
		t.Errorf("清单渲染 =\n%s\nwant\n%s", body, want)
	}
	if !bytes.HasSuffix(body, []byte("}\n")) {
		t.Error("清单必须以单个换行结尾")
	}
}

// 真实客户端那份清单是 Python 写的：Go 的门禁必须能读它，并对同一份客户端判“复用”。
// 相关产物是 .gitignore 的本地构建产物，没有就跳过。
func TestDecideOnThePythonWrittenManifest(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(root, "server", "work", "client-build", "Script.inner.pvf")
	manifestPath := filepath.Join(root, "server", "work", "client-build", "Script.inner.manifest.json")
	if !regularFile(manifestPath) || !regularFile(inner) {
		t.Skipf("本机没有 Python 写的内层 PVF 产物：%s", manifestPath)
	}
	manifest := loadInnerManifest(manifestPath)
	if manifest == nil {
		t.Fatalf("无法解析 Python 写的清单：%s", manifestPath)
	}
	if manifest.Format != innerFormat || manifest.Decoder != innerDecoder {
		t.Errorf("format/decoder = %q/%q", manifest.Format, manifest.Decoder)
	}
	if manifest.Inner.SHA256 == "" || manifest.Inner.Size == nil || *manifest.Inner.Size == 0 {
		t.Errorf("inner 指纹不完整：%+v", manifest.Inner)
	}
	if len(manifest.Cache) != len(innerClientInputs) {
		t.Errorf("cache 长度 = %d, want %d", len(manifest.Cache), len(innerClientInputs))
	}
	client, err := settingsClientDir(filepath.Join(root, "server"))
	if err != nil || !regularFile(filepath.Join(client, "Script.pvf")) {
		t.Skipf("本机没有 launcher.local.json 指向的客户端：%v", err)
	}
	if needsBuild, reason := decideInnerManifest(client, inner, manifest); needsBuild {
		t.Errorf("真实客户端与 Python 写的清单应当判复用：%v（%s）", needsBuild, reason)
	}
}

// --dry-run 一个字节都不写：连轮换都不做，更不会生成归档。
func TestPrepareInnerPVFDryRunWritesNothing(t *testing.T) {
	root, client := buildInnerTree(t)
	inner := InnerPVFPath(root)
	manifestPath := InnerPVFManifestPath(root)

	result, err := PrepareInnerPVF(InnerPVFOptions{
		Root: root, ClientDir: client, Inner: inner, Manifest: manifestPath, DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !result.NeedsBuild || result.Generated || result.Reason != "内层 PVF 不存在" {
		t.Errorf("dry run 判定 = %+v", result)
	}
	if regularFile(inner) || regularFile(manifestPath) {
		t.Error("--dry-run 不应生成产物")
	}
	if entries, err := os.ReadDir(filepath.Dir(inner)); err == nil && len(entries) > 0 {
		t.Errorf("--dry-run 不应在产物目录里留下任何东西：%v", entries)
	}
}

// --force 无视门禁；轮换必须保留回滚件，并且清单与 cache 都要写对。
func TestPrepareInnerPVFBuildsRotatesAndReuses(t *testing.T) {
	root, client := buildInnerTree(t)
	stubUnwrap(t, []byte("fresh-inner-bytes"))
	inner := InnerPVFPath(root)
	manifestPath := InnerPVFManifestPath(root)

	// 第一次：产物目录还不存在，门禁判“内层 PVF 不存在” → 生成。
	first, err := PrepareInnerPVF(InnerPVFOptions{Root: root, ClientDir: client, Inner: inner, Manifest: manifestPath})
	if err != nil {
		t.Fatalf("首次生成: %v", err)
	}
	if !first.Generated || first.Rotated != "" {
		t.Errorf("首次生成 = %+v", first)
	}
	if first.Size != int64(len("fresh-inner-bytes")) || first.Segments != 1 || first.Keys != 1 {
		t.Errorf("产物身份 = %+v", first)
	}
	body, err := os.ReadFile(inner)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "fresh-inner-bytes" {
		t.Errorf("归档内容 = %q", body)
	}

	// 清单：字段、inner 指纹与 cache（三个输入）都要对得上。
	manifest := loadInnerManifest(manifestPath)
	if manifest == nil {
		t.Fatal("清单没写成可读的形态")
	}
	wantHash := sha256.Sum256([]byte("fresh-inner-bytes"))
	if manifest.Inner.SHA256 != hex.EncodeToString(wantHash[:]) || manifest.Inner.Size == nil ||
		*manifest.Inner.Size != int64(len("fresh-inner-bytes")) {
		t.Errorf("inner 指纹 = %+v", manifest.Inner)
	}
	if manifest.Inner.Path != inner {
		t.Errorf("inner.path = %q, want %q", manifest.Inner.Path, inner)
	}
	if states, err := clientStates(client); err != nil || !sameClientStates(states, manifest.Cache) {
		t.Errorf("cache 回填 = %+v (err %v)", manifest.Cache, err)
	}
	// 渲染出来的字节必须能原样喂回门禁（写—读—判一致）。
	if reloaded := loadInnerManifest(manifestPath); reloaded == nil {
		t.Fatal("清单回读失败")
	}

	// 第二次：门禁走快路径复用，一个字节都不生成。
	second, err := PrepareInnerPVF(InnerPVFOptions{Root: root, ClientDir: client, Inner: inner, Manifest: manifestPath})
	if err != nil {
		t.Fatalf("复用: %v", err)
	}
	if second.Generated || second.NeedsBuild {
		t.Errorf("第二次 = %+v，want 复用", second)
	}
	if second.Reason != "客户端与内层 PVF 均未变化，复用现有产物" {
		t.Errorf("复用原因 = %q", second.Reason)
	}

	// 第三次：--force 强制重建，旧件轮换保留。
	stubUnwrap(t, []byte("second-inner-bytes"))
	forced, err := PrepareInnerPVF(InnerPVFOptions{
		Root: root, ClientDir: client, Inner: inner, Manifest: manifestPath, Force: true,
	})
	if err != nil {
		t.Fatalf("强制重建: %v", err)
	}
	if !forced.Generated || forced.Rotated == "" {
		t.Fatalf("强制重建 = %+v", forced)
	}
	if !strings.Contains(forced.Rotated, ".stale-") {
		t.Errorf("轮换名 = %s，want .stale- 后缀", forced.Rotated)
	}
	if rolled, err := os.ReadFile(forced.Rotated); err != nil || string(rolled) != "fresh-inner-bytes" {
		t.Errorf("回滚件 = %q (err %v)，want 旧内容", rolled, err)
	}
	if current, err := os.ReadFile(inner); err != nil || string(current) != "second-inner-bytes" {
		t.Errorf("新归档 = %q (err %v)", current, err)
	}
}

// 生成期间客户端被改动 ⇒ 作废，不发布（脚本 L67-68 的同一条）。
func TestPrepareInnerPVFRefusesClientChangedDuringBuild(t *testing.T) {
	root, client := buildInnerTree(t)
	previous := unwrapOuterTo
	unwrapOuterTo = func(clientDir, source string, out io.Writer) (pvf.OuterUnwrap, error) {
		if err := os.WriteFile(filepath.Join(clientDir, "Script.pvf"), []byte("changed-mid-build"), 0o644); err != nil {
			return pvf.OuterUnwrap{}, err
		}
		if _, err := out.Write([]byte("whatever")); err != nil {
			return pvf.OuterUnwrap{}, err
		}
		return pvf.OuterUnwrap{Size: 8, OuterSHA256: "aa", InnerSHA256: "bb"}, nil
	}
	t.Cleanup(func() { unwrapOuterTo = previous })

	_, err := PrepareInnerPVF(InnerPVFOptions{
		Root: root, ClientDir: client, Inner: InnerPVFPath(root), Manifest: InnerPVFManifestPath(root),
	})
	if err == nil || !strings.Contains(err.Error(), "生成过程中发生了变化") {
		t.Fatalf("error = %v, want the mid-build change refusal", err)
	}
	if regularFile(InnerPVFPath(root)) {
		t.Error("作废的生成不应发布产物")
	}
}

// 边界：缺三件套、落点非法、以及“输出落在客户端目录里”都必须明确拒绝。
func TestPrepareInnerPVFRefusals(t *testing.T) {
	cases := []struct {
		name     string
		missing  string
		inner    func(root, client string) string
		manifest func(root, client string) string
		want     string
	}{
		{
			name: "缺 sk.dat", missing: "sk.dat",
			inner:    func(root, client string) string { return InnerPVFPath(root) },
			manifest: func(root, client string) string { return InnerPVFManifestPath(root) },
			want:     "客户端缺少 sk.dat，无法生成内层 PVF",
		},
		{
			name: "缺 DFO.exe", missing: "DFO.exe",
			inner:    func(root, client string) string { return InnerPVFPath(root) },
			manifest: func(root, client string) string { return InnerPVFManifestPath(root) },
			want:     "DFO.exe",
		},
		{
			name:     "归档与清单同路径",
			inner:    func(root, client string) string { return InnerPVFPath(root) },
			manifest: func(root, client string) string { return InnerPVFPath(root) },
			want:     "不能相同",
		},
		{
			name:     "产物落在客户端目录内",
			inner:    func(root, client string) string { return filepath.Join(client, "inner.pvf") },
			manifest: func(root, client string) string { return InnerPVFManifestPath(root) },
			want:     "不能落在只读的客户端目录内",
		},
		{
			name:     "清单落在客户端目录内",
			inner:    func(root, client string) string { return InnerPVFPath(root) },
			manifest: func(root, client string) string { return filepath.Join(client, "m.json") },
			want:     "不能落在只读的客户端目录内",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, client := buildInnerTree(t)
			if testCase.missing != "" {
				if err := os.Remove(filepath.Join(client, testCase.missing)); err != nil {
					t.Fatal(err)
				}
			}
			_, err := PrepareInnerPVF(InnerPVFOptions{
				Root: root, ClientDir: client,
				Inner: testCase.inner(root, client), Manifest: testCase.manifest(root, client),
			})
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want it to contain %q", err, testCase.want)
			}
		})
	}
}

// 默认值来自 launcher.local.json 与既有约定：不给 --client/--inner/--manifest 也能定位。
func TestPrepareInnerPVFDefaultsFromSettings(t *testing.T) {
	root, client := buildInnerTree(t)
	stubUnwrap(t, []byte("from-settings"))
	result, err := PrepareInnerPVF(InnerPVFOptions{Root: root})
	if err != nil {
		t.Fatalf("默认路径: %v", err)
	}
	if result.Inner != InnerPVFPath(root) || result.Manifest != InnerPVFManifestPath(root) {
		t.Errorf("默认产物位置 = %s / %s", result.Inner, result.Manifest)
	}
	if !result.Generated {
		t.Errorf("应当生成：%+v", result)
	}
	// client_dir 是 "../client"（相对 server\），必须解析成同级目录，产物也必须真在客户端里没动过。
	if !regularFile(filepath.Join(client, "Script.pvf")) {
		t.Error("客户端三件套不该被改动")
	}
	if filepath.Dir(result.Inner) != filepath.Join(root, "server", "work", "client-build") {
		t.Errorf("产物目录 = %s", filepath.Dir(result.Inner))
	}
}

// launch 的几句话：复用（与 Python 逐字一致）、已生成（带耗时）、失败（WARNING 不阻断）。
func TestEnsureInnerPVFForLaunchMessages(t *testing.T) {
	root, client := buildInnerTree(t)
	inner := InnerPVFPath(root)
	manifestPath := InnerPVFManifestPath(root)

	// 1. 三件套不齐：Python 的 ensure() 先抛，_ensure_inner_pvf 把它包成 WARNING。
	for _, name := range innerClientInputs {
		if err := os.Remove(filepath.Join(client, name)); err != nil {
			t.Fatal(err)
		}
	}
	status := ensureInnerPVFForLaunch(client, inner, manifestPath, true)
	want := "WARNING: 内层 PVF 未就绪：客户端缺少 DFO.exe、sk.dat、Script.pvf，无法生成内层 PVF" +
		"（需 DFO.exe + sk.dat + Script.pvf 三件套）"
	if status.Message != want {
		t.Errorf("message = %q\nwant     %q", status.Message, want)
	}
	if !status.Checked || !status.NeedsBuild {
		t.Errorf("status = %+v", status)
	}

	// 2. 生成成功：句子形态与 Python 的 `内层 PVF 已生成（耗时 %.1fs）` 相同。
	freshClient := filepath.Join(root, "client2")
	writeClientTriple(t, freshClient, "fresh-")
	stubUnwrap(t, []byte("generated-inner"))
	status = ensureInnerPVFForLaunch(freshClient, inner, manifestPath, true)
	if !strings.HasPrefix(status.Message, "内层 PVF 已生成（耗时 ") || status.NeedsBuild {
		t.Errorf("message = %q", status.Message)
	}

	// 3. 复用：Stage 1 的输出契约，逐字一致。
	status = ensureInnerPVFForLaunch(freshClient, inner, manifestPath, true)
	if want := "内层 PVF 无需重建：客户端与内层 PVF 均未变化，复用现有产物"; status.Message != want {
		t.Errorf("message = %q\nwant     %q", status.Message, want)
	}

	// 4. --dry-run：只报告；旧件在场也不许改名。
	otherClient := filepath.Join(root, "client3")
	writeClientTriple(t, otherClient, "other-")
	status = ensureInnerPVFForLaunch(otherClient, inner, manifestPath, false)
	if !strings.HasPrefix(status.Message, "内层 PVF 需要重建：") || !status.NeedsBuild {
		t.Errorf("--dry-run message = %q", status.Message)
	}
	if !regularFile(inner) {
		t.Error("--dry-run 不应动到产物")
	}
	if entries, err := os.ReadDir(filepath.Dir(inner)); err == nil {
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".stale-") {
				t.Errorf("--dry-run 不应轮换产物：%s", entry.Name())
			}
		}
	}
}

func writeClientTriple(t *testing.T, dir, prefix string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range innerClientInputs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(prefix+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// --dry-run 与真生成的区别必须在盘上看得见：同一个陈旧状态，dry-run 后原样。
func TestLaunchDryRunDoesNotTouchTheArchive(t *testing.T) {
	root := buildLaunchTree(t)
	inner := filepath.Join(root, "server", "work", "client-build", "Script.inner.pvf")
	manifest := filepath.Join(root, "server", "work", "client-build", "Script.inner.manifest.json")
	// 让门禁判“重建”：清单格式不符。
	if err := os.WriteFile(manifest, []byte(`{"format":"other"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run plan: %v", err)
	}
	if !report.InnerPVF.NeedsBuild || !strings.Contains(report.InnerPVF.Message, "--dry-run") {
		t.Errorf("inner status = %+v", report.InnerPVF)
	}
	body, err := os.ReadFile(manifest)
	if err != nil || string(body) != `{"format":"other"}` {
		t.Errorf("--dry-run 改动了清单：%q (err %v)", body, err)
	}
	if !regularFile(inner) {
		t.Error("--dry-run 不应动到归档")
	}
	// 四个步骤仍要打出来，第二名（内层 PVF）指向 Go 的现场生成。
	if len(report.Plan) != 4 || report.Plan[1].Step != "内层 PVF" ||
		!strings.Contains(report.Plan[1].Line(), "现场生成") {
		t.Errorf("plan = %v", report.Plan)
	}
}
