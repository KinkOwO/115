package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

func bakalEntryForTeam(entry catalog.RaidEntrance, request protocol.RaidCreateRequest) (catalog.RaidEntrance, error) {
	mode, err := request.BakalMode115()
	if err != nil {
		return entry, err
	}
	if mode == byte(catalog.BakalHard) {
		if entry.BakalHard == nil || entry.BakalHard.Mode != catalog.BakalHard || entry.HardPartyMax == 0 {
			return entry, fmt.Errorf("native hard Bakal source missing")
		}
		entry.Bakal = entry.BakalHard
		entry.PartyMax = entry.HardPartyMax
	} else if entry.Bakal == nil || entry.Bakal.Mode != catalog.BakalNormal {
		return entry, fmt.Errorf("native normal Bakal source missing")
	}
	return entry, nil
}

func (w *worldSession) bakalModeAdmission(entry catalog.RaidEntrance) error {
	if entry.Bakal == nil {
		return fmt.Errorf("Bakal source missing")
	}
	if entry.Bakal.Mode == catalog.BakalHard {
		if w.characters == nil || entry.HardLimitFame == 0 {
			return fmt.Errorf("hard Bakal fame source missing")
		}
		fame, err := w.characters.EquipmentFame(w.role.State)
		if err != nil {
			return err
		}
		if fame < entry.HardLimitFame {
			return fmt.Errorf("困难巴卡尔需要名望%d，当前%d", entry.HardLimitFame, fame)
		}
	}
	return nil
}
