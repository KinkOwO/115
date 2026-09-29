package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func (w *worldSession) prepareAdventure(ctx context.Context) (storage.AccountAdventure, error) {
	if w == nil || w.characters == nil || w.characters.Store == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return storage.AccountAdventure{}, fmt.Errorf("冒险团请求缺少所属角色")
	}
	if w.fatigue == nil {
		return storage.AccountAdventure{}, fmt.Errorf("冒险团游戏日历尚未加载")
	}
	return w.characters.Store.PrepareAdventure(ctx, w.role, w.fatigue.Day(time.Now()))
}

// 首次建立团资料时用首个角色的名字；编码长度按客户端 16 字节上限截取，
// 以后从账号档案读取，不随当前角色变化。不能泄露登录账号作为公开团名。
func adventureDefaultName(name string) string {
	out := ""
	for _, r := range name {
		next := out + string(r)
		b, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(next))
		if err != nil || len(b) > 16 {
			break
		}
		out = next
	}
	if out == "" {
		return "冒险团"
	}
	return out
}

func (w *worldSession) handleAdventure(ctx context.Context, selected int64, p []byte) ([]byte, error) {
	if w == nil || w.characters == nil || w.characters.Store == nil || selected == 0 || w.role.ID != selected || w.role.AccountID != w.account {
		return nil, fmt.Errorf("冒险团查询缺少当前账号所属的已选角色")
	}
	req, err := protocol.DecodeAdventureRequest(p)
	if err != nil {
		return nil, err
	}
	if req.Target != w.role.WireID {
		return nil, fmt.Errorf("冒险团查询目标不是当前角色")
	}
	info, err := w.adventureInfo(ctx, selected)
	if err != nil {
		return nil, err
	}
	identity := w.characters.ChannelContext
	return protocol.AdventureResponse(req, identity[0], uint32(identity[1]), info)
}

func (w *worldSession) adventureInfo(ctx context.Context, selected int64) (protocol.AdventureInfo, error) {
	var info protocol.AdventureInfo
	if w == nil || w.characters == nil || w.characters.Store == nil || selected == 0 || w.role.ID != selected || w.role.AccountID != w.account {
		return info, fmt.Errorf("冒险团资料缺少当前账号所属的已选角色")
	}
	roles, err := w.characters.Store.Characters(ctx, w.account)
	if err != nil {
		return info, err
	}
	if len(roles) == 0 {
		return info, fmt.Errorf("冒险团账号没有可用角色")
	}
	if len(roles) > math.MaxUint16 {
		return info, fmt.Errorf("冒险团角色总数超出客户端范围")
	}
	// 固定首个创建的角色，不受角色选择页拖拽排序影响。
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	profile, err := w.characters.Store.LoadAdventure(ctx, w.account, selected, adventureDefaultName(roles[0].Name))
	if err != nil {
		return info, err
	}
	profile, err = w.prepareAdventure(ctx)
	if err != nil {
		return info, err
	}
	identity := w.characters.ChannelContext
	info = protocol.AdventureInfo{Name: profile.Name, Level: profile.Level, Experience: profile.Experience,
		CharacterCount: uint16(len(roles)), ConnectionDays: profile.Data.ConnectionDays,
		RecommendedDungeonClears: profile.Data.RecommendedDungeonClears,
		Points:                   profile.Data.Points, Purchases: profile.Data.Purchases,
		CreatedDate: uint32(profile.CreatedAt.Year()*10000 + int(profile.CreatedAt.Month())*100 + profile.CreatedAt.Day())}
	if profile.Data.PointExperience > math.MaxInt32 {
		return info, fmt.Errorf("冒险团兑换进度超出客户端范围")
	}
	info.PointExperience[0] = uint32(profile.Data.PointExperience)
	representatives := map[[2]byte]protocol.AdventureCharacter{}
	for _, role := range roles {
		var state character.State
		if err := json.Unmarshal(role.State, &state); err != nil {
			return info, fmt.Errorf("读取冒险团角色 %d：%w", role.ID, err)
		}
		if state.Advancement == 0 && role.Profession != 9 && role.Profession != 10 {
			continue // 原生职业代表以职业/转职为键，未转职角色不占用转职收藏格。
		}
		if role.ID < 1 || role.ID > math.MaxUint32 {
			return info, fmt.Errorf("冒险团角色编号超出客户端范围")
		}
		advance := state.Advancement
		if role.Profession == 9 || role.Profession == 10 {
			advance = 1 // 0x14135257F：黑暗武士、缔造者的收藏键固定为 1。
		}
		row := protocol.AdventureCharacter{Profession: role.Profession, Advancement: advance,
			Level: uint32(state.Level), CharacterID: uint32(role.ID), Name: role.Name,
			Server: identity[0], Awakening: uint16(state.Awakening)}
		if row.CharacterID == profile.Data.BestHonorCharacter {
			// 每次重投影当前角色状态，升级后不保留旧等级；软删除角色不在roles中。
			chosen := row
			info.BestHonor = &chosen
		}
		key := [2]byte{row.Profession, row.Advancement}
		old, ok := representatives[key]
		// 一个职业格只容纳一个代表，优先保留觉醒阶段、等级最高的角色。
		if !ok || row.Awakening > old.Awakening || row.Awakening == old.Awakening && row.Level > old.Level {
			representatives[key] = row
		}
	}
	for _, row := range representatives {
		info.Characters = append(info.Characters, row)
	}
	sort.Slice(info.Characters, func(i, j int) bool {
		a, b := info.Characters[i], info.Characters[j]
		return a.Profession < b.Profession || a.Profession == b.Profession && a.Advancement < b.Advancement
	})
	if info.BestHonor == nil {
		// 未手动指定、恢复自动选择或所选角色已删除时，以实际职业代表的显示等级选择。
		// 不把新角色创建、切换当前角色当成一次手动设置；同级按稳定角色编号打破平局。
		for _, row := range info.Characters {
			if info.BestHonor == nil || row.Level > info.BestHonor.Level || row.Level == info.BestHonor.Level && row.CharacterID < info.BestHonor.CharacterID {
				chosen := row
				info.BestHonor = &chosen
			}
		}
	}
	return info, nil
}

func (w *worldSession) setAdventureBestHonor(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	wanted, automatic, err := protocol.DecodeAdventureBestHonor(p)
	if err != nil {
		return nil, err
	}
	info, err := w.adventureInfo(ctx, w.role.ID)
	if err != nil {
		return nil, err
	}
	var selected uint32
	if !automatic {
		// 请求不含数据库角色编号；只允许选择刚才信息页实际提供的本账号职业代表。
		// 等级和觉醒从存档校验，不信任客户端传来的数值，也不用于提升角色属性。
		for _, row := range info.Characters {
			if row.Profession == wanted.Profession && row.Advancement == wanted.Advancement && row.Awakening == wanted.Awakening && row.Level == wanted.Level {
				selected = row.CharacterID
				break
			}
		}
		if selected == 0 {
			return nil, fmt.Errorf("所选冒险团主力角色不存在或资料已变化，请重新打开设置")
		}
	}
	key := fmt.Sprintf("adventure-best-honor:%s:%x", prefix, sha256.Sum256(raw))
	_, _, _, err = w.characters.Store.CommitAdventure(ctx, w.account, w.role.ID, key,
		func(role storage.Character, profile *storage.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			profile.Data.BestHonorCharacter = selected
			receipt, e := json.Marshal(map[string]any{"character_id": selected, "automatic": automatic})
			return role.State, receipt, e
		})
	if err != nil {
		return nil, err
	}
	detail, err := w.adventureDetail(ctx)
	if err != nil {
		return nil, err
	}
	// 14111DCA0成功回执只调用14394D560重绘，必须先用NOTI1331更新资料对象。
	return []outboundPacket{{"冒险团主力角色同步", 0, 1331, detail}, {"冒险团主力角色保存", 1, 2331, []byte{1}}}, nil
}
