package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
)

func TestTownRobotInspectUsesArchiveAndSceneIdentity(t *testing.T) {
	for _, isolation := range []string{"none", "channel", "private", "departed", "unverified", "foreign", "real", "self", "badmode", "missingidentity"} {
		t.Run(isolation, func(t *testing.T) {
			f := robotManagementFixture(t)
			c, out, _ := newDispatchTestClient()
			h := f.hub
			h.townRobots = f
			c.hub = h
			c.selectedCharacterID = 2
			p := &lanPeer{roleID: 2, actorID: 3, info: []byte{1}}
			b := &lanPeer{robot: true, roleID: -1, actorID: 65534, info: []byte{0, 1, 0, 3, 86}}
			f.bots = []*townRobot{{recordID: 1, peer: b}}
			h.add(p)
			h.registerTownRobot(b)
			h.publish(p, 1, 38, 0, 500, 200, false)
			h.publish(b, 1, 38, 0, 600, 200, false)
			c.worldState = &worldSession{hub: h, peer: p, account: 12, role: database.Character{ID: 2, AccountID: 12, WireID: 2}}
			body := make([]byte, 8)
			binary.LittleEndian.PutUint16(body, 65534)
			body[2] = 3
			q := &clientRequest{frame: wire.Frame{Type: 1, ID: 8}, plaintext: body, verified: true}
			switch isolation {
			case "channel":
				b.channel = 2
			case "private":
				b.private = true
			case "departed":
				delete(h.peers, b)
			case "unverified":
				q.verified = false
			case "foreign":
				c.worldState.role.AccountID = 99
			case "real":
				b.robot = false
			case "self":
				binary.LittleEndian.PutUint16(body, 65535)
			case "missingidentity":
				b.info = nil
			case "badmode":
				body[2] = 1
			}
			before := bytes.Clone(f.manager.records[1].Role.State)
			c.dispatchTownRobotInspect(q)
			if !bytes.Equal(before, f.manager.records[1].Role.State) {
				t.Fatal("inspect mutated archive")
			}
			if isolation != "none" {
				if out.Len() != 0 {
					t.Fatal("out-of-scope inspect reply")
				}
				return
			}
			if out.Len() == 0 {
				t.Fatal("no native inspect reply")
			}
			payload, err := wire.DecryptPayload(c.keys, 2, out.Bytes()[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) < 279 || payload[0] != 3 || !bytes.Equal(payload[3:5], b.info[3:5]) || binary.LittleEndian.Uint16(payload[277:]) != 65534 {
				t.Fatalf("wrong native target %x", payload[:5])
			}
		})
	}
}

func TestTownRobotRefreshRetainsIntroducedIdentity(t *testing.T) {
	for _, identity := range [][2]byte{{3, 86}, {1, 10}, {0, 0}, {255, 254}} {
		f := robotManagementFixture(t)
		b := &lanPeer{robot: true, roleID: -1, actorID: 65534, info: []byte{0, 1, 0, identity[0], identity[1]}}
		f.bots = []*townRobot{{recordID: 1, peer: b}}
		f.hub.registerTownRobot(b)
		f.refreshRobot(1)
		if len(b.info) <= 5 || !bytes.Equal(b.info[3:5], identity[:]) {
			t.Fatalf("refresh changed scene identity %x: %x", identity, b.info)
		}
	}
}
