package database_test

import (
	"dfolan/internal/character"
	"dfolan/internal/database"
)

var (
	_ character.Store            = (*database.Store)(nil)
	_ character.ProgressionStore = (*database.Store)(nil)
	_ character.FatigueStore     = (*database.Store)(nil)
)
