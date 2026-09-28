package protocol

// 本文件只提供军团奖励行需要的**行类型**；捐赠者主线里它与征讨（Clear reward）共用，
// 位于 internal/game/protocol/conquest_reward115.go。上游没有这个类型，故随本补丁新增。

// Value is the item's native row value, NOT universally a quantity. Equipment
// uses an instance value (G0260 includes476296939 and999999999); a caller must
// never feed it to Bag.Add as an item count. Rows must come from frozen grants.
type ConquestRewardValue115 struct {
	Equipment bool // validation only; fresh instance value0 is valid, stack quantity0 is not
	Template  uint32
	Value     uint32
	Metadata  [21]byte
}
