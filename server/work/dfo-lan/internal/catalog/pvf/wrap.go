package pvf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// WrapOuter 是 UnwrapOuter 的**逆运算**：把内层归档按客户端三件套重新包成外层
// `Script.pvf`。客户端可见的 PVF mod 必须走这一侧 —— 服务端读的是内层，而
// **启动器的内层重生成门禁**（键 = DFO.exe + sk.dat + Script.pvf）会在客户端三件套
// 任一变化时把内层整个重做，所以只改内层的 mod 会被静默抹掉；改外层才会被客户端与
// 服务端同时看到（内层由门禁自动从外层重新生成）。
//
// # 加壳只有 AES 一层
//
// 依据是仓库内的权威实现 `unwrap_test.go` 的 `writeSyntheticClient`，其注释写明：
//
//	内层归档的**前 48 字节本身也是过流处理的**（`iNfO` 密钥流，读侧 parse.go 会再解一次；
//	剥壳只负责校验它解出来是 nkpi，不改写它）。
//
// 也就是说 `iNfO` 是**内层格式自身**的头部混淆（`parse.go` 解），不是外层包裹的一部分；
// 外层就是"每 10 MiB 段的前 0x2800 字节用该段 32 字节密钥做 AES-CBC(零 IV) 加密"。
// `UnwrapOuter` 里那次 `decryptProtected("iNfO", header)` 只用于**校验** `nkpi` 签名。
//
// 只写新文件（O_CREATE|O_EXCL），绝不就地改原件。
func WrapOuter(clientDir, innerPath, output string) (OuterWrap, error) {
	var stats OuterWrap
	keys, err := wrapperKeys(clientDir)
	if err != nil {
		return stats, err
	}
	stats.Keys = len(keys)
	if stats.Keys == 0 {
		return stats, fmt.Errorf("客户端三件套没有解出任何段密钥")
	}

	in, err := os.Open(innerPath)
	if err != nil {
		return stats, fmt.Errorf("打开内层归档失败：%w", err)
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return stats, err
	}
	if st.Size() == 0 {
		return stats, fmt.Errorf("内层归档是空文件")
	}

	dst, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return stats, err
	}
	defer dst.Close()

	digest := sha256.New()
	block := make([]byte, outerBlockSize)
	index := 0
	for {
		n, readErr := io.ReadFull(in, block)
		if n == 0 {
			if readErr == io.EOF {
				break
			}
			return stats, fmt.Errorf("读内层归档失败：%w", readErr)
		}
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			return stats, fmt.Errorf("读内层归档失败：%w", readErr)
		}
		segment := block[:n]
		// 与 UnwrapOuter 对称：只有前 len(keys) 段有密钥，且受保护段必须够长
		//（读侧对 n < outerPrefixSize 直接报错，所以这里也不能产出那种形态）。
		if index < len(keys) {
			if n < outerPrefixSize {
				return stats, fmt.Errorf("第 %d 段只有 %d 字节，凑不出受保护前缀（需 %d）",
					index+1, n, outerPrefixSize)
			}
			if _, err := aesCBCZeroIVSeal(keys[index], segment[:outerPrefixSize]); err != nil {
				return stats, fmt.Errorf("第 %d 段加壳失败：%w", index+1, err)
			}
		}
		if _, err := dst.Write(segment); err != nil {
			return stats, err
		}
		digest.Write(segment)
		stats.Size += int64(n)
		index++
		if readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	if index > len(keys) {
		// 段数多于密钥数：多出来的段按"无密钥"原样写出。这在真实档上会发生
		//（10 MiB 一段，761 MB 是 73 段而密钥是有限的若干把），读侧同样是
		// `index < len(keys)` 才解密，所以两侧一致。
		stats.PlainTailSegments = index - len(keys)
	}
	stats.Segments = index
	stats.SHA256 = hex.EncodeToString(digest.Sum(nil))
	if err := dst.Sync(); err != nil {
		return stats, err
	}
	return stats, dst.Close()
}

// OuterWrap 是一次加壳的结果。PlainTailSegments 是"没有密钥、原样写出"的尾段数 ——
// 它不是错误，是外层格式与有限段密钥数共同决定的形态（读侧同判据）。
type OuterWrap struct {
	Size              int64  `json:"size"`
	SHA256            string `json:"sha256"`
	Segments          int    `json:"segments"`
	Keys              int    `json:"keys"`
	PlainTailSegments int    `json:"plainTailSegments,omitempty"`
}

// aesCBCZeroIVSeal 是 aesCBCZeroIVDecrypt 的逆运算（命名避开测试里同名的助手）（同一个零 IV、同一把密钥）。
func aesCBCZeroIVSeal(key, buf []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 密钥无效（%d 字节）：%w", len(key), err)
	}
	if len(buf)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("AES 数据长度 %d 不是 %d 的整数倍", len(buf), aes.BlockSize)
	}
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(buf, buf)
	return buf, nil
}

// FirstDifference 返回两段字节首个不同的偏移（相同返回 -1）。给"逐字节比对"的断言用。
func FirstDifference(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
