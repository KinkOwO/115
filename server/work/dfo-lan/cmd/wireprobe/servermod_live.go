package main

import (
	"log"
	"os"
	"sync"
	"time"

	"dfolan/internal/servermod"
)

// 本文件把 internal/servermod 的「热开关 + 运行时实况」接到主程序上。
//
// 两件事：
//  1. **轮询启用清单**：管理器（启动器 MOD 工具页）改 `mods/enabled.json` 之后，
//     服务端自己发现并搬运 live 状态，不需要重启、不需要重编译；
//  2. **落一份实况文件** `runtime/mods-status.json`：界面不再靠"在 30MB 的 exe 里
//     搜 mod id + 比 mtime"来猜生效，而是直接读进程写出的事实。
//
// 为什么用轮询而不是 fsnotify：多带一个依赖换来的收益只有"少一次 stat"，
// 而 stat 的成本是微秒级——不划算。判据用 (存在, 大小, mtime) 指纹，
// 没变就一个字节都不读（见 servermod.RefreshEnabledList）。

const (
	// modLivePoll 是查清单的间隔。一次 tick 只做一次 stat，成本可忽略。
	modLivePoll = time.Second
	// modLiveHeartbeat 是"什么都没变也要重写"的间隔：让界面能凭 writtenAt
	// 分辨"进程还活着、状态是最新的"和"这文件是上一次启动留下的尸体"。
	modLiveHeartbeat = 15 * time.Second
	// modExeHashLimit 是给 exe 算指纹的上限。超了就跳过并写明原因，
	// 绝不为了一份状态文件让主流程等一次大哈希。
	modExeHashLimit = 64 << 20
)

// gatewayStartedAt 是本进程的开始时间（包级变量在 main 之前完成初始化）。
var gatewayStartedAt = time.Now()

// modSelfInfo 组装"正在跑的这份服务端程序"的身份。
//
// exe 指纹只在启动时算一次：它是给界面核对"跑着的是不是磁盘上那一份"用的，
// 而运行期进程自身的 exe 不会变（Windows 上还锁着写不了）。
func modSelfInfo() servermod.SelfInfo {
	self := servermod.SelfInfo{
		ServerVersion: versionString(),
		PID:           os.Getpid(),
		StartedAt:     gatewayStartedAt.UTC().Format(time.RFC3339),
	}
	exe, err := os.Executable()
	if err != nil {
		self.ExeHashSkipped = "取不到自身程序路径：" + err.Error()
		return self
	}
	self.ExePath = exe
	sum, skipped, size, modTime := servermod.ExeSHA256(exe, modExeHashLimit)
	self.ExeSHA256, self.ExeHashSkipped, self.ExeSizeBytes, self.ExeModifiedAt = sum, skipped, size, modTime
	return self
}

// startModLive 写出第一份实况文件并起轮询。返回的函数停止轮询（幂等，可 defer）。
//
// 调用时机固定在 Boot 与声明核对之后：那时钩子登记表才是完整的，
// 第一份实况文件才等于"这个进程真实装载了什么"。
func startModLive() func() {
	self := modSelfInfo()
	path := servermod.StatusPath(serverModuleDir())
	if err := servermod.WriteRuntimeStatus(path, self); err != nil {
		// 实况文件写不下去不影响服务端运行，但它必须是**看得见**的失败：
		// 界面读不到这个文件就会退回猜测，而"悄悄没有"正是这套东西要解决的问题。
		log.Printf("servermod: 写运行时实况失败（%s）：%v", path, err)
	}

	done := make(chan struct{})
	var stopOnce sync.Once
	ticker := time.NewTicker(modLivePoll)
	go func() {
		last := time.Now()
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				transitions, err := servermod.Reconcile(self.ServerVersion)
				if err != nil {
					log.Printf("servermod: 轮询启用清单失败：%v", err)
				}
				if len(transitions) == 0 && time.Since(last) < modLiveHeartbeat {
					continue
				}
				if err := servermod.WriteRuntimeStatus(path, self); err != nil {
					log.Printf("servermod: 刷新运行时实况失败（%s）：%v", path, err)
					continue
				}
				last = time.Now()
			}
		}
	}()
	log.Printf("servermod: 热开关轮询已启动（每 %s 查一次启用清单；实况文件 %s）", modLivePoll, path)
	return func() { stopOnce.Do(func() { close(done) }) }
}

// shutdownMods 在网关退出路径上请还在跑的 mod 收尾。
//
// 它必须排在 cleanup()（关存储/连接）之前跑，所以用 defer 的 LIFO 顺序：
// 注册在 `defer cleanup()` 之后 → 执行在它之前。收尾失败只记日志，
// 不该让一次正常退出变成报错退出。
func shutdownMods() {
	servermod.ShutdownLive(versionString())
}
