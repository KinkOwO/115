package inventory

import "testing"

// 实机 2026-09-23（角色 test-jh）复现的真实缺陷：
//
//	23:57 买过一次 10417798 → 事件键 buy:10417798:1:1
//	00:09 重启服务端（进程内自增归零）
//	00:11 再买同一个盒子 → 自增又给到 1 → 键完全相同
//	→ CommitCharacterEvent 判定"已处理过" → 不发货、不改存档
//	→ 但 buyItem 仍回了成功 ack（用的是旧 receipt 里的 slot 66）
//	→ 客户端凭空画出一个存档里不存在的盒子，右键时被服务端如实拒绝
//	→ 客户端把拒绝码显示成「库存已满」，重选角色后"幽灵盒子"消失
//
// 这条用例锁住"事件键跨重启也绝不重复"这个前提。
func TestShopEventKeyNeverRepeatsAfterRestart(t *testing.T) {
	const boot1, boot2 = int64(1790088276000000000), int64(1790093355000000000)
	first := shopEventKeyAt(boot1, "buy", 1, 10417798, 1)
	// 模拟重启：进程内计数重新从 1 开始、请求内容完全相同，只有启动标记不同。
	if again := shopEventKeyAt(boot2, "buy", 1, 10417798, 1); first == again {
		t.Fatalf("重启后的同一次购买撞上了旧事件键: %s", first)
	}
	// 同一次进程内，计数不同也必须不同。
	if first == shopEventKeyAt(boot1, "buy", 2, 10417798, 1) {
		t.Fatal("同一进程内不同请求生成了相同的事件键")
	}
	if first == shopEventKeyAt(boot1, "buy", 1, 10417799, 1) {
		t.Fatal("不同模板生成了相同的事件键")
	}
	if first == shopEventKeyAt(boot1, "sell", 1, 10417798, 1) {
		t.Fatal("buy 与 sell 生成了相同的事件键")
	}
	if first[:4] != "buy:" {
		t.Fatalf("键前缀丢了: %s", first)
	}
}

// 生产路径用的键也必须带上本次进程的启动标记。
func TestShopEventKeyCarriesBootStamp(t *testing.T) {
	if got, want := shopEventKey("buy", 7, 10417798, 1), shopEventKeyAt(shopBootStamp, "buy", 7, 10417798, 1); got != want {
		t.Fatalf("店铺事件键没有使用本次进程的启动标记: %s != %s", got, want)
	}
}
