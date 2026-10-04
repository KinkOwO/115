package main

import (
	"errors"
	"testing"
)

// [AZURE-REVIVE-COUNT] N2621 的 [32:36] 是**剩余复活次数**（上限 8），用币复活要递减。
//
// 私服此前写死 8，所以复活后客户端看到的次数不动（业主实机 2026-10-04）。
func TestAzureCoinReviveDecrementsCount(t *testing.T) {
	w := &worldSession{channelType: azureMainChannelType}
	w.azure.revivesLeft = azureMainReviveLimit

	plan, err := w.afterAzureCoinRevive([]outboundPacket{{"life_token_revive_ack", 1, 41, []byte{1}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("用币复活成功应追加一帧 NOTI2621，packets = %d", len(plan))
	}
	if w.azure.revivesLeft != azureMainReviveLimit-1 {
		t.Fatalf("剩余次数 = %d, want %d", w.azure.revivesLeft, azureMainReviveLimit-1)
	}
	last := plan[len(plan)-1]
	if last.Kind != 0 || last.ID != 2621 {
		t.Fatalf("追加的应是 NOTI2621，得到 kind=%d id=%d", last.Kind, last.ID)
	}
	body := last.Payload
	got := int(body[32]) | int(body[33])<<8 | int(body[34])<<16 | int(body[35])<<24
	if got != azureMainReviveLimit-1 {
		t.Fatalf("NOTI2621 [32:36] = %d, want %d（客户端按它刷新计数）", got, azureMainReviveLimit-1)
	}

	// 扣到 0 之后不该再刷（也不该变成负数）。
	w.azure.revivesLeft = 0
	plan, err = w.afterAzureCoinRevive([]outboundPacket{{"life_token_revive_ack", 1, 41, []byte{1}}}, nil)
	if err != nil || len(plan) != 1 || w.azure.revivesLeft != 0 {
		t.Fatalf("0 次时应当只回原包：packets=%d left=%d err=%v", len(plan), w.azure.revivesLeft, err)
	}

	// 复活失败（报错 / 没有包）一律不扣。
	w.azure.revivesLeft = azureMainReviveLimit
	if plan, err := w.afterAzureCoinRevive(nil, errors.New("boom")); plan != nil || err == nil {
		t.Fatal("复活失败时不该产出新包")
	}
	if plan, _ := w.afterAzureCoinRevive(nil, nil); plan != nil {
		t.Fatal("没有复活包时不该扣次数")
	}
	if w.azure.revivesLeft != azureMainReviveLimit {
		t.Fatalf("失败路径不该扣次数：left=%d", w.azure.revivesLeft)
	}

	// 别的频道不归蔚蓝号管。
	other := &worldSession{}
	plan, _ = other.afterAzureCoinRevive([]outboundPacket{{"life_token_revive_ack", 1, 41, []byte{1}}}, nil)
	if len(plan) != 1 || other.azure.revivesLeft != 0 {
		t.Fatal("非蔚蓝号频道不该被扣复活次数")
	}
}
