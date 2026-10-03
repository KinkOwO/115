# next79 §27: generate internal/game/protocol/ispins_settlement.go from the
# official capture archive. The N2 settlement template is official s4 frame 488
# (480B body); the only per-stage differences live at template bytes 416..419
# (2B nonce + u16 progress 4*stage+3), taken from each stage's own frame
# (488/583/666/772). Manual hex transcription corrupted the §26 N14 tables, so
# this script is the only sanctioned way to write the constants.
import json

BODIES = r"d:\115us\analysis-tools\output\next79_settlement_tail_bodies.json"
ALL = r"d:\115us\analysis-tools\output\next79_n2_n9_all.json"
OUT = r"d:\115us\115-server\server\work\dfo-lan\internal\game\protocol\ispins_settlement.go"

with open(BODIES, "r", encoding="utf-8") as f:
    bodies = json.load(f)
with open(ALL, "r", encoding="utf-8") as f:
    allframes = json.load(f)

tmpl_hex = bodies["frame488_id2"]["body_hex"]
assert len(tmpl_hex) == 960, f"template hex {len(tmpl_hex)} chars, want 960 (480B)"
t = bytes.fromhex(tmpl_hex)

# structure assertions on the official template
assert t[0] == 0 and t[1] == 1 and t[2] == 0, "mode/count head"
assert t[3] == 3 and t[4] == 0x56, "row context {3, 0x56}"
assert int.from_bytes(t[28:32], "little") == 5, "name1 length slot"
assert t[32:37] == b"yan55", "name1"
assert t[165:167] == bytes([0xed, 0x00]), "official actor u16 at 165"
assert int.from_bytes(t[167:171], "little") == 5, "name2 length slot"
assert t[171:176] == b"yan55", "name2"
assert t[180] == 1, "settlement state byte (post-settlement standby frame 507 has 0)"

stage_frames = {0: 488, 1: 583, 2: 666, 3: 772}
patch_rows = []
for st in range(4):
    fr = stage_frames[st]
    key = f"stage{st}_id2_frame{fr}"
    assert key in allframes, f"missing {key} in archive"
    other = bytes.fromhex(allframes[key]["body_hex"])
    assert len(other) == 480, f"stage{st} frame {fr}: {len(other)}B"
    diffs = [i for i in range(480) if other[i] != t[i]]
    assert all(416 <= i < 420 for i in diffs), f"stage{st} frame {fr} unexpected diffs at {diffs}"
    patch = ", ".join(f"0x{b:02x}" for b in other[416:420])
    patch_rows.append(f"\t{{{patch}}}, // stage{st} (s4 frame {fr})")
    print(f"stage{st} frame{fr} patch = {other[416:420].hex()}")

SRC = '''package protocol

import (
\t"encoding/binary"
\t"encoding/hex"
\t"fmt"
)

// [ISPINS-SETTLEMENT-N2] next79 §27（2026-10-03 八测）：官服阶段结算链尾段
// （s4 帧 488-496）以一帧 N2（op=2，480B）开头，客户端收到该段后 10ms 才
// 发 CMD1654 进结算面板；私服链缺这一帧时客户端改发官服不存在的 op=2060，
// 1.2s 后 op=682 闪退（八测实证）。官服帧两处内嵌会话角色名 yan55 与官服
// actor 0x00ed（§21.5 verbatim 回放=本地 actor 表查无此人→闪退先例），必须
// 本地构造。op=2 行读取器为游标式顺序解析（docs/protocol/entry-userinfo.md
// “307 + UTF-8 name bytes”，行宽随名字长度变化），故按本地名字长度重排名
// 后字段；模板取自官服 s4 帧 488，两处名字块与第二块的 actor 槽（模板偏移
// 165）替换为本地值，其余字节 verbatim。
//
// 模板内第二名字块后紧跟迷你角色行：+0 职业、+1 转职、+2 等级（0x73=115）、
// +3 PvP、+4 状态（结算=01，回待机版帧 507 为 00，模板自带 01 无需再改）。
// 四阶段帧（488/583/666/772）除模板偏移 416..419 外逐字节一致：2 字节
// per-stage nonce + u16 阶段进度（3/7/11/15 = 4*stage+3），按阶段回放。
const ispinsSettlementTemplateHex = "__TMPL__"

var ispinsSettlementTemplate = mustSettlementTemplate(ispinsSettlementTemplateHex)

var ispinsSettlementStagePatch = [4][4]byte{
__PATCH__
}

func mustSettlementTemplate(s string) []byte {
\tb, err := hex.DecodeString(s)
\tif err != nil {
\t\tpanic("ispins settlement template: " + err.Error())
\t}
\tif len(b) != 480 {
\t\tpanic(fmt.Sprintf("ispins settlement template: %d bytes, want 480", len(b)))
\t}
\treturn b
}

// IspinsSettlementCharacterInfo 构造结算链尾段的 N2（官服 s4 帧 488 族）：
// 官服 480B 模板 + 本地角色名/actor 重排 + 每阶段 4 字节补丁。名字长度与
// 官服（5 字节 yan55）不同会平移名字后字段，游标式读取器按顺序消费即可。
func IspinsSettlementCharacterInfo(name string, actor uint16, stage int) ([]byte, error) {
\tif len(name) < 1 || len(name) > 63 {
\t\treturn nil, fmt.Errorf("结算角色名无效")
\t}
\tif actor == 0 || actor == 65535 {
\t\treturn nil, fmt.Errorf("结算角色 actor 无效")
\t}
\tif stage < 0 || stage > 3 {
\t\treturn nil, fmt.Errorf("伊斯阶段号 %d 越界", stage)
\t}
\ttmpl := ispinsSettlementTemplate
\tp := make([]byte, 0, len(tmpl)+2*len(name))
\tp = append(p, tmpl[:28]...)
\tp = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
\tp = append(p, name...)
\tp = append(p, tmpl[37:165]...)
\tp = binary.LittleEndian.AppendUint16(p, actor)
\tp = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
\tp = append(p, name...)
\t// 模板偏移 176 起 = 第二名字块后的迷你角色行（职业/转职/等级/状态 01）。
\tp = append(p, tmpl[176:]...)
\t// 官服模板偏移 416..419 是四阶段间唯一差异（nonce + 阶段进度），本地
\t// 名字长度与官服（5）的差量把该区平移 2*(5-len(name))。
\tpatchAt := 416 - 2*(5-len(name))
\tif patchAt < 0 || patchAt+4 > len(p) {
\t\treturn nil, fmt.Errorf("结算 N2 阶段补丁越界")
\t}
\tcopy(p[patchAt:patchAt+4], ispinsSettlementStagePatch[stage][:])
\treturn p, nil
}
'''

src = SRC.replace("__TMPL__", tmpl_hex).replace("__PATCH__", "\n".join(patch_rows))
with open(OUT, "w", encoding="utf-8", newline="\n") as f:
    f.write(src)
print(f"wrote {OUT} ({len(src)} chars)")
