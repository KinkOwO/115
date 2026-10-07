package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestEventRequest115LiveWidths 钉住两种实机/夹具正文宽度：
// 662 的 680/681 是两包（事件号 + 关卡号），665 是 A 类三包。
// 8 字节那帧来自实机 2026-10-04 18:04:04（第一关对话后点「Get」）。
func TestEventRequest115LiveWidths(t *testing.T) {
	p, e := hex.DecodeString("9602000001000000")
	if e != nil || len(p) != 8 {
		t.Fatal(len(p), e)
	}
	r, e := DecodeEventRequest115(p, true)
	if e != nil || r.Event != 662 || r.Parameter != 1 || !r.HasParameter {
		t.Fatal(r, e)
	}
	if _, e := DecodeEventRequest115(p[:4], true); e == nil {
		t.Fatal("one-word body accepted")
	}
	wide := make([]byte, 12)
	live := append([]byte{}, p...)
	wide[0] = 0x99
	wide[1] = 0x02 // 事件号 665
	if q, e := DecodeEventRequest115(wide, false); e != nil || q.Event != 665 || q.Parameter != 0 || !q.HasParameter {
		t.Fatal(q, e)
	}
	// 三包正文仍按第三包取参数，两包口径不得回吞 665。
	three := append(live, 0, 0, 0, 0)
	three[8] = 7
	if q, e := DecodeEventRequest115(three, true); e != nil || q.Parameter != 7 {
		t.Fatal(q, e)
	}
}

// TestBoostTrainingStatusGraduatedFrame 钉住官服的**毕业态** 2638：
// `02 0c 00 00 01`（mode=2 / step=12 / phase=0 / active=1）。
//
// 真源：参考包 `活动Boost与胶囊教学-20260927.zip` 的
// `规格文档/2638-BOOSTUPCHARACTER.md`「已推翻」一节 ——
// 「第12步不是第12张训练副本：该源只有 11 个 step info，官服在最后领取后转为 02/0c/00/00/01」。
// 与本仓 `boostup.next()` 越过最后一关后的 `Step=12 / Phase=0 / Finished=true` 正好吻合。
//
// 原实现有一条 `mode==2 && active` 的守卫会拒发这条帧（零测试覆盖），
// 于是第 11 关领奖整笔被拒 —— 实机 2026-10-06 03:39:44
// `boost_event_request_refused: finished boost training cannot be active`。
// 这条测试把官服口径钉住，防止守卫被"修"回来。
func TestBoostTrainingStatusGraduatedFrame(t *testing.T) {
	got, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 2, Step: 12, Phase: 0, Active: true})
	if e != nil {
		t.Fatalf("毕业态被拒：%v", e)
	}
	if want := []byte{0x02, 0x0c, 0x00, 0x00, 0x01}; !bytes.Equal(got, want) {
		t.Fatalf("毕业帧 = % x, want % x", got, want)
	}
	// 保留的不变量：未激活不得携带进度。
	if _, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 2, Step: 12, Active: false}); e == nil {
		t.Fatal("未激活却携带进度被放过")
	}
	// 训练中（mode=0）不受影响。
	if got, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 0, Step: 11, Phase: 1, Active: true}); e != nil || !bytes.Equal(got, []byte{0x00, 0x0b, 0x01, 0x00, 0x01}) {
		t.Fatalf("训练中帧 = % x err=%v", got, e)
	}
}
