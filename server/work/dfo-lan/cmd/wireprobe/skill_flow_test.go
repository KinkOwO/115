package main

import (
	"bytes"
	"dfolan/internal/storage"
	"testing"
)

// VP 点 Apply（CMD29 带 variation 槽）之后不能再补 id19：id19 不带 VP 块，
// 客户端会用空 VP 状态覆盖刚渲染好的面板，表现为"Apply 后显示被重置"。
// 普通加点、以及被拒/幂等的 CMD29 请求仍然要补 id19。
func TestSkillTreeRefreshPlan(t *testing.T) {
	if skillTreeRefreshRequired(29, true, true) {
		t.Fatal("VP Apply 之后仍补 id19，会把刚下发的 VP 面板覆盖成空")
	}
	if !skillTreeRefreshRequired(29, true, false) {
		t.Fatal("普通加点（CMD29 不带 variation）被去掉 id19，palette 刷新会丢")
	}
	if !skillTreeRefreshRequired(29, false, false) {
		t.Fatal("被拒/幂等请求必须补 id19，把客户端拉回存档状态")
	}
	for _, applied := range []bool{true, false} {
		if skillTreeRefreshRequired(28, applied, false) {
			t.Fatalf("CMD28 applied=%t 不能追加 id19", applied)
		}
		body := []byte{1, 0, 1, 5}
		plan, err := skillMutationResponsePlan(nil, storage.Character{}, 28, body, applied, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan) != 1 || plan[0].ID != 28 || plan[0].Kind != 1 || plan[0].Name != "skill_committed_response" || !bytes.Equal(plan[0].Payload, body) {
			t.Fatalf("CMD28 applied=%t 响应计划应只有原始 ACK: %+v", applied, plan)
		}
	}
}
