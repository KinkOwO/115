"""Read-only candidate audit; placements and source rules do not prove visibility.

Runs without a database or client. Outputs only under the chosen output folder.
Uses source script cells, including all prerequisite branches; never treats
numeric quest adjacency or an NPC's presence in any phase as authorization.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from collections import Counter, defaultdict, deque
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CONFIGS = ROOT / "server/work/dfo-lan/configs"


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def checksum(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def sections(cells, tag):
    """All simple tag sections, preserving types and repeated occurrences."""
    for i, cell in enumerate(cells):
        if cell.get("type") == 3 and cell.get("text") == tag:
            end = i + 1
            while end < len(cells) and cells[end].get("type") != 3:
                end += 1
            yield cells[i + 1:end]


def blocks(cells, tag):
    closing = "[/" + tag[1:]
    for i, cell in enumerate(cells):
        if cell.get("type") == 3 and cell.get("text") == tag:
            end = i + 1
            while end < len(cells) and cells[end].get("text") != closing:
                end += 1
            if end == len(cells):
                raise ValueError(f"unclosed source block {tag}")
            yield cells[i + 1:end]


def scalar(cells, tag):
    rows = list(sections(cells, tag))
    if len(rows) == 1 and len(rows[0]) == 1:
        c = rows[0][0]
        return c.get("text") if c.get("type") == 6 else c.get("value")
    return None


def numbers(cells):
    return [c["value"] for c in cells if c.get("type") == 0 and c["value"] > 0]


def npc_show_override_sources(quests):
    """Source [visible npc] writes +2320, consumed as a show-override key.

    1476577D9 defaults this field to -1. 144F394FD registers the native
    quest ID, then 146D14060 shows the entity without changing its logical
    show flag. This index is not an accepted-membership visibility verdict.
    """
    result = []
    for quest_id, quest in sorted(quests.items(), key=lambda pair: int(pair[0])):
        script = quest["script"]
        rows = list(sections(script["cells"], "[visible npc]"))
        if not rows:
            continue
        resolved = (len(rows) == 1 and len(rows[0]) == 1 and rows[0][0].get("type") == 0
                    and type(rows[0][0].get("value")) is int
                    and -2147483648 <= rows[0][0]["value"] <= 2147483647)
        npc = rows[0][0]["value"] if resolved else None
        result.append({"quest_id": int(quest_id), "npc": npc, "source_tag": "[visible npc]",
                       "source_path": script["path"], "source_sha256": script["sha256"],
                       "raw_sections": rows, "projection_resolved": resolved,
                       "native_field_offset": 2320, "native_default": -1,
                       "registers_show_override_key": npc >= 0 if resolved else None,
                       "evidence": ["14766927E", "147677458", "144F394FD", "146CF6650", "146D14060", "144F65C6A"],
                       "meaning": "source field only; native quest-ID rewriting and operation replay remain required"})
    return result


def visibility_rules(quests):
    by_npc = defaultdict(list)
    for quest_id, quest in quests.items():
        script = quest["script"]
        for ordinal, block in enumerate(blocks(script["cells"], "[npc visibility]")):
            ids = [n for row in sections(block, "[npc]") for n in numbers(row)]
            rule = {"quest_id": int(quest_id), "block_ordinal": ordinal,
                    "condition": scalar(block, "[condition]"),
                    "visibility": scalar(block, "[visibility]"),
                    "source_path": script["path"], "source_sha256": script["sha256"],
                    "raw_cells": block}
            # Native block defaults: 1476549D0. Tags: 14767493A..147674F5E.
            # These describe a source block, not the effects of replaying it.
            conditions = {"[accept]": 0, "[clear]": 1,
                          "[clearable]": 2, "[clearing]": 3}
            visibility = {"[show]": (1, 0), "[hide]": (0, 0), "[delete]": (0, 1)}
            flags = visibility.get(rule["visibility"], (None, None))
            revert_rows = list(sections(block, "[revert]"))
            revert = True if not revert_rows else {"[true]": True, "[false]": False}.get(scalar(block, "[revert]"))
            rule["native_block"] = {
                "condition_code": conditions.get(rule["condition"]),
                "show_flag": flags[0], "protection_flag": flags[1],
                "revert_on_transition": revert,
                # Quest reset sets +2793=1; the no-revert tag sets it to0.
                "quest_cancel_revert_enabled": not any(c.get("type") == 3 and c.get("text") == "[npc visivility not revert]" for c in script["cells"]),
                "projection_resolved": rule["condition"] in conditions and flags[0] is not None and revert is not None,
                "evidence": ["1476549D0", "1476608F0", "14767493A..147674F5E", "147657AB3", "14766D67B", "14527DD70", "144F3B900"]}
            for npc in sorted(set(ids)):
                by_npc[npc].append(rule)
    return by_npc


def map_npc_rows(cells):
    """Project native placement fields; final NPC visibility remains separate."""
    rows, unresolved = [], []
    markers = ("[visible on dungeon if quest clear]", "[visible on dungeon clear]")
    npc_sections = list(sections(cells, "[NPC]"))
    # 1471C31CE..1471C31DF initializes once per parser invocation. The row
    # loop only overwrites supplied fields; omitted quest IDs/flags carry.
    state = {"quest_id": -1, "completed_flag": False, "dungeon_clear_flag": False}
    writes = {key: None for key in state}
    state_resolved = True
    for section_ordinal, section in enumerate(npc_sections):
        i = 0
        while i < len(section):
            start = i
            npc = section[i]
            i += 1
            presence_markers = []
            supplied_quest = None
            # Native order: completed marker, optional quest ID, dungeon marker,
            # direction, x/y/z. A marker by itself does not supply a quest ID.
            if i < len(section) and section[i].get("type") == 6 and section[i].get("text") == markers[0]:
                presence_markers.append(markers[0])
                state["completed_flag"] = True
                writes["completed_flag"] = [section_ordinal, i]
                i += 1
            if i < len(section) and section[i].get("type") == 0:
                supplied_quest = section[i]["value"]
                state["quest_id"] = supplied_quest
                writes["quest_id"] = [section_ordinal, i]
                i += 1
            if i < len(section) and section[i].get("type") == 6 and section[i].get("text") == markers[1]:
                presence_markers.append(markers[1])
                state["dungeon_clear_flag"] = True
                writes["dungeon_clear_flag"] = [section_ordinal, i]
                i += 1
            tail = section[i:i + 4]
            if (npc.get("type") != 0 or len(tail) != 4
                    or [c.get("type") for c in tail] != [6, 0, 0, 0]
                    or tail[0].get("text") not in {"[left]", "[right]"}):
                unresolved.append({"section_ordinal": section_ordinal, "offset": start,
                                   "reason": "unresolved NPC row syntax", "raw_tail": section[start:]})
                state_resolved = False
                break
            predicate = ("none" if state["quest_id"] < 0 else
                         "completed" if state["completed_flag"] else "accepted")
            rows.append({"id": npc["value"], "direction": tail[0]["text"],
                         "x": tail[1]["value"], "y": tail[2]["value"], "z": tail[3]["value"],
                         "source_presence_markers": presence_markers,
                         "source_quest_id": supplied_quest,
                         "native_placement": {**state, "predicate": predicate if state_resolved else None,
                                              "projection_resolved": state_resolved,
                                              "last_source_writes": dict(writes),
                                              "survives_last_npc_section": section_ordinal == len(npc_sections) - 1},
                         "presence_condition_resolved": False if presence_markers else None,
                         "section_ordinal": section_ordinal, "source_cell_offset": start,
                         "evidence": ["1471C31C8..1471C31E9", "1471D3887..1471D3A74",
                                      "1471E4790", "145DEBA20", "144F54570", "144F54990"]})
            i += 4
    return rows, unresolved


def map_placement_gate(row, accepted=None, completed=None):
    """Offline placement predicate only; never an interaction authorization.

    None means that the required character input was not provided. An empty
    supplied set means known absence, not an unknown input. Progress does not
    enter the native accepted-membership predicate.
    """
    native = row.get("native_placement", {})
    result = {"allowed": None, "predicate": native.get("predicate"),
              "quest_id": native.get("quest_id"),
              "meaning": "placement gate only; active map, creation and visibility remain required"}
    if not native.get("projection_resolved"):
        return {**result, "reason": "unresolved native placement projection"}
    if not native["survives_last_npc_section"]:
        return {**result, "allowed": False, "reason": "replaced by a later NPC source section"}
    if native["predicate"] == "none":
        return {**result, "allowed": True, "reason": "negative quest ID bypasses placement quest gate"}
    state = completed if native["predicate"] == "completed" else accepted
    if state is None:
        return {**result, "reason": "required character quest set not supplied"}
    return {**result, "allowed": native["quest_id"] in state,
            "reason": native["predicate"] + " quest membership"}


def base_placements(world):
    result = defaultdict(list)
    malformed = []
    for area_key, area in world["areas"].items():
        for script in [area["map"], *area.get("imported_scripts", [])]:
            rows, unresolved = map_npc_rows(script.get("cells", []))
            malformed.extend({"area": area_key, "path": script["path"], **row} for row in unresolved)
            for row in rows:
                npc, x, y = row["id"], row["x"], row["y"]
                if npc > 0 and 0 <= x <= 65535 and 0 <= y <= 65535:
                    result[npc].append({"area": area_key, "map_path": script["path"],
                                        "map_sha256": script["sha256"], **row})
    return result, malformed


def town_phase_projection(world):
    """Project source conditions and phase slots without choosing an active map."""
    kinds = {"[recent clear dungeon]": 0, "[regional movement]": 1,
             "[completed quest]": 2, "[by event]": 3,
             "[depend townarea event]": 4, "[while event active]": 5}
    # Only tags confirmed by 1479FF000 may acquire a native kind.
    index = world["town_index"]["cells"]
    paths = defaultdict(list)
    for i in range(0, len(index), 2):
        pair = index[i:i + 2]
        if len(pair) != 2 or [c.get("type") for c in pair] != [0, 6]:
            raise ValueError("unresolved town index syntax")
        paths[pair[1]["text"].replace("\\", "/").lower()].append(pair[0]["value"])
    towns = []
    by_town = defaultdict(list)
    for town in world["towns"]:
        rule_rows = []
        for ordinal, block in enumerate(blocks(town["cells"], "[phase shift]")):
            condition_rows = list(sections(block, "[condition]"))
            conditions = []
            unresolved = []
            for row in condition_rows:
                if len(row) == 2 and [c.get("type") for c in row] == [6, 0]:
                    kind = kinds.get(row[0].get("text"))
                    conditions.append({"tag": row[0].get("text"), "kind": kind, "value": row[1]["value"]})
                    if kind is None:
                        unresolved.append("unknown native condition tag")
                else:
                    unresolved.append("unresolved condition syntax")
            # Preserve multi-condition source but do not silently drop it.
            if list(sections(block, "[multi condition]")):
                unresolved.append("multi condition projection not implemented")
            if not conditions:
                unresolved.append("no projected condition")
            phase_index = scalar(block, "[phase Index]")
            # Native selector explicitly handles -1 by choosing the base resource.
            if not isinstance(phase_index, int) or phase_index < -1:
                unresolved.append("invalid or absent phase Index")
            if any(c.get("type") == 3 and c.get("text") not in {"[condition]", "[phase Index]"} for c in block):
                unresolved.append("additional phase source syntax")
            rule_rows.append({"source_ordinal": ordinal, "phase_index": phase_index,
                              "resource_kind": "base" if phase_index == -1 else "phase" if isinstance(phase_index, int) and phase_index >= 0 else None,
                              "conditions": conditions, "projection_resolved": not unresolved,
                              "unresolved": unresolved, "raw_cells": block,
                              "evidence": ["1479FF000", "147A00860..147A00AAC"]})
        if rule_rows:
            ids = paths.get(town["path"].replace("\\", "/").lower(), [])
            entry = {"town_id_candidates": ids, "path": town["path"], "sha256": town["sha256"],
                     "rules": rule_rows, "meaning": "source projection; no active phase selected"}
            towns.append(entry)
            for town_id in ids:
                by_town[town_id].append(entry)
    areas = []
    for key, area in world["areas"].items():
        phase_sections = list(sections(area["definition"], "[phase]"))
        if not phase_sections:
            continue
        slots = []
        ordinal = 0
        for section in phase_sections:
            for cell in section:
                # Ordinals are preserved even if a source cell is unresolved.
                slots.append({"phase_index": ordinal,
                              "source_path": cell.get("text") if cell.get("type") == 6 else None,
                              "matching_source_rules": [
                                  {"town_path": t["path"], "town_sha256": t["sha256"], **r}
                                  for t in by_town.get(area["town"], []) for r in t["rules"]
                                  if r["phase_index"] == ordinal], "raw_cell": cell})
                ordinal += 1
        areas.append({"area": key, "base_map_path": area["map_path"], "phase_slots": slots,
                      "multiple_phase_sections": len(phase_sections) != 1,
                      "meaning": "root resource slots; imported-map ownership remains unresolved"})
    return towns, areas


def completed_phase_scope(town):
    """Whether phase-index projection has only the closed kind-2 semantics."""
    reasons = []
    rules = town["rules"]
    if len(town["town_id_candidates"]) != 1:
        reasons.append("town source binding is not unique")
    if not rules:
        reasons.append("town phase rules are absent")
    keys = []
    for rule in rules:
        conditions = rule["conditions"]
        if not rule["projection_resolved"]:
            reasons.append("unresolved source rule")
        if (type(rule["phase_index"]) is not int
                or not -1 <= rule["phase_index"] <= 2147483647):
            reasons.append("invalid source phase index")
        if len(conditions) != 1 or conditions[0]["kind"] != 2:
            reasons.append("condition outside single completed-quest scope")
            continue
        value = conditions[0]["value"]
        if type(value) is not int or value < 0 or value > 2147483647:
            reasons.append("invalid completed-quest condition value")
        else:
            keys.append(value)
    if len(keys) != len(set(keys)):
        reasons.append("duplicate condition key has unresolved native ownership")
    return {"supported": not reasons, "reasons": sorted(set(reasons)),
            "condition_kinds": sorted({c["kind"] for r in rules for c in r["conditions"]
                                       if c["kind"] is not None}),
            "meaning": "phase-index scenario only; map overrides and NPC visibility are separate"}


def project_completed_phase(town, completed=None, initial_phase=None):
    """Pure kind-2 scenario, with an explicit initial phase rather than a login guess.

    146D06B80 requires completed membership AND currentPhase < targetPhase.
    146D02440 scans the whole condition tree; 146CF8C80 updates the saved
    condition. For single kind-2 rules the resulting phase is the maximum
    of the supplied initial phase and matching targets, independent of tree
    order. This does not establish which cache state a real login restores.
    """
    scope = completed_phase_scope(town)
    result = {"phase_index": None, "initial_phase": initial_phase,
              "matching_rules": [], "determining_rules": [], "scope": scope,
              "evidence": ["146D06B80", "146CF8C80", "146D02440"],
              "meaning": "supplied-state phase scenario; not an observed client phase"}
    if not scope["supported"]:
        return {**result, "reason": "town has conditions outside the closed projection scope"}
    if completed is None:
        return {**result, "reason": "completed quest set not supplied"}
    if (not isinstance(completed, (list, set, frozenset, tuple))
            or any(type(q) is not int or q < 0 or q > 2147483647 for q in completed)):
        raise ValueError("invalid completed quest set")
    matched = [r for r in town["rules"] if r["conditions"][0]["value"] in completed]
    result["matching_rules"] = [{"source_ordinal": r["source_ordinal"],
                                  "quest_id": r["conditions"][0]["value"],
                                  "phase_index": r["phase_index"]} for r in matched]
    if initial_phase is None:
        return {**result, "reason": "initial phase not supplied; no explicit cache lifecycle boundary"}
    valid = {-1, *(r["phase_index"] for r in town["rules"])}
    if type(initial_phase) is not int or initial_phase not in valid:
        raise ValueError("initial phase is not a source phase index")
    selected = max([initial_phase, *(r["phase_index"] for r in matched)])
    result["determining_rules"] = [r for r in result["matching_rules"]
                                    if r["phase_index"] == selected and selected > initial_phase]
    return {**result, "phase_index": selected,
            "reason": "completed membership can advance phase but cannot lower the supplied initial phase"}


def phase_scenario(result, scenario):
    """Compare phase roots against explicit inputs, without a database or client."""
    if (not isinstance(scenario, dict) or type(scenario.get("schema_version")) is not int or scenario["schema_version"] != 1
            or set(scenario) - {"schema_version", "source", "completed_quests", "initial_phases"}):
        raise ValueError("invalid phase scenario schema")
    if scenario.get("source") != result["source"]:
        raise ValueError("phase scenario and catalogs come from different PVF sources")
    completed = scenario.get("completed_quests")
    if completed is not None and (not isinstance(completed, list)
                                 or any(type(q) is not int or q < 0 or q > 2147483647 for q in completed)):
        raise ValueError("invalid completed quest set")
    initial = scenario.get("initial_phases", {})
    known = {str(n) for t in result["town_phase_rules"] for n in t["town_id_candidates"]}
    if (not isinstance(initial, dict) or set(initial) - known
            or any(type(v) is not int or v < -1 or v > 2147483647 for v in initial.values())):
        raise ValueError("invalid initial phase map")
    by_town = defaultdict(list)
    for town in result["town_phase_rules"]:
        for town_id in town["town_id_candidates"]:
            by_town[town_id].append(town)
    phases = {}
    for town_id, candidates in sorted(by_town.items()):
        if len(candidates) != 1:
            phases[town_id] = {"phase_index": None, "reason": "multiple town sources bind this town ID"}
            continue
        town = candidates[0]
        phases[town_id] = {"town_path": town["path"], "town_sha256": town["sha256"],
                           **project_completed_phase(town, completed, initial.get(str(town_id)))}
    rows = []
    for quest in result["quests"]:
        if quest["category"] != "phase_only_candidate":
            continue
        placements = []
        for p in quest["phase_placements"]:
            town_id = int(p["area"].split("/")[0])
            phase = phases.get(town_id, {"phase_index": None, "reason": "town phase source absent"})
            selected = phase["phase_index"]
            matches = None
            reason = phase["reason"]
            if selected is not None:
                if not p["root_ownership_resolved"]:
                    reason = "imported phase map root ownership is not exported"
                else:
                    matches = selected in p["root_phase_index_candidates"]
                    reason = "source root phase-index comparison only"
            placements.append({**p, "scenario_phase_index": selected,
                               "root_phase_index_matches": matches, "reason": reason})
        rows.append({"quest_id": quest["quest_id"], "npc": quest["npc"],
                     "default_hidden": quest["default_hidden"], "placements": placements,
                     "final_visibility": None,
                     "remaining_inputs": ["area map override", "native NPC creation", "visibility effect replay", "observed client cache state"]})
    counts = Counter("unknown" if p["root_phase_index_matches"] is None else
                     "matching" if p["root_phase_index_matches"] else "different"
                     for q in rows for p in q["placements"])
    return {"schema_version": 1, "source": result["source"], "inputs": scenario,
            "meaning": "offline phase-index comparisons only; no presence, failure or interaction verdict",
            "summary": dict(counts), "towns": {str(k): v for k, v in phases.items()}, "quests": rows}


def phase_trace(result, trace):
    """Project explicit condition-cache operations, not complete packet handlers.

    select_phase_reset means the point AFTER 14525A6D7/14525A6EB, not the
    end of CMD4: later CMD4 code can already resolve a map before NOTI342.
    Snapshots supply membership at a specific point; they do not prove when
    the client received it. Unlisted side effects require an explicit gap.
    """
    fields = {"schema_version", "source", "completed_quests", "initial_phases", "events"}
    if (not isinstance(trace, dict) or type(trace.get("schema_version")) is not int
            or trace["schema_version"] != 1 or set(trace) - fields
            or not isinstance(trace.get("events"), list)):
        raise ValueError("invalid phase trace schema")
    initial = {k: v for k, v in trace.items() if k != "events"}
    baseline = phase_scenario(result, initial)  # Validate source and supplied state.
    known = set(baseline["towns"])
    completed = trace.get("completed_quests")
    cache = {key: trace.get("initial_phases", {}).get(key) for key in known}
    by_town = defaultdict(list)
    for town in result["town_phase_rules"]:
        for town_id in town["town_id_candidates"]:
            by_town[str(town_id)].append(town)
    for key, value in trace.get("initial_phases", {}).items():
        valid = {-1, *(r["phase_index"] for t in by_town[key] for r in t["rules"]
                       if type(r["phase_index"]) is int)}
        if value not in valid:
            raise ValueError("initial trace phase is not a source phase index")
    checkpoints = []
    for ordinal, event in enumerate(trace["events"]):
        if not isinstance(event, dict):
            raise ValueError("invalid phase trace event")
        kind = event.get("kind")
        projection = None
        if kind == "select_phase_reset" and set(event) == {"kind"}:
            cache = dict.fromkeys(known, -1)
            # Invalidate our knowledge, not a claim that these helpers clear
            # QuestManager's completed set or the town event containers.
            completed = None
            reason = "successful select reset point; completed membership needs a new snapshot"
        elif kind == "completed_snapshot" and set(event) == {"kind", "quests"}:
            completed = event["quests"]
            if completed is not None and (not isinstance(completed, list) or any(
                    type(q) is not int or not 0 <= q <= 2147483647 for q in completed)):
                raise ValueError("invalid completed quest snapshot")
            reason = "membership snapshot replaced; phase cache is retained"
        elif kind == "resolve_town_phase" and set(event) == {"kind", "town"}:
            town_id = event["town"]
            if type(town_id) is not int or str(town_id) not in known:
                raise ValueError("invalid trace town")
            key = str(town_id)
            candidates = by_town[key]
            projection = ({"phase_index": None, "reason": "multiple town sources bind this town ID"}
                          if len(candidates) != 1 else
                          project_completed_phase(candidates[0], completed, cache[key]))
            cache[key] = projection["phase_index"]
            reason = projection["reason"]
        elif kind == "gap" and set(event) == {"kind"}:
            cache = dict.fromkeys(known, None)
            completed = None
            reason = "unmodeled client operations invalidate phase and membership knowledge"
        else:
            raise ValueError("unknown or malformed phase trace event")
        checkpoints.append({"event_ordinal": ordinal, "event": event, "reason": reason,
                            "completed_snapshot_known": completed is not None,
                            "phase_cache": dict(sorted(cache.items())), "projection": projection})
    return {"schema_version": 1, "source": result["source"], "inputs": trace,
            "meaning": "offline explicit cache operations; not a CMD4/342 decoder or full client replay",
            "reset_evidence": ["14525A6D7", "146CFA9B0", "14525A6EB", "146D04FE0"],
            "checkpoints": checkpoints, "phase_cache": dict(sorted(cache.items())),
            "final_visibility": None,
            "remaining_inputs": ["complete native call order", "town and area events",
                                 "map channel limits", "NPC show override", "visibility effect replay"]}


def hidden_catalog(export_dir, source):
    if export_dir is None:
        return None, {"loaded": False, "reason": "hidden NPC source export not supplied"}
    archive = read_json(export_dir / "archive.json")
    if archive["checksum"] != source:
        raise ValueError("hidden NPC export and catalogs come from different PVF sources")
    manifest = read_json(export_dir / "exports.json")
    entry = next(e for e in manifest if e.get("entry", {}).get("archive_path") == "etc/hiddennpc.etc")
    raw = export_dir / entry["raw_file"]
    if checksum(raw) != entry["raw_sha256"]:
        raise ValueError("hidden NPC raw export checksum mismatch")
    token_path = raw.with_suffix(".tokens.json")
    cells = read_json(token_path)
    hidden = [n for row in sections(cells, "[hidden npc]") for n in numbers(row)]
    sequence = [row for row in sections(cells, "[npc visibility quest sequence]")]
    sequence = [c for row in sequence for c in row]
    if len(sequence) % 3 or any(c.get("type") != 0 for c in sequence):
        raise ValueError("unresolved global visibility sequence syntax")
    return set(hidden), {"loaded": True, "path": "etc/hiddennpc.etc",
                         "raw_sha256": entry["raw_sha256"], "tokens_sha256": checksum(token_path),
                         "hidden_rows": len(hidden), "hidden_unique": len(set(hidden)),
                         "sequence_raw_triples": [[c["value"] for c in sequence[i:i + 3]]
                                                  for i in range(0, len(sequence), 3)],
                         "sequence_semantics": "target quest, reference quest, sequence offset; fallback only when native quest unit lookup fails"}


def quest_order_catalog(export_dir, source):
    """Project source unit order, preserving ambiguous native ownership."""
    if export_dir is None:
        return {}, {"loaded": False, "reason": "quest unit source export not supplied"}
    if read_json(export_dir / "archive.json")["checksum"] != source:
        raise ValueError("quest unit export and catalogs come from different PVF sources")
    manifest = read_json(export_dir / "exports.json")
    path = "n_quest/connectquestlist.etc"
    entry = next(e for e in manifest if e.get("entry", {}).get("archive_path") == path)
    raw = export_dir / entry["raw_file"]
    if checksum(raw) != entry["raw_sha256"]:
        raise ValueError("quest unit raw export checksum mismatch")
    token_path = raw.with_suffix(".tokens.json")
    units = list(blocks(read_json(token_path), "[connect quest unit]"))
    by_quest = defaultdict(list)
    unresolved = []
    for ordinal, unit in enumerate(units):
        parent = scalar(unit, "[parent key]")
        sequence = list(blocks(unit, "[sequence]"))
        if (not isinstance(parent, int) or parent <= 0 or len(sequence) != 1
                or list(sections(unit, "[appoint sequence]"))):
            unresolved.append({"unit_ordinal": ordinal, "reason": "unresolved unit ordering form"})
            continue
        cells = sequence[0]
        if len(cells) % 2 or any(c.get("type") != 0 for c in cells):
            raise ValueError(f"unresolved quest sequence syntax at unit {ordinal}")
        for index in range(0, len(cells), 2):
            quest_id, auxiliary = cells[index]["value"], cells[index + 1]["value"]
            if quest_id <= 0:
                raise ValueError(f"invalid quest sequence target at unit {ordinal}")
            # Native 147663660 assigns increasing keys, starting at one.
            # The second source cell is stored separately, not used as this key.
            by_quest[quest_id].append({"parent_key": parent, "sequence_index": index // 2 + 1,
                                      "auxiliary_value": auxiliary, "unit_ordinal": ordinal,
                                      "source_path": path, "source_sha256": entry["raw_sha256"]})
    return by_quest, {"loaded": True, "path": path, "raw_sha256": entry["raw_sha256"],
                      "tokens_sha256": checksum(token_path), "units": len(units),
                      "quest_rows": sum(len(v) for v in by_quest.values()),
                      "unique_quests": len(by_quest), "unresolved_units": unresolved,
                      "ambiguous_quest_ids": sorted(k for k, v in by_quest.items() if len(v) > 1),
                      "native_binding_evidence": ["14519E760", "1451A0A30", "1451A0710"],
                      "meaning": "source rank candidates; native binder writes unit parent key to existing quest metadata +64; duplicate unit ownership and character replay require further evidence"}


def audit(world_path, quest_path, hidden_export, order_export=None):
    world, catalog = read_json(world_path), read_json(quest_path)
    source = world["source"]["checksum"]
    if source != catalog["source"]["checksum"]:
        raise ValueError("world and quest catalogs come from different PVF sources")
    quests = catalog["quests"]
    hidden, hidden_meta = hidden_catalog(hidden_export, source)
    order, order_meta = quest_order_catalog(order_export, source)
    hidden_meta["sequence_reference_candidates"] = [
        {"target_quest": target, "reference_quest": reference, "offset": offset,
         "reference_source_units": order.get(reference, []),
         "use_condition": "native quest metadata exists but native unit lookup fails"}
        for target, reference, offset in hidden_meta.get("sequence_raw_triples", [])]
    base, malformed = base_placements(world)
    town_rules, area_phase_slots = town_phase_projection(world)
    slots_by_area = {row["area"]: row["phase_slots"] for row in area_phase_slots}
    phase = defaultdict(list)
    repeated = []
    phase_areas = []
    ambiguous = []
    for area_key, area in world["areas"].items():
        paths = [c["text"].replace("\\", "/").lower()
                 for row in sections(area["definition"], "[phase]") for c in row if c.get("type") == 6]
        if paths:
            phase_areas.append(area_key)
        for path, count in Counter(paths).items():
            if count > 1:
                repeated.append({"area": area_key, "path": path,
                                 "source_ordinals": [i for i, p in enumerate(paths) if p == path]})
        positions = defaultdict(set)
        for row in area.get("phase_npcs", []):
            path = row["map_path"].replace("\\", "/").lower()
            matches = [slot for slot in slots_by_area.get(area_key, [])
                       if slot["source_path"] is not None
                       and slot["source_path"].replace("\\", "/").lower() in {path, path.removeprefix("map/")}]
            phase[row["id"]].append({"area": area_key, **row,
                                      "root_phase_index_candidates": [s["phase_index"] for s in matches],
                                      "root_ownership_resolved": bool(matches),
                                      "ownership_note": "direct source root match" if matches else "phase import ownership not exported"})
            positions[row["id"]].add((row["x"], row["y"]))
        for npc, coordinates in positions.items():
            if len(coordinates) > 1:
                ambiguous.append({"area": area_key, "npc": npc,
                                  "coordinates": sorted(coordinates)})
    rules = visibility_rules(quests)
    override = npc_show_override_sources(quests)
    for npc_rules in rules.values():
        for rule in npc_rules:
            rule["source_order_candidates"] = order.get(rule["quest_id"], []) if order_meta["loaded"] else None
    parents = {int(k): [n for row in sections(v["script"]["cells"], "[pre required quest]")
                        for n in numbers(row)] for k, v in quests.items()}
    rows = []
    for quest_id, quest in sorted(quests.items(), key=lambda pair: int(pair[0])):
        if quest["kind"] != "[meet npc]":
            continue
        cs = quest["script"]["cells"]
        objective = quest.get("objective_cells") or []
        npc = objective[0]["value"] if len(objective) == 1 and objective[0].get("type") == 0 and objective[0]["value"] > 0 else None
        category = ("special_objective" if npc is None
                    else "base_candidate" if any(not p["source_presence_markers"] for p in base.get(npc, []))
                    else "conditional_base_candidate" if base.get(npc)
                    else "phase_only_candidate" if phase.get(npc)
                    else "no_exported_placement")
        depths = {int(quest_id): 0}
        todo = deque((parent, 1) for parent in parents[int(quest_id)])
        while todo:
            parent, depth = todo.popleft()
            if parent in depths:
                continue
            depths[parent] = depth
            todo.extend((p, depth + 1) for p in parents.get(parent, []))
        ancestors = set(depths) - {int(quest_id)}
        relevant = rules.get(npc, [])
        own = [r for r in relevant if r["quest_id"] == int(quest_id)]
        lineage = [{**r, "minimum_ancestor_depth": depths[r["quest_id"]]}
                   for r in relevant if r["quest_id"] in ancestors]
        external = [r for r in relevant if r["quest_id"] not in ancestors | {int(quest_id)}]
        subtype = scalar(cs, "[sub type]")
        rows.append({"quest_id": int(quest_id), "npc": npc, "category": category,
                     "source_order_candidates": order.get(int(quest_id), []) if order_meta["loaded"] else None,
                     "minimum_level": quest.get("minimum_level"), "subtype": subtype,
                     "server_remote_policy_candidate": npc is not None and subtype == 1 and not quest.get("pending"),
                     "pending": quest.get("pending", []), "source_path": quest["script"]["path"],
                     "source_sha256": quest["script"]["sha256"], "objective_cells": objective,
                     "completion_npc": scalar(cs, "[complete npc index]"),
                     "base_placements": base.get(npc, []), "phase_placements": phase.get(npc, []),
                     "default_hidden": (npc in hidden) if hidden is not None and npc is not None else None,
                     "source_prerequisites": parents[int(quest_id)], "ancestor_count": len(ancestors),
                     "own_visibility": own, "ancestor_visibility": lineage, "outside_lineage_visibility": external,
                     "show_override_sources": [s for s in override if npc is not None and s["npc"] == npc],
                     "guide_cells": list(sections(cs, "[go guide]"))})
    summary = dict(Counter(r["category"] for r in rows))
    summary.update({"meet_npc_total": len(rows), "quests_total": len(quests), "areas_total": len(world["areas"]),
                    "phase_areas": len(phase_areas), "repeated_phase_paths": len(repeated),
                    "ambiguous_phase_coordinates": len(ambiguous), "malformed_base_npc_sections": len(malformed),
                    "phase_only_without_own_visibility": sum(r["category"] == "phase_only_candidate" and not r["own_visibility"] for r in rows),
                    "phase_only_without_own_or_ancestor_visibility": sum(r["category"] == "phase_only_candidate" and not (r["own_visibility"] or r["ancestor_visibility"]) for r in rows),
                    "phase_only_remote_policy_candidates": sum(r["category"] == "phase_only_candidate" and r["server_remote_policy_candidate"] for r in rows),
                    "no_placement_remote_policy_candidates": sum(r["category"] == "no_exported_placement" and r["server_remote_policy_candidate"] for r in rows),
                    "phase_only_default_hidden": sum(r["category"] == "phase_only_candidate" and r["default_hidden"] is True for r in rows)})
    summary.update({"quest_show_override_source_rows": len(override),
                    "quest_show_override_unresolved_rows": sum(not s["projection_resolved"] for s in override),
                    "meet_targets_with_show_override_sources": sum(bool(r["show_override_sources"]) for r in rows)})
    all_phase_rules = [r for town in town_rules for r in town["rules"]]
    summary.update({"town_phase_rules": len(all_phase_rules),
                    "town_phase_rules_with_resolved_projection": sum(r["projection_resolved"] for r in all_phase_rules),
                    "town_phase_completed_quest_rules": sum(any(c["kind"] == 2 for c in r["conditions"]) for r in all_phase_rules),
                    "town_phase_event_rules": sum(any(c["kind"] in {3, 4} for c in r["conditions"]) for r in all_phase_rules),
                    "town_phase_base_reset_rules": sum(r["phase_index"] == -1 for r in all_phase_rules),
                    "conditional_base_npc_rows": sum(bool(p["source_presence_markers"]) for ps in base.values() for p in ps),
                    "conditional_base_npc_ids": sum(any(p["source_presence_markers"] for p in ps) for ps in base.values()),
                    "base_placement_gate_projection_unresolved": sum(not p["native_placement"]["projection_resolved"] for ps in base.values() for p in ps),
                    "base_placement_gated_by_accepted": sum(p["native_placement"]["predicate"] == "accepted" for ps in base.values() for p in ps),
                    "base_placement_gated_by_completed": sum(p["native_placement"]["predicate"] == "completed" for ps in base.values() for p in ps),
                    "source_marker_rows_without_placement_quest_gate": sum(bool(p["source_presence_markers"]) and p["native_placement"]["predicate"] == "none" for ps in base.values() for p in ps)})
    order_meta["visibility_rule_quests"] = len({r["quest_id"] for rs in rules.values() for r in rs})
    order_meta["visibility_rule_quests_with_source_units"] = (len({r["quest_id"] for rs in rules.values()
                                                                  for r in rs if order.get(r["quest_id"])})
                                                            if order_meta["loaded"] else None)
    unique_blocks = {(r["quest_id"], r["block_ordinal"]): r for rs in rules.values() for r in rs}
    summary.update({"visibility_blocks": len(unique_blocks),
                    "visibility_blocks_with_resolved_projection": sum(r["native_block"]["projection_resolved"] for r in unique_blocks.values()),
                    "visibility_blocks_no_transition_revert": sum(r["native_block"]["revert_on_transition"] is False for r in unique_blocks.values()),
                    "quests_no_cancel_revert": sum(any(c.get("type") == 3 and c.get("text") == "[npc visivility not revert]" for c in q["script"]["cells"]) for q in quests.values())})
    phase_scope = []
    for town in town_rules:
        ids = set(town["town_id_candidates"])
        targets = [r["quest_id"] for r in rows if r["category"] == "phase_only_candidate"
                   and any(int(p["area"].split("/")[0]) in ids for p in r["phase_placements"])]
        phase_scope.append({"town_id_candidates": town["town_id_candidates"], "path": town["path"],
                            **completed_phase_scope(town), "phase_only_quest_ids": targets})
    return {"schema_version": 8, "meaning": "candidate coverage and native placement predicates only; no runtime visibility or failure verdict",
            "source": source, "inputs": {str(p): checksum(p) for p in (world_path, quest_path)},
            "summary": summary, "global_hidden_source": hidden_meta, "quest_order_source": order_meta,
            "town_phase_rules": town_rules, "area_phase_slots": area_phase_slots, "completed_phase_scope": phase_scope,
            "npc_show_override_sources": override,
            "conditional_base_placements": [p for ps in base.values() for p in ps if p["source_presence_markers"]],
            "repeated_phase_paths": repeated, "ambiguous_phase_coordinates": ambiguous,
            "malformed_base_npc_sections": malformed, "quests": rows}


def markdown(result):
    lines = ["# Quest NPC presence candidate audit", "", "This is a static coverage report, not a list of broken quests.", "",
             "PVF source: `" + result["source"] + "`", "", "| Category | Count |", "| --- | ---: |"]
    lines += [f"| {k} | {v} |" for k, v in result["summary"].items()]
    order = result["quest_order_source"]
    lines += ["", "## Native ordering source coverage", "",
              "Native unit binding is traced. Order candidates do not select a runtime winner; effect replay and character state remain required.", "",
              f"Source loaded: {order['loaded']}; visibility-rule quests: {order['visibility_rule_quests']}; "
              f"with source units: {order['visibility_rule_quests_with_source_units']}."]
    lines += ["", "## Quest show-override source coverage", "",
              "The source tag [visible npc] feeds native show override. Listed sources do not prove an active override or a broken quest.", "",
              "| Quest | NPC key | Resolved | Registers key | Source |", "| --- | --- | --- | --- | --- |"]
    for row in result["npc_show_override_sources"]:
        lines.append(f"| {row['quest_id']} | {row['npc']} | {row['projection_resolved']} | {row['registers_show_override_key']} | {row['source_path']} |")
    lines += ["", "## Phase-only meet targets", "", "Any listed phase may be inactive. Hidden/show rules require actual character state and native ordering.", "",
              "| Quest | NPC | Areas | Own rules | Ancestor rules | Outside rules | Default hidden | Remote policy |",
              "| --- | --- | --- | ---: | ---: | ---: | --- | --- |"]
    for r in result["quests"]:
        if r["category"] == "phase_only_candidate":
            areas = ", ".join(sorted({p["area"] for p in r["phase_placements"]}))
            lines.append(f"| {r['quest_id']} | {r['npc']} | {areas} | {len(r['own_visibility'])} | {len(r['ancestor_visibility'])} | {len(r['outside_lineage_visibility'])} | {r['default_hidden']} | {r['server_remote_policy_candidate']} |")
    lines += ["", "## Conditional base-map meet targets", "",
              "Marker rows remain a source category. Negative native quest IDs bypass the placement quest gate; hidden/show rules still apply.", "",
              "| Quest | NPC | Areas | Source markers | Native quest IDs | Placement predicates |", "| --- | --- | --- | --- | --- | --- |"]
    for r in result["quests"]:
        if r["category"] == "conditional_base_candidate":
            areas = ", ".join(sorted({p["area"] for p in r["base_placements"]}))
            markers = ", ".join(sorted({m for p in r["base_placements"] for m in p["source_presence_markers"]}))
            ids = ", ".join(str(v) for v in sorted({p["native_placement"]["quest_id"] for p in r["base_placements"]}))
            predicates = ", ".join(sorted({p["native_placement"]["predicate"] or "unknown" for p in r["base_placements"]}))
            lines.append(f"| {r['quest_id']} | {r['npc']} | {areas} | {markers} | {ids} | {predicates} |")
    lines += ["", "## Completed-quest phase scenario scope", "",
              "Support requires explicit initial phase and completed quest set. It does not establish the live map or NPC visibility.", "",
              "| Town | Source | Supported | Phase-only targets | Missing semantics |", "| --- | --- | --- | ---: | --- |"]
    for town in result["completed_phase_scope"]:
        if town["phase_only_quest_ids"]:
            ids = ", ".join(map(str, town["town_id_candidates"]))
            reasons = "; ".join(town["reasons"])
            lines.append(f"| {ids} | {town['path']} | {town['supported']} | {len(town['phase_only_quest_ids'])} | {reasons} |")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--world", type=Path, default=CONFIGS / "world.generated.json")
    parser.add_argument("--quests", type=Path, default=CONFIGS / "quests.generated.json")
    parser.add_argument("--hidden-npc-export", type=Path)
    parser.add_argument("--quest-order-export", type=Path)
    parser.add_argument("--phase-scenario", type=Path, help="explicit offline phase inputs; never inferred from login or the database")
    parser.add_argument("--phase-trace", type=Path, help="explicit offline cache-operation inputs; no packet or database inference")
    parser.add_argument("--output", type=Path, default=ROOT / "server/work/dfo-lan/runtime/npc-presence-audit")
    args = parser.parse_args()
    result = audit(args.world, args.quests, args.hidden_npc_export, args.quest_order_export)
    comparison = phase_scenario(result, read_json(args.phase_scenario)) if args.phase_scenario else None
    trace = phase_trace(result, read_json(args.phase_trace)) if args.phase_trace else None
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / "audit.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (args.output / "audit.md").write_text(markdown(result), encoding="utf-8")
    if comparison is not None:
        comparison["scenario_file_sha256"] = checksum(args.phase_scenario)
        (args.output / "phase-scenario.json").write_text(json.dumps(comparison, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if trace is not None:
        trace["trace_file_sha256"] = checksum(args.phase_trace)
        (args.output / "phase-trace.json").write_text(json.dumps(trace, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result["summary"], sort_keys=True))


if __name__ == "__main__":
    main()
