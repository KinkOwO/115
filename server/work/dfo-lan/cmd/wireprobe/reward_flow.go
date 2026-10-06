package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/reward"
	"dfolan/internal/servermod"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// buildRewardService constructs the optional event-triggered Lua reward add-on.
//
// 规则来源有两份，**必须合成一个 fs.FS**（Options.Scripts 是整套替换，不是追加）：
//
//  1. 内嵌规则集（internal/reward 的 scripts/*.lua）—— 服务端自带的运营规则；
//  2. mod 提供的规则脚本（internal/servermod 收集）—— mod 层追加的事件规则，
//     例如"新角色创建时发一件装备"。
//
// 合成顺序：内嵌在前、mod 在后。奖励管线按 `fs.Glob` 的字典序加载，且每个脚本的
// on() 注册是**按脚本独立收集**的，所以后加载的脚本不会顶掉先加载的注册——两者叠加生效。
//
// 它从不失败启动：脚本编译错误只禁用这个可选特性并留一条警告。
func buildRewardService(store *database.Store, awarder *inventory.Awarder, skinUnlocks func() []database.AccountSkin,
	vaultRules func() *inventory.VaultRules) *reward.Service {
	if store == nil || awarder == nil {
		return nil
	}
	scripts := rewardScriptFS()
	service, err := reward.New(reward.Options{
		Scripts:      scripts,
		Grant:        rewardGrantFunc(store, awarder),
		Mail:         rewardMailFunc(store, awarder),
		Cera:         rewardCeraFunc(store),
		Entitlements: rewardEntitlementFunc(store, awarder, skinUnlocks, vaultRules),
		Log:          func(format string, args ...any) { log.Printf(format, args...) },
	})
	if err != nil {
		log.Printf("warning: reward rules disabled: %v", err)
		return nil
	}
	// 把"构造管线时到底看到几份 mod 脚本"打进日志。
	//
	// 为什么专门记这条：本行是**构造顺序的见证**。若 mod 的规则脚本是在
	// 管线构造之后才登记的，这里就会是 0 份 —— 而"已装载 1 个 mod"那行仍然打得出来，
	// 两者并存就是"登记了但没生效"的确凿信号（2026-10-06 02:22 现场就是这样）。
	// 有了它，这类顺序缺陷看一眼日志就能判定，不必再去查库。
	loaded := len(servermod.RewardScripts())
	if loaded > 0 {
		log.Printf("reward rules enabled (embedded scripts + %d mod script(s))", loaded)
	} else {
		log.Printf("reward rules enabled (embedded scripts)")
	}
	return service
}

// rewardScriptFS 把内嵌规则集与 mod 规则脚本合成一个只读 fs.FS。
//
// 落盘补充口：<服务端模块>/mods/scripts/*.lua —— 不想重编译就能加一条规则时用。
// 同名时磁盘优先（便于就地覆盖调试）。
func rewardScriptFS() fs.FS {
	bundled := reward.BundledScripts()
	if bundled == nil {
		// 内嵌规则集拿不到（二进制异常）：退回只读 mod 脚本，至少不让启动崩。
		log.Printf("warning: 内嵌奖励规则集不可用，只加载 mod 规则脚本")
		return servermod.NewRewardScriptFS(filepath.Join(serverModsDir(), "scripts"))
	}
	// 注意 second 是**视图**不是快照：构造之后登记的 mod 脚本依然可见
	// （见 internal/servermod.modScriptFS 的注释）。这里的份数只用于诊断。
	if n := len(servermod.RewardScripts()); n == 0 {
		log.Printf("servermod: 构造奖励规则视图时暂无 mod 脚本（若随后 mod 才登记，" +
			"视图仍会看到它们；若始终为 0 请检查 mods.RegisterMods() 的调用时机）")
	}
	return &compositeScriptFS{
		first:  bundled,
		second: servermod.NewRewardScriptFS(filepath.Join(serverModsDir(), "scripts")),
	}
}

// compositeScriptFS 依次在 first、second 里找文件；第一个命中的胜出。
//
// 只用于平铺 *.lua 的读取（奖励管线就是这么用的：fs.Glob(src, "*.lua") + fs.ReadFile）。
type compositeScriptFS struct {
	first  fs.FS
	second fs.FS
}

func (c *compositeScriptFS) Open(name string) (fs.File, error) {
	if f, err := c.first.Open(name); err == nil {
		return f, nil
	}
	return c.second.Open(name)
}

// ReadDir **必须实现**：奖励管线用 fs.Glob(src, "*.lua") 列脚本，而 fs.Glob 走的是
// ReadDir（不是靠 Open 试探）。少了它，glob 会静默返回空——mod 的规则脚本一份都读不到，
// 而且**不报错**。这里把两层目录项合并，first 优先（内置规则集在前）。
func (c *compositeScriptFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." && name != "" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	seen := map[string]bool{}
	var out []fs.DirEntry
	collect := func(src fs.FS) {
		rd, ok := src.(fs.ReadDirFS)
		if !ok {
			return
		}
		entries, err := rd.ReadDir(".")
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(e.Name()), ".lua") {
				continue
			}
			if seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			out = append(out, e)
		}
	}
	collect(c.first)
	collect(c.second)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

// Stat 让 fs.Glob/fs.ReadFile 的某些路径也能走通。
func (c *compositeScriptFS) Stat(name string) (fs.FileInfo, error) {
	f, err := c.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

// rewardCeraFunc persists account-level cera through the audited, idempotent
// operator-grant path (admin_grants.grant_id is the idempotency gate).
func rewardCeraFunc(store *database.Store) reward.CeraFunc {
	return func(ctx context.Context, r reward.Recipient, key string, amount uint64) error {
		if amount == 0 || amount > math.MaxInt64 {
			return fmt.Errorf("reward cera amount out of range")
		}
		_, err := store.ApplyGrant(ctx, database.Grant{
			ID:        key,
			AccountID: r.AccountID,
			Cera:      int64(amount),
			Operator:  "reward",
			Reason:    "event reward",
		}, nil)
		return err
	}
}

// rewardGrantFunc commits the script's item grants through the existing
// character-event path, which makes the stable key replay-safe.
func rewardGrantFunc(store *database.Store, awarder *inventory.Awarder) reward.GrantFunc {
	return func(ctx context.Context, r reward.Recipient, key string, items []reward.ItemGrant) error {
		_, _, err := store.CommitCharacterEvent(ctx, r.AccountID, r.CharacterID, r.ConfigVersion, key, "reward-item-v1",
			func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				state := current.State
				receipt := make([]map[string]any, 0, len(items))
				for _, item := range items {
					updated, granted, grantErr := awarder.Grant(state, item.ID, item.Count)
					if grantErr != nil {
						return nil, nil, grantErr
					}
					state = updated
					receipt = append(receipt, map[string]any{"template": item.ID, "amount": item.Count, "slots": granted.Slots})
				}
				raw, marshalErr := json.Marshal(map[string]any{"items": receipt})
				if marshalErr != nil {
					return nil, nil, marshalErr
				}
				return state, raw, nil
			})
		return err
	}
}

// rewardMailFunc delivers a SYSTEM mail (sender_id NULL). Store.SendMail is
// sender-oriented and rejects self-send, so this inserts directly inside the
// same character-event transaction used by the Odyssey honor mail.
func rewardMailFunc(store *database.Store, awarder *inventory.Awarder) reward.MailFunc {
	return func(ctx context.Context, r reward.Recipient, key string, mail reward.MailReward) error {
		attachments, err := rewardMailAssets(awarder, mail)
		if err != nil {
			return err
		}
		subject := truncateUTF8(mail.Subject, 29)
		body := truncateUTF8(mail.Body, 512)
		_, _, err = store.CommitSystemMail(ctx, r.AccountID, r.CharacterID, r.ConfigVersion, key, "reward-mail-v1", subject, body, attachments)
		return err
	}
}

// rewardEntitlementFunc 把脚本收集到的"角色待遇"分派到各自的落库入口。
//
// 为什么要分派而不是一笔事务：这批待遇跨了五类存储 ——
//
//	① 角色 state：挂锁 / 扩容档位 / 复活币 / 宠物本体 / 宠物用品 → CommitCharacterEvent
//	② 个人金库容量：character_vaults / character_secondary_vaults → Store.GrantVaultSlots（只升不降，天然幂等）
//	③ 账号金库容量：account_vaults → CommitAccountVault（幂等表 account_vault_events）
//	④ 账号材料仓：account_material_storage → CommitAccountMaterialEvent（幂等表 character_events）
//	⑤ 账号皮肤仓库：account_skin_cargo → Store.UnlockSkins（ON CONFLICT DO NOTHING）
//
// ① 失败即整批返回错误（脚本作者能看见）；其余各项各自带**后缀键**，互不影响、也不会互相吞掉重放。
// 皮肤那一项还要靠运行期目录补全"全部可解锁模板"（skinUnlocks 为 nil 或空时视为没有目录，明确报错）。
//
// vaultRules 是**惰性**访问器：金库规则在奖励服务之后才装配（bootstrap 的 startup.VaultRules 分支），
// 而奖励只在建号时触发，那时早已装配完毕。个人金库必须用**金库规则自己的身份**
// （rules.SourceSHA256）与**政策初始档**去 Ensure —— 2026-10-06 实机踩到：用角色存档身份
// （Recipient.ConfigVersion）建出来的行，版本与金库入口的判据不符，之后每次入场都被
// workflow/vault.go 的 `v.ConfigVersion != s.Rules.SourceSHA256` 判成"需要迁移配置"，
// 玩家表现为**建完号进不去游戏**。
func rewardEntitlementFunc(store *database.Store, awarder *inventory.Awarder, skinUnlocks func() []database.AccountSkin,
	vaultRules func() *inventory.VaultRules) reward.EntitlementFunc {
	return func(ctx context.Context, r reward.Recipient, key string, e reward.Entitlements) error {
		if e.Empty() {
			return nil
		}
		if err := commitRewardStateEntitlements(ctx, store, awarder, r, key, e); err != nil {
			return err
		}
		// ② 个人金库两层：只升不降。
		//
		// 三件事必须同时正确，否则会把玩家挡在门外：
		//   - 行里的 config_version 必须是**金库规则的 SourceSHA256**（不是角色存档身份）；
		//   - 建行时的初始档要用政策里的 initial/initial_secondary（金库2 的初始档与金库1 不同）；
		//   - 目标档必须在政策 verified_slots 里（入口还有一道 Rules.Allows 判据）。
		// 建号时这两个行还不存在（唯一建档时机是登录），所以这里顺带把它们建出来 —— 顺带也是
		// 为什么参数必须全对：写错一次，这个号就再也进不去了。
		if e.Vault1Slots != nil || e.Vault2Slots != nil {
			rules := vaultRules()
			if rules == nil {
				return fmt.Errorf("金库容量：金库规则未装配（DFO_VAULT_RULES 没给），拒绝写入以免建出进不去的号")
			}
			for _, v := range []struct {
				slots     *uint16
				secondary bool
				label     string
			}{
				{e.Vault1Slots, false, "金库1"},
				{e.Vault2Slots, true, "金库2"},
			} {
				if v.slots == nil {
					continue
				}
				if !rules.Allows(*v.slots) {
					return fmt.Errorf("金库容量：%s 目标 %d 不是源政策里的合法档位（见 configs/pvf-vault-policy.json 的 verified_slots）", v.label, *v.slots)
				}
				initial := rules.InitialSlots
				if v.secondary && rules.InitialSecondarySlots != 0 {
					initial = rules.InitialSecondarySlots
				}
				if err := store.GrantVaultSlots(ctx, r.AccountID, r.CharacterID, rules.SourceSHA256, initial, *v.slots, v.secondary); err != nil {
					return fmt.Errorf("金库容量（%s）：%w", v.label, err)
				}
			}
		}
		// ③ 账号金库容量：走账号金库事务（它自带 EnsureAccountVault），只升不降。
		if e.AccountVaultSlots != nil {
			target := *e.AccountVaultSlots
			if target > 320 {
				target = 320
			}
			target -= target % 8 // 源规则按 8 格一档
			if _, _, _, _, err := store.CommitAccountVault(ctx, r.AccountID, r.CharacterID, r.ConfigVersion,
				key+":account-vault", rewardAccountVaultOperation,
				func(role database.Character, materials json.RawMessage, vault database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
					if target > vault.Slots {
						vault.Slots = target
					}
					return role.State, materials, vault, nil
				}); err != nil {
				return fmt.Errorf("账号金库容量：%w", err)
			}
		}
		// ④ 账号材料仓：按模板累加，落库时映射到固定槽位；不在 19 个可入库模板里的会被拒并点名。
		if len(e.AccountMaterialByTemplate) > 0 {
			if _, _, _, err := store.CommitAccountMaterialEvent(ctx, r.AccountID, r.CharacterID, r.ConfigVersion,
				key+":account-material", "reward-account-material-v1",
				func(role database.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
					materials, err := inventory.ReadAccountMaterials(raw)
					if err != nil {
						return nil, nil, err
					}
					for template, amount := range e.AccountMaterialByTemplate {
						updated, _, addErr := materials.Add(template, amount)
						if addErr != nil {
							return nil, nil, fmt.Errorf("模板 %d：%w", template, addErr)
						}
						materials = updated
					}
					out, err := materials.Save()
					if err != nil {
						return nil, nil, err
					}
					// 角色 state 原样返回：这笔事务只动账号材料仓。
					return role.State, out, nil
				}); err != nil {
				return fmt.Errorf("账号材料仓：%w", err)
			}
		}
		// ⑤ 皮肤仓库：账号级、ON CONFLICT DO NOTHING ⇒ 天然幂等。
		if e.UnlockSkins || len(e.SkinTemplates) > 0 {
			if skinUnlocks == nil {
				return fmt.Errorf("皮肤仓库：运行期没有皮肤目录（skins 域未准备）")
			}
			all := skinUnlocks()
			byTemplate := make(map[uint32]uint32, len(all))
			for _, s := range all {
				if s.SourceTemplate != 0 && s.SkinKey != 0 {
					byTemplate[s.SourceTemplate] = s.SkinKey
				}
			}
			batch := make([]database.AccountSkin, 0, len(byTemplate))
			if e.UnlockSkins {
				if len(byTemplate) == 0 {
					return fmt.Errorf("皮肤仓库：目录里没有任何可解锁条目")
				}
				for template, key := range byTemplate {
					batch = append(batch, database.AccountSkin{SourceTemplate: template, SkinKey: key})
				}
			} else {
				for _, template := range e.SkinTemplates {
					key, ok := byTemplate[template]
					if !ok {
						return fmt.Errorf("皮肤仓库：模板 %d 在运行期目录里取不到 skin key", template)
					}
					batch = append(batch, database.AccountSkin{SourceTemplate: template, SkinKey: key})
				}
			}
			if _, err := store.UnlockSkins(ctx, r.AccountID, batch); err != nil {
				return fmt.Errorf("皮肤仓库：%w", err)
			}
		}
		return nil
	}
}

// rewardAccountVaultOperation 是"奖励脚本扩容账号金库"这条路径自己的流水号。
//
// 既有流水号（305/306/307/308/19）没有枚举表，事务只校验"同一个键重放时操作号必须一致"
// （internal/database/account_vault.go 的 prior.Operation 判定），所以这里取一个未被占用的号，
// 并在注释里声明归属：它是**服务端发起**的扩容，没有对应的客户端 CMD。
const rewardAccountVaultOperation uint16 = 309

// commitRewardStateEntitlements 把"角色 state 那部分"待遇写进一次角色事件事务。
func commitRewardStateEntitlements(ctx context.Context, store *database.Store, awarder *inventory.Awarder,
	r reward.Recipient, key string, e reward.Entitlements) error {
	_, _, err := store.CommitCharacterEvent(ctx, r.AccountID, r.CharacterID, r.ConfigVersion, key, "reward-entitlement-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, receipt, applyErr := applyRewardEntitlements(current.State, e, awarder)
			if applyErr != nil {
				return nil, nil, applyErr
			}
			raw, marshalErr := json.Marshal(receipt)
			if marshalErr != nil {
				return nil, nil, marshalErr
			}
			return state, raw, nil
		})
	return err
}

// applyRewardEntitlements 把一批角色待遇写进角色 state（纯函数 + awarder，便于单测）。
//
// 口径（业主 2026-10-06）：**能不能改是服务端能力，改成多少由 mod 的脚本决定**。
// 所以这里只做三件事：按位或挂锁、扩容档位**只升不降**、复活币累加 ——
// 一律不会把玩家已有的待遇改小。宠物本体/宠物用品复用 Awarder 的门禁与落位实现。
func applyRewardEntitlements(state json.RawMessage, e reward.Entitlements, awarder *inventory.Awarder) (json.RawMessage, map[string]any, error) {
	if e.Empty() {
		return state, map[string]any{}, nil
	}
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, nil, err
	}
	receipt := map[string]any{}
	if e.EquipSlotMask != 0 {
		before := bag.ExpandEquipFlags
		// 复用挂锁的既有实现（按位或 + ReadBag/SaveBag 括起来），别在这里另写一份位运算。
		updated, unlockErr := inventory.UnlockEquipSlots(state, e.EquipSlotMask)
		if unlockErr != nil {
			return nil, nil, unlockErr
		}
		state = updated
		bag, err = inventory.ReadBag(state)
		if err != nil {
			return nil, nil, err
		}
		receipt["equip_slot_mask"] = map[string]any{"before": before, "after": bag.ExpandEquipFlags}
	}
	if e.BagTier != nil {
		tier := *e.BagTier
		if tier > maxRewardBagTier {
			// 上限来自源商城的两张扩展券（internal/cashshop.InventoryExpansionTier：1/2）。
			tier = maxRewardBagTier
		}
		if tier > bag.Expansion {
			before := bag.Expansion
			bag.Expansion = tier
			receipt["bag_expansion"] = map[string]any{"before": before, "after": tier}
		}
	}
	if e.AvatarTier != nil {
		tier := *e.AvatarTier
		if limit := byte(protocol.MaxAvatarInventoryExpansion); tier > limit {
			tier = limit
		}
		if tier > bag.AvatarExpansion {
			before := bag.AvatarExpansion
			bag.AvatarExpansion = tier
			receipt["avatar_expansion"] = map[string]any{"before": before, "after": tier}
		}
	}
	if e.ReviveCoins > 0 {
		before := bag.Coin
		// 与 Bag.Add 同一套溢出口径：到顶就停在 MaxUint32，不回绕成 0。
		if uint64(bag.Coin)+uint64(e.ReviveCoins) > math.MaxUint32 {
			bag.Coin = math.MaxUint32
		} else {
			bag.Coin += e.ReviveCoins
		}
		receipt["revive_coin"] = map[string]any{"before": before, "after": bag.Coin}
	}
	out, err := inventory.SaveBag(state, bag)
	if err != nil {
		return nil, nil, err
	}
	state = out
	// 宠物本体 / 宠物用品：复用 Awarder 的门禁（是不是 [creature]、是不是蛋、模板是不是宠物用品），
	// 逐个落到宠物容器（本体 0..139 / 用品 376..431），每次都是 ReadBag→改→SaveBag 的独立一步。
	if awarder == nil && (len(e.Pets) > 0 || len(e.PetItems) > 0) {
		return nil, nil, fmt.Errorf("宠物待遇需要奖励目录（awarder 为空）")
	}
	for _, pet := range e.Pets {
		updated, petReceipt, petErr := awarder.GrantPet(state, pet.ID, 1)
		if petErr != nil {
			return nil, nil, petErr
		}
		state = updated
		receipt[fmt.Sprintf("pet_%d", pet.ID)] = petReceipt.Slots
	}
	for _, item := range e.PetItems {
		updated, itemReceipt, itemErr := awarder.GrantPetItem(state, item.ID, item.Count)
		if itemErr != nil {
			return nil, nil, itemErr
		}
		state = updated
		receipt[fmt.Sprintf("pet_item_%d", item.ID)] = itemReceipt.Slots
	}
	return state, receipt, nil
}

// maxRewardBagTier 是背包扩容的**档位上限**（源商城只卖到 2 档；
// 客户端按 40+8*档位算容量，写超过它会给出客户端算不出来的格子数）。
const maxRewardBagTier byte = 2

// rewardMailAssets builds the mail attachments. A stackable becomes a mail
// stack, anything else (equipment, pet gear) is materialised through the
// equipment catalog's reward rule so the claim path can rebuild the instance.
func rewardMailAssets(awarder *inventory.Awarder, mail reward.MailReward) ([]database.MailAsset, error) {
	if len(mail.Attachments) == 0 {
		return nil, nil
	}
	assets := make([]database.MailAsset, 0, len(mail.Attachments))
	for i, item := range mail.Attachments {
		if item.ID == 0 || item.Count == 0 {
			return nil, fmt.Errorf("reward mail attachment %d is invalid", i+1)
		}
		attachment, err := rewardMailItem(awarder, item)
		if err != nil {
			return nil, err
		}
		if _, err := attachment.Row(); err != nil {
			return nil, err
		}
		raw, err := json.Marshal(attachment)
		if err != nil {
			return nil, err
		}
		assets = append(assets, database.MailAsset{Item: raw})
	}
	return assets, nil
}

func rewardMailItem(awarder *inventory.Awarder, item reward.ItemGrant) (inventory.MailItem, error) {
	if awarder == nil {
		return inventory.MailItem{}, fmt.Errorf("模板 %d：奖励目录未装配（awarder 为空）", item.ID)
	}
	// 先看目录怎么认这个模板。**必须把模板号写进报错**：
	// 2026-10-06 现场只得到一句 "equipment definition missing"，
	// 连是池子里哪个模板错了都不知道，排查全靠猜。规则脚本的池子通常是一串
	// 手抄的模板号，报错带上号才能一眼定位。
	//
	// [MOD-CAPABILITY-20261006] 这里刻意**不再**要求模板先出现在掉落目录里：
	// 堆叠物照旧按目录发；其余一律交给装备目录判——只要 PVF 里有这件装备的定义就能发。
	// 口径（业主 2026-10-06）：**能不能发是服务端能力，发什么由 mod 的脚本决定**；
	// 旧口径把"发什么"锁死在掉落表上，等于让内容侧改不动自己的补给清单。
	entry, known := awarder.Catalog.Items[item.ID]
	if known && entry.Kind == "stackable" {
		return inventory.MailItem{Stack: &inventory.BagItem{
			Template: item.ID, Amount: item.Count, ExpireTime: inventory.GrantExpireTime}}, nil
	}
	if awarder.Equipment == nil {
		return inventory.MailItem{}, fmt.Errorf("模板 %d：装备目录未装配（equipment source missing）", item.ID)
	}
	if item.Count != 1 {
		return inventory.MailItem{}, fmt.Errorf("模板 %d：装备附件数量必须是 1（现在是 %d）", item.ID, item.Count)
	}
	if !known {
		// 不在掉落目录里却能按装备发出来 —— 记一行，便于事后审计"这条规则到底发了什么、
		// 走的哪条路"（内容由 mod 决定，服务端只保证可观测）。
		log.Printf("reward mail: template %d is not in the drop catalog; materialising it as equipment from the PVF definition", item.ID)
	}
	durability, err := awarder.Equipment.Reward(item.ID)
	if err != nil {
		return inventory.MailItem{}, fmt.Errorf(
			"模板 %d 取不到奖励耐久（equipment definition missing）：%w"+
				"；该模板号在 PVF 装备表里不存在或不可发放，请核对模板号", item.ID, err)
	}
	return inventory.MailItem{Equipment: &inventory.BagEquipment{Template: item.ID, Durability: durability}}, nil
}

// truncateUTF8 limits s to at most limit octets without splitting a rune, so
// PostgreSQL text columns always receive valid UTF-8.
func truncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	b := []byte(s)[:limit]
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}
