军团 UI 修复（2026-10-12）

本补丁从 G:\电脑问题\上游仓库版本服务端 的当前已验证改动提取，基于 origin/main e5bd2036；不包含该工作区其它整合改动、玩家存档或候选二进制。

回廊：当前中文 String/UI.uv.str 的五个键将 %s 写在 %d 前，但当前 US 原生 1420a2910 向 146e8cf20 传入整数后接宽字符串。Windows CRT 独立复现五项访问违规；修复仅调整对应中文字符串顺序。WriteEntryCopy 扩展现有 Type3 UTF-16 表写回，保持路径、字符串池、其它记录，拒绝奇数字节和覆盖已有文件。

资源补丁不会由服务端自动应用。cloister-format-fix.json 提供五个精确原值/新值及输入 SHA256；repair-cloister-formats.go.txt 为可复现生成器。将其复制为模块内 runtime/test/cloister-format-repair/main.go 后，进入 server/work/dfo-lan 执行：
  go run ./runtime/test/cloister-format-repair <原始内层PVF> <客户端目录> <cloister-format-fix.json> <新的输出目录>
生成器会检查原始 PVF 和 DFO.exe 的 SHA256，修改五键，生成内外层 PVF 并反解验证摘要；输入不同必须先重新审计，不能略过校验。
本地已配对安装内层 829140e68b700558d7da888c4143a08bf4fdc101f79f59f26cbc5e57d25abf3d / 外层 4ecacc44f4bc273894df87c3124b236c5757cc0c7beff076f59af55574698548。其它机器安装前备份原始内外层PVF、profile、manifest，关闭游戏并同步安装两端资源与hash/cache身份。

幽暗岛：N2467 接收器 141c22e60 只查询已存在导航。C2143 ACK 接收器 141c28e90 通过 14667eb40 的 lazy factory 创建 0x289 导航窗，原生 141c11220 装配已有 mini XUI。在倒计时完成的 N2467 后补一次现有20B action1/index=-1/close=1/secondary=0/deadline=0 ACK，进入导航 state0。无自动选卡、不消耗重选次数、不改变选择期限，重复轮询和跨角色均不发送。不生成美术、不修改或启用 DLL。

协议运行链按保守口径 attempt 3/3。本轮仍是待实机验收的候选；如失败，继续取证，不做第四次盲改。静态分析中的 14182b0f0 是热键检测，不是 UI 功能门禁。

本地整合端已通过 build/vet 与全量测试 5525 pass /369 skip /0 fail；本发布分支另行重新执行编译、vet和全量测试，结果见合并请求说明。Windows CRT复现、资源往返和Go测试不能代替游戏实机验收。当前两处UI尚未用户复测，不更新 confirmed baseline。