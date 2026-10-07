package savecontract

// CatalogIdentity 从**任意**携带 `Source` 快照的目录对象里取出**存档身份**。
//
// ## 为什么需要它（而不是直接读 .Source.Checksum）
//
// 各目录的 `Source`（`pvf.ArchiveSnapshot`）里的 `Checksum` 是**内层归档哈希**，
// 它有两个完全不同的用途被混在了一处：
//
//   - **L3 目录自校**：`a.Snapshot().Checksum != index.Source.Checksum` ——
//     「这份 JSON 目录是不是从这份归档生成的」。**必须**用内层哈希。
//   - **L1 存档身份**：`role.ConfigVersion != s.Catalog.Source.Checksum` ——
//     「这行存档属于哪个内容世代」。**绝不能**用内层哈希（见包注释）。
//
// 本函数是 **L1 专用**。凡是「拿存档行里的版本号跟目录比」的地方，
// 都应当用 `CatalogIdentity(catalog)` 取代 `catalog.Source.Checksum`。
//
// ## 实现
//
// 现阶段直接返回 `Identity()` —— 因为存档身份已经**与目录内容无关**，
// 是服务端定义的契约版本。参数保留是为了：
//
//  1. 让调用点表达「我在比的是这个目录的存档身份」，语义自明；
//  2. 将来若需要「不同目录有不同契约世代」（例如道具目录与任务目录分别演进），
//     可以在这里按目录类型分流，而**不改调用点**。
//
// ## 为什么签名用 `any`
//
// 调用点传进来的类型五花八门（`catalog.QuestCatalog`、`*loot.Catalog`、
// `inventory.WearRules`、`map[uint32]...` 等几十种），且都在别的包。
// 引入接口会强迫几十个类型实现方法；用 `any` 则**零侵入**。
// 这与本模块「只改身份取值，不改结构」的策略一致。
func CatalogIdentity(catalog any) string {
	_ = catalog
	return Identity()
}
