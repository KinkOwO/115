package pvf

// 本文件用一枚**合成 PE** 覆盖整条外层剥壳链：PE 字面量 → RSA/PKCS#1 v1.5 → AES 段密钥
// → 10 MiB 分块 → nkpi 校验 → 双边哈希。
//
// 合成客户端的三件套是“先造内层明文，再按同一套规则加壳”得来的，所以断言是真的往返比对
// （逐字节），而不是让同一份实现自证。真实客户端（760 MB）的比对不放在单测里，见
// docs/go-launch-migration-plan.md Stage 4 的验收记录。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// 合成 PE 的布局：ImageBase 0x140000000，一个节覆盖 RVA 0xDC98000 起 0x4000 字节，
	// 原始数据从文件偏移 0x400 开始 —— 这样两个字面量槽正好落在生产代码写死的
	// wrapperPrivateKeyVA / wrapperAESKeyVA 上。
	syntheticImageBase  = 0x140000000
	syntheticSectionRVA = 0xDC98000
	syntheticSectionRaw = 0x400
	syntheticSectionLen = 0x4000
	syntheticPEMOffset  = 0x200 // 节内偏移 → VA 0x14DC98200
	syntheticKeyOffset  = 0x1000
	// 段密钥的“节内偏移”就是两个字面量槽：RVA 0xDC98110 / 0xDC98118。
	syntheticSlotOffset = 0x110
)

// syntheticClient 是一套合成客户端：目录、内层明文，以及它对应的 12 把段密钥。
type syntheticClient struct {
	dir   string
	inner []byte
	keys  [][]byte
}

// writeSyntheticClient 在 dir 下造一套能跑通剥壳的三件套，返回内层明文的期望字节。
func writeSyntheticClient(t *testing.T, dir string) syntheticClient {
	t.Helper()

	// 1. DFO.exe 内嵌的 RSA-2048 私钥（PEM 就是字面量的内容）。
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// 2. AES key 字面量（十六进制字符串）与 12 把段密钥。
	//    384 字节 > 256，这样 wrapperKeys 的 `prefix = len/256*256` 那一段（前 256 字节
	//    要再解一次 AES）才真的被覆盖到。
	aesKey := randomBytes(t, 32)
	keys := make([]byte, 0, 12*wrapperKeySize)
	for index := 0; index < 12; index++ {
		keys = append(keys, randomBytes(t, wrapperKeySize)...)
	}
	// sk.dat 里存的是“加壳后”的段密钥：前 256 字节先用同一把 AES key 加密。
	wrapped := append([]byte(nil), keys...)
	aesCBCZeroIVEncrypt(t, aesKey, wrapped[:256])
	skDat := rsaBlocks(t, key, wrapped)

	writeFile(t, filepath.Join(dir, "DFO.exe"), syntheticPE(privatePEM, []byte(hex.EncodeToString(aesKey))))
	writeFile(t, filepath.Join(dir, "sk.dat"), skDat)

	// 3. 内层明文 → 客户端 Script.pvf：全 10 MiB 段加壳，尾段 0x3000 字节（≥ 0x2800，
	//    所以是一段合法的“受保护前缀”）。
	//
	//    注意内层归档的**前 48 字节本身也是过流处理的**（`iNfO` 密钥流，读侧
	//    parse.go 会再解一次；剥壳只负责校验它解出来是 nkpi，不改写它）。
	//    所以这里的明文首 48 字节要预先写成“过完流是 nkpi”的形态。
	inner := make([]byte, outerBlockSize+0x3000)
	header := make([]byte, 48)
	copy(header, "nkpi")
	decryptProtected("iNfO", header)
	copy(inner, header)
	for index := 48; index < len(inner); index++ {
		inner[index] = byte(index * 31)
	}
	outer := wrapOuter(t, inner, splitKeys(keys))
	writeFile(t, filepath.Join(dir, "Script.pvf"), outer)
	return syntheticClient{dir: dir, inner: inner, keys: splitKeys(keys)}
}

// syntheticPE 造一枚 debug/pe 能读、且字面量槽落在生产代码写死的 VA 上的 PE32+。
func syntheticPE(privatePEM, aesKeyLiteral []byte) []byte {
	file := make([]byte, syntheticSectionRaw+syntheticSectionLen)
	copy(file, "MZ")
	binary.LittleEndian.PutUint32(file[0x3C:], 0x80) // e_lfanew
	copy(file[0x80:], "PE\x00\x00")

	coff := file[0x84:]
	binary.LittleEndian.PutUint16(coff[0:], 0x8664)  // Machine: AMD64
	binary.LittleEndian.PutUint16(coff[2:], 1)       // NumberOfSections
	binary.LittleEndian.PutUint16(coff[16:], 0xF0)   // SizeOfOptionalHeader
	binary.LittleEndian.PutUint16(coff[18:], 0x0022) // EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

	optional := file[0x98:]
	binary.LittleEndian.PutUint16(optional[0:], 0x20B) // PE32+
	binary.LittleEndian.PutUint32(optional[32:], 0x1000)
	binary.LittleEndian.PutUint32(optional[36:], 0x200)
	binary.LittleEndian.PutUint64(optional[24:], syntheticImageBase)
	binary.LittleEndian.PutUint32(optional[56:], 0xDC9D000) // SizeOfImage
	binary.LittleEndian.PutUint32(optional[60:], syntheticSectionRaw)
	binary.LittleEndian.PutUint16(optional[68:], 2) // Subsystem: GUI
	binary.LittleEndian.PutUint32(optional[108:], 16)

	section := file[0x188:]
	copy(section, ".rdata\x00\x00")
	binary.LittleEndian.PutUint32(section[8:], syntheticSectionLen)  // VirtualSize
	binary.LittleEndian.PutUint32(section[12:], syntheticSectionRVA) // VirtualAddress
	binary.LittleEndian.PutUint32(section[16:], syntheticSectionLen) // SizeOfRawData
	binary.LittleEndian.PutUint32(section[20:], syntheticSectionRaw) // PointerToRawData

	// 两个字面量槽：qword 指针（绝对 VA）→ 字符串数据。
	raw := file[syntheticSectionRaw:]
	pemVA := uint64(syntheticImageBase + syntheticSectionRVA + syntheticPEMOffset)
	keyVA := uint64(syntheticImageBase + syntheticSectionRVA + syntheticKeyOffset)
	binary.LittleEndian.PutUint64(raw[syntheticSlotOffset:], pemVA)
	binary.LittleEndian.PutUint64(raw[syntheticSlotOffset+8:], keyVA)
	copy(raw[syntheticPEMOffset:], append(privatePEM, 0))
	copy(raw[syntheticKeyOffset:], append(aesKeyLiteral, 0))
	return file
}

// wrapOuter 是生产代码剥壳的逆操作：把内层明文变成客户端那份带保护的 Script.pvf。
//
// 只做 AES 这一层：`iNfO` 那层是**归档内部**的混淆（读侧 parse.go 自己会解），不是外层
// 保护的一部分，剥壳既不解除它、也不改写它，两边应当原样保留。
func wrapOuter(t *testing.T, inner []byte, keys [][]byte) []byte {
	t.Helper()
	outer := append([]byte(nil), inner...)
	for index := 0; index < len(keys); index++ {
		start := index * outerBlockSize
		end := start + outerPrefixSize
		if end > len(outer) {
			break
		}
		aesCBCZeroIVEncrypt(t, keys[index], outer[start:end])
	}
	return outer
}

// aesCBCZeroIVEncrypt 是生产代码那个解密原语的逆运算（只在本测试里用；服务端启动链
// 只需要剥壳方向，参考实现见 reference/analysis-tools/rewrap_pvf*.py）。
func aesCBCZeroIVEncrypt(t *testing.T, key, buf []byte) {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf)%aes.BlockSize != 0 {
		t.Fatalf("AES 数据长度 %d 不是 %d 的整数倍", len(buf), aes.BlockSize)
	}
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(buf, buf)
}

// rsaBlocks 把原始密钥材料切块后逐块 RSA/PKCS#1 v1.5 加密，构成 sk.dat。
// 每块不超过 k-11 字节，解开后拼起来正好是原样的材料。
func rsaBlocks(t *testing.T, key *rsa.PrivateKey, plain []byte) []byte {
	t.Helper()
	limit := key.Size() - 11
	var out []byte
	for offset := 0; offset < len(plain); offset += limit {
		end := offset + limit
		if end > len(plain) {
			end = len(plain)
		}
		block, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, plain[offset:end])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, block...)
	}
	return out
}

func splitKeys(flat []byte) [][]byte {
	keys := make([][]byte, 0, len(flat)/wrapperKeySize)
	for offset := 0; offset < len(flat); offset += wrapperKeySize {
		keys = append(keys, flat[offset:offset+wrapperKeySize])
	}
	return keys
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return buf
}

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// firstDifference 报出两段字节的第一个差异位置，失败信息才有诊断价值。
func firstDifference(left, right []byte) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return -1
}

// 往返：合成客户端 → 剥壳 → 与内层明文逐字节相同，且双边哈希与段数正确。
func TestUnwrapOuterRoundTrip(t *testing.T) {
	client := writeSyntheticClient(t, t.TempDir())

	// 先钉住密钥派生本身：段密钥序列必须与加壳时用的完全一致。
	keys, err := wrapperKeys(client.dir)
	if err != nil {
		t.Fatalf("wrapperKeys: %v", err)
	}
	if len(keys) != len(client.keys) {
		t.Fatalf("段密钥数 = %d, want %d", len(keys), len(client.keys))
	}
	for index := range keys {
		if !bytes.Equal(keys[index], client.keys[index]) {
			t.Fatalf("第 %d 把段密钥不同", index)
		}
	}

	source := filepath.Join(client.dir, "Script.pvf")
	output := filepath.Join(t.TempDir(), "Script.inner.pvf")

	stats, err := UnwrapOuter(client.dir, source, output)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, client.inner) {
		t.Fatalf("剥壳结果与内层明文不同（got %d 字节，want %d 字节），首个差异在偏移 0x%X",
			len(got), len(client.inner), firstDifference(got, client.inner))
	}
	if stats.Size != int64(len(client.inner)) {
		t.Errorf("size = %d, want %d", stats.Size, len(client.inner))
	}
	if stats.Segments != 2 || stats.Keys != 12 {
		t.Errorf("segments/keys = %d/%d, want 2/12", stats.Segments, stats.Keys)
	}
	if stats.InnerSHA256 != hashBytes(client.inner) {
		t.Errorf("inner sha256 = %s, want %s", stats.InnerSHA256, hashBytes(client.inner))
	}
	outer, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if stats.OuterSHA256 != hashBytes(outer) {
		t.Errorf("outer sha256 = %s, want %s", stats.OuterSHA256, hashBytes(outer))
	}
}

// 产物必须“拒绝覆盖”：目标已存在时立刻失败，且不碰那个文件（发布靠硬链接，契约如此）。
func TestUnwrapOuterRefusesExistingOutput(t *testing.T) {
	client := t.TempDir()
	writeSyntheticClient(t, client)
	output := filepath.Join(t.TempDir(), "Script.inner.pvf")
	writeFile(t, output, []byte("already-there"))

	if _, err := UnwrapOuter(client, filepath.Join(client, "Script.pvf"), output); err == nil {
		t.Fatal("已存在的输出必须被拒绝")
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "already-there" {
		t.Errorf("被拒绝的输出不应被改动：%q", body)
	}
}

// 失败不留半截产物：输入被截断时错误要明确，且目标文件不存在。
func TestUnwrapOuterRejectsTruncatedOuter(t *testing.T) {
	client := t.TempDir()
	writeSyntheticClient(t, client)
	// 首段完整、第二段不足 0x2800：脚本在同样的位置报 “truncated protected outer PVF segment”。
	body, err := os.ReadFile(filepath.Join(client, "Script.pvf"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(client, "Script.pvf"), body[:outerBlockSize+0x1000])

	output := filepath.Join(t.TempDir(), "Script.inner.pvf")
	_, err = UnwrapOuter(client, filepath.Join(client, "Script.pvf"), output)
	if err == nil || !strings.Contains(err.Error(), "外层保护被截断") {
		t.Fatalf("error = %v, want the truncation refusal", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Errorf("失败后不应留下产物：%v", statErr)
	}
}

// 三件套不配套时（密钥对不上）必须在首段校验处失败，而不是产出一份垃圾归档。
func TestUnwrapOuterRejectsMismatchedKeys(t *testing.T) {
	client := t.TempDir()
	writeSyntheticClient(t, client)
	// 换一份 sk.dat：密钥解出来是另一套，首段解出的头部就不是 nkpi。
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(client, "sk.dat"), rsaBlocks(t, other, randomBytes(t, 384)))

	output := filepath.Join(t.TempDir(), "Script.inner.pvf")
	_, err = UnwrapOuter(client, filepath.Join(client, "Script.pvf"), output)
	if err == nil {
		t.Fatal("密钥不配套时必须报错")
	}
	if !strings.Contains(err.Error(), "nkpi") && !strings.Contains(err.Error(), "sk.dat") {
		t.Errorf("error = %v, want an nkpi or sk.dat diagnosis", err)
	}
}

// DFO.exe 不是 PE（或读不到）时要报“解析 PE 失败”，不能崩。
func TestUnwrapOuterRejectsUnreadableEXE(t *testing.T) {
	client := t.TempDir()
	writeFile(t, filepath.Join(client, "DFO.exe"), []byte("not-a-pe"))
	writeFile(t, filepath.Join(client, "sk.dat"), randomBytes(t, 512))
	writeFile(t, filepath.Join(client, "Script.pvf"), randomBytes(t, 0x2800))

	_, err := UnwrapOuter(client, filepath.Join(client, "Script.pvf"), filepath.Join(t.TempDir(), "out.pvf"))
	if err == nil || !strings.Contains(err.Error(), "DFO.exe") {
		t.Fatalf("error = %v, want the DFO.exe diagnosis", err)
	}
}

// sk.dat 不是模长的整数倍时，脚本的 assert 会炸；这边必须是明确的中文错误。
func TestUnwrapOuterRejectsMisalignedSkDat(t *testing.T) {
	client := t.TempDir()
	writeSyntheticClient(t, client)
	// 加一个字节，破坏 len(sk.dat) % RSA 模长 == 0。
	body, err := os.ReadFile(filepath.Join(client, "sk.dat"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(client, "sk.dat"), append(body, 0))

	_, err = UnwrapOuter(client, filepath.Join(client, "Script.pvf"), filepath.Join(t.TempDir(), "out.pvf"))
	if err == nil || !strings.Contains(err.Error(), "整数倍") {
		t.Fatalf("error = %v, want the alignment refusal", err)
	}
}
