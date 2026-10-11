//go:build windows

package wfpisolate

import (
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// Verify APP_ID matching separately from loopback exclusion. The dynamic filter
// applies only to this standalone test executable and is closed on every exit.
func TestLiveWFPAppIDBlock(t *testing.T) {
	if os.Getenv("DFO_WFP_LIVE_TEST") != "1" || !isElevated() {
		t.Skip("requires opt-in, administrator and a standalone executable copy")
	}
	engine, subLayer, err := openEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer closeEngine(engine)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	appID, err := appIDFromFileName(exe)
	if err != nil {
		t.Fatal(err)
	}
	filter := BuildFilterLayers(subLayer, appID)[0]
	filter.Conditions = filter.Conditions[:1]
	if err := addFilter(engine, filter, appID); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conn, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("APP_ID-only block did not block this executable")
	}
	if errno := winsockErrno(err); errno != int(syscall.WSAEACCES) {
		t.Fatalf("expected WSAEACCES; got %d: %v", errno, err)
	}
}
