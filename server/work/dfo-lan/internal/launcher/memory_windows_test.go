package launcher

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestProtectedChildHelper(t *testing.T) {
	if os.Getenv("DFOLAN_MEMORY_TEST_CHILD") != "1" {
		return
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	err := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil)
	if err != nil || limits.ProcessMemoryLimit != 64<<20 || limits.JobMemoryLimit != 64<<20 {
		fmt.Fprintln(os.Stderr, "child began without expected memory protection", err)
		os.Exit(11)
	}
	// This allocates at most 64 MiB, not the real production budget. Windows must
	// reject a commit beyond the job limit; no game or user storage is involved.
	var allocations []uintptr
	for i := 0; i < 16; i++ {
		p, err := windows.VirtualAlloc(0, 8<<20, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if err != nil {
			for _, p := range allocations {
				windows.VirtualFree(p, 0, windows.MEM_RELEASE)
			}
			fmt.Fprintln(os.Stdout, "commit rejected by active job limit")
			os.Exit(0)
		}
		allocations = append(allocations, p)
	}
	os.Exit(12)
}

func TestProtectedChildCannotRunOrCommitOutsideJob(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestProtectedChildHelper$")
	cmd.Env = append(os.Environ(), "DFOLAN_MEMORY_TEST_CHILD=1")
	var out, log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &log
	child, err := startMemoryBoundedChild(cmd, 64<<20, "limit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer child.terminate(time.Second)
	if !child.waitTimeout(15 * time.Second) {
		t.Fatal("child did not finish")
	}
	if child.exitCode() != 0 || !strings.Contains(out.String(), "commit rejected") {
		t.Fatalf("code=%d stdout=%s stderr=%s", child.exitCode(), out.String(), log.String())
	}
	if !strings.Contains(log.String(), "MEMORY_GUARD active") {
		t.Fatal(log.String())
	}
}

func TestProtectedChildWaitAndTerminate(t *testing.T) {
	cmd := exec.Command("ping.exe", "-n", "6", "127.0.0.1")
	var log bytes.Buffer
	cmd.Stderr = &log
	child, err := startMemoryBoundedChild(cmd, 64<<20, "termination-test")
	if err != nil {
		t.Fatal(err)
	}
	if child.waitTimeout(150 * time.Millisecond) {
		t.Fatal("child should still be running")
	}
	child.terminate(5 * time.Second)
	if !child.waitTimeout(time.Second) {
		t.Fatal("protected child was not reaped")
	}
}

func TestProtectedMemoryMonitoringSurvivesSessionLogClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.err")
	log, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ping.exe", "-n", "6", "127.0.0.1")
	cmd.Stderr = log
	child, err := startMemoryBoundedChild(cmd, 64<<20, "log-lifetime-test")
	log.Close() // exactly what startSession does when ready is returned
	if err != nil {
		t.Fatal(err)
	}
	defer child.terminate(time.Second)
	time.Sleep(3 * time.Second)
	child.terminate(5 * time.Second)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), "MEMORY_GUARD sample") < 2 || !strings.Contains(string(body), "MEMORY_GUARD ended") {
		t.Fatalf("monitor stopped with caller log handle: %s", body)
	}
}
