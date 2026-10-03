// npcpresenceaudit evaluates an explicit offline native-operation trace.
// It never connects to storage, starts a client, or changes quest decisions.
package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/npcpresence"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

type operation struct {
	Kind        string                      `json:"kind"`
	NPC         uint32                      `json:"npc"`
	Quest       uint32                      `json:"quest"`
	SourceQuest uint32                      `json:"source_quest"`
	Town        uint32                      `json:"town"`
	Phase       int32                       `json:"phase"`
	Condition   int32                       `json:"condition"`
	Apply       *bool                       `json:"apply"`
	Ranks       map[uint32]npcpresence.Rank `json:"ranks"`
}

type query struct {
	Town, Area, NPC uint32
	Phase           *int32           `json:"phase"`
	FinalRoot       string           `json:"final_root"`
	Instances       map[uint32]*bool `json:"instances"`
}

type trace struct {
	Schema     int         `json:"schema_version"`
	Source     string      `json:"source"`
	Accepted   *[]uint32   `json:"accepted_quests"`
	Completed  *[]uint32   `json:"completed_quests"`
	Operations []operation `json:"operations"`
	Queries    []query     `json:"queries"`
}

func questSet(ids *[]uint32) npcpresence.QuestSet {
	s := npcpresence.QuestSet{Known: ids != nil, IDs: make(map[uint32]bool)}
	if ids != nil {
		for _, id := range *ids {
			s.IDs[id] = true
		}
	}
	return s
}

func evaluate(index *npcpresence.Index, t trace) ([]npcpresence.Result, error) {
	if t.Schema != 1 || t.Source != index.Source {
		return nil, fmt.Errorf("trace schema or PVF source identity mismatch")
	}
	accepted, completed := questSet(t.Accepted), questSet(t.Completed)
	visibility := npcpresence.NewVisibilityReplay()
	var phases npcpresence.PhaseCache
	phaseResults := make(map[uint32]*int32)
	for n, op := range t.Operations {
		switch op.Kind {
		case "manager_constructed":
			visibility = npcpresence.NewConstructedVisibilityReplay()
		case "selection_reset":
			phases.SuccessfulSelectionReset()
			phaseResults = make(map[uint32]*int32)
		case "show":
			visibility.RequestShow(op.NPC)
		case "hide":
			visibility.RequestHide(op.NPC)
		case "show_entity":
			visibility.ShowEntity(op.NPC)
		case "add_show_override":
			visibility.AddShowOverride(op.NPC, op.Quest)
		case "remove_show_override":
			visibility.RemoveShowOverride(op.NPC, op.Quest)
		case "clear_protection":
			visibility.ClearProtection()
		case "condition":
			if op.Apply == nil {
				return nil, fmt.Errorf("operation%d condition lacks apply/revert input", n)
			}
			if err := index.ApplyCondition(visibility, op.SourceQuest, op.Quest, op.Condition, *op.Apply); err != nil {
				return nil, fmt.Errorf("operation%d: %w", n, err)
			}
		case "resolve_batch":
			for _, rank := range op.Ranks {
				if rank.Status > npcpresence.RankKnown {
					return nil, fmt.Errorf("operation%d has invalid rank status", n)
				}
			}
			visibility.ResolveBatch(op.Ranks)
		case "observe_phase":
			phases.ObservePhase(op.Town, op.Phase)
			phase := op.Phase
			phaseResults[op.Town] = &phase
		case "resolve_completed_phase":
			if _, bound := index.TownRules[op.Town]; !bound || len(index.TownGaps[op.Town]) != 0 {
				phaseResults[op.Town] = nil
				continue
			}
			phaseResults[op.Town], _ = phases.EvaluateCompleted(op.Town, index.TownRules[op.Town], completed)
		case "gap":
			visibility.Forget()
			phases.Forget()
			phaseResults = make(map[uint32]*int32)
		default:
			return nil, fmt.Errorf("operation%d unsupported kind %q", n, op.Kind)
		}
	}
	var results []npcpresence.Result
	for _, q := range t.Queries {
		instances := make(map[uint32]npcpresence.Truth)
		for id, present := range q.Instances {
			if present == nil {
				continue
			}
			instances[id] = npcpresence.False
			if *present {
				instances[id] = npcpresence.True
			}
		}
		phase := q.Phase
		if phase == nil {
			phase = phaseResults[q.Town]
		}
		results = append(results, index.ResolveNPC(npcpresence.Query{Town: q.Town, Area: q.Area, NPC: q.NPC, Phase: phase, FinalRoot: q.FinalRoot, Instances: instances, Accepted: accepted, Completed: completed, Visibility: visibility}))
	}
	return results, nil
}

func main() {
	flag.String("world", "", "deprecated; world and phase maps are read from native PVF")
	archive := flag.String("pvf-archive", "../client-build/Script.inner.pvf", "read-only inner PVF")
	flag.String("quests", "", "deprecated; quests are read from native PVF")
	traceFile := flag.String("trace", "", "explicit offline trace with source identity")
	outFile := flag.String("output", "", "output report (stdout if omitted)")
	flag.Parse()
	native, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *archive})
	if err != nil {
		log.Fatal(err)
	}
	defer native.Close()
	w, err := native.World("")
	if err != nil {
		log.Fatal(err)
	}
	q, err := native.Quests("")
	if err != nil {
		log.Fatal(err)
	}
	index, err := npcpresence.NewIndex(w, q)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(*traceFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var t trace
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&t); err != nil {
		log.Fatal(err)
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		log.Fatal("trace must contain exactly one JSON document")
	}
	results, err := evaluate(index, t)
	if err != nil {
		log.Fatal(err)
	}
	coverage := map[string]int{"quests": len(index.Quests)}
	for _, p := range index.Quests {
		coverage["visibility_blocks"] += len(p.Blocks)
		if len(p.Gaps) != 0 {
			coverage["quests_with_projection_gaps"]++
		}
		if p.ShowOverride != nil {
			coverage["show_override_sources"]++
		}
	}
	for _, rules := range index.TownRules {
		coverage["phase_rules"] += len(rules)
	}
	for _, gaps := range index.TownGaps {
		if len(gaps) != 0 {
			coverage["towns_with_projection_gaps"]++
		}
	}
	for _, a := range index.Areas {
		coverage["phase_source_slots"] += len(a.PhaseMaps)
	}
	b, err := json.MarshalIndent(map[string]any{"schema_version": 1, "source": index.Source, "meaning": "supplied-state projection, not a live client observation or interaction authorization", "source_coverage": coverage, "results": results}, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if *outFile == "" {
		fmt.Println(string(b))
	} else if err = os.WriteFile(*outFile, append(b, '\n'), 0600); err != nil {
		log.Fatal(err)
	}
}
