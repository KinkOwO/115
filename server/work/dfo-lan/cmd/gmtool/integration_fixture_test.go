package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Explicit export of a newly created temporary save for desktop integration.
// No runtime profile is read; the path must be absent and the source is a fixture.
func TestExportIntegrationGUIFixture(t *testing.T) {
	out := os.Getenv("INTEGRATION_GUI_FIXTURE")
	if out == "" {
		t.Skip("explicit private fixture export")
	}
	if !filepath.IsAbs(out) {
		t.Fatal("absolute new fixture path required")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("refusing to replace existing file", err)
	}
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := character.New(f.Storage(), c, character.Rules{MaxCharacters: 24, InitialLevel: 1})
	if err != nil {
		t.Fatal(err)
	}
	account, err := f.DevelopmentAccount(ctx, "integration-gui-test")
	if err != nil {
		t.Fatal(err)
	}
	name := []byte("IntegrationTest")
	request := []byte{0}
	request = binary.LittleEndian.AppendUint32(request, uint32(len(name)))
	request = append(request, name...)
	request = append(request, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0)
	role, err := s.Create(ctx, account, request)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := database.Backup(ctx, database.SQLiteTarget(f.Path()), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Files) != 1 {
		t.Fatal("unexpected SQLite backup files")
	}
	body, err := os.ReadFile(filepath.Join(entry.Directory, entry.Files[0].Name))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(out), 0700); err != nil {
		t.Fatal(err)
	}
	fout, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fout.Write(body); err != nil {
		fout.Close()
		t.Fatal(err)
	}
	if err = fout.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported new test-only SQLite: account=%d role=%d file=%s", account, role.ID, out)
}
