package launcher

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"path/filepath"

	"dfolan/internal/adventureelite"
)

// The script selects the isolated launcher/server candidates when opted in.
// Both children consume the same effective environment; this does not enable
// unimplemented ordinary/story/Odyssey entry adapters.
func adventureEliteDLL(root string, env *childEnv) (string, error) {
	if env == nil || !adventureelite.EnabledValue(env.Get(adventureelite.EnvKey)) {
		return "", nil
	}
	if forceProbeFallback(env) {
		return "", fmt.Errorf("%s=1 需要 Go 宿主注入，不能同时启用 %s", adventureelite.EnvKey, probeFallbackEnvKey)
	}
	path := filepath.Join(root, "client-patchs", "adventure-elite", "dist", "AdventureElite.dll")
	if _, err := eliteModStartRVA(path); err != nil {
		return "", fmt.Errorf("精锐资格 DLL 预检失败：%w；先运行 scripts/build-adventure-elite.ps1", err)
	}
	return path, nil
}

// Read the export without loading the plugin into the launcher. Forwarded or
// non-executable exports are rejected; a remote address is module-base + RVA.
func eliteModStartRVA(path string) (uint32, error) {
	f, err := pe.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	header, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok || f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || f.Characteristics&pe.IMAGE_FILE_DLL == 0 {
		return 0, fmt.Errorf("不是 x64 DLL：%s", path)
	}
	read := func(rva, size uint32) ([]byte, error) {
		for _, section := range f.Sections {
			if uint64(rva) >= uint64(section.VirtualAddress) && uint64(rva)+uint64(size) <= uint64(section.VirtualAddress)+uint64(section.Size) {
				data := make([]byte, size)
				_, e := section.ReadAt(data, int64(rva-section.VirtualAddress))
				return data, e
			}
		}
		return nil, fmt.Errorf("导出 RVA 超出文件节：0x%x", rva)
	}
	export := header.DataDirectory[0]
	if export.Size < 40 || uint64(export.VirtualAddress)+uint64(export.Size) > uint64(header.SizeOfImage) {
		return 0, fmt.Errorf("无有效导出表")
	}
	directory, err := read(export.VirtualAddress, 40)
	if err != nil {
		return 0, err
	}
	u32 := func(offset int) uint32 { return binary.LittleEndian.Uint32(directory[offset:]) }
	functions, names := u32(20), u32(24)
	if functions == 0 || functions > 65536 || names > 65536 {
		return 0, fmt.Errorf("导出表数量无效")
	}
	addresses, err := read(u32(28), functions*4)
	if err != nil {
		return 0, err
	}
	nameRVAs, err := read(u32(32), names*4)
	if err != nil {
		return 0, err
	}
	ordinals, err := read(u32(36), names*2)
	if err != nil {
		return 0, err
	}
	for i := uint32(0); i < names; i++ {
		nameRVA := binary.LittleEndian.Uint32(nameRVAs[i*4:])
		name, e := read(nameRVA, uint32(len("ModStartInjected\x00")))
		if e != nil || string(name) != "ModStartInjected\x00" {
			continue
		}
		ordinal := uint32(binary.LittleEndian.Uint16(ordinals[i*2:]))
		if ordinal >= functions {
			return 0, fmt.Errorf("ModStart ordinal 无效")
		}
		rva := binary.LittleEndian.Uint32(addresses[ordinal*4:])
		if uint64(rva) >= uint64(export.VirtualAddress) && uint64(rva) < uint64(export.VirtualAddress)+uint64(export.Size) {
			return 0, fmt.Errorf("不支持转发 ModStart")
		}
		for _, section := range f.Sections {
			if section.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE != 0 && rva >= section.VirtualAddress && uint64(rva) < uint64(section.VirtualAddress)+uint64(section.Size) {
				return rva, nil
			}
		}
		return 0, fmt.Errorf("ModStart 不在可执行节")
	}
	return 0, fmt.Errorf("DLL 缺少 ModStart")
}
