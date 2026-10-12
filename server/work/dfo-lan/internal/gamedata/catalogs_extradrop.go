package gamedata

import (
	"dfolan/internal/extradrop"
	"dfolan/internal/oracleresearch"
	"fmt"
	"log"
)

// ExtraDrop 读活动 1093（额外掉落奖励）的源脚本 `.ctp`。
//
// 与 313/331 同样的取舍（见 catalogs_honeytime.go）：**不按域门禁** —— 这份脚本 3.7 KB，
// 为一个只读脚本动 `SupportedDomains` + `configs/pvf-default.json` + **重编 dfolauncher.exe**
// 不划算。活动开关仍在 op108 那条行（`DFO_EVENT_INFO_ACTIVITY`）。
//
// 读失败**只降级不熔断**：活动整体不装配，玩家照常进城，不伪造内容。
func (s *Source) ExtraDrop() (*extradrop.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, fmt.Errorf("extradrop import requires PVF")
	}
	return extradrop.Load(s.archive)
}

// preparePVFExtraDrop 装配活动 1093 的目录。失败只记 warning。
func preparePVFExtraDrop(c *Catalogs, s *Source) {
	cat, err := s.ExtraDrop()
	if err != nil {
		log.Printf("warning: extradrop 1093 disabled; PVF source read failed: %v", err)
		return
	}
	c.ExtraDrop = cat
	log.Printf("PVF extradrop 1093 prepared: path=%s dungeon=%q groups=%d maxAmount=%d types=%v",
		cat.Path, cat.TargetDungeonType, len(cat.Groups), cat.MaxAmount, cat.MaxTypes)
}

// Oracle685 读活动 685 的源脚本 `.ctp`（`OracleSimulationResearch.ctp`）。
//
// 与 ExtraDrop/HoneyTime 同样的取舍：**不按域门禁**（15 KB 的只读脚本不值得动
// SupportedDomains + pvf-default.json + 重编 dfolauncher.exe）。
// 读失败只降级不熔断：活动不装配，玩家照常进城。
func (s *Source) Oracle685() (*oracleresearch.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, fmt.Errorf("oracle685 import requires PVF")
	}
	return oracleresearch.Load(s.archive)
}

// preparePVFOracle685 装配活动 685 的目录。失败只记 warning。
func preparePVFOracle685(c *Catalogs, s *Source) {
	cat, err := s.Oracle685()
	if err != nil {
		log.Printf("warning: oracle685 disabled; PVF source read failed: %v", err)
		return
	}
	c.Oracle685 = cat
	log.Printf("PVF oracle685 prepared: %s", cat.Describe())
}
