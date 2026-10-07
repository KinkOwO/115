package launcher

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file is the protocol-fixture half of Stage 2 (docs/go-launch-migration-plan.md):
// the four files channel_probe.py writes before it starts the gateway, reproduced here
// byte for byte.
//
// 对齐口径（都是实测结论，不是猜的）：
//   - channel_probe.py 的 `write_bytes`/`write_text` 走 Python 文本模式，Windows 上把
//     "\n" 翻译成 "\r\n"；json.dumps 的输出里没有换行，所以 responses.json/run.json
//     是单行无换行，而 fixture.json/breakpoints.txt 是 CRLF 且结尾无换行。
//   - json.dumps 默认分隔符是 `", "` / `": "`，indent=2 时每条占一行、键序即插入序；
//     ensure_ascii=True 会把非 ASCII 转成 \uXXXX。

// pythonTextNewline 是 Python 文本模式在 Windows 上写出的换行（os.linesep）。
// 启动链本身只跑 Windows（launch_local.py 第一句就拒绝非 nt），所以这里写死。
const pythonTextNewline = "\r\n"

// fixtureCandidate / fixtureCRCNote 是 fixture.json 里的两条说明文字，逐字照抄。
const (
	fixtureCandidate = "channelinfo transport gate"
	fixtureCRCNote   = "native polynomial 0x4db89129; folded low byte; measured in channel_02"
)

// channelFixture 是 channelinfo.bin 的构造过程与产物。中间值都留着，是为了让黄金样本
// 测试能分别核对 pb/plain/crc/fold，而不是只比一个总哈希 —— 差异出在哪一步才知道。
type channelFixture struct {
	PB     []byte
	Plain  []byte
	Cipher []byte
	CRC    uint32
	Fold   byte
	File   []byte
}

// varint 是 protobuf 的 base-128 变长整数（channel_probe.py L35-L41）。
func varint(value int) []byte {
	out := make([]byte, 0, 5)
	for value >= 128 {
		out = append(out, byte(value&127|128))
		value >>= 7
	}
	return append(out, byte(value))
}

// pbInteger 编码一个 varint 字段：varint(n<<3) + varint(v)。
func pbInteger(field, value int) []byte {
	return append(varint(field<<3), varint(value)...)
}

// pbString 编码一个 length-delimited 字段：varint(n<<3|2) + varint(len) + s。
func pbString(field int, value []byte) []byte {
	out := append(varint(field<<3|2), varint(len(value))...)
	return append(out, value...)
}

// buildChannelFixture 复刻 channel_probe.py L34-L73：字段取自原生 PB 解析器
// （required status=2、key bytes=3），其余可选字段保持默认值。
func buildChannelFixture() channelFixture {
	keys := make([]byte, 1024)
	for i := range keys {
		keys[i] = byte((i % 127) + 1)
	}
	pb := pbInteger(2, 1)
	pb = append(pb, pbString(3, keys)...)
	pb = append(pb, pbString(4, []byte("LAN Local"))...)

	plain := binary.LittleEndian.AppendUint32(make([]byte, 0, 4+len(pb)), uint32(len(pb)))
	plain = append(plain, pb...)

	// 自定义的异或+循环移位：先 ^0xB5，再右移 6 位与左移 2 位拼起来。
	cipher := make([]byte, len(plain))
	for i, x := range plain {
		shifted := x ^ 0xB5
		cipher[i] = (shifted >> 6) | (shifted << 2)
	}

	crc := nativeCRC(cipher)
	fold := byte(crc&255) ^ byte((crc>>8)&255) ^ byte((crc>>16)&255) ^ byte((crc>>24)&255) ^ 0x18

	// struct.pack("<B H I I I B", 0, 1, 16+len(cipher), 0, fold, 0)：小端、无对齐填充，共 16 字节。
	header := make([]byte, 0, 16)
	header = append(header, 0)
	header = binary.LittleEndian.AppendUint16(header, 1)
	header = binary.LittleEndian.AppendUint32(header, uint32(16+len(cipher)))
	header = binary.LittleEndian.AppendUint32(header, 0)
	header = binary.LittleEndian.AppendUint32(header, uint32(fold))
	header = append(header, 0)

	file := make([]byte, 0, len(header)+len(cipher))
	file = append(header, cipher...)
	return channelFixture{PB: pb, Plain: plain, Cipher: cipher, CRC: crc, Fold: fold, File: file}
}

// nativeCRCTable 是 channel_probe.py L58-L63 的查表；多项式 0x4DB89129 是原生客户端的
// 那一条（不是常见的反射 CRC32）。
var nativeCRCTable = func() [256]uint32 {
	var table [256]uint32
	for i := range table {
		value := uint32(i)
		for bit := 0; bit < 8; bit++ {
			if value&1 != 0 {
				value = (value >> 1) ^ 0x4DB89129
			} else {
				value >>= 1
			}
		}
		table[i] = value
	}
	return table
}()

// nativeCRC 复刻 channel_probe.py L64-L67：初值/尾值都是 0xFFFFFFFF。
func nativeCRC(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, x := range data {
		crc = (crc >> 8) ^ nativeCRCTable[(crc^uint32(x))&255]
	}
	return crc ^ 0xFFFFFFFF
}

// fixtureJSON 复刻 channel_probe.py L74-L84 的 json.dumps(..., indent=2)：键序
// candidate/crc/pb_hex/plain_hex，CRLF 分行，结尾无换行。
func fixtureJSON(f channelFixture) string {
	lines := []string{
		"{",
		`  "candidate": ` + pythonJSONString(fixtureCandidate) + ",",
		`  "crc": ` + pythonJSONString(fixtureCRCNote) + ",",
		`  "pb_hex": ` + pythonJSONString(hex.EncodeToString(f.PB)) + ",",
		`  "plain_hex": ` + pythonJSONString(hex.EncodeToString(f.Plain)),
		"}",
	}
	return strings.Join(lines, pythonTextNewline)
}

// breakpointsText 复刻 channel_probe.py L85-L144 的追加顺序：9 行固定（含无条件的
// 登录两行），再按 tag 前缀追加。顺序即文件顺序，不能重排。
func breakpointsText(tag string) string {
	lines := []string{
		"6d76a47 PACKET_CRC",
		"52c7e28 CHANNEL_PB_RESULT",
		"6ca9740 SESSION_KEYS",
		"6d77447 SEND_RAW",
		"6ca9660 SEND_PLAIN",
		"59a1bb0 DISPATCH_PACKET",
		"5250350 NAME_CHECK_RESULT",
		"5255c70 PRECHECK_RESULT",
		"52543d0 LOGIN_RESULT",
	}
	if strings.HasPrefix(tag, "login_") {
		lines = append(lines, "52553b2 LOGIN_FIELDS_DONE", "5255a1f LOGIN_TAIL")
	}
	if strings.HasPrefix(tag, "roles_") {
		lines = append(lines,
			"5637a20 USERINFO_HANDLER",
			"5637dd8 USERINFO_ROWS_DONE",
			"5638781 USERINFO_TAIL",
			"5250d40 CREATE_RESULT",
		)
		if strings.HasPrefix(tag, "roles_row") {
			lines = append(lines,
				"563e280 CHARACTER_ROW_BEGIN",
				"563ead2 CHARACTER_ROW_FIELDS_DONE",
				"563ec14 CHARACTER_ROW_TAIL",
			)
		}
	}
	if strings.HasPrefix(tag, "roles_persist_select") {
		lines = append(lines,
			"525a120 SELECT_RESULT",
			"525a2e3 SELECT_FIELDS_BEGIN",
			"525b409 SELECT_LAST_COUNT",
			"525b4c4 SELECT_FIELDS_DONE",
		)
	}
	if strings.HasPrefix(tag, "roles_persist_select_actor") {
		lines = append(lines,
			"563ec60 ENTRY_BASIC_BEGIN",
			"5641000 ENTRY_BASIC_DONE",
			"563d400 ENTRY_ADDITION_BEGIN",
		)
	}
	if strings.HasPrefix(tag, "roles_persist_select_actor_town") {
		lines = append(lines,
			"52fc5b0 AREA_USERS_BEGIN",
			"52fcf01 AREA_MAP_LOAD",
			"52fd9e3 AREA_USERS_DONE",
		)
	}
	return strings.Join(lines, pythonTextNewline) + pythonTextNewline
}

// LoginResponseBin 是"1"号响应用的登录成功报文：优先 testdata 里那一条 next22 报文，
// 缺失才退回 runtime/login_ok.bin（channel_probe.py L95-L100）。
func LoginResponseBin(project string) string {
	preferred := filepath.Join(project, "cmd", "wireprobe", "testdata", "login-normal22.bin")
	if regularFile(preferred) {
		return preferred
	}
	return filepath.Join(project, "runtime", "login_ok.bin")
}

// responsesJSON 复刻 channel_probe.py L101-L144：默认只有 1554（precheck），login_ 加
// "1"，roles_ 换成四键表（1554/1/8/684），roles_row 再把 "8" 换成行样本。
// 值都是绝对路径，键序即 json.dumps 的插入序。
func responsesJSON(project, tag string) string {
	precheck := filepath.Join(project, "runtime", "precheck_ok.bin")
	switch {
	case strings.HasPrefix(tag, "roles_"):
		characters := filepath.Join(project, "runtime", "characters_ok.bin")
		if strings.HasPrefix(tag, "roles_row") {
			characters = filepath.Join(project, "runtime", "characters_row_ok.bin")
		}
		return orderedJSON{
			keys: []string{"1554", "1", "8", "684"},
			values: map[string]string{
				"1554": precheck,
				"1":    LoginResponseBin(project),
				"8":    characters,
				"684":  filepath.Join(project, "runtime", "name_ok.bin"),
			},
		}.String()
	case strings.HasPrefix(tag, "login_"):
		return orderedJSON{
			keys:   []string{"1554", "1"},
			values: map[string]string{"1554": precheck, "1": LoginResponseBin(project)},
		}.String()
	default:
		return orderedJSON{
			keys:   []string{"1554"},
			values: map[string]string{"1554": precheck},
		}.String()
	}
}

// WriteSessionFixtures writes the four files channel_probe.py wrote before starting the
// gateway, in the same order and with the same bytes. It returns the fixture so callers
// (and tests) can inspect the intermediate values.
func WriteSessionFixtures(project, out, tag string) (channelFixture, error) {
	fixture := buildChannelFixture()
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fixture, err
	}
	files := []struct {
		name string
		body []byte
	}{
		{"channelinfo.bin", fixture.File},
		{"fixture.json", []byte(fixtureJSON(fixture))},
		{"breakpoints.txt", []byte(breakpointsText(tag))},
		{"responses.json", []byte(responsesJSON(project, tag))},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(out, file.name), file.body, 0o644); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

// OverrideLoginResponse 复刻 channel_probe.py L465-L476：环境里给了 DFO_LOGIN_RESPONSE
// 且文件存在时，把 responses.json 里的 "1" 号响应改指到它。相对路径按包根解析。
// 解析或写回失败与 Python 一样静默跳过（那条分支只影响一次登录应答，不值得拦启动）。
func OverrideLoginResponse(responsesPath, project, override string) {
	if override == "" {
		return
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(project, override)
	}
	if !regularFile(override) {
		return
	}
	data, err := os.ReadFile(responsesPath)
	if err != nil {
		return
	}
	mapping, err := parseOrderedJSON(data)
	if err != nil {
		return
	}
	mapping.set("1", filepath.Clean(override))
	_ = os.WriteFile(responsesPath, []byte(mapping.String()), 0o644)
}

// orderedJSON 是一个保持成员顺序的 JSON 对象（值都是字符串）。Python 的 dict 按插入序
// 序列化，而 Go 的 map 不保序，所以这里显式记键序。
type orderedJSON struct {
	keys   []string
	values map[string]string
}

// parseOrderedJSON 读一个"字符串→字符串"的 JSON 对象并保留文件里的键序。
func parseOrderedJSON(data []byte) (orderedJSON, error) {
	object := orderedJSON{values: map[string]string{}}
	// json.Unmarshal 到 map 会丢键序，这里用 Decoder 逐成员读。
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return object, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return object, fmt.Errorf("expected a JSON object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return object, err
		}
		key, ok := token.(string)
		if !ok {
			return object, fmt.Errorf("JSON object keys must be strings")
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return object, err
		}
		object.set(key, value)
	}
	if _, err := decoder.Token(); err != nil {
		return object, err
	}
	return object, nil
}

// set 写入或更新一个成员，键已存在时保持原位（与 json.dumps 重新序列化 dict 一致）。
func (o *orderedJSON) set(key, value string) {
	if o.values == nil {
		o.values = map[string]string{}
	}
	if _, seen := o.values[key]; !seen {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// String 按 json.dumps 的默认风格渲染：`", "` 分隔、`": "` 赋值、无换行。
func (o orderedJSON) String() string {
	parts := make([]string, 0, len(o.keys))
	for _, key := range o.keys {
		parts = append(parts, pythonJSONString(key)+": "+pythonJSONString(o.values[key]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// pythonJSONString renders a string the way json.dumps did with its defaults
// (ensure_ascii=True): the JSON escapes, \uXXXX for every non-ASCII rune, and a surrogate
// pair beyond the BMP. Byte-identical output matters because these files are compared
// against the Python's own, and a client directory can legitimately contain non-ASCII.
func pythonJSONString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(&out, `\u%04x`, r)
			case r < 0x7F:
				out.WriteRune(r)
			case r <= 0xFFFF:
				fmt.Fprintf(&out, `\u%04x`, r)
			default:
				rest := r - 0x10000
				fmt.Fprintf(&out, `\u%04x\u%04x`, 0xD800+(rest>>10), 0xDC00+(rest&0x3FF))
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}
