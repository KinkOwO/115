"""Scoped replacement of the probe's already-built entry response sequence."""
from pathlib import Path
p = Path(__file__).parent.parent / 'dfo-lan/cmd/wireprobe/main.go'
s = p.read_text(encoding='utf-8')
start = s.index('\t\t\t\t\tpayload, e := protocol.SelectProbeSuccess(profile)')
end = s.index('\t\t\t\t\tcontinue\n', s.index('"kind": "fatigue_sent"', start))
new = '''                    payload, e := protocol.SelectProbeSuccess(profile)
                    if e != nil {
                        event(map[string]any{"kind":"entry_prepare_error", "error":e.Error()})
                        continue
                    }
                    var userArea []byte
                    if worldState != nil && len(areaPayload) > 0 {
                        userArea, e = worldState.userAreaPayload()
                        if e != nil {
                            event(map[string]any{"kind":"entry_prepare_error", "error":e.Error()})
                            continue
                        }
                    }
                    plan := entryPayloads{Select:payload, Basic:basic, Addition:addition, Vault:vaultPayload, UserArea:userArea, Area:areaPayload, Fatigue:fatiguePayload}
                    prepared, e := preparePackets(keys, plan.packets())
                    if e != nil {
                        event(map[string]any{"kind":"entry_encode_error", "character_id":role.ID, "error":e.Error(), "frames_sent":0})
                        continue
                    }
                    event(map[string]any{"kind":"entry_preflight_passed", "character_id":role.ID, "frame_count":len(prepared)})
                    c.SetWriteDeadline(time.Now().Add(5 * time.Second))
                    e = writePackets(c, prepared, func(p preparedPacket) {
                        entry := map[string]any{"kind":p.Name, "character_id":role.ID, "actor_server_id":role.WireID, "type":p.Kind, "id":p.ID, "plain_bytes":len(p.Payload), "client_acceptance":"pending"}
                        if p.ID == 4 { entry["name"] = role.Name }
                        if p.ID == 23 || p.ID == 24 {
                            if worldState != nil {
                                entry["position"] = worldState.state.Position
                            } else {
                                entry["town_id"], entry["area_id"] = townCatalog.TownID, townCatalog.AreaID
                            }
                        }
                        if p.ID == 13 || p.ID == 36 { entry["plain_hex"] = hex.EncodeToString(p.Payload) }
                        event(entry)
                    })
                    if e != nil {
                        event(map[string]any{"kind":"entry_write_error", "character_id":role.ID, "error":e.Error()})
                        return
                    }
                    selectedCharacterID = role.ID
                    selectedBasic, selectedAddition = basic, addition
'''
p.write_text(s[:start] + new + s[end:], encoding='utf-8')
print('entry response sequence replaced with preflight and logged writes')
