package charactercheck

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestNativeEquipmentSeparatesContentAndSaveIdentity(t *testing.T) {
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("8b", 32)}
	if source.SaveIdentity() == source.Checksum {
		t.Fatal("fixture must distinguish identities")
	}
	if err := validateNativeEquipmentIdentity(source, source.SaveIdentity(), source.Checksum); err != nil {
		t.Fatal(err)
	}
	if err := validateNativeEquipmentIdentity(source, source.Checksum, source.Checksum); err == nil {
		t.Fatal("content hash accepted as save identity")
	}
	if err := validateNativeEquipmentIdentity(source, source.SaveIdentity(), strings.Repeat("7e", 32)); err == nil {
		t.Fatal("foreign quest content accepted")
	}
}
