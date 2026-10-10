// Package featurebridge implements the explicitly owned local extension
// protocol. These frames are never passed to a native DFO packet reader.
package featurebridge

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	Version           = 1
	HeaderSize        = 24
	MaxBody           = 60 * 1024
	Snapshot   uint16 = 1
	Claim      uint16 = 2
	Response   uint16 = 0x8000
)
const (
	StatusOK uint32 = iota
	StatusNotReady
	StatusUnsupported
	StatusRefused
	StatusInternal
)
const (
	Weekly     byte = 1
	Daily      byte = 2
	Online     byte = 3
	Membership byte = 4
	Emblem     byte = 5
	Period     byte = 1
	Cumulative byte = 2
	Day        byte = 3
	Locked     byte = 0
	Claimable  byte = 1
	Claimed    byte = 2
)

type Frame struct {
	Kind   uint16
	ID     uint64
	Status uint32
	Body   []byte
}

func (f Frame) Bytes() ([]byte, error) {
	kind := f.Kind &^ Response
	if f.ID == 0 || len(f.Body) > MaxBody || kind < Snapshot || kind > SymbolRefresh || f.Status > StatusInternal || (f.Kind&Response == 0 && f.Status != 0) {
		return nil, fmt.Errorf("invalid extension frame")
	}
	p := make([]byte, HeaderSize+len(f.Body))
	copy(p, "U115")
	binary.LittleEndian.PutUint16(p[4:], Version)
	binary.LittleEndian.PutUint16(p[6:], f.Kind)
	binary.LittleEndian.PutUint64(p[8:], f.ID)
	binary.LittleEndian.PutUint32(p[16:], uint32(len(f.Body)))
	binary.LittleEndian.PutUint32(p[20:], f.Status)
	copy(p[24:], f.Body)
	return p, nil
}
func Read(r io.Reader) (Frame, error) {
	var f Frame
	h := make([]byte, HeaderSize)
	if _, e := io.ReadFull(r, h); e != nil {
		return f, e
	}
	if string(h[:4]) != "U115" || binary.LittleEndian.Uint16(h[4:]) != Version || binary.LittleEndian.Uint32(h[16:]) > MaxBody {
		return f, fmt.Errorf("extension header/version/length mismatch")
	}
	f.Kind = binary.LittleEndian.Uint16(h[6:])
	f.ID = binary.LittleEndian.Uint64(h[8:])
	f.Status = binary.LittleEndian.Uint32(h[20:])
	f.Body = make([]byte, binary.LittleEndian.Uint32(h[16:]))
	if _, e := io.ReadFull(r, f.Body); e != nil {
		return Frame{}, e
	}
	_, e := f.Bytes()
	return f, e
}

type ClaimRequest struct {
	Family, Group    byte
	Index            uint32
	Source, Campaign string
}

func DecodeClaim(p []byte) (ClaimRequest, error) {
	var c ClaimRequest
	if len(p) != 104 || p[2] != 0 || p[3] != 0 {
		return c, fmt.Errorf("claim width/reserved mismatch")
	}
	c = ClaimRequest{p[0], p[1], binary.LittleEndian.Uint32(p[4:]), string(p[8:72]), string(p[72:104])}
	if c.Family < Weekly || c.Family > Emblem || c.Group < Period || c.Group > Day || c.Index == 0 || !hexString(c.Source, 64) || !hexString(c.Campaign, 32) {
		return ClaimRequest{}, fmt.Errorf("invalid claim identity")
	}
	return c, nil
}
func hexString(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Campaign is the first 128 bits of the existing SHA256 campaign identity.
// The full source checksum is included independently and checked before writes.
type View struct {
	Role      uint64
	Campaigns []CampaignView
}
type CampaignView struct {
	Family, State                   byte
	Title, Detail, Source, Campaign string
	Rows                            []Row
}
type Row struct {
	Group, State       byte
	Index, Item, Count uint32
	Progress, Goal     uint64
	Label              string
}

func (v View) Bytes() ([]byte, error) {
	if len(v.Campaigns) > 16 {
		return nil, fmt.Errorf("too many campaigns")
	}
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, v.Role)
	binary.Write(b, binary.LittleEndian, uint32(len(v.Campaigns)))
	text := func(s string) error {
		if len(s) > 4096 || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
			return fmt.Errorf("invalid view text")
		}
		binary.Write(b, binary.LittleEndian, uint16(len(s)))
		b.WriteString(s)
		return nil
	}
	for _, c := range v.Campaigns {
		if c.Family < Weekly || c.Family > Emblem || c.State > 2 || len(c.Rows) > 128 || (!hexString(c.Source, 64) && c.Source != "") || (!hexString(c.Campaign, 32) && c.Campaign != "") {
			return nil, fmt.Errorf("invalid campaign view")
		}
		b.WriteByte(c.Family)
		b.WriteByte(c.State)
		binary.Write(b, binary.LittleEndian, uint16(len(c.Rows)))
		for _, s := range []string{c.Title, c.Detail, c.Source, c.Campaign} {
			if e := text(s); e != nil {
				return nil, e
			}
		}
		for _, r := range c.Rows {
			if r.Group < Period || r.Group > Day || r.State > Claimed || r.Index == 0 || r.Item < 2 || r.Count == 0 {
				return nil, fmt.Errorf("invalid view row")
			}
			b.WriteByte(r.Group)
			b.WriteByte(r.State)
			binary.Write(b, binary.LittleEndian, uint16(0))
			for _, n := range []uint32{r.Index, r.Item, r.Count} {
				binary.Write(b, binary.LittleEndian, n)
			}
			binary.Write(b, binary.LittleEndian, r.Progress)
			binary.Write(b, binary.LittleEndian, r.Goal)
			if e := text(r.Label); e != nil {
				return nil, e
			}
		}
	}
	if b.Len() > MaxBody {
		return nil, fmt.Errorf("view exceeds frame limit")
	}
	return b.Bytes(), nil
}

type Call struct {
	Request Frame
	Reply   chan Frame
}
