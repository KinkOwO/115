package protocol

import (
	"bytes"
	"dfolan/internal/social"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// CMD2387 uses a copied 15-byte native structure. The first thirteen bytes
// are opaque stack data (three live vectors differ); they are not a role ID.
// 14048FDC0: kind=0 is the current actor; kind=1 is a season costume choice.
func DecodeSocialRepresentative(p []byte) (kind, selection byte, err error) {
	if len(p) < 15 {
		return 0, 0, fmt.Errorf("short community representative")
	}
	if err = padding(p[15:], 16); err != nil {
		return
	}
	kind, selection = p[13], p[14]
	if kind > 1 || kind == 0 && selection != 0 {
		err = fmt.Errorf("invalid community representative mode")
	}
	return
}

func socialFixedGBK(p []byte) (string, error) {
	end := bytes.IndexByte(p, 0)
	if end < 0 {
		return "", fmt.Errorf("unterminated community text")
	}
	raw := p[:end]
	text, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return "", err
	}
	roundtrip, err := simplifiedchinese.GBK.NewEncoder().Bytes(text)
	if err != nil || !bytes.Equal(roundtrip, raw) {
		return "", fmt.Errorf("invalid community GBK text")
	}
	return string(text), nil
}

// 14390EB7C constructs 44 bytes, with a 31-byte CP_ACP string at +13.
// The captured client machine uses ACP936. Native ACK2388 success triggers
// 14047C420, which requests this account's HTTP metadata again.
func DecodeSocialMessage(p []byte) (string, error) {
	if len(p) < 44 {
		return "", fmt.Errorf("short community message")
	}
	if err := padding(p[44:], 16); err != nil {
		return "", err
	}
	return socialFixedGBK(p[13:44])
}

// CMD1676, 14047EB00: raw157 = opaque13 + colors32 + orders8 + names104.
// Native group labels use CP_ACP on upload and UTF-8 length strings on NOTI1682.
func DecodeSocialGroups(p []byte) (groups [8]social.Group, err error) {
	if len(p) < 157 {
		return groups, fmt.Errorf("short community groups")
	}
	if err = padding(p[157:], 16); err != nil {
		return
	}
	seen := [9]bool{}
	for i := range groups {
		g := &groups[i]
		g.Color = binary.LittleEndian.Uint32(p[13+i*4:])
		g.Order = p[45+i]
		if g.Order < 1 || g.Order > 8 || seen[g.Order] {
			return groups, fmt.Errorf("invalid community group order")
		}
		seen[g.Order] = true
		g.Name, err = socialFixedGBK(p[53+i*13 : 53+(i+1)*13])
		if err != nil {
			return
		}
		// 146D78070 at the group reader accepts fewer than 13 UTF-8 bytes.
		if len(g.Name) >= 13 {
			return groups, fmt.Errorf("community group label exceeds native UTF-8 reader")
		}
	}
	return
}

// NOTI1682, 1452FC3F0: exactly eight (u32 color,u8 order,UTF-8 string) rows.
// An unset group is all zero, letting the client select its own default label
// and palette. It is not a friend-list notification.
func SocialGroupInfo(groups *[8]social.Group) ([]byte, error) {
	var p []byte
	for i := 0; i < 8; i++ {
		var g social.Group
		if groups != nil {
			g = groups[i]
		}
		if len(g.Name) >= 13 {
			return nil, fmt.Errorf("community group label exceeds native reader")
		}
		p = binary.LittleEndian.AppendUint32(p, g.Color)
		p = append(p, g.Order)
		p = binary.LittleEndian.AppendUint32(p, uint32(len(g.Name)))
		p = append(p, []byte(g.Name)...)
	}
	return p, nil
}
