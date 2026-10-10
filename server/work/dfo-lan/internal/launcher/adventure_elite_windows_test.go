//go:build windows

package launcher

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// An import-free x64 test DLL that only returns a chosen DWORD. No game files,
// MSVC installation, administrator privileges or optional test switch required.
func eliteTestDLL(t *testing.T, code uint32) string {
	t.Helper()
	data := make([]byte, 0x600)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:], 0x80)
	copy(data[0x80:], "PE\x00\x00")
	header := pe.FileHeader{Machine: pe.IMAGE_FILE_MACHINE_AMD64, NumberOfSections: 1, SizeOfOptionalHeader: 240,
		Characteristics: pe.IMAGE_FILE_DLL | pe.IMAGE_FILE_EXECUTABLE_IMAGE | pe.IMAGE_FILE_LARGE_ADDRESS_AWARE}
	optional := pe.OptionalHeader64{Magic: 0x20b, SizeOfCode: 0x400, BaseOfCode: 0x1000, ImageBase: 0x180000000,
		SectionAlignment: 0x1000, FileAlignment: 0x200, MajorOperatingSystemVersion: 6, MajorSubsystemVersion: 6,
		SizeOfImage: 0x2000, SizeOfHeaders: 0x200, Subsystem: pe.IMAGE_SUBSYSTEM_WINDOWS_CUI, DllCharacteristics: 0x140,
		SizeOfStackReserve: 0x100000, SizeOfStackCommit: 0x1000, SizeOfHeapReserve: 0x100000, SizeOfHeapCommit: 0x1000, NumberOfRvaAndSizes: 16}
	optional.DataDirectory[0] = pe.DataDirectory{VirtualAddress: 0x1100, Size: 0x80}
	optional.DataDirectory[5] = pe.DataDirectory{VirtualAddress: 0x1180, Size: 12}
	section := pe.SectionHeader32{VirtualSize: 0x400, VirtualAddress: 0x1000, SizeOfRawData: 0x400, PointerToRawData: 0x200,
		Characteristics: pe.IMAGE_SCN_CNT_CODE | pe.IMAGE_SCN_MEM_EXECUTE | pe.IMAGE_SCN_MEM_READ}
	copy(section.Name[:], ".text")
	var headers bytes.Buffer
	for _, item := range []any{header, optional, section} {
		require.NoError(t, binary.Write(&headers, binary.LittleEndian, item))
	}
	copy(data[0x84:], headers.Bytes())
	data[0x200] = 0xb8 // mov eax, <DWORD>; ret
	binary.LittleEndian.PutUint32(data[0x201:], code)
	data[0x205] = 0xc3
	put := func(offset int, value uint32) { binary.LittleEndian.PutUint32(data[offset:], value) }
	put(0x30c, 0x1150)
	put(0x310, 1)
	put(0x314, 1)
	put(0x318, 1)
	put(0x31c, 0x1140)
	put(0x320, 0x1144)
	put(0x324, 0x1148)
	put(0x340, 0x1000)
	put(0x344, 0x1160)
	copy(data[0x350:], "EliteFixture.dll\x00")
	copy(data[0x360:], "ModStartInjected\x00")
	put(0x380, 0x1000)
	put(0x384, 12) // relocation block with two ABSOLUTE entries
	path := filepath.Join(t.TempDir(), "EliteFixture.dll")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func TestAdventureEliteInjectionChild(t *testing.T) {
	if os.Getenv("DFO_ELITE_TEST_CHILD") == "1" {
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
}

func TestAdventureEliteRemoteInjectionAndRejectCleanup(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	for _, code := range []uint32{0, 13} {
		t.Run("return-"+strconv.FormatUint(uint64(code), 10), func(t *testing.T) {
			dll := eliteTestDLL(t, code)
			rva, err := eliteModStartRVA(dll)
			require.NoError(t, err)
			require.Equal(t, uint32(0x1000), rva)
			process, exit, err := startHostProcess(hostProcessSpec{Target: exe, Args: []string{"-test.run=^TestAdventureEliteInjectionChild$"},
				WorkingDir: filepath.Dir(exe), Env: append(os.Environ(), "DFO_ELITE_TEST_CHILD=1"), Timeout: 10 * time.Second})
			require.NoError(t, err)
			require.Equal(t, 0, exit)
			t.Cleanup(func() { process.terminateAndWait(time.Second); process.closeAll() })
			require.ErrorContains(t, injectAdventureElite(process.Pid, dll, exe, 0), "会话 Job")
			require.ErrorContains(t, injectAdventureElite(process.Pid, dll, exe+".wrong", uintptr(process.job)), "路径")
			err = injectAdventureElite(process.Pid, dll, exe, uintptr(process.job))
			if code == 0 {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "code=13")
			}
		})
	}
}

func TestEliteModuleSnapshotRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.dll")
	var module windows.ModuleEntry32
	name, err := windows.UTF16FromString(path)
	require.NoError(t, err)
	copy(module.ExePath[:], name)
	calls := 0
	got, err := eliteFindModuleSnapshot(1, path, time.Now().Add(time.Second), func(uint32) ([]windows.ModuleEntry32, error) {
		calls++
		if calls == 1 {
			return nil, windows.ERROR_NO_MORE_FILES
		}
		return []windows.ModuleEntry32{module}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, module, got)
	_, err = eliteFindModuleSnapshot(1, path, time.Now().Add(-time.Second), func(uint32) ([]windows.ModuleEntry32, error) { return nil, windows.ERROR_NO_MORE_FILES })
	require.ErrorContains(t, err, "超时")
	_, err = eliteFindModuleSnapshot(1, path, time.Now().Add(time.Second), func(uint32) ([]windows.ModuleEntry32, error) { return nil, windows.ERROR_ACCESS_DENIED })
	require.ErrorIs(t, err, windows.ERROR_ACCESS_DENIED)
}
