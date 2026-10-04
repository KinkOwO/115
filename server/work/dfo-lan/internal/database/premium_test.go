package database

import (
	"context"
	"testing"
	"time"
)

func TestStorageNilSafety(t *testing.T) {
	var s *Store
	ctx := context.Background()
	now := time.Now()

	// Nil store must safely return false/nil without panicking
	has, err := s.HasActivePremium(ctx, 1, PremiumConqueror, now)
	if err != nil || has {
		t.Fatalf("expected false, nil from nil store, got %v, %v", has, err)
	}

	premiums, err := s.ActivePremiums(ctx, 1, now)
	if err != nil || premiums != nil {
		t.Fatalf("expected nil, nil from nil store, got %v, %v", premiums, err)
	}

	set, err := s.ActivePremiumSet(ctx, 1, now)
	if err != nil || len(set) != 0 {
		t.Fatalf("expected empty set from nil store, got %v, %v", set, err)
	}
}
