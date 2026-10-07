package pvf

// 本文件负责把 PVF 直读模式要用的“内层归档”从客户端三件套里剥出来。
//
// 为什么要单独一层：客户端 `Script.pvf` 是**带外层保护的**归档 —— 每 0xA00000(10 MiB)
// 一块，前 0x2800(10 KiB) 字节用 AES-256-CBC（零 IV）加密，而密钥来自 `DFO.exe` 内嵌的
// RSA 私钥与 `sk.dat` 密文。服务端读的是剥掉这层保护的版本（FormatDFO20260901，见
// archive.go 的注释“The RSA/AES outer layer must first be removed into a separate file”）。
// 在剥壳之前连归档头都是密文，任何解析都无从谈起，所以这一步必须发生在 Open 之前。
//
// 复刻对象（要求逐字节等价，见 docs/go-launch-migration-plan.md Stage 4）：
//
//	scripts/pvf_archive.py       wrapper_keys / aes           （密钥派生与解密原语）
//	scripts/prepare_inner_pvf.py prepare(client, output, ...) （分块剥壳）
//
// 与那两份脚本的对应关系，逐点写在各函数上方；任何一处“看起来可以简化”的地方都注明了
// 为什么不能简化 —— 这里的产物要按 sha256 与 Python 版对比。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// outerBlockSize 是外层保护的分段长度：脚本按 0xA00000(10 MiB) 遍历 Script.pvf。
	outerBlockSize = 0xA00000
	// outerPrefixSize 是每段里被 AES 保护的前缀长度（0x2800 = 10 KiB）。
	outerPrefixSize = 0x2800
	// wrapperKeySize 是一把段密钥的长度：sk.dat 解开后按 32 字节切开。
	wrapperKeySize = 32
	// wrapperKeyQuantum 是脚本里**写死的** 256：prefix = len(raw)//256*256。
	// 它不是 RSA 模长（虽然当前客户端的两者恰好都是 256），照抄以免改变语义。
	wrapperKeyQuantum = 256
	// wrapperPrivateKeyVA / wrapperAESKeyVA 是 DFO.exe 里两个字面量的**绝对 VA**。
	// 槽里存的是 qword 指针，指针指向真正的字符串；VA 来自原生加载器的取证结论。
	wrapperPrivateKeyVA = 0x14DC98110
	wrapperAESKeyVA     = 0x14DC98118
	// wrapperLiteralLimit 是字面量字符串的读取上限：脚本读 4096 字节后按 NUL 截断。
	wrapperLiteralLimit = 4096
	// wrapperInnerMagic 是首段前 48 字节剥壳后应有的签名（`stream("iNfO", ...)`）。
	wrapperInnerMagic = "nkpi"
)

// OuterUnwrap 描述一次外层剥离的结果，供清单（manifest）组装使用。
//
// 字段名与 scripts/prepare_inner_pvf.py 写进清单的键一一对应：Size 同时是外层与内层
// 的大小（剥壳不改变长度，只改前 0x2800 字节的内容）。
type OuterUnwrap struct {
	Size        int64  `json:"size"`
	OuterSHA256 string `json:"outer_sha256"`
	InnerSHA256 string `json:"inner_sha256"`
	Segments    int    `json:"segments"`
	Keys        int    `json:"keys"`
}

// UnwrapOuter 把 source（客户端 Script.pvf）剥成 output（内层归档）。
//
// output 必须不存在：脚本用 os.link 做原子发布，前提是目标为空（prepare_inner_pvf.py
// L27-28 的 `refusing existing output`）。这里用 O_EXCL 复刻同一条契约，避免把已发布的
// 产物写坏；调用方要重建时应先把旧件轮换走。
//
// 失败时不留半截产物：文件被删掉，错误里带上原因。剥壳很慢（几百 MB），但它是幂等的，
// 失败的残留物只会让下一次门禁误判。
func UnwrapOuter(clientDir, source, output string) (OuterUnwrap, error) {
	out, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return OuterUnwrap{}, fmt.Errorf("创建内层 PVF 失败：%w", err)
	}
	stats, err := UnwrapOuterTo(clientDir, source, out)
	if closeErr := out.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("关闭内层 PVF 失败：%w", closeErr)
	}
	if err != nil {
		os.Remove(output)
		return OuterUnwrap{}, err
	}
	return stats, nil
}

// UnwrapOuterTo 与 UnwrapOuter 相同，但写到调用方给的 writer 上。
//
// 调用方要自己管好“临时文件 → 原子发布”这一段（服务端启动链就是这么做的：先写
// pvf-prepare-*.partial，再用 os.Link 发布）。writer 若实现了 Sync()（*os.File 就是），
// 这里会先刷盘 —— 脚本在发布前也有 flush + fsync（prepare_inner_pvf.py L63-64）。
func UnwrapOuterTo(clientDir, source string, out io.Writer) (OuterUnwrap, error) {
	stats := OuterUnwrap{}

	keys, err := wrapperKeys(clientDir)
	if err != nil {
		return stats, err
	}
	stats.Keys = len(keys)

	in, err := os.Open(source)
	if err != nil {
		return stats, fmt.Errorf("打开客户端 Script.pvf 失败：%w", err)
	}
	defer in.Close()

	if err := unwrapBlocks(in, out, keys, &stats); err != nil {
		return OuterUnwrap{}, err
	}
	if syncer, ok := out.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return OuterUnwrap{}, fmt.Errorf("刷盘内层 PVF 失败：%w", err)
		}
	}
	return stats, nil
}

// unwrapBlocks 是脚本 prepare() 的主循环本体：分块读、剥壳、写、双边取哈希。
//
// 两个哈希的口径必须与脚本一致：outer_hash 累计**原始**（带保护）字节，
// inner_hash 累计**剥壳后**字节；两者长度相同，都是整个文件。
func unwrapBlocks(in io.Reader, out io.Writer, keys [][]byte, stats *OuterUnwrap) error {
	outerHash, innerHash := sha256.New(), sha256.New()
	// 复用一个 10 MiB 的块缓冲：脚本的 iter(lambda: read(0xA00000), b"") 也是一块一块走，
	// 区别只是脚本每次都会新建 bytes 对象。
	block := make([]byte, outerBlockSize)
	index := 0
	for {
		n, readErr := io.ReadFull(in, block)
		if n == 0 {
			// 文件正好是 0xA00000 的整数倍，或多读一次拿到 EOF。
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("读取客户端 Script.pvf 失败：%w", readErr)
		}
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			return fmt.Errorf("读取客户端 Script.pvf 失败：%w", readErr)
		}

		segment := block[:n]
		// 外层哈希取原始字节，必须在剥壳之前累计（脚本 L52 在替换 block 之前 update）。
		outerHash.Write(segment)

		if index < len(keys) {
			// 脚本 L54-55：受保护的段不足 0x2800 字节就直接失败，绝不半解一段。
			if n < outerPrefixSize {
				return fmt.Errorf("客户端 Script.pvf 第 %d 段只有 %d 字节，外层保护被截断（需 %d）",
					index+1, n, outerPrefixSize)
			}
			if _, err := aesCBCZeroIVDecrypt(keys[index], segment[:outerPrefixSize]); err != nil {
				return fmt.Errorf("解开 Script.pvf 第 %d 段失败：%w", index+1, err)
			}
			// 脚本 L57-58：首段前 48 字节用 stream("iNfO") 脱一层，验 `nkpi` 签名。
			// 这是整套三件套是否配套的唯一早期证据 —— 密钥错、sk.dat 错、pvf 错都会在这里露头。
			if index == 0 {
				header := append([]byte(nil), segment[:48]...)
				decryptProtected("iNfO", header)
				if !bytes.HasPrefix(header, []byte(wrapperInnerMagic)) {
					return fmt.Errorf("解出的内层归档头部不是 %s（DFO.exe / sk.dat / Script.pvf 不配套）",
						wrapperInnerMagic)
				}
			}
		}

		if _, err := out.Write(segment); err != nil {
			return fmt.Errorf("写入内层 PVF 失败：%w", err)
		}
		innerHash.Write(segment)
		stats.Size += int64(n)
		index++

		if readErr == io.ErrUnexpectedEOF {
			// 最后一段不足 10 MiB：脚本的 read() 也是这么收尾的。
			break
		}
	}
	stats.Segments = index
	// 脚本 L65-66：空 PVF 直接拒绝，否则会发布一个 0 字节的“内层归档”。
	if stats.Size == 0 {
		return fmt.Errorf("客户端 Script.pvf 是空文件")
	}
	stats.OuterSHA256 = hexDigest(outerHash)
	stats.InnerSHA256 = hexDigest(innerHash)
	return nil
}

// wrapperKeys 复刻 scripts/pvf_archive.py L26-41 的 wrapper_keys()：从 DFO.exe 里取出
// RSA 私钥与 AES key，用私钥解开 sk.dat，再解出每一把 32 字节的段密钥。
//
// 密钥只在内存里存在，绝不落盘、绝不进日志（与 scripts/prepare_inner_pvf.py 的头注释一致）。
func wrapperKeys(clientDir string) ([][]byte, error) {
	exe := filepath.Join(clientDir, "DFO.exe")
	privatePEM, aesKeyLiteral, err := wrapperLiterals(exe)
	if err != nil {
		return nil, err
	}
	private, err := parseWrapperPrivateKey(privatePEM)
	if err != nil {
		return nil, err
	}

	ciphertext, err := os.ReadFile(filepath.Join(clientDir, "sk.dat"))
	if err != nil {
		return nil, fmt.Errorf("读取 sk.dat 失败：%w", err)
	}
	size := private.Size()
	if size <= 0 {
		return nil, fmt.Errorf("DFO.exe 里的 RSA 私钥模长无效：%d", size)
	}
	// 脚本 L35 的 `assert len(raw)%size == 0`：分块前提，破了就只能报错。
	if len(ciphertext) == 0 || len(ciphertext)%size != 0 {
		return nil, fmt.Errorf("sk.dat 长度 %d 不是 RSA 模长 %d 的整数倍", len(ciphertext), size)
	}

	// 脚本 L36：b"".join(private.decrypt(raw[i:i+size], PKCS1v15()) for ...)。
	// Go 侧等价于 PrivateKey.Decrypt(nil, block, nil)：文档明确 nil opts 即 PKCS#1 v1.5。
	plain := make([]byte, 0, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += size {
		block, err := private.Decrypt(nil, ciphertext[offset:offset+size], nil)
		if err != nil {
			return nil, fmt.Errorf("sk.dat 第 %d 块 RSA/PKCS#1 v1.5 解密失败：%w", offset/size+1, err)
		}
		plain = append(plain, block...)
	}

	aesKey, err := hex.DecodeString(strings.TrimSpace(string(aesKeyLiteral)))
	if err != nil {
		return nil, fmt.Errorf("DFO.exe 里的 AES key 字面量不是十六进制：%w", err)
	}
	// 脚本 L37-38：prefix = len(raw)//256*256，只解密前 prefix 字节，尾巴原样拼回。
	// 三段切片（plain[:prefix:prefix]）是为了强制 append 分配新数组 —— Python 的
	// raw[:prefix] 本身就是一份拷贝，不这么写就会与尾巴别名。
	prefix := len(plain) / wrapperKeyQuantum * wrapperKeyQuantum
	head, err := aesCBCZeroIVDecrypt(aesKey, plain[:prefix:prefix])
	if err != nil {
		return nil, fmt.Errorf("解密段密钥失败：%w", err)
	}
	plain = append(head, plain[prefix:]...)

	// 脚本 L39 的 `assert len(raw)%32 == 0`，随后每 32 字节一把。
	if len(plain) == 0 || len(plain)%wrapperKeySize != 0 {
		return nil, fmt.Errorf("段密钥长度 %d 不是 %d 的整数倍", len(plain), wrapperKeySize)
	}
	keys := make([][]byte, 0, len(plain)/wrapperKeySize)
	for offset := 0; offset < len(plain); offset += wrapperKeySize {
		keys = append(keys, plain[offset:offset+wrapperKeySize])
	}
	return keys, nil
}

// wrapperLiterals 读出 DFO.exe 里两处包装字面量：VA 0x14dc98110 的 PEM RSA 私钥，
// 与 VA 0x14dc98118 的十六进制 AES key。
func wrapperLiterals(exePath string) (privatePEM, aesKeyLiteral []byte, err error) {
	// 自己持有 *os.File：debug/pe 的 closer 不导出，而按文件偏移读字节需要 ReaderAt。
	handle, err := os.Open(exePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开客户端 DFO.exe 失败：%w", err)
	}
	defer handle.Close()
	file, err := pe.NewFile(handle)
	if err != nil {
		return nil, nil, fmt.Errorf("解析客户端 DFO.exe 失败（不是可读的 PE？）：%w", err)
	}

	base, err := peImageBase(file)
	if err != nil {
		return nil, nil, err
	}
	if privatePEM, err = peLiteral(handle, file, base, wrapperPrivateKeyVA); err != nil {
		return nil, nil, err
	}
	if aesKeyLiteral, err = peLiteral(handle, file, base, wrapperAESKeyVA); err != nil {
		return nil, nil, err
	}
	return privatePEM, aesKeyLiteral, nil
}

// peImageBase 取 PE 可选头里的 ImageBase：字面量的 VA 要减掉它才是 RVA。
func peImageBase(file *pe.File) (uint64, error) {
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		return header.ImageBase, nil
	case *pe.OptionalHeader32:
		return uint64(header.ImageBase), nil
	}
	return 0, fmt.Errorf("客户端 DFO.exe 缺少 PE 可选头")
}

// peLiteral 复刻 pvf_archive.py 里的 literal(va)：槽里是一个 qword 指针（本身也是绝对
// VA），指针指向以 NUL 结尾的字节串；读满 wrapperLiteralLimit 字节后按第一个 NUL 截断。
func peLiteral(reader io.ReaderAt, file *pe.File, base, va uint64) ([]byte, error) {
	pointerBytes, err := peData(reader, file, base, va, 8)
	if err != nil {
		return nil, fmt.Errorf("读取 DFO.exe VA 0x%x 的指针失败：%w", va, err)
	}
	if len(pointerBytes) < 8 {
		return nil, fmt.Errorf("DFO.exe VA 0x%x 的指针只有 %d 字节", va, len(pointerBytes))
	}
	pointer := binary.LittleEndian.Uint64(pointerBytes)
	raw, err := peData(reader, file, base, pointer, wrapperLiteralLimit)
	if err != nil {
		return nil, fmt.Errorf("读取 DFO.exe VA 0x%x 指向的字面量失败：%w", pointer, err)
	}
	if index := bytes.IndexByte(raw, 0); index >= 0 {
		raw = raw[:index]
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("DFO.exe VA 0x%x 的字面量是空的", va)
	}
	return raw, nil
}

// peData 按 ImageBase 把绝对 VA 换算成文件偏移后读 size 字节。
//
// 读文件而不是读“节”：pefile 的 get_data 也是从文件偏移直接切片，跨节时照样连续读。
// 客户端的节有上百 MB，只读需要的十几字节，不做整节载入。
func peData(reader io.ReaderAt, file *pe.File, base, va uint64, size int) ([]byte, error) {
	if va < base {
		return nil, fmt.Errorf("VA 0x%x 低于 ImageBase 0x%x", va, base)
	}
	rva := uint32(va - base)
	offset := peRVAToOffset(file, rva)
	buf := make([]byte, size)
	n, err := reader.ReadAt(buf, int64(offset))
	if n == 0 && err != nil {
		return nil, fmt.Errorf("RVA 0x%x（文件偏移 0x%x）读取失败：%w", rva, offset, err)
	}
	return buf[:n], nil
}

// peRVAToOffset 在节表里找包含该 RVA 的节；找不到时退回“RVA 即文件偏移”。
//
// 兜底与 pefile.get_offset_from_rva 一致（pefile 找不到节时会警告并直接把 RVA 当偏移），
// 保留它是为了让“字面量不在任何节里”这种异常也能走到 PEM 解析，从而报出可诊断的错误。
func peRVAToOffset(file *pe.File, rva uint32) uint32 {
	for _, section := range file.Sections {
		// 节的可见范围取 SizeOfRawData 与 VirtualSize 的较大者，与 pefile 的
		// contains_rva 判据一致（打包器常把两者写得不一样）。
		size := section.VirtualSize
		if section.Size > size {
			size = section.Size
		}
		if rva >= section.VirtualAddress && rva < section.VirtualAddress+size {
			return section.Offset + (rva - section.VirtualAddress)
		}
	}
	return rva
}

// parseWrapperPrivateKey 解析 DFO.exe 里的 PEM 私钥。
//
// Python 的 load_pem_private_key 同时接受 PKCS#1（BEGIN RSA PRIVATE KEY）与 PKCS#8
// （BEGIN PRIVATE KEY），这里按同样顺序试两种，任何其他类型都明确报错。
func parseWrapperPrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("DFO.exe 里的私钥字面量不是 PEM（%d 字节）", len(raw))
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("DFO.exe 里的 PEM 私钥无法解析（PKCS#1 与 PKCS#8 都不成立）：%w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("DFO.exe 里的 PEM 私钥不是 RSA（%T）", parsed)
	}
	return key, nil
}

// aesCBCZeroIVDecrypt 复刻 pvf_archive.py 的 aes(key, raw)：AES-CBC + 全零 IV，原地解密。
//
// 只提供解密方向：本文件只做“剥壳”。反方向（重新加壳）在 reference/analysis-tools/
// rewrap_pvf*.py 里，服务端启动链不需要它。
func aesCBCZeroIVDecrypt(key, buf []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 密钥无效（%d 字节）：%w", len(key), err)
	}
	if len(buf)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("AES 数据长度 %d 不是 %d 的整数倍", len(buf), aes.BlockSize)
	}
	// 零 IV 与脚本的 modes.CBC(bytes(16)) 相同；CryptBlocks 支持原地加解密。
	cipher.NewCBCDecrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(buf, buf)
	return buf, nil
}

// hexDigest 收尾一个 sha256 累加器。
func hexDigest(digest hash.Hash) string {
	return hex.EncodeToString(digest.Sum(nil))
}
