package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"time"
)

func TestOnlineTimerPreservesPartialFrame(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	done := make(chan struct{})
	defer close(done)
	frames := clientFrames(reader, done)
	raw := make([]byte, 29)
	raw[0] = 1
	binary.LittleEndian.PutUint16(raw[1:], 18)
	binary.LittleEndian.PutUint32(raw[3:], 29)
	first := make(chan error, 1)
	go func() { _, e := writer.Write(raw[:15]); first <- e }()
	if e := <-first; e != nil {
		t.Fatal(e)
	}
	select {
	case <-frames:
		t.Fatal("partial frame returned")
	case <-time.After(5 * time.Millisecond):
	}
	go func() { _, e := writer.Write(raw[15:]); first <- e }()
	select {
	case got := <-frames:
		if got.err != nil || !bytes.Equal(got.frame.Raw, raw) {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("frame blocked")
	}
	if e := <-first; e != nil {
		t.Fatal(e)
	}
}
