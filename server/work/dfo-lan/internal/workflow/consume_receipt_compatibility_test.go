package workflow

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestConsumeReceiptAcceptsOnlyCurrentArchiveOrSaveContract(t *testing.T) {
	current := pvf.ArchiveSnapshot{Checksum: "current-archive"}
	for _, source := range []string{current.SaveIdentity(), current.Checksum} {
		if !consumeReceiptSourceMatches(source, current) {
			t.Fatal("valid receipt source rejected")
		}
	}
	for _, source := range []string{"", "different-archive"} {
		if consumeReceiptSourceMatches(source, current) {
			t.Fatal("foreign receipt accepted")
		}
	}
	if consumeReceiptSourceMatches("current-archive", pvf.ArchiveSnapshot{Checksum: "next-archive"}) {
		t.Fatal("archive change silently aliased")
	}
}
