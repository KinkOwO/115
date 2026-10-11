//go:build windows

package featurebridge

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestExtensionPipeUsesOriginalLocalTCPAndCancels(t *testing.T) {
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, e)
	defer listener.Close()
	c, e := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, e)
	defer c.Close()
	server, e := listener.Accept()
	require.NoError(t, e)
	defer server.Close()
	done := make(chan struct{})
	defer close(done)
	calls, stop, e := Start(server, done)
	require.NoError(t, e)
	defer stop()
	remote := server.RemoteAddr().(*net.TCPAddr)
	local := server.LocalAddr().(*net.TCPAddr)
	pid, e := ownerPID(remote, local)
	require.NoError(t, e)
	require.EqualValues(t, os.Getpid(), pid)
	wrong := *remote
	wrong.Port++
	_, e = ownerPID(&wrong, local)
	require.Error(t, e)
	name, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\115us-features-v1-%d-%d-%d`, pid, remote.Port, local.Port))
	var h windows.Handle
	for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
		h, e = windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, e)
	f := os.NewFile(uintptr(h), "test-private-pipe")
	defer f.Close()
	request, _ := (Frame{Kind: Snapshot, ID: 99}).Bytes()
	_, e = f.Write(request)
	require.NoError(t, e)
	select {
	case call := <-calls:
		require.EqualValues(t, 99, call.Request.ID)
		call.Reply <- Frame{Kind: Snapshot | Response, ID: 99, Body: []byte("owned-session")}
	case <-time.After(3 * time.Second):
		t.Fatal("no private pipe call")
	}
	reply, e := Read(f)
	require.NoError(t, e)
	require.True(t, bytes.Equal(reply.Body, []byte("owned-session")))
	f.Close()
	stop()
	select {
	case _, open := <-calls:
		require.False(t, open)
	case <-time.After(3 * time.Second):
		t.Fatal("pipe did not cancel")
	}
}

func TestExtensionPipeRefusesOtherProcess(t *testing.T) {
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, e)
	defer listener.Close()
	c, e := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, e)
	defer c.Close()
	server, e := listener.Accept()
	require.NoError(t, e)
	defer server.Close()
	done := make(chan struct{})
	defer close(done)
	calls, stop, e := Start(server, done)
	require.NoError(t, e)
	defer stop()
	remote := server.RemoteAddr().(*net.TCPAddr)
	local := server.LocalAddr().(*net.TCPAddr)
	name := fmt.Sprintf(`\\.\pipe\115us-features-v1-%d-%d-%d`, os.Getpid(), remote.Port, local.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExtensionPipeUnauthorizedHelper$")
	cmd.Env = append(os.Environ(), "US115_TEST_FOREIGN_PIPE="+name)
	out, e := cmd.CombinedOutput()
	require.NoError(t, e, string(out))
	select {
	case <-calls:
		t.Fatal("foreign process reached game loop")
	default:
	}
}

func TestExtensionPipeUnauthorizedHelper(t *testing.T) {
	name := os.Getenv("US115_TEST_FOREIGN_PIPE")
	if name == "" {
		return
	}
	p, _ := windows.UTF16PtrFromString(name)
	var h windows.Handle
	var e error
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); {
		h, e = windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, e)
	f := os.NewFile(uintptr(h), "foreign-helper")
	defer f.Close()
	request, _ := (Frame{Kind: Snapshot, ID: 1}).Bytes()
	_, e = f.Write(request)
	if e == nil {
		_, e = Read(f)
	}
	require.Error(t, e, "a different process must not receive an extension reply")
}

// A client may retain its old file handle after the reply deadline. The
// server must reconnect its reserved instance, not lose FIRST_PIPE_INSTANCE.
func TestExtensionPipeReconnectWithOldClientHandle(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	c, err := net.Dial("tcp4", l.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	s, err := l.Accept()
	require.NoError(t, err)
	defer s.Close()
	done := make(chan struct{})
	defer close(done)
	calls, stop, err := Start(s, done)
	require.NoError(t, err)
	defer stop()
	go func() {
		for call := range calls {
			call.Reply <- Frame{Kind: call.Request.Kind | Response, ID: call.Request.ID}
		}
	}()
	name, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\115us-features-v1-%d-%d-%d`, os.Getpid(), c.LocalAddr().(*net.TCPAddr).Port, s.LocalAddr().(*net.TCPAddr).Port))
	open := func() *os.File {
		var h windows.Handle
		var e error
		for end := time.Now().Add(7 * time.Second); time.Now().Before(end); {
			h, e = windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
			if e == nil {
				return os.NewFile(uintptr(h), "reconnect-pipe")
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("endpoint lost: %v", e)
		return nil
	}
	first := open()
	defer first.Close()
	p, _ := (Frame{Kind: SymbolSnapshot, ID: 1}).Bytes()
	_, err = first.Write(p)
	require.NoError(t, err)
	_, err = Read(first)
	require.NoError(t, err)
	// Keep first open: second becomes available after the server deadline.
	second := open()
	defer second.Close()
	p, _ = (Frame{Kind: SymbolSnapshot, ID: 2}).Bytes()
	_, err = second.Write(p)
	require.NoError(t, err)
	r, err := Read(second)
	require.NoError(t, err)
	require.EqualValues(t, 2, r.ID)
}
