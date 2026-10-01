package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	UnifiedOptionWarpFavorites = 11
	WarpFavoriteSlots          = 10
	WarpFavoriteRecordSize     = 12
	WarpFavoritesBlockSize     = 132
	WarpFavoritesAccountOffset = 3397
)

// WarpFavoriteEntry is a changed slot, not a complete list. Native sender
// 1403DA900 writes a u16 slot followed by the 12-byte value. Comparison at
// 1403D0BE0 uses value[0], the u32 at value[4], and value[8]; other bytes are
// native padding. Preserve the record without treating padding as pointers.
type WarpFavoriteEntry struct {
	Position uint16
	Value    [WarpFavoriteRecordSize]byte
}

func ValidateWarpFavorites(entries []WarpFavoriteEntry) error {
	if len(entries) > WarpFavoriteSlots {
		return fmt.Errorf("too many warp favorite slots: %d", len(entries))
	}
	var seen [WarpFavoriteSlots]bool
	for _, entry := range entries {
		if entry.Position >= WarpFavoriteSlots || seen[entry.Position] {
			return fmt.Errorf("invalid or duplicate warp favorite slot %d", entry.Position)
		}
		seen[entry.Position] = true
	}
	return nil
}

func decodeWarpFavorites(p []byte) ([]WarpFavoriteEntry, int, error) {
	if p[13] != UnifiedOptionScopeAccount {
		return nil, 0, fmt.Errorf("warp favorites require account scope")
	}
	count := binary.LittleEndian.Uint32(p[15:19])
	if count > WarpFavoriteSlots {
		return nil, 0, fmt.Errorf("invalid warp favorite count %d", count)
	}
	end := unifiedOptionLen + int(count)*(2+WarpFavoriteRecordSize)
	if len(p) < end || len(p) > end+unifiedOptionMaxTail {
		return nil, 0, fmt.Errorf("invalid warp favorites length %d for %d slots", len(p), count)
	}
	for _, b := range p[end:] {
		if b != 0 {
			return nil, 0, fmt.Errorf("invalid warp favorites padding")
		}
	}
	entries := make([]WarpFavoriteEntry, count)
	for i := range entries {
		off := unifiedOptionLen + i*(2+WarpFavoriteRecordSize)
		entries[i].Position = binary.LittleEndian.Uint16(p[off:])
		copy(entries[i].Value[:], p[off+2:off+2+WarpFavoriteRecordSize])
	}
	return entries, len(p) - end, ValidateWarpFavorites(entries)
}

// WarpFavoritesBlock serializes the account subtype-11 object. It has no slot
// prefixes: valid + padding + ten 12-byte values + ten presence bytes. Missing
// slots retain the native default (type 0, id -1, flag 0), with presence unset.
// Explicit empty slots remain present, so removing a favorite survives relog.
func WarpFavoritesBlock(entries []WarpFavoriteEntry) ([]byte, error) {
	if err := ValidateWarpFavorites(entries); err != nil {
		return nil, err
	}
	p := make([]byte, WarpFavoritesBlockSize)
	p[0] = 1
	for i := 0; i < WarpFavoriteSlots; i++ {
		binary.LittleEndian.PutUint32(p[2+i*WarpFavoriteRecordSize+4:], ^uint32(0))
	}
	for _, entry := range entries {
		off := 2 + int(entry.Position)*WarpFavoriteRecordSize
		copy(p[off:off+WarpFavoriteRecordSize], entry.Value[:])
		p[122+int(entry.Position)] = 1
	}
	return p, nil
}

// FillAccountWarpFavorites uses the existing NOTI2826 login path: 1403E0030
// reads 3648 bytes, 1403FF0D0/14757A4D0 case 11 rebuilds the active slots, then
// 1403E2BC0(group 2)/1403E29D0 refreshes favorites. No NOTI2828 is needed.
// In particular, a 15-byte 2828 header alone is invalid: 1403CF590 also reads
// the 132-byte object from that packet, which the old reference omitted.
func FillAccountWarpFavorites(account []byte, entries []WarpFavoriteEntry) error {
	if len(account) != 3648 {
		return fmt.Errorf("account option block size mismatch: %d", len(account))
	}
	block, err := WarpFavoritesBlock(entries)
	if err != nil {
		return err
	}
	copy(account[WarpFavoritesAccountOffset:WarpFavoritesAccountOffset+WarpFavoritesBlockSize], block)
	return nil
}
