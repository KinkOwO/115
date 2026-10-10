# R5 武器幻化动画插件源码

本目录保存正式累计 R5 使用的冻结 r1 源码。原用户已确认 Deemo10110 普攻叠加修复，最新选角修复亦已确认；不扩大到所有职业与技能逐项验收。源码导入不安装或启用本机插件。

从仓库根执行 client-patchs/weapon-skin-animation-r1/build.cmd；配置 VCVARS64 和可选 US115_SDK_ROOT/US115_SDK_VERSION。脚本编译 DLL 并运行已有机制自测，产物在 dist。期望字节和原生向量来自冻结源 src/native_evidence.h、native_loop.h、source-binding.json；可用 generate-evidence.py --client <匹配的DFO.exe> 重新取证，须提供 pefile/capstone。常规构建无客户端文件依赖。

build-mod.py 生成 schema 2 包，依赖 qol.client-host；其原候选版本标签保持便于原白名单追溯。validate-mod.py 保存历史验证源码，仍需原安装/工具路径，未在本轮运行。源码清单和自测不等于自动安装或新的实机验收。

下方保留原候选说明作为来源记录；默认启用、版本和已确认范围以正式 R5 发布记录及本段为准。

---

# 武器幻化攻击动画优先级候选 r1

用户已明确授权为“副本攻击时武器幻化与武器装扮叠加”制作独立客户端补丁/DLL。
服务端已经发送有效幻化覆盖，但原生动画组合仍分别读取武器装扮槽10和幻化武器槽12；重复同步同一消息不能排除装扮动画。

插件只在原生动画更新和攻击扩展两个调用范围内，让有效幻化角色的武器装扮视觉读取返回空；其他槽位和范围外读取完全转发。原生幻化资格不成立或幻化物件未创建时保持原行为。解除后下次原生动画更新立即恢复。普通/克隆装扮共用同一槽位规则，没有逐件ID例外。

不修改穿戴数据、克隆源、属性、PVF、客户端EXE文件或服务端事务。通过既有客户端宿主加载；仅候选入口设置 `DFO_WEAPON_SKIN_ANIMATION=r1`，其他入口不启用此候选。四个钩子安装前及发布前均逐字节校验，版本或入口冲突则拒绝安装。

日志在DLL所在目录：`.115us-mods/weapon-skin-animation.log`。日志 `READY` 表示钩子准备成功，`OBSERVED` 表示命中有效幻化动画范围；二者不代替用户画面验收。

构建：本机MSVC x64、静态CRT，构建入口位于 `scripts/build-weapon-skin-animation-20261011-r1.cmd`。源码与包在本目录，证据在 `server/analysis/tasks/weapon-skin-attack-20261011/`。

交付使用 schema 2 mod `qol.weapon-skin-animation`，依赖 `qol.client-host`；正常退出游戏后才可安装/卸载。卸载仅移除该插件，保留既有宿主和其他插件。默认程序和存档不升级。

待手动验收：Deemo10110副本普攻、近身动作及技能；普通和克隆武器装扮；启用、切换、解除幻化，重进/过图/回城及换角色。原生组件、编译测试不能证明所有攻击帧或全职业错位已解决。
