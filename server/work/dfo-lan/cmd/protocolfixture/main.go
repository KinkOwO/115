package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: protocolfixture output.bin [login]")
	}
	key := make([]byte, wire.SessionKeyBytes)
	for i := range key {
		key[i] = byte(i%127 + 1)
	}
	id := uint16(1554)
	kind := byte(1)
	payload := []byte{1}
	if len(os.Args) > 2 && os.Args[2] == "login" {
		id = 1
		var b bytes.Buffer
		// Exact flat reads in 0x1452543D0. Fixture values are synthetic;
		// this command only probes the parser, never authenticates an account.
		b.Write([]byte{1, 30, 0, 0, 0}) // success, age, PC room, channel type, flag
		binary.Write(&b, binary.LittleEndian, uint32(time.Now().Unix()))
		binary.Write(&b, binary.LittleEndian, uint32(0)) // first string byte length
		binary.Write(&b, binary.LittleEndian, uint32(0)) // adjacent value A
		binary.Write(&b, binary.LittleEndian, uint32(0)) // adjacent value B
		binary.Write(&b, binary.LittleEndian, uint32(0)) // second string byte length
		b.Write(make([]byte, 7))                         // 0x1452547F7 through 0x1452548D6
		b.Write(make([]byte, 4))                         // 0x145255168, 51B3, 51C0, 5291
		binary.Write(&b, binary.LittleEndian, uint16(0)) // 0x1452552F1
		b.Write([]byte{0, 0})                            // 0x14525537E, 53AD
		payload = b.Bytes()
	}
	if len(os.Args) > 2 && os.Args[2] == "characters" {
		kind, id = 0, 2
		var b bytes.Buffer
		b.Write([]byte{2, 0, 0}) // list mode; two per-packet appearance flags
		for _, v := range []uint16{8, 0, 0} {
			binary.Write(&b, binary.LittleEndian, v)
		}
		binary.Write(&b, binary.LittleEndian, uint32(0))
		binary.Write(&b, binary.LittleEndian, uint16(0)) // empty character count
		b.Write([]byte{1, 0})                            // page number, optional event flag
		binary.Write(&b, binary.LittleEndian, uint32(0))
		binary.Write(&b, binary.LittleEndian, uint32(0))
		payload = b.Bytes()
	}
	if len(os.Args) > 2 && os.Args[2] == "name" {
		id = 684
	}
	if len(os.Args) > 2 && os.Args[2] == "characters-row" {
		kind, id = 0, 2
		var err error
		payload, err = protocol.CharacterList(8, []protocol.CharacterRow{{Slot: 0, Name: "WireProbe01", Profession: 0, Level: 1}})
		if err != nil {
			log.Fatal(err)
		}
	}
	encrypted, err := wire.EncryptPayload(key, id, payload)
	if err != nil {
		log.Fatal(err)
	}
	frame, err := wire.ServerFrame(kind, id, encrypted)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(os.Args[1], frame, 0600); err != nil {
		log.Fatal(err)
	}
}
