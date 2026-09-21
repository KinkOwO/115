package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"testing"
)

// The built-in client block goes out verbatim; only its two lock objects change.
func TestUnifiedCharacPayloadUsesBuiltInBlock(t *testing.T) {
	payload, e := unifiedCharacPayload(nil, []uint16{58, 517}, -1)
	if e != nil {
		t.Fatal(e)
	}
	if len(payload) != protocol.UnifiedCharacOptionSize {
		t.Fatalf("payload length = %d, want %d", len(payload), protocol.UnifiedCharacOptionSize)
	}
	template := protocol.CharacOptionsTemplate()
	block, e := protocol.EncodeSkillLockBlock([]uint16{58, 517})
	if e != nil {
		t.Fatal(e)
	}
	inside := func(i int) bool {
		for _, at := range []int{protocol.UnifiedCharacSkillLockAt, protocol.UnifiedCharacSkillLockSecondAt} {
			if i >= at && i < at+len(block) {
				return true
			}
		}
		return false
	}
	for _, at := range []int{protocol.UnifiedCharacSkillLockAt, protocol.UnifiedCharacSkillLockSecondAt} {
		if !bytes.Equal(payload[at:at+len(block)], block) {
			t.Fatalf("lock object at %d was not written", at)
		}
	}
	for i := range template {
		if inside(i) {
			continue
		}
		if payload[i] != template[i] {
			t.Fatalf("template byte %d changed: %02x", i, payload[i])
		}
	}
}

// A different client build can supply its own block and lock offset; the second
// object is expected at offset+386.
func TestUnifiedCharacPayloadOverridesTemplateAndOffset(t *testing.T) {
	template := make([]byte, protocol.UnifiedCharacOptionSize)
	payload, e := unifiedCharacPayload(template, []uint16{9}, 0)
	if e != nil {
		t.Fatal(e)
	}
	if payload[0] != 1 {
		t.Fatalf("valid byte at offset 0 = %d, want 1", payload[0])
	}
	if payload[protocol.UnifiedSkillLockBlockSize] != 1 {
		t.Fatal("second lock object was not written at offset+386")
	}
	if _, e = unifiedCharacPayload(template, []uint16{9}, len(template)-1); e == nil {
		t.Fatal("offset without room accepted")
	}
	if _, e = unifiedCharacPayload(make([]byte, 64), []uint16{9}, -1); e == nil {
		t.Fatal("template too small for the lock objects accepted")
	}
	tooMany := make([]uint16, protocol.UnifiedSkillSlots+1)
	if _, e = unifiedCharacPayload(nil, tooMany, -1); e == nil {
		t.Fatal("more locked skills than the block holds accepted")
	}
}
