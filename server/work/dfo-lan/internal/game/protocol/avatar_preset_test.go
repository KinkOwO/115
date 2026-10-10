package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// officialAvatarPresetList 是官方 noti1585 的真实包体（624 字节）。
//
// 来源两处、**内容完全一致**：
//   - 115/analysis/captures/official_20261002/20261002-211855_192_168_1_31_DFO_exe/frames.jsonl
//     （op=1585，10 帧全同）
//   - 桌面 official_20261009-223033_live/session_{s3,s6,s30}_s2c.txt（各 1 帧，size=640/body=624）
const officialAvatarPresetList = "" +
	"0101010B0000004E657720507265736574730C00000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000010000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000020000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000300000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000400000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000005000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000006" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000070000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000800000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000900000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"000000000000000A000000000000000000000000000000000000000000000000" +
	"000000000000000000000000000000000000000000000000000B000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"00000000000000000000000000000000"

// TestAvatarPresetListMatchesOfficial 是这套编码器的硬判据：官方默认档位
// （1 页 / 页名 "New Presets" / 每页 12 条）下必须与官方帧**逐字节相同**。
func TestAvatarPresetListMatchesOfficial(t *testing.T) {
	want, err := hex.DecodeString(strings.ToUpper(strings.ReplaceAll(officialAvatarPresetList, "\n", "")))
	if err != nil {
		t.Fatalf("official fixture is not valid hex: %v", err)
	}
	if len(want) != 624 {
		t.Fatalf("official fixture length = %d, want 624", len(want))
	}
	got, err := AvatarPresetList(OfficialAvatarPresetPages)
	if err != nil {
		t.Fatalf("AvatarPresetList: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("AvatarPresetList(%d) mismatch\n got %d bytes: %X\nwant %d bytes: %X",
			OfficialAvatarPresetPages, len(got), got, len(want), want)
	}
}

// TestAvatarPresetListPages 锁住"页数"语义：第 2 字节是页签数，每页结构完整重复，
// 帧长 = 2 + pages*617 + 5。
func TestAvatarPresetListPages(t *testing.T) {
	const pageSize = 1 + 4 + len(AvatarPresetPageName) + 1 + int(OfficialAvatarPresetRecordsPerPage)*avatarPresetRecordSize
	for _, pages := range []byte{1, 2, 3, MaxAvatarPresetPages} {
		body, err := AvatarPresetList(pages)
		if err != nil {
			t.Fatalf("AvatarPresetList(%d): %v", pages, err)
		}
		if want := 2 + int(pages)*pageSize + avatarPresetListTrailer; len(body) != want {
			t.Fatalf("pages=%d length = %d, want %d", pages, len(body), want)
		}
		if body[0] != 1 {
			t.Fatalf("pages=%d current page index = %d, want 1", pages, body[0])
		}
		if body[1] != pages {
			t.Fatalf("pages=%d page count byte = %d", pages, body[1])
		}
		// 每页起点都必须是同样的官方页结构：状态 1 + 长度 11 + 页名 + 条目数 12
		for p := 0; p < int(pages); p++ {
			base := 2 + p*pageSize
			if body[base] != 1 {
				t.Fatalf("pages=%d page %d state = %d", pages, p, body[base])
			}
			if got := string(body[base+5 : base+5+len(AvatarPresetPageName)]); got != AvatarPresetPageName {
				t.Fatalf("pages=%d page %d name = %q", pages, p, got)
			}
			if body[base+pageSize-1-int(OfficialAvatarPresetRecordsPerPage)*avatarPresetRecordSize] != OfficialAvatarPresetRecordsPerPage {
				t.Fatalf("pages=%d page %d record count byte wrong", pages, p)
			}
		}
	}
}

func TestAvatarPresetListRejectsOutOfRange(t *testing.T) {
	for _, pages := range []byte{0, MaxAvatarPresetPages + 1} {
		if _, err := AvatarPresetList(pages); err == nil {
			t.Fatalf("AvatarPresetList(%d) accepted", pages)
		}
	}
}
