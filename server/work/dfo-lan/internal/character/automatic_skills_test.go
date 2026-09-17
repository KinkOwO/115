package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func autoSkillFixture(t *testing.T) (*Service, storage.Character, State) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.auto-skills-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.awakening-candidate.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	p := c.Professions[11]
	return &Service{Catalog: c, Learning: l}, storage.Character{Profession: 11, ConfigVersion: c.Source.Checksum}, State{Level: 35, Advancement: 2, AllJobsPilot: true, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills}
}

func TestAutomaticSpathaNoctis(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	base, err := knownSkills(st, 0)
	if err != nil || base[62] != 0 {
		t.Fatal(base, err)
	}
	t.Log("BASELINE: Spatha Noctis rank0")
	for _, tree := range []int{0, 1} {
		known, err := s.knownSkills(role, st, tree)
		if err != nil || known[62] != 1 {
			t.Fatal(known, err)
		}
		rows, err := s.skillRows(role, st, tree)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range rows {
			if r.ID == 62 {
				found = r.Level == 1 && r.Slot >= 14 && r.Slot < 255
			}
		}
		if !found {
			t.Fatal("automatic skill absent from palette")
		}
		count := 0
		for _, d := range s.Learning.index[11] {
			pre := d.Ints("[pre required skill]")
			for i := 0; i+1 < len(pre); i += 2 {
				if pre[i] == 62 && d.ForAdvancement(2) {
					if _, err = d.costForState(st, 1, known); err == nil {
						count++
					}
				}
			}
		}
		if count == 0 {
			t.Fatal("dependent skills still blocked")
		}
	}
	st.Level = 14
	known, err := s.knownSkills(role, st, 0)
	if err != nil || known[62] != 0 {
		t.Fatal("early grant", known, err)
	}
	st.Level, st.Advancement = 35, 1
	known, err = s.knownSkills(role, st, 0)
	if err != nil || known[62] != 0 {
		t.Fatal("foreign advancement grant", known, err)
	}
	t.Log("MODIFIED: Spatha Noctis rank1; prerequisites available; level14 and foreign advancement not granted")
}

func TestAutomaticAllProfessionGrants(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	for job, p := range s.Catalog.Professions {
		for adv := range p.AdvancementSkills {
			role.Profession = job
			st.SourceSHA256, st.InitialSkills, st.Advancement, st.Level = p.RawSHA256, p.InitialSkills, adv, 115
			if _, err := s.skillRows(role, st, 0); err != nil {
				t.Fatalf("job%d advancement%d: %v", job, adv, err)
			}
		}
	}
}

func TestAutomaticSkillPersistence(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated schema integration")
	}
	ctx := context.Background()
	cfg, err := storage.LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("automatic_skills_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateGrants, store.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role, st := autoSkillFixture(t)
	s.Store = store
	st.SkillPoints[0] = 1000
	st.LearnedSkills[0] = map[uint16]byte{46: 5}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "auto-skills-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID, role.Name, role.State, role.Request = account, "AutoFixture", raw, []byte{0}
	role, err = store.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 503
	known, err := s.knownSkills(role, st, 0)
	if err != nil || known[46] != 5 {
		t.Fatal("existing purchased rank overwritten", known, err)
	}
	var dependent uint16
	var cost int
	for id, d := range s.Learning.index[11] {
		pre := d.Ints("[pre required skill]")
		for i := 0; i+1 < len(pre); i += 2 {
			if pre[i] == 62 && known[id] == 0 {
				if c, e := d.costForState(st, 1, known); e == nil && (dependent == 0 || id < dependent) {
					dependent, cost = id, c
				}
			}
		}
	}
	if dependent == 0 {
		t.Fatal("no dependent skill fixture")
	}
	req := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: dependent, Delta: 1}}}
	saved, applied, err := s.Learn(ctx, role, "dependent", req)
	if err != nil || !applied {
		t.Fatal("dependent purchase", applied, err)
	}
	again, applied, err := s.Learn(ctx, role, "dependent", req)
	if err != nil || applied {
		t.Fatal("retry", applied, err)
	}
	var a, b State
	json.Unmarshal(saved.State, &a)
	json.Unmarshal(again.State, &b)
	if !reflect.DeepEqual(a, b) || a.SkillPoints[0] != uint16(1000-cost) || a.LearnedSkills[0][46] != 5 {
		t.Fatal("SP/persistence/retry mismatch")
	}
	refund := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 62, Delta: 1, Refund: 1}}}
	if _, _, err = s.Learn(ctx, saved, "refund-free", refund); err == nil {
		t.Fatal("free ranks refunded into SP")
	}
	roles, err := store.Characters(ctx, account)
	if err != nil || len(roles) != 1 {
		t.Fatal(err)
	}
	var loaded State
	if err = json.Unmarshal(roles[0].State, &loaded); err != nil {
		t.Fatal(err)
	}
	known, err = s.knownSkills(roles[0], loaded, 0)
	if err != nil || known[62] != 1 || known[dependent] != 1 || loaded.SkillPoints[0] != a.SkillPoints[0] {
		t.Fatal("relogin mismatch", err)
	}
	if _, err = s.EntrySkills(roles[0]); err != nil {
		t.Fatal("relogin payload", err)
	}
	t.Logf("DB PASS: skill%d learned with free62 prerequisite; SP=%d; retry idempotent; refund62 rejected; relog preserved", dependent, a.SkillPoints[0])
}
