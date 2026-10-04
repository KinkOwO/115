package world

// 存档世界位置类型由 world 领域拥有。internal/database 通过类型别名复用这些类型
// （迁移期兼容），因此持久化依赖方向为 storage -> world（契约 R3 允许），
// world 不再反向依赖 storage。
type WorldPosition struct {
	Town   uint32       `json:"town"`
	Area   uint32       `json:"area"`
	X      uint16       `json:"x"`
	Y      uint16       `json:"y"`
	Return *WorldReturn `json:"return,omitempty"`
}
type WorldReturn struct {
	Town uint32 `json:"town"`
	Area uint32 `json:"area"`
	X    uint16 `json:"x"`
	Y    uint16 `json:"y"`
}
type WorldState struct {
	Position      WorldPosition `json:"position"`
	Revision      int64         `json:"revision"`
	ConfigVersion string        `json:"config_version"`
}
