package oath

import (
	"context"
	"fmt"

	"dfolan/internal/storage"
)

// OptionState 是协议层可直接使用的已持久化选项状态。
// 未选择时 SelectedOption 为 -1，调用方必须交给 Resolve 使用定义的默认值。
type OptionState struct {
	CoreInstanceKey string
	SelectedOption  int
	Revision        int64
	Present         bool
}

// LoadOption 读取单个核心实例的选项状态；不存在记录不会被伪造成某个选项。
func LoadOption(ctx context.Context, store *storage.Store, characterID int64, coreInstanceKey string) (OptionState, error) {
	if store == nil {
		return OptionState{}, fmt.Errorf("oath option load: nil store")
	}
	if characterID <= 0 || coreInstanceKey == "" {
		return OptionState{}, fmt.Errorf("oath option load: invalid key")
	}
	state, ok, err := store.OathOption(ctx, characterID, coreInstanceKey)
	if err != nil {
		return OptionState{}, err
	}
	if !ok {
		return OptionState{CoreInstanceKey: coreInstanceKey, SelectedOption: -1}, nil
	}
	return OptionState{CoreInstanceKey: state.CoreInstanceKey, SelectedOption: state.SelectedOption, Revision: state.Revision, Present: true}, nil
}

// SaveOption 持久化一次带版本号的选项变更。过期版本由 storage.ErrOathOptionRevisionConflict 原样返回。
func SaveOption(ctx context.Context, store *storage.Store, characterID int64, coreInstanceKey string, selectedOption int, expectedRevision int64) (OptionState, error) {
	if store == nil {
		return OptionState{}, fmt.Errorf("oath option save: nil store")
	}
	state, err := store.SaveOathOption(ctx, characterID, coreInstanceKey, selectedOption, expectedRevision)
	if err != nil {
		return OptionState{}, err
	}
	return OptionState{CoreInstanceKey: state.CoreInstanceKey, SelectedOption: state.SelectedOption, Revision: state.Revision, Present: true}, nil
}

// ResolveStored loads the persisted selection for the equipped core and resolves
// the runtime snapshot through the strict pure resolver. A missing row is kept
// absent so Resolve applies only the catalog-defined default option.
func ResolveStored(ctx context.Context, store *storage.Store, characterID int64, snapshot CharacterSnapshot, catalog Catalog) (RuntimeSnapshot, error) {
	if snapshot.Core == nil {
		return Resolve(snapshot, catalog)
	}
	state, err := LoadOption(ctx, store, characterID, snapshot.Core.InstanceKey)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if state.Present {
		selected := state.SelectedOption
		snapshot.BoundOption = &BoundOption{CoreInstanceKey: state.CoreInstanceKey, SelectedOption: selected, Revision: state.Revision}
	}
	return Resolve(snapshot, catalog)
}
