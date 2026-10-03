# 用 s1 解密的 706 表 verbatim 重写 Go 文件中的 weeklyDungeonInfoTableHex
# （撤销此前所有误编辑，0x2de 恢复 s1 战前值 00）
import re

GO = r"D:\115us\115-server\server\work\dfo-lan\cmd\wireprobe\weekly_dungeon_info_generated.go"
out = open(r"D:\115us\115-server\server\work\dfo-lan\cmd\wireprobe\_dump781_out.txt").read()
hexes = re.findall(r'^([0-9a-f]{400,})$', out, re.M)
s1 = [h for h in hexes if len(h) == 2928][0]
assert len(s1) == 2928
assert s1[0x2de*2:0x2de*2+2] == "00" and s1[0x2dd*2:0x2dd*2+2] == "01", "s1 sanity"

src = open(GO, encoding="utf-8").read()
# 匹配整个 const 块（从 const 声明到结尾的 `"...";`）
pat = re.compile(r'(const weeklyDungeonInfoTableHex = "" \+\n)((?:\t"[0-9a-f]+"[^\n]*\n)+)')
m = pat.search(src)
assert m, "const block not found"
segs = [s1[i:i+64] for i in range(0, 2928, 64)]
assert len(segs) == 46
block = "\n".join('\t"%s" +' % s for s in segs)
# 最后一段行尾改分号
block = block.rsplit(" +", 1)[0]
new = m.group(1) + block + "\n"
src = src[:m.start()] + new + src[m.end():]
open(GO, "w", encoding="utf-8").write(src)
print("rewritten; verify:")
cur = bytes.fromhex("".join(segs))
print("len:", len(cur), "0x2dd:", hex(cur[0x2dd]), "0x2de:", hex(cur[0x2de]), "0x2df:", hex(cur[0x2df]))
