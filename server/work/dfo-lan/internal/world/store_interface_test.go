package world

import (
	"context"
	"dfolan/internal/testfixture"
	"testing"

	"dfolan/internal/catalog"
)

// fakeStore 是内存实现，证明 world 只依赖自己声明的 Store 接口，
// 不再绑定 internal/storage，因此领域单测无需 PostgreSQL。
type fakeStore struct {
	loaded bool
}

func (f *fakeStore) LoadWorld(_ context.Context, _, _ int64, _ uint32, initial WorldPosition, version string) (WorldState, error) {
	f.loaded = true
	return WorldState{Position: initial, ConfigVersion: version, Revision: 1}, nil
}

func TestServiceUsesInjectedStore(t *testing.T) {
	cat, err := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeStore{}
	svc := &Service{Store: fake, Catalog: cat}
	spawn := WorldPosition{Town: 38, Area: 0, X: 550, Y: 230}
	// channelType = 0 表示普通频道（共享 character_world）。
	if _, e := svc.Enter(context.Background(), 1, 1, 100, false, spawn, 0); e != nil {
		t.Fatalf("Enter with injected store: %v", e)
	}
	if !fake.loaded {
		t.Fatal("world did not use the injected Store")
	}
}
