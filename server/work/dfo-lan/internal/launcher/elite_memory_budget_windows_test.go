package launcher

import (
	"golang.org/x/sys/windows"
	"testing"
	"time"
	"unsafe"
)

func TestEliteRecordedD3D11PressureHasHeadroom(t *testing.T) {
	// r6 recorded 8,459,309,056 bytes just before 333 D3D11 COM errors.
	// The job covers descendants too, so replay a further 2 GiB transient
	// resource reservation without actually allocating gigabytes on this PC.
	recorded := uint64(8459309056)
	peak := recorded + 2<<30
	if clientMemoryBudget <= peak {
		t.Fatalf("recorded client plus resource headroom exceeds job budget: %d <= %d", clientMemoryBudget, peak)
	}
	var guard memoryPressureGuard
	if got := guard.observe(time.Now(), peak, 9<<30, clientMemoryBudget); got.stopTrigger != "" {
		t.Fatalf("legion resource peak still rejected: %+v", got)
	}
	if got := guard.observe(time.Now(), clientMemoryBudget, 9<<30, clientMemoryBudget); got.stopTrigger != "process_commit_limit" {
		t.Fatalf("bounded commit protection lost: %+v", got)
	}
}

func TestEliteClientAndGatewayHaveSeparateBoundedJobs(t *testing.T) {
	for _, budget := range []uint64{clientMemoryBudget, serverMemoryBudget} {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = configureMemoryJob(job, budget)
		var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		if err == nil {
			err = windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil)
		}
		windows.CloseHandle(job)
		if err != nil {
			t.Fatal(err)
		}
		if uint64(limits.ProcessMemoryLimit) != budget || uint64(limits.JobMemoryLimit) != budget || limits.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
			t.Fatalf("job lost memory/descendant protection: %+v", limits)
		}
	}
	if serverMemoryBudget != 4<<30 {
		t.Fatal("gateway budget changed")
	}
}
