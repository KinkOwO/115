// legionimport 导出内层 PVF 里的军团内容表：
//
//	contents/system/legionsystem/legionsystem.cos → configs/legion-contents.generated.json
//
// 这张表是军团（末世录 / 维纳斯 / 达斯岛 / 千海之空…）**入口侧**的真源：招募区/等候区坐标、
// 每个阶段的副本号与 boss 号、每日/每周入口次数、难度×单双人的名声门槛、计数键
// [incount dungeon index]，以及频道页签的 ENABLE_CHANNEL_TAB_EVENT_ID（分享版口中的
// channel OpenEvent）。apocalypse.ctp 只有作战/阶段时钟/奖励，没有这些。
//
// 字段名保留源写法；含义未证的值按位置保留（LegionDungeon.Tail / LegionEnterRow.Values），
// 不翻译语义。可随时从同一份 PVF 重新生成并与 source hash 对照。
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func writeAtomic(name string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(name), ".legion-import-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, name)
}

func main() {
	source := flag.String("source", "", "read-only inner PVF")
	output := flag.String("output", "configs/legion-contents.generated.json", "output catalog")
	replace := flag.Bool("replace", false, "explicitly replace an existing output")
	flag.Parse()
	if *source == "" {
		log.Fatal("-source is required")
	}
	if !*replace {
		if _, e := os.Stat(*output); !os.IsNotExist(e) {
			log.Fatal("output already exists; use a fresh path or explicit -replace")
		}
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	out, e := catalog.ImportLegionContents(a)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(out, "", " ")
	if e != nil {
		log.Fatal(e)
	}
	if e = writeAtomic(*output, b); e != nil {
		log.Fatal(e)
	}
	log.Printf("source=%s path=%s sha256=%s bytes=%d contents=%d",
		out.Source.Checksum, out.Path, out.SHA256, out.Bytes, len(out.Contents))
	for name, c := range out.Contents {
		log.Printf("  %s: limitLevel=%d lastPhase=%d dungeons=%d weeklyEnter=%d phaseReward=%d incount=%d recruiting=%d/%d(%d,%d) waiting=%d/%d(%d,%d) tabEvent=%d",
			name, c.LimitLevel, c.LastPhase, len(c.Dungeons), c.WeeklyEnter, c.PhaseReward, c.InCountIndex,
			c.Recruiting.Town, c.Recruiting.Area, c.Recruiting.X, c.Recruiting.Y,
			c.Waiting.Town, c.Waiting.Area, c.Waiting.X, c.Waiting.Y, c.UIInts["ENABLE_CHANNEL_TAB_EVENT_ID"])
	}
}
