package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
)

type ceraReader interface {
	AccountCera(context.Context, int64) (uint64, error)
}

func ceraQuery(ctx context.Context, store ceraReader, account int64, p []byte) ([]byte, error) {
	if len(p) != 0 {
		return nil, fmt.Errorf("CMD63 requires empty body")
	}
	balance, e := store.AccountCera(ctx, account)
	if e != nil {
		return nil, e
	}
	return protocol.CeraBalance(balance)
}
