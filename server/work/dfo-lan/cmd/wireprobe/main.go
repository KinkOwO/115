// wireprobe is a protocol experiment gateway. It binds the game port on the
// host named by -game-listen, which accepts a wildcard (0.0.0.0:PORT) so
// clients on other machines can reach it, and publishes the dialable address
// through the channel directory via -advertise-host.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/modpolicy"
	"dfolan/internal/servermod"
	"dfolan/mods"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	if _, err := ispinsWeeklyLimited(); err != nil {
		log.Fatal(err)
	}
	startup, configErr := loadConfig(os.Args[1:], os.Getenv, os.Stderr)
	if configErr != nil {
		if errors.Is(configErr, flag.ErrHelp) {
			return
		}
		// The private FlagSet already prints usage for malformed CLI input.
		os.Exit(2)
	}
	if err := runGateway(startup); err != nil {
		log.Fatal(err)
	}
}

// runGateway owns listening sockets and runtime resources until serving stops.
func runGateway(startup Config) error {
	// 模式策略（internal/modpolicy）是**进程级**状态，而网关可以在同一进程里被反复起停
	// ——测试 TestRunGatewayReleasesListenerOnStartupError 就会真起一次网关。
	// 所以策略的生命周期必须跟着**这一次服务端实例**：进来先清空（不继承上一次的规则），
	// 退出再清空（不给下一次/下一个用例留残留）。少任何一半，都会让「上一台服务端的规则」
	// 影响本进程里的其它测试：实测 2026-10-06 装上带 server.boot 钩子的 mod 后，
	// 上游的复活用例与默认放行用例都因此失败。
	modpolicy.Reset()
	defer modpolicy.Reset()
	// —— 服务端层 mod：注册阶段（必须在 prepareRuntime **之前**）——
	//
	// 为什么这么早：奖励管线是在 prepareRuntime 内部构造的
	// （bootstrap.go 的 buildRewardService），它会读一遍 mod 提供的规则脚本。
	// 若把注册放在 prepareRuntime 之后，规则脚本就赶不上那次构造 ——
	// 表现是 mod 的规则**静默不生效**，而日志仍显示"已装载/已登记"。
	// 这正是 2026-10-06 02:22 现场：新角色创建后没收到 mod 的邮件。
	//
	// 顺序（三者不能颠倒）：
	//  1. LoadEnabledList —— 先读"哪些 mod 被勾选"。mod 的 Register() 会问
	//     servermod.Enabled()，所以清单必须在注册之前就位；
	//  2. RegisterMods() —— 由 modkit 生成的 mods/zz_mods_gen.go，
	//     import 每个已装 mod 并按稳定顺序调用它们的 Register()。
	//     此阶段 mod 只应**登记钩子/规则**，不要读配置或访问存储（都还没就绪）；
	//  3. prepareRuntime —— 配置与存储初始化，并构造奖励管线（此时能看到规则）；
	//  4. Boot() —— 跑各 mod 的自检，此时配置与存储已就绪（失败即拒绝启动）。
	servermod.LoadEnabledList(serverModsDir())
	mods.RegisterMods()

	prepared, cleanup, err := prepareRuntime(startup)
	if err != nil {
		return err
	}
	// PVF check mode completed without starting the runtime.
	if prepared == nil {
		return nil
	}
	defer cleanup()
	startup = prepared.config

	// —— 服务端层 mod：自检阶段（配置与存储已就绪，还没开始监听）——
	//
	// mod 的自检失败必须让服务端"起不来"，而不是起一个半残的服务端让玩家撞上。
	servermod.SetEnvSnapshot(os.Environ())
	if err := servermod.Boot(&servermod.BootContext{
		Version:      versionString(),
		ChannelCount: 0,
	}); err != nil {
		return err
	}
	log.Print(servermod.Description())
	if _, disabled, note := servermod.EnabledInfo(); true {
		if note != "" {
			log.Printf("servermod: 启用清单：%s", note)
		}
		if len(disabled) > 0 {
			log.Printf("servermod: 已禁用 %d 个 mod（%s）", len(disabled), strings.Join(disabled, ", "))
		}
	}
	if names := servermod.RewardScriptNames(); len(names) > 0 {
		log.Printf("servermod: mod 提供的奖励规则脚本 %d 份：%s", len(names), strings.Join(names, ", "))
	}
	if claims := servermod.ContentClaims(); len(claims) > 0 {
		for _, c := range claims {
			log.Printf("servermod: mod 声明的内容扩展意图 → %s", c)
		}
	}
	// 模式规则（由 mod 通过 internal/modpolicy 设置）：开没开、谁开的都要在日志里。
	// server/AGENTS §6 的教训是"默认路径悄悄坏掉最难查"，所以这里留一行可核对的证据。
	logModPolicy()
	// 启动期一次性 mod 命令（可选）：DFO_SERVERMOD_CONSOLE="<mod-id> <name> [args]"
	// 服务端没有可交互控制台（启动器以隐藏窗口拉起），所以命令是一次性的、结果进日志。
	if spec := os.Getenv("DFO_SERVERMOD_CONSOLE"); spec != "" {
		if err := servermod.RunConsoleOnce(spec); err != nil {
			return fmt.Errorf("启动期 mod 命令失败：%w", err)
		}
	}

	gameHost := prepared.gameHost
	moonConfig := prepared.moonConfig
	raw := prepared.raw

	// One game port per channel. The client dials the port listed for the channel
	// it picked, and the game connection itself never carries a channel number,
	// so the port a client arrives on is the only way to tell channels apart.
	type channelListener struct {
		channel uint32
		ln      net.Listener
	}
	var listeners []channelListener
	var channelCfg channelrefresh.Config
	var endpoints map[uint32]channelrefresh.ChannelEndpoint
	// channelTypes maps a channel id to the Type of its directory row. The game
	// connection carries no channel number, so the port a client dialled is the
	// only channel identity a session has (see the listeners below); this map
	// turns that identity back into the script value handlers report.
	channelTypes := map[uint32]uint32{}
	if startup.ChannelRefreshConfig != "" {
		channelCfg, err = channelrefresh.Load(startup.ChannelRefreshConfig)
		if err != nil {
			return err
		}
		// 规则层直读：频道属性来自 etc/clientchannelinfo.etc，普通频道的 ID/Area/SourceValues
		// 与 [dungeon] 区域表来自 etc/channel_info.etc。configs 只声明 {ID, Name}
		// （外加必要的本地 Type 覆盖，见 channelrefresh.Config.Resolve）。
		if prepared.channelDirectory == nil || prepared.channelInfo == nil {
			return errors.New("频道目录需要 PVF 直读投影（preparePVFChannels 未装载）")
		}
		ordinary := map[uint32]catalog.ChannelInfoRow{}
		if rows, ok := prepared.channelInfo.Rows(channelCfg.ServerID); ok {
			for _, row := range rows {
				ordinary[row.ID] = row
			}
		}
		if err = channelCfg.Resolve(func(id uint32) (channelrefresh.ChannelAttributes, bool) {
			// 普通频道优先：channel_info.etc 带 Area 与 11 个 SourceValues。
			if row, ok := ordinary[id]; ok {
				values := make([]int32, 0, len(row.SourceValues))
				for _, s := range row.SourceValues {
					v, convErr := strconv.ParseInt(s, 10, 32)
					if convErr != nil {
						log.Fatalf("频道 %d 的 SourceValue %q 不是整数", id, s)
					}
					values = append(values, int32(v))
				}
				return channelrefresh.ChannelAttributes{Type: row.Type, Area: row.Area, SourceValues: values}, true
			}
			// 特殊频道：clientchannelinfo.etc 给属性，源里没有区域与 SourceValues（用 [none] 与 0）。
			if a, ok := prepared.channelDirectory.Attributes(id); ok {
				return channelrefresh.ChannelAttributes{Type: a.Type, Area: "[none]", SourceValues: make([]int32, 11)}, true
			}
			return channelrefresh.ChannelAttributes{}, false
		}); err != nil {
			return err
		}
		channelCfg.Dungeons = prepared.channelInfo.AreaDungeons
		for _, ch := range channelCfg.Channels {
			channelTypes[ch.ID] = ch.Type
		}
		if moonConfig != nil {
			found := false
			for _, ch := range channelCfg.Channels {
				if ch.ID == moonConfig.Channel {
					found = ch.Type == 101
				}
			}
			if !found {
				return errors.New("Moon channel must exist with source online type 101")
			}
		}
	}

	// 主监听与频道端口是**一组**：任一端口抽到 Windows 保留段（WinNAT/Hyper-V 动态保留）就整组重抽
	// —— 见 listen.go。频道端口是主监听基准端口之后的连续端口（base+1 … base+N），所以必须在打开
	// 监听之前就知道要几个频道。
	extraPorts := 0
	if len(channelCfg.Channels) > 1 {
		extraPorts = len(channelCfg.Channels) - 1
	}
	block, err := openGameListeners(startup.GameListen, extraPorts)
	if err != nil {
		return err
	}
	defer closeGameListeners(block)
	l := block[0]
	advertised, err := advertisedGameAddress(startup.AdvertiseHost, gameHost, l.Addr())
	if err != nil {
		return err
	}
	if startup.ChannelRefreshConfig != "" {
		_, portText, _ := net.SplitHostPort(l.Addr().String())
		basePort, convErr := strconv.Atoi(portText)
		if convErr != nil {
			return convErr
		}
		advHost, _, _ := net.SplitHostPort(advertised)
		endpoints = map[uint32]channelrefresh.ChannelEndpoint{}
		for i, ch := range channelCfg.Channels {
			ln := block[i]
			listeners = append(listeners, channelListener{channel: ch.ID, ln: ln})
			endpoints[ch.ID] = channelrefresh.ChannelEndpoint{ID: ch.ID, Host: advHost, Port: uint16(basePort + i)}
		}
	} else {
		listeners = append(listeners, channelListener{channel: 0, ln: l})
	}
	ready, _ := json.Marshal(map[string]any{"address": l.Addr().String(), "advertise": advertised, "pid": os.Getpid(), "fixture_bytes": len(raw), "channels": len(listeners)})
	if err = os.WriteFile(filepath.Join(startup.Output, "ready.json"), ready, 0600); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(startup.Output, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	var mu sync.Mutex
	event := func(v map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		v["time"] = time.Now().UTC().Format(time.RFC3339Nano)
		if err := json.NewEncoder(f).Encode(v); err != nil {
			log.Print(err)
		}
	}
	fmt.Println(string(ready))
	if endpoints != nil {
		cl, e := channelrefresh.Serve(channelCfg, endpoints, event)
		if e != nil {
			return e
		}
		defer cl.Close()
		event(map[string]any{"kind": "channel_refresh_ready", "address": cl.Addr().String(), "channels": len(endpoints)})
	}
	gateway := &gameGateway{runtime: prepared, channels: channelCfg, channelTypes: channelTypes, event: event}
	for _, set := range listeners {
		go func(set channelListener) {
			for {
				c, err := set.ln.Accept()
				if err != nil {
					log.Print(err)
					return
				}
				go gateway.handleClient(c, set.channel)
			}
		}(set)
	}
	select {}
}
