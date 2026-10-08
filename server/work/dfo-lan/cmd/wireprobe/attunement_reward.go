package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// noti 2859 BOUNDARY_OF_ATTUNEMENT_REWARD —— 调律之边界的奖励通知。
//
// ## 这条包的形状（L0 证据）
//
// `opcodes.tsv`（dfocap 从进程内存取的名字表）给出名字与 `table_slot_va`；
// 派发表在 .data 的 BSS 里（运行时才填），所以改用**注册调用**的机器码形状反查：
//
//	lea  r8, <handler>
//	mov  edx, <packet id>
//	call sub_14599D5D0        ; S→C（noti）注册
//
// 用已知的 `2838 → sub_140656A00` 自检通过后，得到 **2859 → sub_1406B18D0**。
// 它的完整反编译（`runtime/ida-attunement/probe7-out.txt`）就是载荷的全部依据：
//
//	__int64 sub_1406B18D0()
//	{
//	  int v1, v2, v3;
//	  v1 = 0; v2 = 72; v3 = 72;      // 短包时的兜底
//	  sub_146EA0BE0(&v1, 12);         // 定长读 12 字节：v1=packet[0:4] v2=[4:8] v3=[8:12]
//	  *(_DWORD *)(sub_1406B1BD0() + 80) = v2;
//	  *(_DWORD *)(sub_1406B1BD0() + 84) = v3;
//	  *(_DWORD *)(sub_1406B1BD0() + 88) = v1;
//	  return sub_1406B1BD0();
//	}
//
// ⇒ **载荷 = 12 字节 = 3 × little-endian u32**；它们写进同一个 240 字节的
// 「调律模块」实例（`sub_1406B1520` 是它的 init，注册了 noti 2756/2859 与 cmd 2343/2406）。
// 模块 init 里的初值是 `+80 = 72`、`+84 = 72`、`+88 = -1`；
// **我们从来没发过这条包**，所以这三个字段一直是初值 —— 这就是「这块还没接线」。
//
// ## 三个 u32 各是什么：**未取证**
//
// 已排除的路径（见 next176 §8.2）：模块 vtable（24 槽位只有 3 个实函数，都不读这三个偏移）、
// getter `sub_1406B1BD0` 的 19 个调用方、以及 `sub_1406B19C0` / `sub_1406B1C50` /
// `sub_1406B1D10` / `sub_1406B2980`。按项目硬约束「禁止猜包」，**默认一个字节都不发**，
// 只提供这个诊断开关，由业主在实机上一组组试值、看珠子/演出怎么变。
//
// ## 与 noti 2838 的关系
//
// 2838（ENDKEEPER_OF_ORDER_REWARD，边界之守）是两个 u32 → 小深渊那条线；
// 2859 的**第一个 u32 落在同一个位移（+88）**，说明两者共享同一套「档位」语义的两个入口。
// 所以试值时优先按 2838 的两个数（引子 / 誓约）去猜前两位，第三位待定 ——
// 但**不因此默认发送**：写错档位会让客户端的珠子演出与实际掉落实质性不符，比不发更糟。

// attunementRewardPayloadSize 是 noti 2859 的载荷长度，**必须**是 12。
//
// 依据是解析器的定长读：`sub_146EA0BE0(&v1, 12)` 一次读满 12 字节，
// 和 2838 的 `(…, 8)` 是同一个成对形状（2838 的长度护栏在
// cmd/wireprobe/oath_info.go 的注释里记着：loc_146EA0C30 是故意的空写陷阱，
// 2026-09-27 用 8 字节的 2839 撞过）。长度不对一律拒绝，不给客户端留下错位的半包。
const attunementRewardPayloadSize = 12

// attunementRewardSpec 解析 `-attunement-reward` / `DFO_ATTUNEMENT_REWARD` 的值。
//
//	""              不发（默认，客户端保留模块里的 72/72/-1）
//	"@<路径>"        **每次进本重读**这个文件；空文件（或只有注释）= 不发
//	其它            直接当载荷：24 位十六进制（空格/逗号/0x 都可），或 3 个十进制 u32
//
// 「文件」这一档是为实机试值加的：三个字段的语义没定，得一组组试，而**换一个值不该
// 重启一次服务端**（重启要连带重启客户端，一轮三分钟）。用文件的话，改完存盘，
// 下一次进本就按新值发 —— 一轮之内可以把候选值挨个试完，日志逐次记下用了哪个值。
//
// 文件内容按「去掉 # 注释行后的第一行」读，写法与命令行一致（例如 `44,43,0`）。
// 让它容错到这种程度是刻意的：试值文件是给人手改的，写错必须马上在日志里说话。
func attunementRewardSpec(spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "@") {
		return parseAttunementReward(trimmed)
	}
	path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
	if path == "" {
		return nil, fmt.Errorf("empty payload file path after @")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		body = line
		break
	}
	if body == "" {
		// 空文件（或只有注释）= 这一段不发。这样「关掉注入」也不用重启。
		return nil, nil
	}
	payload, err := parseAttunementReward(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return payload, nil
}

// attunementRewardFile 把 -attunement-reward 的值里那条 @路径 抠出来（没有则空）。
// 只在日志与诊断里用，解析仍以 attunementRewardSpec 为准。
func attunementRewardFile(spec string) string {
	trimmed := strings.TrimSpace(spec)
	if !strings.HasPrefix(trimmed, "@") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
}

// parseAttunementReward 解析一份具体载荷（不含 @ 文件形式）。
//
// 两种写法，都是为了实机试值方便：
//
//	原始载荷：24 位十六进制（空格、逗号、0x 前缀都忽略）  → 直接当 12 字节
//	三个十进制：如 "44,43,0"                              → 各按 little-endian u32 打包
//
// 留空返回 nil，表示不发送（默认）。
func parseAttunementReward(spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, nil
	}
	compact := strings.NewReplacer(" ", "", "\t", "", ",", "", ":", "", "-", "").Replace(trimmed)
	compact = strings.TrimPrefix(strings.TrimPrefix(compact, "0x"), "0X")
	if len(compact) == attunementRewardPayloadSize*2 {
		raw, err := hex.DecodeString(compact)
		if err != nil {
			return nil, fmt.Errorf("not a hex payload: %w", err)
		}
		return raw, nil
	}
	parts := strings.Split(trimmed, ",")
	if len(parts) != attunementRewardPayloadSize/4 {
		return nil, fmt.Errorf("want %d hex bytes or %d comma separated u32 values, got %q",
			attunementRewardPayloadSize, attunementRewardPayloadSize/4, spec)
	}
	out := make([]byte, attunementRewardPayloadSize)
	for i, part := range parts {
		// 用带符号解析：模块 init 里 `+88` 的初值就是 **-1**，试值时要能照着写
		// （-1 按二补数写成全 F）。非负值仍按 u32 走，范围按 32 位校验。
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || n < math.MinInt32 || n > math.MaxUint32 {
			return nil, fmt.Errorf("field %d: %q is outside the u32 range (a negative value such as -1 is written two's complement)", i, part)
		}
		binary.LittleEndian.PutUint32(out[i*4:], uint32(int64(n)))
	}
	return out, nil
}

// describeAttunementReward 把 12 字节拆成三个 u32 打日志，并对「不在客户端八档阶梯里」
// 的值提一句醒 —— 它不是错误（试值本来就可能故意用 0 或 72 去看兜底行为），
// 但值得在日志里写明，免得事后来回翻代码猜这个数是什么。
func describeAttunementReward(raw []byte) string {
	if len(raw) != attunementRewardPayloadSize {
		return fmt.Sprintf("%d bytes (unexpected length)", len(raw))
	}
	fields := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		v := binary.LittleEndian.Uint32(raw[i*4:])
		name := oathGradeTiers[uint16(v)]
		if name == "" {
			name = "not-a-client-tier"
		}
		fields = append(fields, fmt.Sprintf("[%d]=%d(%s)", i, v, name))
	}
	return fmt.Sprintf("%s hex=%s", strings.Join(fields, " "), hex.EncodeToString(raw))
}

// logAttunementRewardInjection 在启动期把注入内容打出来。
//
// 放启动期是有先例的教训：`-omen-info` 以前在「进频道」那一刻才解析，
// 写错一个字符会 log.Fatalf 在加载日志的最底下，现象是「启动游戏进不去频道」，极难定位。
func logAttunementRewardInjection(spec string, raw []byte) {
	if strings.TrimSpace(spec) == "" {
		return
	}
	if file := attunementRewardFile(spec); file != "" {
		if len(raw) == 0 {
			log.Printf("attunement reward (noti 2859): armed from %s, currently OFF (empty or comment-only file)", file)
			return
		}
		log.Printf("attunement reward (noti 2859): armed from %s, currently %s (re-read on every dungeon entry)",
			file, describeAttunementReward(raw))
		return
	}
	log.Printf("attunement reward (noti 2859): injecting %s", describeAttunementReward(raw))
}
