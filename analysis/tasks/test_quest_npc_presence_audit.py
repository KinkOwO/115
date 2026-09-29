"""Safety checks for source projection, without a client or database."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("npc_audit", Path(__file__).with_name("quest-npc-presence-audit.py"))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


def tag(text):
    return {"type": 3, "text": text}


def integer(value):
    return {"type": 0, "value": value}


def text(value):
    return {"type": 6, "text": value}


def unit(parent, values, extra=None):
    return [tag("[connect quest unit]"), tag("[parent key]"), integer(parent),
            tag("[sequence]"), *map(integer, values), tag("[/sequence]"),
            *(extra or []), tag("[/connect quest unit]")]


class ShowOverrideSourceChecks(unittest.TestCase):
    def quest(self, cells):
        return {"script": {"cells": cells, "path": "fixture.que", "sha256": "source-fixture"}}

    def test_source_override_is_distinct_from_a_visibility_block(self):
        quests = {"7957": self.quest([tag("[visible npc]"), integer(469)]),
                  "7959": self.quest([tag("[visible npc]"), integer(469)])}
        rows = audit.npc_show_override_sources(quests)
        self.assertEqual([r["quest_id"] for r in rows], [7957, 7959])
        self.assertTrue(all(r["registers_show_override_key"] for r in rows))
        self.assertTrue(all(r["npc"] == 469 and r["source_sha256"] == "source-fixture" for r in rows))
        self.assertFalse(audit.visibility_rules(quests))

    def test_absence_negative_default_and_zero_have_distinct_native_boundaries(self):
        quests = {"1": self.quest([]), "2": self.quest([tag("[visible npc]"), integer(-1)]),
                  "3": self.quest([tag("[visible npc]"), integer(0)])}
        rows = audit.npc_show_override_sources(quests)
        self.assertEqual(len(rows), 2)
        self.assertFalse(rows[0]["registers_show_override_key"])
        self.assertTrue(rows[1]["registers_show_override_key"])

    def test_unresolved_source_does_not_drop_its_evidence_or_invent_a_key(self):
        for cells in ([tag("[visible npc]"), text("469")],
                      [tag("[visible npc]"), integer(True)],
                      [tag("[visible npc]"), integer(2147483648)],
                      [tag("[visible npc]"), integer(469), integer(470)],
                      [tag("[visible npc]"), integer(469), tag("[visible npc]"), integer(470)]):
            with self.subTest(cells=cells):
                row = audit.npc_show_override_sources({"1": self.quest(cells)})[0]
                self.assertFalse(row["projection_resolved"])
                self.assertIsNone(row["npc"])
                self.assertIsNone(row["registers_show_override_key"])
                self.assertTrue(row["raw_sections"])


class QuestOrderSourceChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def export(self, cells, source="same-pvf"):
        raw = b"fixture source identity"
        (self.root / "source.bin").write_bytes(raw)
        data = {"archive.json": {"checksum": source},
                "exports.json": [{"entry": {"archive_path": "n_quest/connectquestlist.etc"},
                                  "raw_file": "source.bin", "raw_sha256": hashlib.sha256(raw).hexdigest()}],
                "source.tokens.json": cells}
        for name, value in data.items():
            (self.root / name).write_text(json.dumps(value), encoding="utf-8")
        return self.root

    def test_order_is_parent_key_and_one_based_position(self):
        source = self.export(unit(37, [900, 100000670, 100, 0]) + unit(16, [700, 999999999]))
        rows, meta = audit.quest_order_catalog(source, "same-pvf")
        self.assertEqual((rows[900][0]["parent_key"], rows[900][0]["sequence_index"]), (37, 1))
        self.assertEqual(rows[100][0]["sequence_index"], 2)
        self.assertEqual(rows[700][0]["parent_key"], 16)
        self.assertEqual(meta["unique_quests"], 3)

    def test_missing_source_retains_unknown(self):
        rows, meta = audit.quest_order_catalog(None, "same-pvf")
        self.assertEqual(rows, {})
        self.assertFalse(meta["loaded"])

    def test_duplicate_ownership_remains_ambiguous(self):
        source = self.export(unit(37, [900, 0]) + unit(16, [900, 0]))
        rows, meta = audit.quest_order_catalog(source, "same-pvf")
        self.assertEqual(len(rows[900]), 2)
        self.assertEqual(meta["ambiguous_quest_ids"], [900])

    def test_unsupported_appoint_sequence_is_not_silently_ignored(self):
        source = self.export(unit(37, [900, 0], [tag("[appoint sequence]"), integer(900), integer(8)]))
        rows, meta = audit.quest_order_catalog(source, "same-pvf")
        self.assertEqual(rows, {})
        self.assertEqual(len(meta["unresolved_units"]), 1)

    def test_wrong_source_and_changed_raw_are_rejected(self):
        source = self.export(unit(37, [900, 0]), source="different-pvf")
        with self.assertRaisesRegex(ValueError, "different PVF sources"):
            audit.quest_order_catalog(source, "same-pvf")
        self.export(unit(37, [900, 0]))
        (source / "source.bin").write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            audit.quest_order_catalog(source, "same-pvf")

    def test_incomplete_pair_is_rejected(self):
        source = self.export(unit(37, [900]))
        with self.assertRaisesRegex(ValueError, "sequence syntax"):
            audit.quest_order_catalog(source, "same-pvf")


class VisibilityProjectionChecks(unittest.TestCase):
    def rule(self, condition="[clearable]", visibility="[show]", extra=None, quest_extra=None):
        cells = [tag("[npc visibility]"), tag("[npc]"), integer(100000670), tag("[/npc]"),
                 tag("[condition]"), text(condition), tag("[visibility]"), text(visibility),
                 *(extra or []), tag("[/npc visibility]"), *(quest_extra or [])]
        rows = audit.visibility_rules({"12911": {"script": {"cells": cells, "path": "fixture.que", "sha256": "fixture"}}})
        return rows[100000670][0]["native_block"]

    def test_clearable_and_clearing_remain_distinct(self):
        self.assertEqual(self.rule()["condition_code"], 2)
        self.assertEqual(self.rule(condition="[clearing]")["condition_code"], 3)
        unknown = self.rule(condition="[exposed]")
        self.assertIsNone(unknown["condition_code"])
        self.assertFalse(unknown["projection_resolved"])

    def test_delete_keeps_protection_and_default_revert(self):
        row = self.rule(visibility="[delete]")
        self.assertEqual((row["show_flag"], row["protection_flag"]), (0, 1))
        self.assertTrue(row["revert_on_transition"])
        self.assertTrue(row["quest_cancel_revert_enabled"])

    def test_block_revert_and_quest_cancel_revert_are_separate(self):
        row = self.rule(extra=[tag("[revert]"), text("[false]")])
        self.assertFalse(row["revert_on_transition"])
        self.assertTrue(row["quest_cancel_revert_enabled"])
        row = self.rule(quest_extra=[tag("[npc visivility not revert]")])
        self.assertTrue(row["revert_on_transition"])
        self.assertFalse(row["quest_cancel_revert_enabled"])
        row = self.rule(extra=[tag("[revert]"), text("[unresolved]")])
        self.assertIsNone(row["revert_on_transition"])
        self.assertFalse(row["projection_resolved"])


class MapNPCSourceChecks(unittest.TestCase):
    def parse(self, prefix):
        rows, unresolved = audit.map_npc_rows([tag("[NPC]"), integer(100), *prefix,
                                              text("[left]"), integer(1), integer(2), integer(0), tag("[/NPC]")])
        self.assertFalse(unresolved)
        return rows[0]

    def test_numeric_quest_id_selects_accepted_or_completed_membership(self):
        accepted = self.parse([integer(12911)])
        completed = self.parse([text("[visible on dungeon if quest clear]"), integer(12911)])
        self.assertEqual(accepted["native_placement"]["predicate"], "accepted")
        self.assertEqual(completed["native_placement"]["predicate"], "completed")
        self.assertTrue(audit.map_placement_gate(accepted, accepted={12911}, completed=set())["allowed"])
        self.assertFalse(audit.map_placement_gate(accepted, accepted=set(), completed={12911})["allowed"])
        self.assertTrue(audit.map_placement_gate(completed, accepted=set(), completed={12911})["allowed"])
        self.assertFalse(audit.map_placement_gate(completed, accepted={12911}, completed=set())["allowed"])

    def test_missing_character_input_is_unknown_but_known_empty_is_false(self):
        row = self.parse([integer(12911)])
        self.assertIsNone(audit.map_placement_gate(row, completed={12911})["allowed"])
        self.assertFalse(audit.map_placement_gate(row, accepted=set())["allowed"])
        row = self.parse([text("[visible on dungeon if quest clear]"), integer(12911)])
        self.assertIsNone(audit.map_placement_gate(row, accepted={12911})["allowed"])
        self.assertFalse(audit.map_placement_gate(row, completed=set())["allowed"])

    def test_marker_without_quest_id_and_negative_ids_bypass_only_placement_gate(self):
        for prefix in ([], [integer(-1)], [integer(-2)],
                       [text("[visible on dungeon if quest clear]")],
                       [text("[visible on dungeon clear]")]):
            with self.subTest(prefix=prefix):
                row = self.parse(prefix)
                self.assertEqual(row["native_placement"]["predicate"], "none")
                self.assertTrue(audit.map_placement_gate(row)["allowed"])
        row = self.parse([integer(0)])
        self.assertEqual(row["native_placement"]["predicate"], "accepted")
        self.assertFalse(audit.map_placement_gate(row, accepted=set())["allowed"])

    def test_native_omitted_fields_carry_across_rows(self):
        cells = [tag("[NPC]"), integer(100), text("[visible on dungeon if quest clear]"),
                 integer(12911), text("[visible on dungeon clear]"), text("[left]"), integer(1), integer(2), integer(0),
                 integer(200), text("[right]"), integer(3), integer(4), integer(0),
                 integer(300), integer(-1), text("[left]"), integer(5), integer(6), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertFalse(unresolved)
        self.assertEqual(rows[1]["source_presence_markers"], [])
        self.assertIsNone(rows[1]["source_quest_id"])
        self.assertEqual(rows[1]["native_placement"]["quest_id"], 12911)
        self.assertTrue(rows[1]["native_placement"]["completed_flag"])
        self.assertTrue(rows[1]["native_placement"]["dungeon_clear_flag"])
        self.assertEqual(rows[1]["native_placement"]["last_source_writes"], rows[0]["native_placement"]["last_source_writes"])
        self.assertEqual(rows[2]["native_placement"]["predicate"], "none")

    def test_later_npc_section_replaces_vector_but_carries_parser_fields(self):
        cells = [tag("[NPC]"), integer(100), text("[visible on dungeon if quest clear]"), integer(12911),
                 text("[left]"), integer(1), integer(2), integer(0), tag("[/NPC]"),
                 tag("[NPC]"), integer(200), text("[right]"), integer(3), integer(4), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertFalse(unresolved)
        self.assertFalse(audit.map_placement_gate(rows[0], completed={12911})["allowed"])
        self.assertTrue(audit.map_placement_gate(rows[1], completed={12911})["allowed"])
        self.assertEqual(rows[1]["native_placement"]["quest_id"], 12911)
        # Each separate native parser invocation starts with its own defaults.
        fresh = self.parse([])
        self.assertEqual(fresh["native_placement"]["quest_id"], -1)
        self.assertFalse(fresh["native_placement"]["completed_flag"])

    def test_unresolved_prior_section_does_not_invent_native_defaults(self):
        cells = [tag("[NPC]"), integer(100), text("[unknown marker]"), tag("[/NPC]"),
                 tag("[NPC]"), integer(200), text("[left]"), integer(1), integer(2), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertEqual(len(unresolved), 1)
        self.assertFalse(rows[0]["native_placement"]["projection_resolved"])
        self.assertIsNone(audit.map_placement_gate(rows[0], accepted={12911}, completed={12911})["allowed"])

    def test_conditional_row_does_not_shift_the_following_coordinates(self):
        cells = [tag("[NPC]"), integer(100001627), text("[visible on dungeon if quest clear]"),
                 text("[left]"), integer(704), integer(194), integer(0),
                 integer(319), text("[right]"), integer(361), integer(190), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertFalse(unresolved)
        self.assertEqual([(r["id"], r["x"], r["y"]) for r in rows], [(100001627, 704, 194), (319, 361, 190)])
        self.assertFalse(rows[0]["presence_condition_resolved"])
        self.assertIsNone(rows[1]["presence_condition_resolved"])

    def test_unknown_marker_or_truncated_row_preserves_unresolved_tail(self):
        cells = [tag("[NPC]"), integer(100), text("[unknown visibility]"), text("[left]"),
                 integer(1), integer(2), integer(0), integer(319), text("[right]"),
                 integer(361), integer(190), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertFalse(rows)
        self.assertEqual(unresolved[0]["offset"], 0)
        self.assertEqual(len(unresolved[0]["raw_tail"]), 11)
        rows, unresolved = audit.map_npc_rows([tag("[NPC]"), integer(100), text("[left]"), integer(1), integer(2), tag("[/NPC]")])
        self.assertFalse(rows)
        self.assertEqual(len(unresolved), 1)

    def test_both_native_markers_remain_independent_source_evidence(self):
        cells = [tag("[NPC]"), integer(100), text("[visible on dungeon if quest clear]"),
                 text("[visible on dungeon clear]"), text("[right]"), integer(1), integer(2), integer(0), tag("[/NPC]")]
        rows, unresolved = audit.map_npc_rows(cells)
        self.assertFalse(unresolved)
        self.assertEqual(len(rows[0]["source_presence_markers"]), 2)
        self.assertFalse(rows[0]["presence_condition_resolved"])
        for markers in (["[visible on dungeon clear]", "[visible on dungeon if quest clear]"],
                        ["[visible on dungeon if quest clear]", "[visible on dungeon if quest clear]"]):
            with self.subTest(markers=markers):
                invalid = [tag("[NPC]"), integer(100), *map(text, markers), text("[right]"),
                           integer(1), integer(2), integer(0), tag("[/NPC]")]
                rows, unresolved = audit.map_npc_rows(invalid)
                self.assertFalse(rows)
                self.assertEqual(len(unresolved), 1)


class TownPhaseSourceChecks(unittest.TestCase):
    def fixture(self, blocks):
        return {"town_index": {"cells": [integer(80), text("Town/ChestTown.twn")]},
                "towns": [{"path": "town/chesttown.twn", "sha256": "fixture", "cells": blocks}],
                "areas": {"80/0": {"town": 80, "map_path": "base.map", "definition": [
                    tag("[phase]"), text("Destroyed.map"), text("Destroyed.map"), tag("[/phase]")]}}}

    def block(self, index, kind="[completed quest]", value=12384):
        return [tag("[phase shift]"), tag("[condition]"), text(kind), integer(value),
                tag("[phase Index]"), integer(index), tag("[/phase shift]")]

    def test_repeated_resource_keeps_phase_indices_and_rule_identity(self):
        towns, areas = audit.town_phase_projection(self.fixture(self.block(1, value=12394) + self.block(0)))
        self.assertEqual(towns[0]["town_id_candidates"], [80])
        slots = areas[0]["phase_slots"]
        self.assertEqual([s["phase_index"] for s in slots], [0, 1])
        self.assertEqual(slots[0]["source_path"], slots[1]["source_path"])
        self.assertEqual(slots[0]["matching_source_rules"][0]["conditions"][0]["value"], 12384)
        self.assertEqual(slots[1]["matching_source_rules"][0]["conditions"][0]["value"], 12394)

    def test_unknown_condition_and_multi_condition_are_not_silently_accepted(self):
        cells = self.block(0, kind="[unknown condition]")
        cells += [tag("[phase shift]"), tag("[multi condition]"), text("[completed quest]"),
                  integer(12384), tag("[/multi condition]"), tag("[phase Index]"), integer(1), tag("[/phase shift]")]
        towns, _ = audit.town_phase_projection(self.fixture(cells))
        self.assertTrue(all(not r["projection_resolved"] for r in towns[0]["rules"]))
        self.assertIsNone(towns[0]["rules"][0]["conditions"][0]["kind"])
        self.assertIn("multi condition projection not implemented", towns[0]["rules"][1]["unresolved"])

    def test_minus_one_phase_is_a_base_resource_rule(self):
        towns, _ = audit.town_phase_projection(self.fixture(self.block(-1, kind="[by event]", value=0)))
        rule = towns[0]["rules"][0]
        self.assertTrue(rule["projection_resolved"])
        self.assertEqual(rule["resource_kind"], "base")

    def test_missing_phase_index_and_bad_town_index_remain_errors(self):
        cells = [tag("[phase shift]"), tag("[condition]"), text("[completed quest]"), integer(12384), tag("[/phase shift]")]
        world = self.fixture(cells)
        towns, _ = audit.town_phase_projection(world)
        self.assertFalse(towns[0]["rules"][0]["projection_resolved"])
        self.assertIsNone(towns[0]["rules"][0]["phase_index"])
        world["town_index"]["cells"].pop()
        with self.assertRaisesRegex(ValueError, "town index syntax"):
            audit.town_phase_projection(world)


class CompletedPhaseScenarioChecks(unittest.TestCase):
    fixture = TownPhaseSourceChecks.fixture
    block = TownPhaseSourceChecks.block

    def town(self, cells):
        return audit.town_phase_projection(self.fixture(cells))[0][0]

    def test_phase_uses_target_index_not_numeric_quest_order_or_source_order(self):
        cells = self.block(0, value=999) + self.block(1, value=10)
        for source in (cells, self.block(1, value=10) + self.block(0, value=999)):
            with self.subTest(source=source):
                row = audit.project_completed_phase(self.town(source), {999, 10}, -1)
                self.assertEqual(row["phase_index"], 1)
                self.assertEqual(row["determining_rules"][0]["quest_id"], 10)

    def test_matching_lower_phase_does_not_reset_supplied_higher_cache(self):
        town = self.town(self.block(0, value=999) + self.block(1, value=10))
        row = audit.project_completed_phase(town, {999}, 1)
        self.assertEqual(row["phase_index"], 1)
        self.assertEqual(row["determining_rules"], [])
        self.assertEqual(audit.project_completed_phase(town, set(), 1)["phase_index"], 1)

    def test_missing_initial_state_is_unknown_even_with_matching_completed_quest(self):
        row = audit.project_completed_phase(self.town(self.block(0)), {12384})
        self.assertIsNone(row["phase_index"])
        self.assertEqual(row["matching_rules"][0]["quest_id"], 12384)
        self.assertIn("lifecycle", row["reason"])

    def test_empty_completed_set_differs_from_missing_set(self):
        town = self.town(self.block(0))
        self.assertIsNone(audit.project_completed_phase(town, None, -1)["phase_index"])
        self.assertEqual(audit.project_completed_phase(town, set(), -1)["phase_index"], -1)
        self.assertEqual(audit.project_completed_phase(town, {12384}, -1)["phase_index"], 0)

    def test_completed_condition_cannot_lower_phase_to_minus_one(self):
        town = self.town(self.block(-1))
        self.assertEqual(audit.project_completed_phase(town, {12384}, -1)["phase_index"], -1)

    def test_mixed_event_town_is_unknown_even_if_completed_phase_matches(self):
        town = self.town(self.block(0) + self.block(1, kind="[by event]", value=1))
        row = audit.project_completed_phase(town, {12384}, -1)
        self.assertIsNone(row["phase_index"])
        self.assertFalse(row["scope"]["supported"])

    def test_unknown_rules_duplicate_condition_or_ambiguous_town_remain_unknown(self):
        for town in (self.town(self.block(0, kind="[unknown]")),
                     self.town(self.block(0) + self.block(1))):
            with self.subTest(town=town):
                self.assertIsNone(audit.project_completed_phase(town, {12384}, -1)["phase_index"])
        town = self.town(self.block(0))
        town["town_id_candidates"].append(81)
        self.assertIsNone(audit.project_completed_phase(town, {12384}, -1)["phase_index"])

    def test_invalid_completed_and_initial_inputs_are_rejected(self):
        town = self.town(self.block(0))
        for completed in ({True}, {"12384"}, {-1}, {2147483648}):
            with self.subTest(completed=completed), self.assertRaisesRegex(ValueError, "completed quest"):
                audit.project_completed_phase(town, completed, -1)
        for initial in (True, "0", 1, -2):
            with self.subTest(initial=initial), self.assertRaisesRegex(ValueError, "source phase index"):
                audit.project_completed_phase(town, {12384}, initial)

    def scenario_fixture(self):
        town = self.town(self.block(0) + self.block(1, value=12394))
        placement = {"area": "80/0", "map_path": "destroyed.map", "x": 495, "y": 182,
                     "root_ownership_resolved": True, "root_phase_index_candidates": [0, 1]}
        result = {"source": "same-pvf", "town_phase_rules": [town], "quests": [
            {"category": "phase_only_candidate", "quest_id": 12911, "npc": 100000670,
             "default_hidden": True, "phase_placements": [placement]}]}
        scenario = {"schema_version": 1, "source": "same-pvf",
                    "completed_quests": [12394], "initial_phases": {"80": -1}}
        return result, scenario

    def test_repeated_root_matches_each_slot_without_a_visibility_verdict(self):
        result, scenario = self.scenario_fixture()
        row = audit.phase_scenario(result, scenario)["quests"][0]
        self.assertTrue(row["placements"][0]["root_phase_index_matches"])
        self.assertIsNone(row["final_visibility"])
        scenario["completed_quests"] = [12384]
        row = audit.phase_scenario(result, scenario)["quests"][0]
        self.assertTrue(row["placements"][0]["root_phase_index_matches"])
        self.assertEqual(row["placements"][0]["scenario_phase_index"], 0)

    def test_inactive_root_and_unresolved_import_are_separate(self):
        result, scenario = self.scenario_fixture()
        scenario["completed_quests"] = []
        row = audit.phase_scenario(result, scenario)["quests"][0]
        self.assertFalse(row["placements"][0]["root_phase_index_matches"])
        self.assertIsNone(row["final_visibility"])
        result["quests"][0]["phase_placements"][0]["root_ownership_resolved"] = False
        row = audit.phase_scenario(result, scenario)["quests"][0]
        self.assertIsNone(row["placements"][0]["root_phase_index_matches"])

    def test_scenario_source_schema_and_unknown_town_inputs_are_rejected(self):
        result, scenario = self.scenario_fixture()
        for changes in ({"source": "different-pvf"}, {"schema_version": True},
                        {"initial_phases": {"81": -1}}, {"initial_phases": {"80": True}},
                        {"completed_quests": [True]}, {"accepted_quests": [12911]}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                audit.phase_scenario(result, {**scenario, **changes})
        for value in ([], None, True):
            with self.subTest(value=value), self.assertRaises(ValueError):
                audit.phase_scenario(result, value)


class PhaseTraceChecks(unittest.TestCase):
    scenario_fixture = CompletedPhaseScenarioChecks.scenario_fixture
    town = CompletedPhaseScenarioChecks.town
    fixture = TownPhaseSourceChecks.fixture
    block = TownPhaseSourceChecks.block

    def trace(self, events):
        result, scenario = self.scenario_fixture()
        return result, {**scenario, "initial_phases": {"80": 1}, "events": events}

    def test_select_reset_removes_previous_character_phase_and_membership(self):
        result, trace = self.trace([
            {"kind": "select_phase_reset"},
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80}])
        output = audit.phase_trace(result, trace)
        self.assertEqual(output["checkpoints"][0]["phase_cache"], {"80": -1})
        self.assertFalse(output["checkpoints"][0]["completed_snapshot_known"])
        self.assertEqual(output["phase_cache"], {"80": 0})
        self.assertIsNone(output["final_visibility"])

    def test_snapshot_replacement_and_ordinary_resolve_cannot_lower_cached_phase(self):
        result, trace = self.trace([
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80},
            {"kind": "completed_snapshot", "quests": []},
            {"kind": "resolve_town_phase", "town": 80}])
        self.assertEqual(audit.phase_trace(result, trace)["phase_cache"], {"80": 1})

    def test_resolve_before_new_membership_preserves_unknown_after_snapshot(self):
        result, trace = self.trace([
            {"kind": "select_phase_reset"},
            {"kind": "resolve_town_phase", "town": 80},
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80}])
        output = audit.phase_trace(result, trace)
        self.assertIsNone(output["checkpoints"][1]["phase_cache"]["80"])
        self.assertIsNone(output["phase_cache"]["80"])

    def test_missing_snapshot_is_not_an_empty_completed_set(self):
        result, trace = self.trace([
            {"kind": "select_phase_reset"},
            {"kind": "completed_snapshot", "quests": []},
            {"kind": "resolve_town_phase", "town": 80}])
        self.assertEqual(audit.phase_trace(result, trace)["phase_cache"]["80"], -1)
        trace["events"][1]["quests"] = None
        self.assertIsNone(audit.phase_trace(result, trace)["phase_cache"]["80"])

    def test_gap_loses_cache_knowledge_until_an_explicit_reset(self):
        result, trace = self.trace([
            {"kind": "gap"},
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80}])
        self.assertIsNone(audit.phase_trace(result, trace)["phase_cache"]["80"])
        trace["events"] += [{"kind": "select_phase_reset"},
                            {"kind": "completed_snapshot", "quests": [12384]},
                            {"kind": "resolve_town_phase", "town": 80}]
        self.assertEqual(audit.phase_trace(result, trace)["phase_cache"]["80"], 0)

    def test_town_caches_are_independent(self):
        result, trace = self.trace([
            {"kind": "resolve_town_phase", "town": 80},
            {"kind": "resolve_town_phase", "town": 81}])
        town = self.town(self.block(0))
        town["town_id_candidates"] = [81]
        result["town_phase_rules"].append(town)
        trace["initial_phases"]["81"] = -1
        trace["completed_quests"] = [12384]
        self.assertEqual(audit.phase_trace(result, trace)["phase_cache"], {"80": 1, "81": 0})

    def test_event_based_town_stays_unknown_after_select_reset(self):
        result, trace = self.trace([
            {"kind": "select_phase_reset"},
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80}])
        result["town_phase_rules"][0] = self.town(self.block(0) + self.block(1, kind="[by event]", value=1))
        self.assertIsNone(audit.phase_trace(result, trace)["phase_cache"]["80"])

    def test_failed_ack_packet_and_map_change_are_not_reset_operations(self):
        result, trace = self.trace([])
        for event in ({"kind": "select_ack", "success": False},
                      {"kind": "select_ack", "success": True},
                      {"kind": "map_change", "town": 80},
                      {"kind": "select_phase_reset", "success": False}):
            trace["events"] = [event]
            with self.subTest(event=event), self.assertRaisesRegex(ValueError, "trace event"):
                audit.phase_trace(result, trace)

    def test_malformed_trace_and_snapshot_inputs_are_rejected(self):
        result, trace = self.trace([])
        for change in ({"source": "other-pvf"}, {"schema_version": True},
                       {"events": None}, {"extra": 1}, {"initial_phases": {"99": -1}},
                       {"initial_phases": {"80": 3}, "completed_quests": None}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                audit.phase_trace(result, {**trace, **change})
        for event in (None, {"kind": "resolve_town_phase", "town": True},
                      {"kind": "resolve_town_phase", "town": 99},
                      {"kind": "completed_snapshot", "quests": [True]},
                      {"kind": "completed_snapshot", "quests": [-1]},
                      {"kind": "completed_snapshot", "quests": "12384"}):
            with self.subTest(event=event), self.assertRaises(ValueError):
                audit.phase_trace(result, {**trace, "events": [event]})

    def test_no_boundary_does_not_invent_minus_one(self):
        result, trace = self.trace([
            {"kind": "completed_snapshot", "quests": [12384]},
            {"kind": "resolve_town_phase", "town": 80}])
        del trace["initial_phases"]
        self.assertIsNone(audit.phase_trace(result, trace)["phase_cache"]["80"])


if __name__ == "__main__":
    unittest.main()
