package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"golang.org/x/text/encoding/simplifiedchinese"
	"testing"
)

func TestSocialRepresentativeThreeLiveVectors(t *testing.T) {
	for _, vector := range []string{"80de3b6c010000003c9ef64501000000", "e0c34105000000003c9ef64501000000", "4006dcc2010000003c9ef64501000000"} {
		p, _ := hex.DecodeString(vector)
		kind, selection, err := DecodeSocialRepresentative(p)
		if err != nil || kind != 0 || selection != 0 {
			t.Fatalf("opaque prefix interpreted as identity: %d %d %v", kind, selection, err)
		}
		for _, end := range []int{0, 13, 14} {
			if _, _, err := DecodeSocialRepresentative(p[:end]); err == nil {
				t.Fatal("truncation accepted", end)
			}
		}
		p[15] = 1
		if _, _, err := DecodeSocialRepresentative(p); err == nil {
			t.Fatal("nonzero padding accepted")
		}
	}
}

func TestSocialMessageACP936AndBounds(t *testing.T) {
	p := make([]byte, 48)
	copy(p[:13], bytes.Repeat([]byte{0xa5}, 13))
	raw, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文签名abc"))
	copy(p[13:], raw)
	if got, err := DecodeSocialMessage(p); err != nil || got != "中文签名abc" {
		t.Fatalf("message %q %v", got, err)
	}
	copy(p[13:44], bytes.Repeat([]byte{'a'}, 31))
	if _, err := DecodeSocialMessage(p); err == nil {
		t.Fatal("unterminated message accepted")
	}
	p[43] = 0
	if got, err := DecodeSocialMessage(p); err != nil || len(got) != 30 {
		t.Fatalf("max ASCII message: %q %v", got, err)
	}
	p[13], p[14] = 0x81, 0
	if _, err := DecodeSocialMessage(p); err == nil {
		t.Fatal("invalid GBK accepted")
	}
}

func TestSocialGroupsDistinctColorOrderAndChineseEncoding(t *testing.T) {
	p := make([]byte, 160)
	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint32(p[13+i*4:], uint32(0xaa0000+i))
		p[45+i] = byte(8 - i)
	}
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文好友"))
	copy(p[53:66], gbk)
	groups, err := DecodeSocialGroups(p)
	if err != nil || groups[0].Name != "中文好友" || groups[0].Color != 0xaa0000 || groups[0].Order != 8 {
		t.Fatalf("group layout %+v %v", groups[0], err)
	}
	down, err := SocialGroupInfo(&groups)
	if err != nil || binary.LittleEndian.Uint32(down) != 0xaa0000 || down[4] != 8 || binary.LittleEndian.Uint32(down[5:]) != 12 || string(down[9:21]) != "中文好友" {
		t.Fatalf("native downlink %x %v", down, err)
	}
	long, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文好友组"))
	copy(p[53:66], append(long, 0))
	if _, err := DecodeSocialGroups(p); err == nil {
		t.Fatal("label overflowing native UTF-8 reader accepted")
	}
	copy(p[53:66], make([]byte, 13))
	p[46] = p[45]
	if _, err := DecodeSocialGroups(p); err == nil {
		t.Fatal("duplicate display order accepted")
	}
	defaults, err := SocialGroupInfo(nil)
	if err != nil || len(defaults) != 72 || !bytes.Equal(defaults, make([]byte, 72)) {
		t.Fatalf("native default groups %x %v", defaults, err)
	}
}
