package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
	"os"
)

func WithClearCube(base catalog.LootCatalog, path string) (catalog.LootCatalog, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return base, e
	}
	var item catalog.LootItem
	if e = json.Unmarshal(raw, &item); e != nil {
		return base, e
	}
	return WithClearCubeItem(base, item)
}

// clearCubeSourceChecksum 是无色小晶块（clear cube）叠加目录必须匹配的源身份。
//
// 2026-10-01（next146）：直读模式下 base.Source.Checksum 由当次内层 PVF 决定，
// 不再是编译期写死的 "7ef2db59…"。由启动阶段调用 SetClearCubeSource 切到当次值；
// 未切换时保留旧常量语义（仍拒绝其它版本）。
var clearCubeSourceChecksum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

// clearCubeScriptSHA256s 是无色小晶块（3037）脚本的已审计原始字节哈希集合。
//
// 脚本字节里嵌的是字符串池偏移，Neople 每次重建字符串池都会整体漂移——即使
// 脚本 token 级完全一致，原始 SHA 也会变（2.38.2.34 与 2.38.3.25 的
// cubepiece_clear.stk 已逐 token 核对一致，仅池偏移不同）。因此每个经过审计的
// 客户端基线在此登记一条：
//   - c6b47f4d…：2.38.2.34（汉化 Script.pvf）
//   - 744fe805…：2.38.3.25（原版 Script.pvf，2026-10-04 审计）
var clearCubeScriptSHA256s = map[string]bool{
	"c6b47f4db2c1ba08b809aa54e6a512becba560199699c95cd9a264caf07077d5": true,
	"744fe805683bac92e4b548c9bafb2659d31be39811e0e20d684488f9e43a14f6": true,
}

// SetClearCubeSource 由目录准备阶段调用，把源身份切到当次内层 checksum。
// 只接受 64 位十六进制，否则忽略。
func SetClearCubeSource(checksum string) {
	if len(checksum) != 64 {
		return
	}
	for i := 0; i < len(checksum); i++ {
		c := checksum[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return
		}
	}
	clearCubeSourceChecksum = checksum
}

// WithClearCubeItem applies the same source gate and storage-only projection
// regardless of whether its complete script came from PVF or an audit export.
func WithClearCubeItem(base catalog.LootCatalog, item catalog.LootItem) (catalog.LootCatalog, error) {
	if base.Source.Checksum != clearCubeSourceChecksum || item.ID != 3037 || item.Kind != "stackable" || item.StackableType != "[material]" || !clearCubeScriptSHA256s[item.Script.SHA256] {
		return base, fmt.Errorf("clear cube source mismatch")
	}
	out := base
	out.Items = make(map[uint32]catalog.LootItem, len(base.Items)+1)
	for id, v := range base.Items {
		out.Items[id] = v
	}
	out.Items[item.ID] = item
	return out, nil
}
