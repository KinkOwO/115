import pathlib,json,shutil,hashlib,re,datetime
p=pathlib.Path(__file__).parent;out=p.parent.parent/'outputs';e=out/'DFO运行验证材料';e.mkdir(exist_ok=True)
names=['args_empty.log','args_synthetic.log','loopback_ui.log','normal_ui.log','loopback_capture.json','normal_capture.json','normal_windows.json','decoded_args_synthetic.txt','decoded_normal_startup.txt','preservation.json','argument_breakpoints.txt','devconfig_constructor.asm','argument_parser_entry.asm','startup_mode.asm','startup_annotated.asm','startup_resource_literals.json','native_literals.json','probe.cpp','capture_loopback.py','decode_literals.py','scan_literals.py','inspect_startup_mode.py','verify_preservation.py']
for name in names:shutil.copy2(p/name,e/name)
pres=json.loads((p/'preservation.json').read_text());normal=json.loads((p/'normal_capture.json').read_text());debug=json.loads((p/'loopback_capture.json').read_text());prior=json.loads((out/'DFO校验证据.json').read_text(encoding='utf-8'));literals=json.loads((p/'native_literals.json').read_text(encoding='utf-8'))
facts={
 'checked_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),
 'source_directory':r'F:\dnfop\DFO','isolated_copy':str(p.parent/'dfo_probe_client'),'build':{'version':'2.38.2.34','machine':'AMD64','sha256':prior['core_hashes']['DFO.exe'],'pvf_sha256':prior['core_hashes']['Script.pvf']},
 'scope':'Controlled startup, resource initialization, and passive loopback connectivity; no game server implementation.',
 'confirmed':{'original_entry_reached':True,'no_argument_exit_code':1,'no_argument_sets_alone_flag':True,'synthetic_argument_delimiter':'?','synthetic_arg_index4_nonempty_clears_alone_path':True,'pvf_open_observed':True,'resource_preload_return':1,'module_load_return':1,'game_core_create_return':1,'normal_no_debugger_initmain_finished':True,'normal_startup_seconds_from_trace':21.10,'select_character_module_log_observed':True,'loopback_accept_observed':True,'native_literal_objects_decoded':len(literals),'all_64_original_core_hashes_unchanged':pres['all_original_core_hashes_unchanged'],'all_64_copy_core_hashes_unchanged':pres['all_copy_core_hashes_unchanged']},
 'addresses_exact_build_only':{'entry':'0x148861010','argument_parser':'0x144ED05B0','missing_field_sets_alone':'0x144ECFD8B','encoded_alone_flag':'0x14E682A50','resource_gate_test':'0x145998678','resource_preload_result':'0x1459987A3','game_core_create':'0x1459968C0','game_core_result':'0x146DE1EB2','wide_literal_getter':'0x146E8C7D0','narrow_literal_getter':'0x146E8C7B0'},
 'normal_run_capture':normal,'debug_run_capture':debug,
 'limitations':['No successful login, role response, town entry, dungeon, or save persistence.','No protocol payload was received during these passive captures; server-first handshake is an unconfirmed hypothesis.','PVF was consumed by the original client but was not externally extracted or independently decoded.','Some script and image references reported missing files or invalid image indexes; full asset compatibility remains unproven.','CEF.exe displayed a Themida debugger warning under child-process debugging; this warning was not present in the observed normal-run window list.','Screenshot capture failed with SetIsBorderRequired / E_NOINTERFACE 0x80004002. A game window and module log are not visual role-selection acceptance.','The controlled runs block non-loopback traffic for exact executable/AES paths in the copy. They do not establish online authentication or game-security acceptance.','Isolation is a separate client directory, not a full OS sandbox. Normal DNF user logs, CEF cache and graphics caches may be written.','Client sessions were deliberately terminated at 55 seconds; test timeout exit codes are not client crash findings.','The first synthetic debug run hit a probe first-chance exception counter; later probes corrected this and repeated successfully.'],
 'process_cleanup':'Final live process query found no processes in dfo_probe_client and no probe.exe. Both final runs logged JOB_CLOSED and WFP_DYNAMIC_SESSION_CLOSED.',
 'preservation':pres,
 'evidence_files':[{'file':n,'bytes':(e/n).stat().st_size,'sha256':hashlib.file_digest((e/n).open('rb'),'sha256').hexdigest()} for n in names]
}
(out/'DFO运行校验证据.json').write_text(json.dumps(facts,ensure_ascii=False,indent=2),encoding='utf-8')
report=r'''# DFO 启动与单机改造继续验证

检查日期：2026-09-10。原目录：`F:\dnfop\DFO`。独立测试副本：`E:\codex\2026-09-10\zhe\work\dfo_probe_client`。

**结论：这份 x64 客户端能够实际启动、读取当前 Script.pvf、完成初始化，并按测试参数连接本机 TCP 端口。它具备继续做单机兼容改造的条件；尚未实现登录、角色数据、城镇、战斗或存档。**

本报告更新此前静态评估中的“尚未验证启动”和 DLL 风险判断。EXE 仍是 2.38.2.34，SHA-256 为 `FDA6C33F2A155124A4262F3E7CD6F7A18D4E2191E5F147AEC5754716AFE5AA7C`。下列地址只对这一份文件有效。

## 实际通过的验证

| 项目 | 观察结果 | 证据与范围 |
|---|---|---|
| 原始程序入口 | 命中 `0x148861010` | 无磁盘 EXE 补丁 |
| 基础系统 | DirectX 11、图片包、输入、字体、声音完成初始化 | 调试运行及无调试器对照日志 |
| PVF 读取 | 实际打开两次 `Script.pvf`，资源预加载返回 1 | 并非只看到文件名字符串 |
| 脚本初始化 | 角色、怪物、装备、技能、地图、地下城等初始化日志通过 | 不代表对应玩法已经验收 |
| 游戏初始化 | `CNGameCore::Create` 返回 1，`InitMain is Finished` | 正常对照启动约 21.10 秒 |
| 选角模块 | 日志切换到 `MODULE_TYPE_SELECT_CHARACTER(0)` | 没有服务端角色数据，也没有画面验收 |
| 本机连接 | 监听器实际接受了来自客户端的 TCP 连接 | 两次测试均成功，均未发送服务端回应 |
| 退出性质 | 正常对照运行到 55 秒仍存活，由探针主动结束 | 不能把超时清理码当成客户端崩溃 |

无调试器对照的日志记录：

```text
SCRIPT>> path = Script/, use pack = 1, name = Script.pvf
INIT>> InitCharacterScript is passed.
INIT>> InitEquipmentScript is passed.
INIT>> InitDungeonScript is passed.
INIT>> InitSkillScript is passed.
INIT>> PATCH_DATE Client version : 20260901
change module : [MODULE_TYPE_MAX:NOT INITIALIZE YET(41)] -> [MODULE_TYPE_SELECT_CHARACTER(0)]
NETWORK>> try to connect at (127.0.0.1:60849)
NETWORK>> Connection Success
INIT>> InitMain is Finished.
* StartupTime : 21.10 sec
```

这里的端口 60849 是监听器本次自动分配的临时端口，不是服务端固定配置。

## 无参数启动为何退出

已经用静态反汇编和两组运行断点交叉确认：

1. 当前构建的 `ClientArgumentParser` 把传入字符串按 `?` 分割，并补齐至少 22 个字段。
2. 第 5 个字段，即下标 4，为空时，在 `0x144ECFD8B` 设置独立测试模式标志。无参数启动确实命中了这里。
3. 初始化函数在 `0x145998678` 检查这个标志。值为 1 时，跳入失败出口，游戏初始化返回 `0x80000000`，最终正常退出码为 1。
4. 使用非空占位字段后，该标志为 0，客户端实际越过了这个出口，随后读取 PVF 并完成初始化。

受控测试参数样本：

```text
13?127.0.0.1?60849?probe?00000000000000000000000000000000?0?0?30?0?0?0
```

其中账户和令牌字段均为合成占位内容，没有使用真实账户、会话或登录凭据。当前测试路径中，下标 1 实际用于目标地址，下标 2 实际用于端口。此样本只证明参数被解析以及本机连接可达，**不能当作已经验证的正式启动器参数契约，更不能作为登录凭据**。

程序保留 `_DevConfig.xml`、`run_client_alone`、`run_client_alone_config_file` 和 `DebugConfig.txt` 相关代码，但当前配置构造函数末尾还会重置部分开发选项。仅增改 DebugConfig.txt 或看到这些名称，不能证明发行版本支持完整独立运行。本次没有修改这些配置，也没有改跳转来强行通过。

## 壳与保护的判断更明确了

主 EXE 具有正常可分析代码、导入表和类型信息，且原始入口及初始化流程已运行。没有发现需要先解开整体壳才能开展分析的证据；这仍不等于每个函数、数值或字符串都没有保护。

本次从当前 EXE 的字符串解码函数恢复了 114,614 个唯一字符串对象，包含资源路径、初始化日志、类/函数和源码位置线索。它们使用内部编码，说明“字符串看不见”不等同于“整个程序有壳”。完整列表见运行验证材料中的 `native_literals.json`。

**整个目录不能称为“完全无壳”。** 调试器同时附加到子进程时，`CEF.exe` 实际出现了 Themida 提示：`A debugger has been found running in your system.`。不挂调试器的对照运行中，观察到的窗口列表没有该提示，客户端仍完成初始化和本机连接。NGClient/BlackCipher 组件也仍在运行；本次没有移除它们，也没有证明它们能长期配合离线运行。

此前怀疑的本地 `xinput9_1_0.dll`，在调试日志中已经看到实际加载路径，且程序继续完成了输入与整体初始化。**它没有阻断本次启动，不应仅凭 PE 机器类型字段就直接替换。** 这不解释该文件全部内部实现，也不保证所有手柄功能正常。

## 当前边界和遗留问题

- 本机监听器在调试运行约 24.22 秒、正常运行约 21.42 秒接受连接，但到各自 55 秒清理前，收到的应用数据都是 0 字节。监听器也没有发送任何回复。“客户端可能先等待服务端握手”只是后续假设，消息头、加密、长度和指令编号均未恢复。
- PVF 已被这份客户端实际消费，但尚未完成独立 PVF 解包、索引导出或原始脚本恢复。
- 日志仍有缺少脚本、图片不存在和图片索引越界信息，例如 `Etc/ItemDimmedList.etc`、`UI/HUD/Animation/hud_new.ani` 与部分国服路径。它们没有阻断这次初始化，但全量资源兼容性尚未通过。不能仅凭这些信息断定具体混用版本。
- 原 EXE 与包清单的大小及哈希差异仍然存在；尚未取得清单所对应的原始 EXE，无法解释差异来源和改动范围。清单缺少的一条技能回放也没有补造。
- Windows 窗口列表确认了 `Dungeon Fighter Online` 主窗口。截图接口失败：`SetIsBorderRequired failed: 不支持此接口 (0x80004002)`，因此没有把窗口存在、选角模块日志当成实际可见角色选择画面的验收。
- 首次带参数的调试探针把累计 first-chance C++ 异常计入停止阈值，约 34 秒主动清理；已纠正探针计数方式，之后调试与正常对照均跑到 55 秒。不能把首次探针清理结果归为游戏崩溃。

## 测试隔离与文件保全

测试使用约 48.41 GiB 的完整独立副本。未修改原客户端二进制、资源和配置；调试断点只存在于测试进程内存。另做了一轮不挂调试器、不下断点的正常启动对照。

测试期间，WFP 动态规则按副本内具体 EXE/AES 路径限制非 loopback 连接，覆盖 IPv4/IPv6。每次先验证本机连接可用、非本机连接得到 WSAEACCES，再启动游戏。未启用或修改整机防火墙配置。此限制不等于一台完整隔离虚拟机，也不证明线上认证通过；正常运行的 Windows systeminfo/conhost 仍属于系统进程。

游戏正常写入过当前用户 DNF 日志、CEF 缓存及图形缓存；这些用户目录没有删除或清空。原客户端目录保全复核结果：22,153 个文件，总大小仍为 51,975,680,118 字节，没有新增、删除或大小变化。64 个核心文件与前次基线 SHA-256 全部相同，副本中这 64 个文件也全部相同。未逐一重算其余约 48 GiB 资源内容哈希。

最终两个测试均记录 `JOB_CLOSED` 和 `WFP_DYNAMIC_SESSION_CLOSED`；结束后进程检查未发现副本或探针残留进程，临时本机监听器已关闭。

## 接下来可直接开展的工作

下一步是沿这个构建已经命中的连接路径恢复收包入口和首次握手，再做最小本机兼容服务端。验证顺序应为：握手和登录 → 角色列表/建角 → 选角进城 → 移动 → 退出重进与存档 → 副本、战斗和结算。已有结果使“客户端能否启动并读入资源”这一阶段有了实证，但没有完成单机玩法。

机器证据：[DFO运行校验证据.json](/E:/codex/2026-09-10/zhe/outputs/DFO运行校验证据.json)。完整日志、参数观察、哈希复核、字符串列表和探针源码位于 `E:\codex\2026-09-10\zhe\outputs\DFO运行验证材料`。
'''
report=report.replace('](/E:/','](E:/')
(out/'DFO启动验证报告.md').write_text(report,encoding='utf-8')
old=out/'DFO单机改造评估.md';text=old.read_text(encoding='utf-8');notice='> 后续更新：已完成隔离副本的实际启动、PVF 加载及本机 TCP 连接验证。本文保留为前一阶段静态记录；当前结论和 xinput 风险修正请见 [DFO启动验证报告](E:/codex/2026-09-10/zhe/outputs/DFO启动验证报告.md)。\n\n'
if '后续更新：已完成隔离副本' not in text:
 head,rest=text.split('\n',1);old.write_text(head+'\n\n'+notice+rest.lstrip('\n'),encoding='utf-8')
print('REPORT',str(out/'DFO启动验证报告.md'));print('EVIDENCE_FILES',len(names));print('DECODED_LITERALS',len(literals))
