package main

import (
	"errors"
	"net"
	"strconv"
	"testing"
)

type reservedTestListener struct {
	port   int
	closed bool
}

func (l *reservedTestListener) Accept() (net.Conn, error) { return nil, errors.New("test listener") }
func (l *reservedTestListener) Close() error              { l.closed = true; return nil }
func (l *reservedTestListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: l.port}
}

func TestGameListenerBlockRetriesRejectedNeighbourWithoutLeaking(t *testing.T) {
	var allocated []*reservedTestListener
	bases := 0
	listen := func(network, addr string) (net.Listener, error) {
		_, text, _ := net.SplitHostPort(addr)
		port, _ := strconv.Atoi(text)
		if port == 0 {
			bases++
			port = 20000 + bases*100
		}
		if port == 20101 {
			return nil, errors.New("reserved neighbour")
		}
		l := &reservedTestListener{port: port}
		allocated = append(allocated, l)
		return l, nil
	}
	block, err := openGameListenersWith("127.0.0.2:0", 2, listen)
	if err != nil || len(block) != 3 || bases != 2 {
		t.Fatalf("block=%v bases=%d error=%v", block, bases, err)
	}
	if !allocated[0].closed {
		t.Fatal("rejected partial block leaked its base listener")
	}
	for i, l := range block {
		if l.Addr().(*net.TCPAddr).Port != 20200+i {
			t.Fatal("non-contiguous advertised block")
		}
	}
	closeGameListeners(block)
	for _, l := range allocated {
		if !l.closed {
			t.Fatal("listener leaked")
		}
	}
}

func TestGameListenerFixedBlockDoesNotMoveAndReleasesPartialBind(t *testing.T) {
	first := &reservedTestListener{port: 21000}
	calls := 0
	listen := func(network, addr string) (net.Listener, error) {
		calls++
		if calls == 1 {
			return first, nil
		}
		return nil, errors.New("occupied")
	}
	block, err := openGameListenersWith("127.0.0.2:21000", 1, listen)
	if err == nil || len(block) != 0 || calls != 2 || !first.closed {
		t.Fatalf("block=%v error=%v calls=%d closed=%v", block, err, calls, first.closed)
	}
}

func TestGameListenerRejectsPortOverflow(t *testing.T) {
	first := &reservedTestListener{port: 65535}
	_, err := openGameListenersWith("127.0.0.2:65535", 1, func(string, string) (net.Listener, error) { return first, nil })
	if err == nil || !first.closed {
		t.Fatal("overflow accepted or listener leaked")
	}
}
