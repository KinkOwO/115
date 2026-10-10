package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

func TestIspinsPostSelectionWithoutOutputDoesNotStartKeepalive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &gameConnection{worldState: &worldSession{channelType: 81}}
		c.ispinsPostSelection(7)
		if !c.worldState.pendingLegionEntryInfo {
			t.Fatal("standby quota restoration was not scheduled")
		}
		// The original panic happened after the first 30-second tick, long
		// after the test that created this incomplete connection had returned.
		time.Sleep(31 * time.Second)
		synctest.Wait()
	})
}

func TestIspinsPostSelectionKeepaliveStopsWithConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, conn, events := newDispatchTestClient()
		c.worldState.channelType = 81
		c.connection = &connectionSession{done: make(chan struct{})}
		c.ispinsPostSelection(7)
		synctest.Wait()
		time.Sleep(29 * time.Second)
		synctest.Wait()
		if conn.Len() != 0 {
			t.Fatal("keepalive sent before the native 30-second interval")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		raw := conn.Bytes()
		if len(raw) < 16 || raw[0] != 0 || binary.LittleEndian.Uint16(raw[1:3]) != 1719 {
			t.Fatalf("missing SEC_PING_CHECK frame: %x", raw)
		}
		plain, err := wire.DecryptPayload(c.keys, 1719, raw[16:])
		want := []byte{0, 0, 0, 0, 0xe1, 0x6b, 0xa3, 0x76, 0x3f, 0, 0, 0, 0, 0, 0, 0}
		if err != nil || !bytes.Equal(plain, want) {
			t.Fatalf("keepalive payload changed: %x, %v", plain, err)
		}
		if len(*events) != 1 || (*events)[0]["kind"] != "sec_ping_check_sent" || (*events)[0]["character_id"] != int64(7) {
			t.Fatalf("keepalive owner/log changed: %v", *events)
		}
		written := conn.Len()
		close(c.connection.done)
		synctest.Wait()
		time.Sleep(60 * time.Second)
		synctest.Wait()
		if conn.Len() != written {
			t.Fatal("keepalive continued after connection closure")
		}
	})
}

func TestIspinsPostSelectionCancelsDelayedLoginOnClose(t *testing.T) {
	for _, channelType := range []uint32{0, legion.DimCloisterHallChannelType} {
		t.Run(fmt.Sprint(channelType), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c, conn, events := newDispatchTestClient()
				c.worldState.channelType = channelType
				c.connection = &connectionSession{done: make(chan struct{})}
				c.ispinsPostSelection(7)
				synctest.Wait()
				close(c.connection.done)
				synctest.Wait()
				time.Sleep(31 * time.Second)
				synctest.Wait()
				if conn.Len() != 0 || len(*events) != 0 {
					t.Fatalf("closed connection received login/keepalive output: %d bytes, %v", conn.Len(), *events)
				}
			})
		})
	}
}

func TestIspinsDispatchStartPushesWaitingStateFromSnapshot(t *testing.T) {
	t.Setenv("DFO_ISPINS_MODE", "unlimited")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	events := make(chan map[string]any, 16)
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7, keys: make([]byte, wire.SessionKeyBytes), worldState: &worldSession{channelType: 81, role: database.Character{ID: 7, WireID: 7}}, event: func(e map[string]any) { events <- e }}
	c.output = newConnectionOutput(server, c.keys, "test", c.event)
	c.worldState.dungeons = ispinsCardFixtureCatalog()
	body := make([]byte, 24)
	body[13] = 101
	result := make(chan dispatchAction, 1)
	go func() {
		result <- c.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 2043}, plaintext: body, verified: true})
	}()
	read := func() (byte, uint16, []byte) {
		t.Helper()
		peer.SetReadDeadline(time.Now().Add(4 * time.Second))
		h := make([]byte, 16)
		if _, e := io.ReadFull(peer, h); e != nil {
			t.Fatal(e)
		}
		size := int(binary.LittleEndian.Uint32(h[3:7]))
		if size < 16 || size > wire.MaxPacketSize {
			t.Fatal(size)
		}
		b := make([]byte, size-16)
		if _, e := io.ReadFull(peer, b); e != nil {
			t.Fatal(e)
		}
		id := binary.LittleEndian.Uint16(h[1:3])
		plain, e := wire.DecryptPayload(c.keys, id, b)
		if e != nil {
			t.Fatal(e)
		}
		return h[0], id, plain
	}
	for _, want := range []uint16{2254, 2255, 2043} {
		_, id, _ := read()
		if id != want {
			t.Fatal("immediate start sequence", id, want)
		}
	}
	if got := <-result; got != dispatchHandled {
		t.Fatal(got)
	}
	order := c.worldState.ispins.order
	want, err := c.worldState.ispinsWaitInfo()
	if err != nil {
		t.Fatal(err)
	}
	// Change state after the start dispatch. Its scheduled notice must still
	// describe the original waiting stage and owner, not this mutable state.
	c.worldState.ispins.cleared[0] = true
	c.selectedCharacterID = 99
	kind, id, plain := read()
	for i, index := range order {
		binary.LittleEndian.PutUint32(want[19+8*i:], uint32(index))
	}
	if kind != 0 || id != 2255 || len(plain) < len(want) || !bytes.Equal(plain[:len(want)], want) {
		t.Fatal("missing native wait0 notice", kind, id)
	}
	for {
		select {
		case e := <-events:
			if e["kind"] == "ispins_info_wait_pushed" {
				if e["character_id"] != int64(7) {
					t.Fatal("notice owner was not snapshotted", e)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("wait notice delivery not logged")
		}
	}
}

func TestIspinsDispatchTownPositionFallsThrough(t *testing.T) {
	c, conn, _ := newDispatchTestClient()
	c.selectedCharacterID = 7
	c.worldState.role = database.Character{ID: 7, WireID: 7}
	request := &clientRequest{frame: wire.Frame{Type: 1, ID: 35}, plaintext: make([]byte, 8), verified: true}
	if got := c.dispatchIspins(request); got != dispatchNext {
		t.Fatal("Ispins swallowed ordinary town movement", got)
	}
	if conn.Len() != 0 {
		t.Fatal("plain town input emitted Ispins packets")
	}
	// An initial standby CMD35 must deliver pending quota information and
	// still reach the ordinary world handler after those notifications.
	c.worldState.channelType = 81
	c.worldState.pendingLegionEntryInfo = true
	if got := c.dispatchIspins(request); got != dispatchNext || c.worldState.pendingLegionEntryInfo {
		t.Fatal("pending standby did not fall through", got)
	}
	if conn.Len() == 0 {
		t.Fatal("pending standby packets missing")
	}
}
