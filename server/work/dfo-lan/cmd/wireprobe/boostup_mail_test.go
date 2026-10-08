package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func assertBoostMailSQL(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	must115(t, store.MigrateMailbox(ctx))
	account, e := store.DevelopmentAccount(ctx, "boost-mail-fixture")
	must115(t, e)
	version := strings.Repeat("a", 64)
	intent := boostup.GraduationMail{Item: 590015954, Count: 1, Sender: "Starter Boost", Body: "earned reward"}
	c := &boostup.Catalog{Town: 222, ReservedMail: &intent, Steps: []boostup.Step{{Number: 1, Area: 0, Guide: "normal", Mission: "none", Rewards: []boostup.Reward{{Item: 6001, Count: 1}}}}}
	ls := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.LootItem{6001: {ID: 6001, Kind: "stackable", StackableType: "[booster]", StackLimit: 10}, 590015954: {ID: 590015954, Kind: "stackable", StackableType: "[booster]", StackLimit: 10}}}, BagRules: inventory.BagRules{Source: version, Slots: map[string][2]uint16{"[booster]": {65, 67}}, MissingStackLimit: 10}}
	lsvc := &workflow.LootService{Store: store, Loot: ls}
	create := func(name string, finished bool) database.Character {
		st := boostup.State{Version: 1, Activated: true, PendingMail: &intent, Training: boostup.Training{Step: 1, Phase: 1}}
		if finished {
			st.Training = boostup.Training{Step: 2, Finished: true, Claimed: map[byte]bool{1: true}}
		}
		raw, err := boostup.WriteState(json.RawMessage(`{"level":115,"untouched":42}`), st)
		must115(t, err)
		r, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, Request: []byte{0}, ConfigVersion: version, State: raw}, 100)
		must115(t, err)
		return r
	}
	role := create("boost-mail-earned", false)
	if _, sent, err := lsvc.DeliverBoostGraduationMail(ctx, role, c); err != nil || sent {
		t.Fatal("mail sent during training", err)
	}
	changed := intent
	changed.Item = 999999
	c.ReservedMail = &changed // should not rewrite a frozen intent
	notified := 0
	w := &worldSession{account: account, role: role, loot: ls, characters: &character.Service{Store: store}, boostup: c, state: database.WorldState{Position: database.WorldPosition{Town: 222}}, notifyBoostMail: func(id int64) {
		if id != role.ID {
			t.Fatal("wrong notified owner")
		}
		notified++
	}}
	request := make([]byte, 8)
	binary.LittleEndian.PutUint32(request, 662)
	binary.LittleEndian.PutUint32(request[4:], 1)
	plan, e := w.boostStepRequest(ctx, request, 680)
	must115(t, e)
	rosterAt, finishedAt, ackAt := -1, -1, -1
	for i, p := range plan {
		if p.Kind == 0 && p.ID == 2639 {
			rosterAt = i
		}
		if p.Kind == 0 && p.ID == 2638 {
			finishedAt = i
		}
		if p.Kind == 1 && p.ID == 680 {
			ackAt = i
		}
	}
	if rosterAt < 0 || finishedAt <= rosterAt || ackAt <= finishedAt {
		t.Fatal("graduation ordering differs from captured final claim", rosterAt, finishedAt, ackAt)
	}
	st, e := boostup.ReadState(w.role.State)
	must115(t, e)
	if !st.MailSent || st.PendingMail != nil || notified != 1 {
		t.Fatal("mail completion state/notification", st, notified)
	}
	mails, e := store.Mailbox(ctx, account, role.ID)
	must115(t, e)
	if len(mails) != 1 || len(mails[0].Assets) != 1 {
		t.Fatal("missing/duplicate graduate mail")
	}
	var item inventory.MailItem
	must115(t, json.Unmarshal(mails[0].Assets[0].Item, &item))
	if item.Stack == nil || item.Stack.Template != 590015954 || item.Stack.Amount != 1 {
		t.Fatal("frozen reward changed")
	}
	w.flushBoostGraduationMail()
	if notified != 1 {
		t.Fatal("duplicate mailbox notification")
	}
	keys := make([]byte, wire.SessionKeyBytes)
	claim := make([]byte, 16)
	binary.LittleEndian.PutUint32(claim[1:], 1)
	binary.LittleEndian.PutUint64(claim[5:], uint64(mails[0].Assets[0].ID))
	_, _, e = w.claimMail(ctx, claim, keys, "boost-mail-claim")
	must115(t, e)
	_, _, e = w.claimMail(ctx, claim, keys, "boost-mail-claim")
	must115(t, e)
	bag, e := inventory.ReadBag(w.role.State)
	must115(t, e)
	count := uint32(0)
	for _, i := range bag.Items {
		if i.Template == 590015954 {
			count += i.Amount
		}
	}
	if count != 1 {
		t.Fatal("mail claim missing/duplicated", count)
	}
	// Simulate crash after CommitSystemMail committed but before marking the
	// event state. The actual mail receipt must stop a second delivery.
	c.ReservedMail = &intent
	resume := create("boost-mail-resume", false)
	resume, _, _, e = lsvc.BoostStepRequest(ctx, resume, c, 1, true)
	must115(t, e)
	assets, e := database.SystemMailAssets(0, []database.GrantItem{{Template: intent.Item, Amount: 1}})
	must115(t, e)
	_, sent, e := store.CommitSystemMail(ctx, account, resume.ID, version, "boostup-max-level-v1", "system-mail-v1", intent.Sender, intent.Body, assets)
	must115(t, e)
	if !sent {
		t.Fatal("fixture did not simulate initial delivery")
	}
	resume, sent, e = lsvc.DeliverBoostGraduationMail(ctx, resume, c)
	must115(t, e)
	if !sent {
		t.Fatal("crash recovery did not request the missing mailbox notification")
	}
	mails, e = store.Mailbox(ctx, account, resume.ID)
	must115(t, e)
	if len(mails) != 1 {
		t.Fatal("mail duplicated during recovery")
	}
	st, e = boostup.ReadState(resume.State)
	must115(t, e)
	if !st.MailSent || st.PendingMail != nil {
		t.Fatal("recovery did not clear pending state")
	}
	fake := create("boost-mail-no-proof", true)
	if _, sent, e = lsvc.DeliverBoostGraduationMail(ctx, fake, c); e == nil || sent {
		t.Fatal("finished flag without receipt granted mail")
	}
	// Missing source holds a durable promise rather than consuming it.
	pending := create("boost-mail-missing", false)
	pending, _, _, e = lsvc.BoostStepRequest(ctx, pending, c, 1, true)
	must115(t, e)
	def := ls.Catalog.Items[590015954]
	delete(ls.Catalog.Items, 590015954)
	if _, sent, e = lsvc.DeliverBoostGraduationMail(ctx, pending, c); e == nil || sent {
		t.Fatal("missing source silently granted")
	}
	ls.Catalog.Items[590015954] = def
	pending, sent, e = lsvc.DeliverBoostGraduationMail(ctx, pending, c)
	must115(t, e)
	if !sent {
		t.Fatal("pending source repair could not recover")
	}
	// The source's level-up bonus is a SECOND mail, with a distinct receipt.
	c.GoalLevel = 115
	c.ChallengeLevelRewards = map[byte]boostup.Reward{115: {Item: 590015933, Count: 1}}
	ls.Catalog.Items[590015933] = catalog.LootItem{ID: 590015933, Kind: "stackable", StackableType: "[booster]", StackLimit: 10}
	graduate := create("boost-mail-two-types", false)
	if _, sent, e = lsvc.DeliverBoostLevelBonusMail(ctx, graduate, c); e != nil || sent {
		t.Fatal("level mail sent before training finished", e)
	}
	alarms := 0
	w2 := &worldSession{account: account, role: graduate, loot: ls, characters: &character.Service{Store: store}, boostup: c, state: database.WorldState{Position: database.WorldPosition{Town: 222}}, notifyBoostMail: func(int64) { alarms++ }}
	_, e = w2.boostStepRequest(ctx, request, 680)
	must115(t, e)
	mails, e = store.Mailbox(ctx, account, graduate.ID)
	must115(t, e)
	if len(mails) != 2 || alarms != 1 {
		t.Fatal("two mail kinds were collapsed or notification duplicated", len(mails), alarms)
	}
	templates := map[uint32]uint32{}
	for _, mail := range mails {
		for _, asset := range mail.Assets {
			var it inventory.MailItem
			must115(t, json.Unmarshal(asset.Item, &it))
			if it.Stack == nil {
				t.Fatal("not a source box")
			}
			templates[it.Stack.Template] += it.Stack.Amount
		}
	}
	if templates[590015954] != 1 || templates[590015933] != 1 {
		t.Fatal("wrong independent reward sources", templates)
	}
	w2.flushBoostGraduationMail()
	if alarms != 1 {
		t.Fatal("settled level bonus re-notified")
	}
	st, e = boostup.ReadState(w2.role.State)
	must115(t, e)
	if !st.MailSent || !st.LevelBonusSent || st.PendingMail != nil || st.PendingLevelBonus != nil {
		t.Fatal("mail kinds did not settle independently")
	}
	// Existing system-mail receipt survives a stop before the character marker.
	resumeBonus := create("boost-level-mail-resume", false)
	resumeBonus, _, _, e = lsvc.BoostStepRequest(ctx, resumeBonus, c, 1, true)
	must115(t, e)
	bonus := c.CapsuleLevelMail(115)
	assets, e = database.SystemMailAssets(0, []database.GrantItem{{Template: bonus.Item, Amount: bonus.Count}})
	must115(t, e)
	_, sent, e = store.CommitSystemMail(ctx, account, resumeBonus.ID, version, "boostup-challenge-level-v1", "system-mail-v1", bonus.Sender, bonus.Body, assets)
	must115(t, e)
	if !sent {
		t.Fatal("bonus crash fixture missing")
	}
	resumeBonus, sent, e = lsvc.DeliverBoostLevelBonusMail(ctx, resumeBonus, c)
	must115(t, e)
	if !sent {
		t.Fatal("bonus crash recovery lost notification")
	}
	mails, e = store.Mailbox(ctx, account, resumeBonus.ID)
	must115(t, e)
	if len(mails) != 1 {
		t.Fatal("level bonus duplicated after crash")
	}
	if _, sent, e = lsvc.DeliverBoostLevelBonusMail(ctx, fake, c); e == nil || sent {
		t.Fatal("bonus granted without graduation receipt")
	}
}
