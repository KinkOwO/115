package main

import (
	"context"
	"dfolan/internal/database"
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
func buildRewardService(store *database.Store, awarder *inventory.Awarder) *reward.Service {
	if store == nil || awarder == nil {
		return nil
	}
	scripts := rewardScriptFS()
	service, err := reward.New(reward.Options{
		Scripts: scripts,
		Grant:   rewardGrantFunc(store, awarder),
		Mail:    rewardMailFunc(store, awarder),
		Cera:    rewardCeraFunc(store),
		Log:     func(format string, args ...any) { log.Printf(format, args...) },
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
	entry, known := awarder.Catalog.Items[item.ID]
	if !known {
		return inventory.MailItem{}, fmt.Errorf(
			"模板 %d 不在奖励目录里（reward mail attachment unknown template）", item.ID)
	}
	if entry.Kind == "stackable" {
		return inventory.MailItem{Stack: &inventory.BagItem{
			Template: item.ID, Amount: item.Count, ExpireTime: inventory.GrantExpireTime}}, nil
	}
	if awarder.Equipment == nil {
		return inventory.MailItem{}, fmt.Errorf("模板 %d：装备目录未装配（equipment source missing）", item.ID)
	}
	if item.Count != 1 {
		return inventory.MailItem{}, fmt.Errorf("模板 %d：装备附件数量必须是 1（现在是 %d）", item.ID, item.Count)
	}
	durability, err := awarder.Equipment.Reward(item.ID)
	if err != nil {
		return inventory.MailItem{}, fmt.Errorf(
			"模板 %d 取不到奖励耐久（equipment definition missing）：%w"+
				"；该模板号在奖励目录里不是可发放的装备，请换成 PVF 里确实存在的装备", item.ID, err)
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
