package character

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
)

// ApplyWeaponSkin writes the skin the player applied in the skin storage window
// into the character save (inventory.weapon_skin).
//
// The trigger is CMD1565: the client's Apply handler (0x1441d7b70) hard-codes
// the weapon-shape subtype 4 and re-uses the very frame the tab refresh sends,
// so the request body is a container sync, not a dedicated "apply" command. The
// skin id it reports is the replicated weapon's own template (live 2026-09-26:
// 101010912), and the server resolves no model mapping of its own - the id is
// projected straight onto the weapon slot of the equipped-appearance block and
// the client looks the model up.
//
// skin == 0 is the window's "解除" (unapply): the frame arrives with an empty id
// list, and clearing the stored skin is what restores the worn weapon's own
// model, because the appearance projection falls back to the equipped template
// whenever WeaponSkin is zero. Refusing zero made the button do nothing (live
// 2026-09-27).
//
// The write lands in one CommitCharacterEvent transaction keyed by the request,
// so a retry replays the receipt instead of racing a second write. The returned
// bool reports whether the stored skin actually changed: false means either a
// replay or a sync that named the skin already on file, and the caller must then
// skip the appearance refresh - the client re-sends this frame on every browse,
// and a mode0 userinfo per click would rebuild the actor needlessly. Every real
// change also bumps inventory.weapon_skin_seq, which is what keeps the caller's
// idempotency key unique across an A -> B -> A sequence of applies.
//
// A non-zero skin also has to pass the job gate (inventory.WeaponSkinUsable):
// the storage can hold entries from before the replication check existed - live
// 2026-09-27, a berserker with a beamsword - and refusing them here is the real
// enforcement point, since no player data gets deleted. A refusal returns the
// closure's error, so nothing is written and nothing is recorded. Zero (unapply)
// never hits the gate: a character must always be able to get out of a skin.
func (s *Service) ApplyWeaponSkin(ctx context.Context, role storage.Character, key string, skin uint32) (storage.Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, fmt.Errorf("weapon skin without a store")
	}
	if key == "" {
		return role, false, fmt.Errorf("weapon skin without an event key")
	}
	changed := false
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "weapon-skin-apply-v1",
		func(cur storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, err := inventory.ReadBag(cur.State)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(map[string]any{"skin": skin, "previous": b.WeaponSkin})
			if err != nil {
				return nil, nil, err
			}
			if b.WeaponSkin == skin {
				return cur.State, receipt, nil
			}
			if skin != 0 {
				if err := s.weaponSkinUsable(cur, skin); err != nil {
					return nil, nil, err
				}
			}
			b.WeaponSkin = skin
			b.WeaponSkinSeq++
			next, err := inventory.SaveBag(cur.State, b)
			if err != nil {
				return nil, nil, err
			}
			changed = true
			return next, receipt, nil
		})
	if e != nil {
		return role, false, e
	}
	saved.WireID = role.WireID
	return saved, applied && changed, nil
}

// weaponSkinUsable 在事务里按存档现状核对这件外观本角色能不能戴。
//
// 职业与转职必须从 cur 读，role 是入场时的旧快照，佩戴可能排在一次转职之后——与
// ReplicateWeaponSkin 同一条理由。目录/职业查不到就跳过（返回 nil），交给
// inventory.WeaponSkinUsable 去兜：缺目录不能让本来正常的佩戴全部失败。
func (s *Service) weaponSkinUsable(cur storage.Character, skin uint32) error {
	if s.Equipment == nil {
		return nil
	}
	prof, ok := s.Catalog.Professions[cur.Profession]
	if !ok {
		return nil
	}
	var state struct {
		Advancement byte `json:"advancement"`
		Awakening   byte `json:"awakening"`
	}
	if err := json.Unmarshal(cur.State, &state); err != nil {
		return err
	}
	return s.Equipment.WeaponSkinUsable(skin, inventory.ReplicationActor{
		Job:         prof.Job,
		Advancement: state.Advancement,
		CanUseSkill: s.canLearnSkill(cur.Profession, int(state.Advancement), int(state.Awakening)),
	})
}

// WeaponSkinUsableFor 把佩戴那一道门交成单 id 谓词，供幻化仓库的两帧列表在一处决定
// 「这一行还发不发」。
//
// 必须筛的是服务端：客户端的页 4 面板 `sub_1441E47B0` 对每个条目只做一次参数恒为
// 0x9C40 的注册表检查（`analysis/dumps/skin-noti/df22_ui_1441E47B0.c:38-55`），既不看
// 条目自身的 id、也不看职业 ⇒ 服务端发什么它就画什么。修好复制校验之前存下的条目
// （实机 2026-09-27 狂战士仓库里躺着光剑）因此一直显示到玩家眼前。
//
// 只有谓词明确判「本职业用不上」（ErrWeaponSkinNotUsable）才返回 false：没有装备目录、
// 职业查不到、存档读不出转职，都算「无从判断」而放行。缺目录不能让本来正常的仓库变成
// 空的，而且**正在佩戴的那一条绝不能从页里消失**——消失后客户端就选不中它，玩家再也
// 点不到「解除」，会被卡在一件本职业戴不上的外观里出不来（调用方自己保留那一条）。
func (s *Service) WeaponSkinUsableFor(cur storage.Character) func(uint32) bool {
	if s == nil || s.Equipment == nil {
		return nil
	}
	return func(skin uint32) bool {
		return !errors.Is(s.weaponSkinUsable(cur, skin), inventory.ErrWeaponSkinNotUsable)
	}
}

// ReplicateWeaponSkin settles one replication (CMD1592): the weapon in bag slot
// and one Linus mold are consumed, and the weapon's own template is recorded as a
// skin id in the skin storage window.
//
// The request carries no item and no materials - just the bag slot the window
// addressed - so everything spent is resolved here from the save, and the whole
// settlement lands in one CommitCharacterEvent transaction. A client retry of the
// same request replays the receipt, so the weapon cannot be eaten twice.
//
// mode is the window's material selection (inventory.MoldForMode): the frame carries
// no item id at all, so the mold the player picked only exists as this field.
//
// The weapon has to be usable by this character: the client's own confirmation only
// refuses non-weapons and equipped items, so a cross-class weapon replicated
// cleanly and then sat unusable in the storage (live 2026-09-27). Job and
// advancement come from the catalog and the save; the same rule decides whether the
// piece can be worn (inventory.UsableByJob), and the source's own [required job
// skill] then narrows it to the subclasses that can learn that weapon's mastery
// (inventory.RequiredJobSkill + canLearnSkill - a berserker passes [usable job] for
// a beamsword because its job text is [swordman] too, but it can never learn skill
// 33 at advancement 3).
//
// A refusal (slot empty, not a weapon, not usable by this class, no mold) returns an
// error and the closure never applies, which is what keeps the event row out of the
// table: there is nothing to replay, and buying a mold and confirming again must work.
//
// The client's skin cargo container is session state - NOTI1545 only rides along
// with this confirmation, and a reconnect starts from an empty container - which
// is why the list is persisted here and re-pushed at entry.
func (s *Service) ReplicateWeaponSkin(ctx context.Context, role storage.Character, key string, slot uint16, mode uint32) (storage.Character, inventory.WeaponReplication, bool, error) {
	if s == nil || s.Store == nil {
		return role, inventory.WeaponReplication{}, false, fmt.Errorf("weapon replication without a store")
	}
	if key == "" {
		return role, inventory.WeaponReplication{}, false, fmt.Errorf("weapon replication without an event key")
	}
	var spent inventory.WeaponReplication
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "weapon-skin-replicate-v1",
		func(cur storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, err := inventory.ReadBag(cur.State)
			if err != nil {
				return nil, nil, err
			}
			// 职业与转职必须在同一个事务里从 cur 读：role 是入场时的旧快照，
			// 而幻化可能排在一次转职之后。
			prof, ok := s.Catalog.Professions[cur.Profession]
			if !ok {
				return nil, nil, fmt.Errorf("weapon replication profession unavailable")
			}
			var state struct {
				Advancement byte `json:"advancement"`
				Awakening   byte `json:"awakening"`
			}
			if err := json.Unmarshal(cur.State, &state); err != nil {
				return nil, nil, err
			}
			next, cost, err := b.ReplicateWeaponSkin(slot, s.Equipment, inventory.MoldForMode(mode),
				inventory.ReplicationActor{
					Job:         prof.Job,
					Advancement: state.Advancement,
					CanUseSkill: s.canLearnSkill(cur.Profession, int(state.Advancement), int(state.Awakening)),
				})
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(cost)
			if err != nil {
				return nil, nil, err
			}
			spent = cost
			if cost.Duplicate {
				// 已经登记过同一件外观：一个模具都不扣，状态原样返回。客户端自己的
				// 重复提示（100007186）走的就是这条；这里只兜住重放。
				return cur.State, receipt, nil
			}
			out, err := inventory.SaveBag(cur.State, next)
			if err != nil {
				return nil, nil, err
			}
			return out, receipt, nil
		})
	if e != nil {
		return role, inventory.WeaponReplication{}, false, e
	}
	saved.WireID = role.WireID
	return saved, spent, applied && !spent.Duplicate, nil
}

// canLearnSkill answers "does this character's job/advancement own this job skill?"
// for the weapon source's [required job skill] gate. Ownership is the same test the
// learning path already uses (learning.go: ForAdvancement or ForAwakening), so a
// subclass that merely has not spent the points yet still passes - the question is
// whether the skill is theirs at all, not whether it is ranked.
//
// A skill missing from the catalog, or an unavailable learning catalog, returns true:
// the gate only narrows what the source itself already narrows, and an unmodellable
// row must not start refusing replications the client considers fine.
func (s *Service) canLearnSkill(profession byte, advancement, awakening int) func(uint16) bool {
	if s == nil || s.Learning == nil {
		return nil
	}
	return func(skill uint16) bool {
		d, ok := s.Learning.index[profession][skill]
		if !ok {
			return true
		}
		return d.ForAdvancement(advancement) || d.ForAwakening(advancement, awakening)
	}
}
