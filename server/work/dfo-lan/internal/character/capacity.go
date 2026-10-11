package character

import "context"

type capacityStore interface {
	AccountCharacterCapacity(context.Context, int64, int) (int, error)
}

// Capacity publishes the same durable account limit used by creation and
// roster changes. Older test stores retain their configured base capacity.
func (s *Service) Capacity(ctx context.Context, account int64) (int, error) {
	if store, ok := s.Store.(capacityStore); ok {
		return store.AccountCharacterCapacity(ctx, account, s.Rules.MaxCharacters)
	}
	return s.Rules.MaxCharacters, nil
}
