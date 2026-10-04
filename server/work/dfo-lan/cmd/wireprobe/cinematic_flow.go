package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"time"
)

func cinematicRestore(raw json.RawMessage) ([]byte, error) {
	var state struct {
		Scenes []uint16 `json:"cinematic_skipped"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return protocol.CinematicSkippedScenes(state.Scenes)
}

// storyDigestRestore 由存档的 story_digest_level 生成 NOTI1370 载荷。
//
// 载荷形状：裸小端 u32，恰好 4 字节就是整个 body——没有长度前缀、没有计数、
// 没有任何包装。缺键 / 0 也恒发 4 个 0 字节：preparePackets 会丢弃零长载荷帧，
// 缺键返回空数组 ⇒ 帧静默消失 ⇒ 开场回顾影片原样重播（且日志无痕迹）。
func storyDigestRestore(raw json.RawMessage) ([]byte, error) {
	var state struct {
		StoryDigestLevel uint32 `json:"story_digest_level"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	buf := make([]byte, 4) // 恒 4 字节，绝不为空
	buf[0] = byte(state.StoryDigestLevel)
	buf[1] = byte(state.StoryDigestLevel >> 8)
	buf[2] = byte(state.StoryDigestLevel >> 16)
	buf[3] = byte(state.StoryDigestLevel >> 24)
	return buf, nil
}

// advanceStoryDigest 是 CMD1438 写库规则的纯函数：读存档里的 story_digest_level，
// 仅当 level 更大时推进并返回新状态。缺键视作 0。
func advanceStoryDigest(current json.RawMessage, level uint32) (json.RawMessage, bool, error) {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(current, &fields); e != nil {
		return nil, false, e
	}
	var cur uint32
	if value := fields["story_digest_level"]; value != nil {
		if e := json.Unmarshal(value, &cur); e != nil {
			return nil, false, e
		}
	}
	if level <= cur {
		// 只向前推进：低于/等于存档值时不写，防止读旧档/回档把进度往回拉。
		return nil, false, nil
	}
	b, _ := json.Marshal(level)
	fields["story_digest_level"] = b
	raw, e := json.Marshal(fields)
	return raw, true, e
}

// saveStoryDigest 处理 CMD1438：客户端播完开场回顾影片后上报（空载荷），
// 服务端把"收到帧时的人物当前等级"记为已看进度。
//
// 只向前推进：仅当 level > 存档值才写回，防止读旧档 / 回档把进度往回拉，
// 已看过的影片被重新"上膛"又重播。幂等键按等级区分，同级重复上报不重复写库；
// 升级后再播会用新等级键继续推进。
func saveStoryDigest(store *database.Store, w *worldSession, p []byte) error {
	if w == nil || w.role.ID == 0 {
		return fmt.Errorf("story digest requires character")
	}
	level := uint32(w.level) // CMD1438 无载荷，播的是哪段由服务端按当前等级判断
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		fmt.Sprintf("story-digest:%d", level), "story-digest-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, advanced, e := advanceStoryDigest(current.State, level)
			if e != nil {
				return nil, nil, e
			}
			if !advanced {
				// 未推进：返回当前状态原样（不制造无意义写入）。
				return current.State, json.RawMessage(`{"advanced":false}`), nil
			}
			return state, json.RawMessage(fmt.Sprintf(`{"advanced":true,"level":%d}`, level)), nil
		})
	if err != nil {
		return err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return nil
}

func cinematicSkip(store *database.Store, w *worldSession, p []byte) error {
	if w == nil || w.role.ID == 0 {
		return fmt.Errorf("cinematic requires character")
	}
	id, err := protocol.DecodeCinematicSkip(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("cinematic-skip:%d", id), "cinematic-skip-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(current.State, &fields); e != nil {
			return nil, nil, e
		}
		var ids []uint16
		if value := fields["cinematic_skipped"]; value != nil {
			if e := json.Unmarshal(value, &ids); e != nil {
				return nil, nil, e
			}
		}
		found := false
		for _, v := range ids {
			found = found || v == id
		}
		if !found {
			ids = append(ids, id)
		}
		if _, e := protocol.CinematicSkippedScenes(ids); e != nil {
			return nil, nil, e
		}
		fields["cinematic_skipped"], _ = json.Marshal(ids)
		raw, e := json.Marshal(fields)
		receipt, _ := json.Marshal(id)
		return raw, receipt, e
	})
	if err == nil {
		saved.WireID = w.role.WireID
		w.role = saved
	}
	return err
}
