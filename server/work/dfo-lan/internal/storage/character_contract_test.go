package storage_test

import (
	"dfolan/internal/character"
	"dfolan/internal/storage"
)

var (
	_ character.Store            = (*storage.Store)(nil)
	_ character.ProgressionStore = (*storage.Store)(nil)
	_ character.FatigueStore     = (*storage.Store)(nil)
)
