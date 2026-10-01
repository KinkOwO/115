// Package savecontract 定义**服务端侧**的存档契约版本。
//
// 2026-10-01（next146 结构性根治）：存档身份**不得**依赖任何客户端资源哈希。
//
// ## 为什么需要这个包
//
// 直读模式下，服务端曾把内层归档 `Script.inner.pvf` 的 SHA256 当作存档身份
// （`characters.config_version` 等 6 张表）。运行时拿 `role.ConfigVersion` 与
// 当次目录的 `Catalog.Source.Checksum` 逐处比对，不等就**硬拒**：
//
//	quest %d requires source migration
//	character event source mismatch
//	accepted quest set changed or requires source migration
//
// 这在**契约上就是错的**，有三条独立的错：
//
//  1. **内层哈希不可复现**：同一份客户端三件套（DFO.exe + sk.dat + Script.pvf）
//     重新解包，得到的是**另一个** SHA256（实测 `7ef2db59…` → `b2b503b5…` →
//     `be95d64e…`，内容语义未变）。服务端自己重打包一次，全体存档作废。
//  2. **玩家改 PVF 是合法行为**：改掉率、改物品属性、改技能倍率**不产生新的存档状态形状** ——
//     存档里存的仍是「角色 ID / 任务 ID / 模板号 / 数量」，这些**键**没变。
//     后果由玩家自己承担，服务端**没有任何理由替他把档作废**。
//  3. **契约本该由服务端定义**：存档里每一行的格式、每个字段的语义、每个引用的有效性，
//     全部由**服务端**的 schema 与领域逻辑决定 —— 客户端资源只是**输入**，不是**契约**。
//
// ## 因此
//
// 存档身份 = **服务端契约版本**（本包的常量），随服务端**自己的**语义变更走。
//
//	| 事件 | 旧行为 | 新行为 |
//	| --- | --- | --- |
//	| 服务端重打包内层 PVF | 存档全部作废 | 不受影响（版本没变） |
//	| 玩家改 PVF 数值 | 存档全部作废 | 不受影响（版本没变） |
//	| 玩家换客户端 exe | 存档全部作废 | 不受影响（版本没变） |
//	| 服务端改了存档 schema/语义 | —— | 主动 bump，触发定向迁移 |
//
// ## 分级：三类判定不许混淆
//
//   - **L1 存档身份**（本包）：这行档属于哪个「契约世代」。用 `Identity()`。
//   - **L2 引用完整性**：存档里的引用在新目录里还解得出吗。按 **ID 查表**判定，
//     缺了就**逐条降级**（丢这一条），**不是**整体拒绝角色。
//   - **L3 运行时校验**：当前资源是不是我以为的那份。用内层 SHA256，
//     **仅用于启动期自检**，绝不参与存档门禁。
//
// 内层 PVF 的历史错误在于同时违反了「不可复现的产物不配做身份」与
// 「玩家能改的东西不配做拒档理由」这两条。
package savecontract

import (
	"crypto/sha256"
	"encoding/hex"
)

// Generation 是**当前**的存档契约世代号。
//
// ## 什么时候 +1
//
// 只在「**旧的存档行按新代码读会出错或语义漂移**」时：
//
//   - 给某个存档 JSON 增删/改名了字段，且旧行没有默认值可用
//   - 改了引用的语义（例如 quest 的 progress_model 换了算法）
//   - 改了主键/唯一约束的含义
//
// ## 什么时候**不**要 +1（这是绝大多数情况）
//
//   - 加了新功能，老档照常读 ⇒ 不 +1
//   - 玩家改了 PVF（掉率/属性/技能倍率）⇒ 不 +1
//   - 服务端重新打包了内层 PVF ⇒ 不 +1
//   - 换了客户端 DFO.exe ⇒ 不 +1
//   - 加了新表/新列（纯增量）⇒ 不 +1
//
// 记住：**+1 的后果是全体老档要走迁移路径**。宁可少 +1，也不要为了「看起来严谨」乱 +1。
const Generation = 1

// identity 是契约版本的 64 位 hex 编码。
//
// ## 为什么必须是 64 位 hex
//
// 现有代码里有大量 `if len(x) != 64 { return error }` 形状的**入参校验**
// （`vault.go:34`、`LoadBagRules`、`CreateCharacter`、`CommitCharacterEvent` 的
// `version` 参数…）。这些校验本身是对的（防备拼错/空串），但它们要求身份是
// 「32 字节的 hex」。把契约版本也做成同一形状，就能让**校验代码一行不改**，
// 同时语义从「内层归档哈希」换成「服务端契约版本」。
//
// 编码方式 = SHA256("dfolan-save-contract:v<Generation>")：
// 稳定、可复算、与任何客户端产物无关。
var identity = func() string {
	sum := sha256.Sum256([]byte("dfolan-save-contract:v1"))
	return hex.EncodeToString(sum[:])
}()

// Identity 返回当前存档契约身份（64 位 hex）。
//
// 这是**唯一**应当写入 `config_version` / 参与存档身份比较的值。
// 内层归档的 SHA256 只用于启动期自检（L3），不得流入这里。
func Identity() string { return identity }

// IsIdentity 判断一个字符串是否是**格式合法**的契约身份。
//
// 注意：这只检查形状（64 位 hex），不检查是否等于当次身份 ——
// 历史世代的身份同样合法，迁移时要能识别。
func IsIdentity(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
