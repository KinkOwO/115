package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAdventureNameLiveUTF8RequestAndRestore(t *testing.T) {
	// 2026-10-07 06:48:01+08, current client CMD1436, checksum_ok=true.
	p, err := hex.DecodeString("09000000e8b0b7e69ca8e8afad000000")
	if err != nil {
		t.Fatal(err)
	}
	name, err := DecodeAdventureName(p)
	if err != nil || name != "谷木语" {
		t.Fatal("live UTF-8 name was rejected", name, err)
	}
	info, err := AdventureNameInfo(name, false)
	if err != nil || !bytes.Equal(info, p) {
		t.Fatal("NOTI1363 must use UTF-8 through reader 149185A00", info, err)
	}
	info, err = AdventureResponse(AdventureRequest{Target: 27, Kind: 0}, 1, 10, AdventureInfo{Name: name, Level: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(info[len(info)-9-(len(p)-3):len(info)-9], p[:len(p)-3]) {
		t.Fatal("basic-info name is not UTF-8")
	}
}

func TestAdventureNameUTF8KeepsLogicalCapacity(t *testing.T) {
	for _, name := range []string{"一二三四五六七八", "abcdefghijklmnop", "一二三四abcdefgh"} {
		wire, err := AdventureNameInfo(name, false)
		if err != nil {
			t.Fatal(name, err)
		}
		decoded, err := DecodeAdventureName(wire[:len(wire)-3])
		if err != nil || decoded != name {
			t.Fatal(name, decoded, err)
		}
		if len([]rune(name)) > 16 || len([]byte(name)) > 24 {
			t.Fatal("name exceeds native wide buffer or patched UTF-8 bound")
		}
	}
	for _, name := range []string{"一二三四五六七八九", "abcdefghijklmnopq", "一二三四abcdefghi"} {
		if _, err := AdventureNameInfo(name, false); err == nil {
			t.Fatal("oversize name accepted", name)
		}
	}
}
