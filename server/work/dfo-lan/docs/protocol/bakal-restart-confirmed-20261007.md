# 巴卡尔失败重开实机确认（2026-10-07）

用户在021400会话确认“点击第二次开始没有重复生成”。确认对象为当时默认recovery程序，SHA256 `63e345740142978c1c11eb396502697b5106b5baa18641afbfa7e2239b9f9dad`。精确副本和profile位于runtime/baselines/bakal-restart-confirmed-20261007。只确认失败重开不重复生成，不确认邪龙空房出口、二阶段、退出勾选警告或所有随机波次。

该会话随后指出首轮邪龙区域接近失败前没有出口：日志已定位为击杀布洛娜后再次进入已清空arena0,1。后续cinematic-lifecycle候选修复见同日期专项文档，尚未实机确认，不与本快照混用。

CHANGELOG已记录。工作区仍缺scripts/check-commit-hygiene.ps1，未绕过提交门禁或将其它改动暂存／提交。
