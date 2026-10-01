// Package model 是跨领域共享的纯数据聚合（共享内核）。
//
// 规则（见 docs/architecture-contract.md）：
//   - 只放数据结构，不含任何游戏规则或行为；
//   - 不 import 任何 internal 包；
//   - 领域与 internal/storage 都可依赖它，用来打破 character<->inventory 之类的双向依赖。
package model

import (
	"encoding/json"
	"time"
)

// Character 是账号下的角色聚合。它被多个领域（character/inventory/loot/quest/cashshop）
// 和持久化层共同引用，因此放在中立 model 中，而不是任一领域内，避免形成依赖环。
type Character struct {
	ID            int64
	AccountID     int64
	WireID        uint16
	FixedSlot     byte
	Name          string
	Profession    byte
	Request       []byte
	ConfigVersion string
	State         json.RawMessage
	CreatedAt     time.Time
}
