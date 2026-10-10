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

// Four entries are a native wire bound; their dungeon IDs remain in the COS.
// The operator explicitly requested a different permutation on each challenge.
func newIspinsOrder(previous [4]byte, hasPrevious bool) ([4]byte, error) {
	for {
		order := [4]byte{0, 1, 2, 3}
		for i := len(order) - 1; i > 0; i-- {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			if err != nil {
				return [4]byte{}, fmt.Errorf("Ispins order entropy: %w", err)
			}
			j := int(n.Int64())
			order[i], order[j] = order[j], order[i]
		}
		if !hasPrevious || order != previous {
			return order, nil
		}
	}
}

func (r *ispinsRun) dungeonIndex(phase int) byte {
	if r.orderOwned {
		return r.order[phase]
	}
	// Existing captured sessions and diagnostic vectors precede owned ordering.
	return legion.IspinsStageDungeonIndex[phase]
}

func (r *ispinsRun) captureStage(phase int) int {
	index := r.dungeonIndex(phase)
	for i, captured := range legion.IspinsStageDungeonIndex {
		if index == captured {
			return i
		}
	}
	panic("invalid owned Ispins dungeon index")
}

// N2255 reader142530c30 stores these four source-index/status pairs; the UI
// uses the source index for C2046 and the phase ordinal for C2045.
func (w *worldSession) ispinsInfoPayload(state string, nonce [5]byte) ([]byte, error) {
	p, err := legion.IspinsInfoPayload(state, nonce)
	if err != nil || w.ispins == nil || !w.ispins.orderOwned || state == "initial" {
		return p, err
	}
	for i, index := range w.ispins.order {
		binary.LittleEndian.PutUint32(p[19+8*i:], uint32(index))
	}
	phase := int(binary.LittleEndian.Uint32(p[11:]))
	if phase < 0 || phase >= len(w.ispins.cards) {
		return nil, fmt.Errorf("invalid native Ispins phase")
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
