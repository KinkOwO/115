# 末世录第三档原生卡片（候选）

当前 US PVF 的 `contents/2026/apocalypse/xui/selectoperationcard.xui` 已有
`operation_2`、`OperationCard2.xui` 和 Card_2 动画，但初始隐藏且禁用。
当前 EXE 的创建、点击、刷新、销毁循环只处理两张卡片。服务端开放第三档
不能替代这部分界面实现。本插件复用这套原生资源与 C2354 确认路径。

按用户要求，美术依据为本地 CN 原版：Card_2 四个动画均引用
`Contents/2026/Apocalypse/UI/LevelSelect.img` 第 3 帧，卡片符号引用
`Interface2/Legion/LegionCommon/Symbol_Flat_L.img`。实际解包核对：CN 与
当前 US 的第三卡像素及偏移完全一致，符号 IMG 整体也逐字节相同。
因此可直接调用当前客户端中的同一份原版素材，不绘制或生成替代美术。
两边 LevelSelect 整体文件有其它帧差异，不能把整文件差异当作第三卡差异。
只读复核命令为 `python src/audit-resources.py --cn-client <CN目录>
--us-client <US目录> --out dist/artwork-audit.json`；不一致即报错，不能静默
声称使用了相同 CN 素材。编译 ANI 的字符串池索引也不可直接跨 PVF 复制。

适用取证版本：DFO.exe SHA256
`515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb`。
逐条指令记录在 `src/patches.json`，运行时全部核对通过才修改内存。
插件不写 EXE/PVF 文件；原卡片数组旁的确认窗口不能被第三指针覆盖，
因此将数组移至对象末尾 +0x428，同时扩展分配/释放尺寸到 0x440。
创建、销毁、就绪、点击、刷新、更新、隐藏和渲染中的数组引用一起迁移。

## 构建与检查

运行 `build-plugin.cmd`（x64 MSVC，静态 CRT，警告视为错误），执行上述
CN/US 原版资源核对，再运行 `python build-mod.py`。打包要求核对结果通过，
包内附 `artwork-provenance.json`，保留资源路径及像素校验值，不包含本机目录。
脚本通过 vswhere 查找编译器，也接受 `VCVARS`。
便携 Windows SDK 可通过 `SDK_INCLUDE`、`SDK_LIB` 指定头文件和库目录列表。
构建输出只放在忽略的 `dist/`。

`../tests/build-apocalypse-card.cmd` 只在独立测试进程验证字节预检、拒绝部分
写入、内存页权限恢复、扩展数组不覆盖确认窗口、第三卡片启用和显示。
它不启动游戏，不能证明当前游戏里的点击、确认和进图成功。

## 安装、状态和回滚

候选包使用 schema 2，依赖 `qol.client-host`，DLL 安装到
`<客户端>/.115us-mods/ApocalypseThirdCard.dll`。使用 modkit 的
verify、plan、install、uninstall；本任务不自动安装到正在使用的客户端。
必须关闭游戏后安装或卸载，由用户手动启动进行实机验证。

日志与 `apocalypse-third-card.status.json` 写在 DLL 自己目录。
`enabled=true` 仅代表内存修改完成；`initializedPanels>0` 才代表三张原生卡
已创建。字节不匹配、线程暂停失败或窗口管理器已存在时拒绝加载，日志记录原因。
卸载后重新启动游戏恢复原生两卡行为；状态和日志是运行期证据，可保留。

尚需实机确认：宿主加载时机、第三张卡片布局、点击/确认索引 2、进图及返回。
未确认之前不得写成已完成修复。

服务端第三档流程现已接入对应候选源码：C2354 key2 对应 CTP index3，
首个 BOSS 的服务端耗时冻结四条源分支门，C2062 实际目的地选择路线；
后续只推进选定路线，到源终点 5 后按 master 规则事务入库并结算。
真实 PVF 四路线、关闭门拒绝、SQLite 最终领奖/重复请求/片尾返回已自动验证。
这些服务端检查不证明客户端第三卡已实机成功；仍按上述状态文件验收。
