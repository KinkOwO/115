// wireprobe is a protocol experiment gateway. It binds the game port on the
// host named by -game-listen, which accepts a wildcard (0.0.0.0:PORT) so
// clients on other machines can reach it, and publishes the dialable address
// through the channel directory via -advertise-host.
package main

import (
	"dfolan/internal/channelrefresh"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

func main() {
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

	gameHost := prepared.gameHost
	moonConfig := prepared.moonConfig
	raw := prepared.raw

	l, err := net.Listen("tcp4", startup.GameListen)
	if err != nil {
		return err
	}
	defer l.Close()
	advertised, err := advertisedGameAddress(startup.AdvertiseHost, gameHost, l.Addr())
	if err != nil {
		return err
	}
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
		bindHost, _, _ := net.SplitHostPort(startup.GameListen)
		_, portText, _ := net.SplitHostPort(l.Addr().String())
		basePort, convErr := strconv.Atoi(portText)
		if convErr != nil {
			return convErr
		}
		advHost, _, _ := net.SplitHostPort(advertised)
		endpoints = map[uint32]channelrefresh.ChannelEndpoint{}
		for i, ch := range channelCfg.Channels {
			ln := l
			if i > 0 {
				ln, err = net.Listen("tcp4", net.JoinHostPort(bindHost, strconv.Itoa(basePort+i)))
				if err != nil {
					return err
				}
				defer ln.Close()
			}
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
