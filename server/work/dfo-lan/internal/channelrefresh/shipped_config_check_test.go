package channelrefresh

import (
	"path/filepath"
	"testing"
)

// 随仓库发布的频道配置必须能过生产加载器（channelrefresh.Load），并且
// 凡显式写出 SourceValues 的频道，条目数必须正好 11 —— 合并 MR !148 后
// Resolve 会据此硬校验；配置一旦不合规，next37 档（SQLite 与 PG 都用它）启动即失败。
func TestShippedLocalChannelConfigsLoad(t *testing.T) {
	for _, name := range []string{"channel.local34.json", "channel.local35.json"} {
		path := filepath.Join("..", "..", "configs", name)
		cfg, err := Load(path)
		if err != nil {
			t.Errorf("%s: Load 失败: %v", name, err)
			continue
		}
		explicit := 0
		for _, ch := range cfg.Channels {
			if ch.SourceValues == nil {
				continue
			}
			explicit++
			if len(ch.SourceValues) != 11 {
				t.Errorf("%s 频道 %d 显式 SourceValues 有 %d 项，必须 11 项", name, ch.ID, len(ch.SourceValues))
			}
		}
		t.Logf("%s: %d 个频道，其中显式写 SourceValues 的 %d 个", name, len(cfg.Channels), explicit)
	}
}
