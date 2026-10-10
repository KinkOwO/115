# 巴卡尔基础战斗确认基线（2026-10-06）

用户确认：“现在小怪需要击杀才有效了……邪龙、狂龙及部分门将现在也可以打了”。

确认范围：小怪正常战斗/击杀、邪龙/狂龙及部分门将实际战斗对象生成，保留冰龙已可战斗的反馈。不将7组自动入口测试等同于7种首领全部实机验收，不将本轮确认扩大到免疫BUFF、随机觉醒、后续刷新、奖励或完整通关。

- 精确程序：`bin/wireprobe-bakal-live-arena-candidate.exe`。
- SHA256：`75427e459ed989330ea893143f29a06a2ecf1b4a9aa9700a74aa232526b079b2`。
- 实机记录：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261006_162624_638704_next37`，用户最新反馈为验收依据。
- 客户端：`D:\115us\DFO`，启动路线PostgreSQL。
- 精确二进制与当时profile已保存到 `runtime/baselines/bakal-core-confirmed-20261006`，不进入Git。

确认后的机制审查发现波次怪物类型枚举有错误，新候选纠正斯万9/埃可莱尔11及其它原生类型；新构建必须与上述已确认程序区别记录，不能以源码再构建覆盖已确认指纹。BUFF与源计时事件缺口见 `bakal-mechanics-audit-20261006.md`。

收口提交限制：server/AGENTS.md要求提交前执行scripts/check-commit-hygiene.ps1；实际调用失败且全仓没有该脚本。本轮保留确认记录和变更，未跳过门禁提交。
