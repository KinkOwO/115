package database

import (
	"context"
	"errors"
)

var ErrAdventureNameTaken = errors.New("冒险团名称已被使用")

func (s *Store) AdventureNameAvailable(ctx context.Context, account int64, name string) (bool, error) {
	return s.queries.AdventureNameAvailable(ctx, account, name)
}
