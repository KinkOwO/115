package main

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
)

// N2255 reader142530c30 stores these four source-index/status pairs; the UI
// uses the source index for C2046 and the phase ordinal for C2045.
func (w *worldSession) ispinsInfoPayload(state string, nonce [5]byte) ([]byte, error) {
	p, err := legion.IspinsInfoPayload(state, nonce)
	if err != nil || w.ispins == nil || !w.ispins.sourceCards || state == "initial" {
		return p, err
	}
	phase := int(binary.LittleEndian.Uint32(p[11:]))
	if phase < 0 || phase >= len(w.ispins.cards) {
		return p, nil
	}
	if err := w.ensureIspinsCards(phase); err != nil {
		return nil, err
	}
	if strings.HasPrefix(state, "dungeon") {
		p[2] = 0xff
	}
	for slot, card := range w.ispins.cards[phase] {
		at := 51 + 8*slot
		binary.LittleEndian.PutUint16(p[at:], card.Index)
		binary.LittleEndian.PutUint16(p[at+2:], card.Value)
		binary.LittleEndian.PutUint32(p[at+4:], card.Grade)
		if strings.HasPrefix(state, "dungeon") && card.Index == w.ispins.operationIndex {
			p[2] = byte(slot)
		}
	}
	if strings.HasPrefix(state, "dungeon") && p[2] == 0xff {
		return nil, fmt.Errorf("Ispins selected card not owned")
	}
	return p, nil
}

func (w *worldSession) ensureIspinsCards(phase int) error {
	if w == nil || w.ispins == nil || phase < 0 || phase >= len(w.ispins.cards) {
		return fmt.Errorf("Ispins phase/run unavailable")
	}
	if w.ispins.cardsOwned[phase] {
		return nil
	}
	if w.dungeons == nil {
		return fmt.Errorf("Ispins operation source missing")
	}
	cards, err := catalog.ChooseIspinsCards(w.dungeons.IspinsOperations, "party", "normal", func(bound int64) (int64, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(bound))
		if err != nil {
			return 0, err
		}
		return n.Int64(), nil
	})
	if err != nil {
		return err
	}
	w.ispins.cards[phase], w.ispins.cardsOwned[phase] = cards, true
	return nil
}
