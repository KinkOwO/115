package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/legion"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func ispinsWeeklyLimited() (bool, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DFO_ISPINS_MODE"))) {
	case "", "unlimited":
		return false, nil
	case "weekly":
		return true, nil
	default:
		return false, fmt.Errorf("DFO_ISPINS_MODE must be unlimited or weekly")
	}
}
func (w *worldSession) ispinsWeeklyUsed() (bool, error) {
	limited, e := ispinsWeeklyLimited()
	if e != nil || !limited {
		return false, e
	}
	if w == nil || w.store == nil || w.role.ID == 0 {
		return false, fmt.Errorf("伊斯每周模式缺少角色存储")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.store.IspinsWeeklyUsed(ctx, w.account, w.role.ID, time.Now())
}
func (w *worldSession) ispinsStandbyQuotaInfo() ([]byte, error) {
	used, e := w.ispinsWeeklyUsed()
	if e != nil {
		return nil, e
	}
	if used {
		return legion.IspinsSpentStandbyEntryCharacterInfo()
	}
	return legion.IspinsStandbyEntryCharacterInfo([5]byte{})
}

func (w *worldSession) ispinsLoginQuotaInfo() ([]byte, error) {
	used, e := w.ispinsWeeklyUsed()
	if e != nil {
		return nil, e
	}
	if used {
		return legion.IspinsSpentStandbyEntryCharacterInfo()
	}
	return legion.IspinsEntryCharacterInfo(true, [4]bool{}, [5]byte{})
}
func (w *worldSession) recordIspinsFullClear() error {
	limited, e := ispinsWeeklyLimited()
	if e != nil {
		return e
	}
	if w.store == nil {
		if limited {
			return fmt.Errorf("伊斯每周模式缺少角色存储")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := w.store.RecordIspinsWeeklyClear(ctx, w.account, w.role.ID, w.role.ConfigVersion, w.activeDungeon.RunID, time.Now(), limited)
	if err != nil && !limited {
		// Unlimited is the confirmed baseline: a bookkeeping outage must not
		// replace its working settlement with a new failure gate.
		log.Printf("伊斯无限模式通关回执未记录，正常结算保留：角色=%d 错误=%v", w.role.ID, err)
		return nil
	}
	return err
}
func (w *worldSession) checkIspinsWeeklyAdmission() error {
	used, e := w.ispinsWeeklyUsed()
	if e != nil {
		return e
	}
	if used {
		return database.ErrIspinsWeeklyCleared
	}
	return nil
}
