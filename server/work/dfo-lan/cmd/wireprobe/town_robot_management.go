package main

import (
	"crypto/rand"
	"crypto/subtle"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Stored apart from player saves. Records contain owned item instances, not
// exported source catalogs. A stable ordinal survives scene actor reassignment.
type townRobotRecord struct {
	ID       int                `json:"id"`
	Revision uint64             `json:"revision"`
	Role     database.Character `json:"role"`
}
type townRobotArchive struct {
	Version int                     `json:"version"`
	Records map[int]townRobotRecord `json:"records"`
}
type townRobotManager struct {
	display                 map[uint32]catalog.ItemDisplay
	displayReady            bool
	displayError            string
	path                    string
	records                 map[int]townRobotRecord
	characters              *character.Service
	award                   *inventory.Awarder
	wear                    *inventory.WearService
	growth                  *character.ProgressionService
	items                   *inventory.ItemService
	server                  *http.Server
	host, token, accessFile string
}

//go:embed town_robot_management.html
var townRobotManagementHTML string

func readTownRobotArchive(path string) (map[int]townRobotRecord, error) {
	records := map[int]townRobotRecord{}
	if path == "" {
		return records, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	var a townRobotArchive
	if err = json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if a.Version != 1 || a.Records == nil || len(a.Records) > 100 {
		return nil, fmt.Errorf("unsupported robot archive")
	}
	for id, r := range a.Records {
		if id < 1 || id > 100 || r.ID != id || r.Role.ID != -int64(id) || r.Role.AccountID != 0 || r.Role.WireID != 0 || r.Revision == 0 || len(r.Role.ConfigVersion) != 64 || !json.Valid(r.Role.State) {
			return nil, fmt.Errorf("invalid robot record %d", id)
		}
		if _, err := inventory.ReadBag(r.Role.State); err != nil {
			return nil, err
		}
	}
	return a.Records, nil
}

// A failed write never replaces the in-memory authority. A same-directory
// temporary file is flushed and renamed before publishing the new revision.
func (m *townRobotManager) save(next map[int]townRobotRecord) error {
	if m.path == "" {
		m.records = next
		return nil
	}
	b, err := json.MarshalIndent(townRobotArchive{Version: 1, Records: next}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(m.path), ".robots-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, m.path); err != nil {
		return err
	}
	m.records = next
	return nil
}

func (m *townRobotManager) store(r townRobotRecord) error {
	next := make(map[int]townRobotRecord, len(m.records)+1)
	for k, v := range m.records {
		next[k] = v
	}
	next[r.ID] = r
	return m.save(next)
}

func (f *townRobotFleet) managedRole(w *worldSession, id int, role database.Character) (database.Character, error) {
	m := f.manager
	if m == nil {
		return role, nil
	}
	projection := *w.characters
	projection.Store = nil
	projection.ProfileFrames = nil
	projection.Rewards = nil
	m.characters = &projection
	if w.items != nil {
		items := *w.items
		m.items = &items
	}
	if w.progression != nil {
		growth := *w.progression
		growth.Store = nil
		growth.Rewards = nil
		m.growth = &growth
	}
	if w.loot != nil && w.characters.Equipment != nil {
		m.award = &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.characters.Equipment}
		wear := inventory.WearService{Catalog: w.characters.Equipment, Professions: w.characters.Catalog, BagRules: w.loot.BagRules, Rules: w.characters.WearRules}
		wear.PremiumStore = nil
		m.wear = &wear
	}
	r, ok := m.records[id]
	if !ok {
		saved := role
		saved.ID = -int64(id)
		saved.WireID = 0
		saved.AccountID = 0
		r = townRobotRecord{ID: id, Revision: 1, Role: saved}
		if err := m.store(r); err != nil {
			return role, err
		}
	}
	if r.Role.ConfigVersion != role.ConfigVersion {
		return role, fmt.Errorf("robot %d source identity mismatch; migration required", id)
	}
	result := r.Role
	result.State = append(json.RawMessage(nil), r.Role.State...)
	result.WireID = role.WireID
	return result, nil
}

type townRobotChange struct {
	Emblem      protocol.UseEmblemRequest `json:"emblem"`
	Advancement byte                      `json:"advancement"`
	Awakening   byte                      `json:"awakening"`
	Skills      protocol.SkillPurchase    `json:"skills"`
	ID          int                       `json:"id"`
	Revision    uint64                    `json:"revision"`
	Action      string                    `json:"action"`
	Template    uint32                    `json:"template"`
	Amount      uint32                    `json:"amount"`
	Level       int                       `json:"level"`
	Move        protocol.ItemMoveRequest  `json:"move"`
}

// Called with fleet.mu held. Reuse native bag/wear checks and validate both
// scene and companion snapshots before committing. No player store is used.
func (f *townRobotFleet) changeRobot(q townRobotChange) error {
	m := f.manager
	if m == nil || m.characters == nil || m.award == nil || m.wear == nil {
		return fmt.Errorf("请先进入公共城镇，等待假人生成")
	}
	r, ok := m.records[q.ID]
	if !ok || r.Revision != q.Revision {
		return fmt.Errorf("假人或版本已变化，请刷新后重试")
	}
	if r.Role.ConfigVersion != m.characters.Catalog.Source.SaveIdentity() {
		return fmt.Errorf("假人资源身份不一致")
	}
	// The admitted party holds an immutable character snapshot. Require leave
	// before progression so the HUD, dungeon entity and saved skills agree.
	if q.Action == "advancement" || q.Action == "awakening" || q.Action == "skills" || q.Action == "emblem" {
		for _, party := range f.parties {
			for _, bot := range party.bots {
				if bot.recordID == q.ID {
					return fmt.Errorf("请先将假人请出队伍，再调整职业或技能")
				}
			}
		}
	}
	r.Role.State = append(json.RawMessage(nil), r.Role.State...)
	var err error
	switch q.Action {
	case "advancement":
		var state character.State
		if err = json.Unmarshal(r.Role.State, &state); err != nil {
			return err
		}
		if state.Advancement != 0 {
			return fmt.Errorf("已转职假人不能直接切换职业，避免遗留不兼容的技能和装备")
		}
		r.Role.State, err = m.characters.ApplyAdvancement(r.Role, q.Advancement)
	case "awakening":
		r.Role.State, err = m.characters.ApplyAwakening(r.Role, q.Awakening)
	case "skills":
		r.Role.State, err = m.characters.ApplySkillPurchase(r.Role, q.Skills)
	case "emblem":
		if m.items == nil {
			return fmt.Errorf("徽章规则未就绪")
		}
		role := inventory.Role{Profession: r.Role.Profession, ConfigVersion: r.Role.ConfigVersion, State: r.Role.State}
		var seq uint64
		seq, err = m.items.EmblemInlaySequence(role)
		if err == nil {
			r.Role.State, _, err = m.items.PrepareEmblemInlay(role, q.Emblem, seq)
		}
	case "grant":
		if q.Template == 0 || q.Amount < 1 || q.Amount > 50 {
			return fmt.Errorf("请选择真实物品，数量须为1至50")
		}
		r.Role.State, err = m.grant(r.Role.State, q.Template, q.Amount)
	case "move":
		r.Role.State, err = m.wear.MoveOrdinary(inventory.Role{Profession: r.Role.Profession, ConfigVersion: r.Role.ConfigVersion, State: r.Role.State}, q.Move)
	case "level":
		if m.growth == nil || q.Level < 2 || q.Level > int(m.growth.Rules.LevelCap) {
			return fmt.Errorf("等级超出当前PVF范围")
		}
		var before character.State
		if err = json.Unmarshal(r.Role.State, &before); err != nil {
			return err
		}
		if q.Level <= int(before.Level) {
			return fmt.Errorf("目标等级必须高于当前等级")
		}
		r.Role, err = m.growth.AdminLevel(r.Role, q.Level)
	default:
		return fmt.Errorf("不支持的假人操作")
	}
	if err != nil {
		return err
	}
	// Encoding needs a valid scene wire identity, never persist it.
	projection := r.Role
	projection.WireID = uint16(60000 + q.ID)
	if _, err = m.characters.EntryBasicProbe(projection, [2]byte{}); err != nil {
		return err
	}
	if _, err = m.characters.TownRobotEntryAddition(projection); err != nil {
		return err
	}
	if _, err = m.characters.TownRobotCompanionSnapshot(projection); err != nil {
		return fmt.Errorf("同伴装备编码不支持：%w", err)
	}
	r.Revision++
	if err = m.store(r); err != nil {
		return err
	}
	f.refreshRobot(q.ID)
	log.Printf("town_robot_management id=%d revision=%d action=%s template=%d amount=%d player_store=false", r.ID, r.Revision, q.Action, q.Template, q.Amount)
	return nil
}

// Ordinary Awarder does not route avatar wardrobes. Reuse the existing mail
// delivery primitive, including source default sockets and native capacity.
func (m *townRobotManager) grant(raw json.RawMessage, template, amount uint32) (json.RawMessage, error) {
	if m.award.Equipment.IsAvatarBagItem(template) {
		bag, err := inventory.ReadBag(raw)
		if err != nil {
			return nil, err
		}
		for n := uint32(0); n < amount; n++ {
			bag, err = bag.AddMailItem(m.award.Catalog, m.award.Rules, m.award.Equipment, inventory.MailItem{Space: 1, Equipment: &inventory.BagEquipment{Template: template, Period: inventory.GrantExpireTime}})
			if err != nil {
				return nil, err
			}
		}
		return inventory.SaveBag(raw, bag)
	}
	if kind, err := m.award.Equipment.EquipmentKind(template); err == nil && kind == "[creature]" {
		// The shop's existing source-backed delivery owns creature bag capacity
		// and instance placement. Never deposit a pet in the ordinary gear list.
		d, err := m.award.Equipment.Definition(template)
		if err != nil {
			return nil, err
		}
		p := &cashshop.Pilot{ItemCatalog: map[uint32]cashshop.ItemInfo{template: {Kind: "equipment", Path: d.Path}}}
		return p.DeliverCreatureForManagement(raw, template, amount)
	}
	next, _, err := m.award.Grant(raw, template, amount)
	return next, err
}

func (f *townRobotFleet) refreshRobot(id int) {
	m := f.manager
	for _, b := range f.bots {
		if b.recordID != id {
			continue
		}
		role := m.records[id].Role
		role.WireID = b.peer.actorID
		identity, ok := townRobotSceneIdentity(b.peer)
		if !ok {
			continue
		}
		basic, err := m.characters.EntryBasicProbe(role, identity)
		if err != nil {
			continue
		}
		addition, err := m.characters.TownRobotEntryAddition(role)
		if err != nil {
			continue
		}
		f.hub.mu.Lock()
		b.peer.info, b.peer.addition = basic, addition
		others := f.hub.sharedLocked(b.peer)
		f.hub.mu.Unlock()
		for _, p := range others {
			if p.send != nil {
				p.send(0, 2, basic)
				p.send(0, 2, addition)
			}
		}
	}
}

type townRobotItemView struct {
	List     byte    `json:"list"`
	Slot     uint16  `json:"slot"`
	Template uint32  `json:"template"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	WearSlot *uint16 `json:"wear_slot,omitempty"`
	Details  any     `json:"details,omitempty"`
}

func (m *townRobotManager) itemView(list byte, e inventory.BagEquipment) townRobotItemView {
	v := townRobotItemView{List: list, Slot: e.Slot, Template: e.Template, Name: fmt.Sprint(e.Template), Details: e}
	if m.award != nil {
		if row, ok := m.award.Catalog.Items[e.Template]; ok {
			v.Name = row.Script.Path
		}
		if d, err := m.award.Equipment.Definition(e.Template); err == nil {
			v.Details = map[string]any{"instance": e, "source_path": d.Path, "source_sha256": d.SHA256, "source_fields": d.Fields}
			if cells := d.Fields["[equipment type]"]; len(cells) > 0 {
				v.Kind = cells[0].Text
			}
			if m.wear != nil {
				if s, ok := m.wear.Rules.Slots[v.Kind]; ok {
					v.WearSlot = &s
				}
			}
		}
	}
	if d, ok := m.display[e.Template]; ok && d.Name != "" {
		v.Name = d.Name
	}
	return v
}

func (f *townRobotFleet) managementData() any {
	m := f.manager
	rows := []any{}
	for id := 1; id <= 100; id++ {
		r, ok := m.records[id]
		if !ok {
			continue
		}
		var state character.State
		_ = json.Unmarshal(r.Role.State, &state)
		bag, err := inventory.ReadBag(r.Role.State)
		if err != nil {
			continue
		}
		items := []townRobotItemView{}
		for _, e := range bag.Worn {
			items = append(items, m.itemView(3, e))
		}
		for _, e := range bag.Equipment {
			items = append(items, m.itemView(0, e))
		}
		for _, list := range []byte{1, 7} {
			for _, e := range bag.Special[list] {
				items = append(items, m.itemView(list, e))
			}
		}
		actors := []uint16{}
		for _, b := range f.bots {
			if b.recordID == id {
				actors = append(actors, b.peer.actorID)
			}
		}
		branches := []int{}
		if m.characters != nil {
			for branch := range m.characters.Catalog.Professions[r.Role.Profession].AdvancementGrowth {
				branches = append(branches, int(branch))
			}
		}
		sort.Ints(branches)
		rows = append(rows, map[string]any{"advancements": branches, "id": id, "revision": r.Revision, "name": r.Role.Name, "profession": r.Role.Profession, "level": state.Level, "advancement": state.Advancement, "awakening": state.Awakening, "learned_skills": state.LearnedSkills[0], "skill_points": state.SkillPoints[0], "attributes": state.Attributes, "items": items, "stackables": bag.Items, "actors": actors})
	}
	return map[string]any{"robots": rows, "item_names_ready": m.displayReady, "item_names_error": m.displayError, "native_party_supported": false, "native_inspect_supported": false, "combat_equipment_verified": false, "grant_mode": "管理员发放至假人独立背包，不扣玩家物品"}
}

func (f *townRobotFleet) managementHandler(w http.ResponseWriter, r *http.Request) {
	m := f.manager
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Host != m.host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+m.host) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.URL.Path == "/" && r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, townRobotManagementHTML)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Robot-Key")), []byte(m.token)) != 1 {
		http.Error(w, "Forbidden", 403)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.stop:
		http.Error(w, "Stopping", 503)
		return
	default:
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch {
	case r.URL.Path == "/api/robots" && r.Method == "GET":
		json.NewEncoder(w).Encode(f.managementData())
	case r.URL.Path == "/api/skills" && r.Method == "POST":
		var q struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&q); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		record, ok := m.records[q.ID]
		if !ok || m.characters == nil {
			http.Error(w, "unknown robot", 400)
			return
		}
		rows, err := m.characters.CompanionSkillChoices(record.Role)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(rows)
	case r.URL.Path == "/api/items" && r.Method == "GET":
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		rows := []map[string]any{}
		if query != "" && m.award != nil {
			ids := []uint32{}
			for id, item := range m.award.Catalog.Items {
				if strings.Contains(m.display[id].Name, query) || strings.Contains(item.Script.Path, query) || fmt.Sprint(id) == query {
					ids = append(ids, id)
				}
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, id := range ids {
				if len(rows) >= 100 {
					break
				}
				item := m.award.Catalog.Items[id]
				name := m.display[id].Name
				if name == "" {
					name = item.Script.Path
				}
				rows = append(rows, map[string]any{"id": id, "name": name, "kind": item.Kind})
			}
		}
		json.NewEncoder(w).Encode(rows)
	case r.URL.Path == "/api/change" && r.Method == "POST":
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "JSON required", 415)
			return
		}
		var q townRobotChange
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if err := d.Decode(&q); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if d.Decode(new(any)) != io.EOF {
			http.Error(w, "Trailing data", 400)
			return
		}
		if err := f.changeRobot(q); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		json.NewEncoder(w).Encode(f.managementData())
	default:
		http.NotFound(w, r)
	}
}

func (f *townRobotFleet) startManagement() error {
	path := os.Getenv("DFO_TOWN_ROBOT_STORAGE")
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	records, err := readTownRobotArchive(abs)
	if err != nil {
		return err
	}
	m := &townRobotManager{path: abs, records: records}
	f.manager = m
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return err
	}
	m.token = hex.EncodeToString(key)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	m.host = l.Addr().String()
	m.server = &http.Server{Handler: http.HandlerFunc(f.managementHandler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	m.accessFile = abs + ".access.json"
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		l.Close()
		return err
	}
	access, _ := json.Marshal(map[string]any{"url": "http://" + m.host + "/#" + m.token, "pid": os.Getpid(), "storage": abs})
	if err = os.WriteFile(m.accessFile, access, 0600); err != nil {
		l.Close()
		return err
	}
	go func() {
		if err := m.server.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Printf("robot management: %v", err)
		}
	}()
	go f.loadManagementNames()
	log.Printf("town robot management ready: access_file=%s loopback_only=true", m.accessFile)
	return nil
}

// Build display-only metadata from the same pinned archive. It never becomes
// a grant rule or a JSON content fallback. Separate source ownership avoids
// sharing mutable archive caches with the running game catalogs.
func (f *townRobotFleet) loadManagementNames() {
	m := f.manager
	display := map[uint32]catalog.ItemDisplay{}
	err := func() error {
		s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: os.Getenv("DFO_PVF_ARCHIVE"), ExpectedChecksum: os.Getenv("DFO_PVF_SHA256"), StartupHashWarnings: true})
		if err != nil {
			return err
		}
		defer s.Close()
		index, err := s.ItemIndex("")
		if err != nil {
			return err
		}
		return s.VisitItemDisplay(index, func(d catalog.ItemDisplay) error {
			select {
			case <-f.stop:
				return fmt.Errorf("stopped")
			default:
			}
			display[d.ID] = d
			return nil
		})
	}()
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		m.displayError = err.Error()
		log.Printf("robot item metadata unavailable: %v", err)
		return
	}
	m.display = display
	m.displayReady = true
	log.Printf("robot item display names ready: %d source_pvf=true", len(display))
}
