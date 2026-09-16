package protocol

import (
	"encoding/hex"
	"testing"
)

func TestObservedQuestRequest(t *testing.T) {
	p, _ := hex.DecodeString("1f00490c000000000000000000000000")
	q, e := DecodeQuestRequest(31, p)
	if e != nil || q != 3145 {
		t.Fatalf("quest=%d err=%v", q, e)
	}
	p[0] = 32
	if _, e = DecodeQuestRequest(31, p); e == nil {
		t.Fatal("wrong operation accepted")
	}
	if hex.EncodeToString(QuestAccepted(3145, 0)) != "01490c0000000000" {
		t.Fatal("native accept reader sequence")
	}
}
