//go:build windows

package featurebridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"net"
	"time"
	"unsafe"
)

var tcpTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

// ownerPID matches all four endpoint fields, rather than trusting a port or a
// PID supplied by the extension. Only the original local game TCP is accepted.
func ownerPID(client, server *net.TCPAddr) (uint32, error) {
	var size uint32
	r, _, _ := tcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 5, 0)
	if r != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || size < 4 || size > 16<<20 {
		return 0, fmt.Errorf("TCP owner size unavailable")
	}
	for tries := 0; tries < 3; tries++ {
		p := make([]byte, size)
		r, _, _ = tcpTable.Call(uintptr(unsafe.Pointer(&p[0])), uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 5, 0)
		if r == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			if size > 16<<20 {
				return 0, fmt.Errorf("TCP table too large")
			}
			continue
		}
		if r != 0 {
			return 0, fmt.Errorf("TCP owner query failed")
		}
		n := binary.LittleEndian.Uint32(p)
		if uint64(n)*24+4 > uint64(len(p)) {
			return 0, fmt.Errorf("invalid TCP owner table")
		}
		for i := uint32(0); i < n; i++ {
			row := p[4+i*24 : 4+(i+1)*24]
			if binary.LittleEndian.Uint32(row) == 5 && bytes.Equal(row[4:8], client.IP.To4()) && bytes.Equal(row[12:16], server.IP.To4()) && int(binary.BigEndian.Uint16(row[8:10])) == client.Port && int(binary.BigEndian.Uint16(row[16:18])) == server.Port {
				return binary.LittleEndian.Uint32(row[20:]), nil
			}
		}
		return 0, fmt.Errorf("original game TCP owner not found")
	}
	return 0, fmt.Errorf("TCP table changed during query")
}

// Start attaches one private local IPC endpoint to this game connection. Calls
// must be serviced on the game's existing single-owner connection loop. Closing
// the game socket cancels reads, queued calls and pipe ownership without hooks.
func Start(c net.Conn, done <-chan struct{}) (<-chan Call, func(), error) {
	remote, ok := c.RemoteAddr().(*net.TCPAddr)
	local, lok := c.LocalAddr().(*net.TCPAddr)
	if !ok || !lok || !remote.IP.IsLoopback() || !local.IP.IsLoopback() || remote.IP.To4() == nil || local.IP.To4() == nil {
		return nil, func() {}, nil
	}
	pid, e := ownerPID(remote, local)
	if e != nil || pid == 0 {
		return nil, func() {}, fmt.Errorf("extension game process unavailable: %w", e)
	}
	name, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\115us-features-v1-%d-%d-%d`, pid, remote.Port, local.Port))
	// Keep the first-instance handle for the entire TCP lifetime. Recreating
	// it after each reply races the client's closing handle and can permanently
	// lose the endpoint with ERROR_ACCESS_DENIED.
	h, e := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE, windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_NOWAIT|windows.PIPE_REJECT_REMOTE_CLIENTS, 1, 65536, 65536, 0, nil)
	if e != nil {
		return nil, func() {}, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := make(chan Call)
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	go func() {
		defer close(calls)
		defer cancel()
		defer windows.CloseHandle(h)
		for ctx.Err() == nil {
			servePipe(ctx, h, remote, local, pid, calls)
			windows.DisconnectNamedPipe(h)
		}
	}()
	return calls, cancel, nil
}

func pause(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}
func servePipe(ctx context.Context, h windows.Handle, remote, local *net.TCPAddr, pid uint32, calls chan<- Call) {
	for {
		e := windows.ConnectNamedPipe(h, nil)
		if e == nil || errors.Is(e, windows.ERROR_PIPE_CONNECTED) {
			break
		}
		if !errors.Is(e, windows.ERROR_PIPE_LISTENING) || pause(ctx) != nil {
			return
		}
	}
	var peer uint32
	if windows.GetNamedPipeClientProcessId(h, &peer) != nil || peer != pid {
		return
	}
	current, e := ownerPID(remote, local)
	if e != nil || current != pid {
		return
	}
	deadline := time.Now().Add(5 * time.Second)
	reader := &pipeReader{ctx: ctx, h: h, deadline: deadline}
	req, e := Read(reader)
	if e != nil || req.Kind&Response != 0 {
		return
	}
	call := Call{req, make(chan Frame, 1)}
	select {
	case calls <- call:
	case <-ctx.Done():
		return
	case <-time.After(time.Until(deadline)):
		return
	}
	var reply Frame
	select {
	case reply = <-call.Reply:
	case <-ctx.Done():
		return
	case <-time.After(time.Until(deadline)):
		return
	}
	p, e := reply.Bytes()
	if e != nil {
		return
	}
	for len(p) > 0 && time.Now().Before(deadline) && ctx.Err() == nil {
		var n uint32
		e = windows.WriteFile(h, p, &n, nil)
		if e != nil {
			return
		}
		p = p[n:]
		if n == 0 && pause(ctx) != nil {
			return
		}
	}
	// Keep the instance alive until the client drains its response. NOWAIT mode
	// avoids FlushFileBuffers blocking forever when the client exits early.
	for ctx.Err() == nil && time.Now().Before(deadline) {
		var n uint32
		buf := make([]byte, 1)
		e = windows.ReadFile(h, buf, &n, nil)
		if e != nil && !errors.Is(e, windows.ERROR_NO_DATA) {
			return
		}
		if n != 0 {
			return
		}
		if pause(ctx) != nil {
			return
		}
	}
}

type pipeReader struct {
	ctx      context.Context
	h        windows.Handle
	deadline time.Time
}

func (r *pipeReader) Read(p []byte) (int, error) {
	for r.ctx.Err() == nil && time.Now().Before(r.deadline) {
		var n uint32
		e := windows.ReadFile(r.h, p, &n, nil)
		if n > 0 {
			return int(n), nil
		}
		if e != nil && !errors.Is(e, windows.ERROR_NO_DATA) {
			return 0, e
		}
		if pause(r.ctx) != nil {
			break
		}
	}
	return 0, io.ErrUnexpectedEOF
}
