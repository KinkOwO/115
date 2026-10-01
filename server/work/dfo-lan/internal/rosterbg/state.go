// Package rosterbg 管理账号的五页选角背景，不与角色资料皮肤共用编号。
package rosterbg

import (
	"fmt"
	"math"
)

const Pages = 5

type Background struct {
	Category uint8
	ID       uint16
}

// Unlock 的时间为原生NOTI1759使用的Unix秒；0表示永久。
type Unlock struct {
	Background
	ExpiresAt uint32
}

type State struct {
	Selected [Pages]Background
	Owned    []Unlock
}

// Valid 对照当前原版 etc/selectcharacterver2/selectcharacterver2.etc。
// 资源 SHA256：2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167。
// 仅证明资源存在；特殊背景仍须账号拥有，不能由编号范围自动授予。
func (b Background) Valid() bool {
	if catalog := currentTicketCatalog.Load(); catalog != nil {
		return catalog.ValidBackground(b)
	}
	return legacyBackgroundValid(b)
}

func legacyBackgroundValid(b Background) bool {
	return b.Category == 0 && b.ID <= 5 || b.Category == 1 && (b.ID <= 51 || b.ID >= 500 && b.ID <= 504)
}

func (s State) CanSelect(b Background) bool {
	if !b.Valid() {
		return false
	}
	if b.Category == 0 {
		return true
	}
	for _, owned := range s.Owned {
		if owned.Background == b {
			return true
		}
	}
	return false
}

func (s State) Validate() error {
	if len(s.Owned) > 255 {
		return fmt.Errorf("选角背景拥有数量超出协议范围")
	}
	seen := make(map[Background]bool, len(s.Owned))
	for _, b := range s.Owned {
		if b.Category != 1 || !b.Valid() || seen[b.Background] || b.ExpiresAt > math.MaxInt32 {
			return fmt.Errorf("选角背景拥有记录无效或重复")
		}
		seen[b.Background] = true
	}
	for _, b := range s.Selected {
		if !s.CanSelect(b) {
			return fmt.Errorf("选角背景不存在或尚未解锁")
		}
	}
	return nil
}
