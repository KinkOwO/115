package channelrefresh

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestEmptyConnectionCloseIsNonErrorEvent(t *testing.T) {
	events := make(chan map[string]any, 1)
	c := Config{Listen: "127.0.0.1:0", ServerID: 1, MaxUsers: 100, Channels: []Channel{{ID: 10, Name: "test", Area: "[none]"}}}
	listener, err := Serve(c, map[uint32]ChannelEndpoint{10: {ID: 10, Host: "127.0.0.1", Port: 7002}}, func(e map[string]any) { events <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conn, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	select {
	case e := <-events:
		if e["error"] != "" || e["success"] != false || e["outcome"] != "empty_connection_closed" {
			t.Fatalf("empty TCP close is not a completed refresh or protocol error: %#v", e)
		}
	case <-time.After(time.Second * 3):
		t.Fatal("missing close event")
	}
}

type endReadConn struct {
	r       *bytes.Reader
	end     error
	written bytes.Buffer
}

func (c *endReadConn) Read(b []byte) (int, error) {
	n, e := c.r.Read(b)
	if e == io.EOF && c.end != nil {
		return n, c.end
	}
	return n, e
}
func (c *endReadConn) Write(b []byte) (int, error)      { return c.written.Write(b) }
func (c *endReadConn) Close() error                     { return nil }
func (c *endReadConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *endReadConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *endReadConn) SetDeadline(time.Time) error      { return nil }
func (c *endReadConn) SetReadDeadline(time.Time) error  { return nil }
func (c *endReadConn) SetWriteDeadline(time.Time) error { return nil }
func clientRequest(id byte, size int, body []byte) []byte {
	b := make([]byte, 11)
	b[1] = id
	b[10] = 1
	binary.LittleEndian.PutUint32(b[2:], uint32(size))
	return append(b, body...)
}

func TestIncompleteChannelRequestsRemainErrors(t *testing.T) {
	reset := errors.New("connection reset by peer")
	cases := []struct {
		name           string
		data           []byte
		end, errorWant error
	}{
		{"partial header", []byte{0}, nil, io.ErrUnexpectedEOF},
		{"body never sent", clientRequest(11, 43, nil), nil, io.EOF},
		{"partial body", clientRequest(11, 43, []byte{1}), nil, io.ErrUnexpectedEOF},
		{"authenticated peer quits before request", clientRequest(11, 43, make([]byte, 32)), nil, io.EOF},
		{"read timeout", nil, os.ErrDeadlineExceeded, os.ErrDeadlineExceeded},
		{"connection reset", nil, reset, reset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := &endReadConn{r: bytes.NewReader(tc.data), end: tc.end}
			err := handle(conn, []byte("script"), []byte("directory"))
			if !errors.Is(err, tc.errorWant) {
				t.Fatalf("got %v, want retained %v", err, tc.errorWant)
			}
		})
	}
	for _, b := range [][]byte{clientRequest(9, 11, nil), clientRequest(11, 11, nil), clientRequest(99, 11, nil)} {
		conn := &endReadConn{r: bytes.NewReader(b)}
		if err := handle(conn, nil, nil); err == nil {
			t.Fatal("malformed or unauthenticated request accepted")
		}
	}
}
