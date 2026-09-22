package main

import "testing"

// VP 点 Apply（CMD29 带 variation 槽）之后不能再补 id19：id19 不带 VP 块，
// 客户端会用空 VP 状态覆盖刚渲染好的面板，表现为"Apply 后显示被重置"。
// 普通加点、以及被拒/幂等的请求仍然要补 id19。
func TestSkillTreeRefreshSuppressedForVariationApply(t *testing.T) {
	if skillTreeRefreshRequired(29, true, true) {
		t.Fatal("VP Apply 之后仍补 id19，会把刚下发的 VP 面板覆盖成空")
	}
	if !skillTreeRefreshRequired(29, true, false) {
		t.Fatal("普通加点（CMD29 不带 variation）被去掉 id19，palette 刷新会丢")
	}
	if !skillTreeRefreshRequired(29, false, false) {
		t.Fatal("被拒/幂等请求必须补 id19，把客户端拉回存档状态")
	}
	if !skillTreeRefreshRequired(28, true, false) {
		t.Fatal("CMD28 槽位移动必须补 id19")
	}
}
