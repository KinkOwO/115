# 流程测试夹具

这些历史投影只供 `_test.go` 使用，生产代码不加载 testdata，也不作为游戏内容配置。

- `booster-flow.json`：63 个被流程用例引用的旧定义及嵌套定义。
- `booster-items.json`：上述奖励与流程用例所需的 1436 条元数据。
- `selection-flow.json`：26 个自选盒，包括奥德赛、章节掉落、装扮和固定盒边界。
- `selection-historical-templates.json`：原完整自选范围的模板 ID；原生装备发放审计保留全部历史范围，不保存第二份内容。

历史来源为已退休的 JSON 导出（7ef2 构建），只用于独立验证选择、发放、事务和存档身份等流程。当前 8b2a PVF 的三域完整内容指纹见 `internal/gamedata/catalogs_native_only_test.go`；原生范围/历史流程夹具对照及完整历史装备发放审计由 `DFO_PVF_CORE_TEST_ARCHIVE` 显式启用。PVF 更新应重新取证，不把这些夹具作为运行内容维护。
