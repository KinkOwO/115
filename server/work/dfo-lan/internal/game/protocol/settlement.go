package protocol

import (
	"encoding/binary"
	"fmt"
)

type PlayResultRequest struct {
	Actor     uint16
	RankPoint byte
}

func DecodePlayResult(p []byte) (PlayResultRequest, error) {
	var r PlayResultRequest
	// Current solo native writer emits113B. Send flush appends the folded
	// MD5, then cipher padding rounds117B to128B.
	if len(p) != 113 && len(p) != 128 {
		return r, fmt.Errorf("unsupported current play-result size")
	}
	if p[0] != 0 || p[3] != 1 {
		return r, fmt.Errorf("play-result requires ordinary solo row")
	}
	if e := digestRequestTail(p, 113, 16); e != nil {
		return r, e
	}
	r.Actor = binary.LittleEndian.Uint16(p[4:])
	r.RankPoint = p[10]
	if r.Actor == 0 || r.Actor == 65535 || r.RankPoint != p[75] {
		return r, fmt.Errorf("invalid solo result identity or rank")
	}
	return r, nil
}

func PlayResultNotice(actor uint16, grade, rank byte, elapsed uint32, allClear bool) ([]byte, error) {
	return PlayResultNoticeRecord(actor, grade, rank, elapsed, elapsed, allClear, true)
}

func PlayResultNoticeRecord(actor uint16, grade, rank byte, elapsed, best uint32, allClear, improved bool) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid result actor")
	}
	p := add32([]byte{grade}, elapsed)
	flag := byte(0)
	if allClear {
		flag = 1
	}
	p = append(p, 0, rank, flag, 1)
	// Native1452b1caf..1cdc uses the first time and its result flag for the
	// personal-record announcement. -1 suppresses a record-update notice.
	flagRecord := uint32(0xffffffff)
	if improved {
		flagRecord = 0
	}
	p = add32(add16(p, actor), best)
	p = add32(p, flagRecord)
	p = add32(p, 0) // no party record
	p = add32(p, 0)
	return add32(p, 0xffffffff), nil
}

type CardReward struct {
	Template, Amount uint32
	Metadata         [21]byte
}
type ClearRewardState struct {
	BaseExperience, ScoreExperience, MonsterExperience uint32
	Cards                                              [8][]CardReward
}

func ClearReward(p ClearRewardState) ([]byte, error) {
	// [MERGE-20261001-ZERO-CLEAR-REWARD] 通关经验为 0 是源数据允许的取值，不是「未结算」：
	// 奥德赛「大陆漂移」组（100004984..989）的 [experience increasing point] 就写着 0，
	// 该图的通关经验与怪物经验都是 0（实机 2026-10-01 06:31 会话：进图与清完最后一格后
	// 角色经验快照完全一致）。原来把 BaseExperience==0 当无效，CMD46 被拒
	// （"invalid committed clear reward"），结算批次 34/37/35/261 一条都发不出，
	// 客户端本地播完「前往天界」传送动画后仍停在副本里，只能靠「撤退」离开。
	// 真正非法的只有总和溢出。
	if uint64(p.BaseExperience)+uint64(p.ScoreExperience) > 0xffffffff {
		return nil, fmt.Errorf("invalid committed clear reward")
	}
	// Current NOTI35 full empty-group path is281B. Kept separate from the
	// reference90 210B grammar. This encoder covers EXP and empty card groups.
	out := make([]byte, 281)
	binary.LittleEndian.PutUint32(out[0:], p.BaseExperience)
	binary.LittleEndian.PutUint32(out[4:], p.ScoreExperience)
	out[167] = 1 // current result-window flag read at1452a7b73
	binary.LittleEndian.PutUint32(out[169:], p.MonsterExperience)
	// Native tail uses -1 for the absence of two optional event results.
	binary.LittleEndian.PutUint32(out[265:], 0xffffffff)
	binary.LittleEndian.PutUint32(out[277:], 0xffffffff)
	// First card group at1452a74c8: eight counts, each followed by
	// (template32, amount32, metadata21). 90's two-u32 row is too short.
	group := make([]byte, 0, 8)
	for _, rows := range p.Cards {
		if len(rows) > 127 {
			return nil, fmt.Errorf("too many card rewards")
		}
		group = append(group, byte(len(rows)))
		for _, row := range rows {
			if row.Amount == 0 {
				return nil, fmt.Errorf("empty card reward")
			}
			group = add32(add32(group, row.Template), row.Amount)
			group = append(group, row.Metadata[:]...)
		}
	}
	return append(append(out[:131:131], group...), out[139:]...), nil
}
