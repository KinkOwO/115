package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 普通频道目录的**直读**真源（不经过任何导出 JSON）：
//
//	etc/channel_info.etc
//
// 结构（实测文本）：
//
//	[dungeon]                       ← 区域 → 地下城 id 列表
//	`[elven_guard]` 1 2
//	[/dungeon]
//
//	[server]                        ← 第一个数是 ServerID，随后是若干组
//	1 1 2 `[elven_guard]` 10 0 0 0 0 0 0 0 0 0 0 6 3 `[none]` 0 0 …
//	[/server]
//
// 每组 14 个 token：`<ID> <Type> <Area> <11 个 SourceValues>`。
//
// ⚠️ 这张表**只有普通频道**：全部 11 个 server 的 type 都在 0..6 之间，区域也只有
// elven_guard/granfloris/sky_catle/behemoth/stormpass/Alfhlyra/north_myre/[none]。
// 特殊频道（军团、F7 那些）**不在这里** —— 它们的属性在 clientchannelinfo.etc，
// 面板归属在 channelslotinfo.etc（见 channel_directory.go）。
const ChannelInfoPath = "etc/channel_info.etc"

// channelInfoGroupTokens 是一组频道行的 token 数：ID + Type + Area + 11 个 SourceValues。
const channelInfoGroupTokens = 3 + 11

// ChannelInfoRow 是源里一条普通频道行。
//
// SourceValues 保留源里的**原样文本**（严格来说 11 个 scalar）：普通 server 都是整数，
// 但 server 98 写的是 `0.50 0.30 0.20` 这类小数 —— 不做数值解释就不必替源选精度。
type ChannelInfoRow struct {
	ID           uint32   `json:"id"`
	Type         uint32   `json:"type"`
	Area         string   `json:"area"`
	SourceValues []string `json:"source_values"`
}

// ChannelInfoServer 是一个 `[server]` 块。
type ChannelInfoServer struct {
	ServerID uint32           `json:"server_id"`
	Rows     []ChannelInfoRow `json:"rows"`
}

// ChannelInfo 是 etc/channel_info.etc 的直读投影。
type ChannelInfo struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	Servers map[uint32]ChannelInfoServer `json:"servers"`
	// AreaDungeons 是 [dungeon] 段的区域 → 地下城 id 列表（供诊断与区域校验）。
	AreaDungeons map[string][]uint32 `json:"area_dungeons"`
}

// ServerIDs 返回全部 server 编号（升序）。
func (c ChannelInfo) ServerIDs() []uint32 {
	out := make([]uint32, 0, len(c.Servers))
	for id := range c.Servers {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Rows 返回指定 server 的频道行（不存在时 ok=false）。
func (c ChannelInfo) Rows(serverID uint32) ([]ChannelInfoRow, bool) {
	s, ok := c.Servers[serverID]
	if !ok {
		return nil, false
	}
	return s.Rows, true
}

// ImportChannelInfo 从内层归档直读 etc/channel_info.etc。
func ImportChannelInfo(a *pvf.Archive) (ChannelInfo, error) {
	out := ChannelInfo{
		Path:         ChannelInfoPath,
		Servers:      map[uint32]ChannelInfoServer{},
		AreaDungeons: map[string][]uint32{},
	}
	if a == nil {
		return out, fmt.Errorf("channel info: archive is nil")
	}
	raw, text, err := readChannelScript(a, ChannelInfoPath)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)

	root, err := parseJournalTree(mergeDanglingTagValues(text))
	if err != nil {
		return out, fmt.Errorf("%s: %w", ChannelInfoPath, err)
	}
	for _, sec := range root.children("dungeon") {
		fields := strings.Fields(nodeHead(sec))
		if len(fields) < 1 {
			return out, fmt.Errorf("%s: a [dungeon] row has no area", ChannelInfoPath)
		}
		area := strings.Trim(fields[0], "`")
		if area == "" {
			return out, fmt.Errorf("%s: a [dungeon] row has an empty area", ChannelInfoPath)
		}
		ids := make([]uint32, 0, len(fields)-1)
		for _, f := range fields[1:] {
			v, ok := journalUint(f)
			if !ok {
				return out, fmt.Errorf("%s: [dungeon] %s has a non-numeric id %q", ChannelInfoPath, area, f)
			}
			ids = append(ids, v)
		}
		out.AreaDungeons[area] = ids
	}
	for _, sec := range root.children("server") {
		fields := strings.Fields(nodeHead(sec))
		if len(fields) == 0 {
			return out, fmt.Errorf("%s: a [server] row is empty", ChannelInfoPath)
		}
		serverID, ok := journalUint(fields[0])
		if !ok || serverID == 0 {
			return out, fmt.Errorf("%s: [server] %q is not a usable server id", ChannelInfoPath, fields[0])
		}
		if _, dup := out.Servers[serverID]; dup {
			return out, fmt.Errorf("%s: duplicate [server] %d", ChannelInfoPath, serverID)
		}
		rest := fields[1:]
		if len(rest)%channelInfoGroupTokens != 0 {
			return out, fmt.Errorf("%s: [server] %d has %d field(s) after the server id, not a multiple of %d",
				ChannelInfoPath, serverID, len(rest), channelInfoGroupTokens)
		}
		rows := make([]ChannelInfoRow, 0, len(rest)/channelInfoGroupTokens)
		for i := 0; i < len(rest); i += channelInfoGroupTokens {
			g := rest[i : i+channelInfoGroupTokens]
			id, ok1 := journalUint(g[0])
			ct, ok2 := journalUint(g[1])
			area := strings.Trim(g[2], "`")
			if !ok1 || id == 0 || !ok2 || area == "" {
				return out, fmt.Errorf("%s: [server] %d has an unusable channel row starting at %q",
					ChannelInfoPath, serverID, g[0])
			}
			values := make([]string, 0, 11)
			for _, f := range g[3:] {
				if _, err := strconv.ParseFloat(f, 64); err != nil {
					return out, fmt.Errorf("%s: [server] %d channel %d has a non-numeric value %q",
						ChannelInfoPath, serverID, id, f)
				}
				values = append(values, f)
			}
			rows = append(rows, ChannelInfoRow{ID: id, Type: ct, Area: area, SourceValues: values})
		}
		out.Servers[serverID] = ChannelInfoServer{ServerID: serverID, Rows: rows}
	}
	if len(out.Servers) == 0 {
		return out, fmt.Errorf("%s has no [server] block", ChannelInfoPath)
	}
	out.Source = a.Snapshot()
	return out, nil
}
