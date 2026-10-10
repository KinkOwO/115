# request-hook-pilot —— 请求侧钩子（`protocol.request`）样板

> **这是给"要把新玩法写成 mod"的人用的骨架**，不是一个玩法 mod。
> 它证明 `protocol.request` + `reply.send` 这条新通路真的能装、能验、能编译、能生效，
> 并且**对真实游戏零影响**（见下）。

## 它解决什么问题

在 `protocol.request` 出现之前，服务端层 mod 只有三种接入点：

| 钩子 | 时机 | 能力 |
| --- | --- | --- |
| `server.boot` | 启动一次 | 登记策略、自检、记日志 |
| `console.command` | 操作者敲一次 | 诊断命令 |
| `protocol.response` | 每条 S2C | **只读观察**，改不了报文 |

也就是说：**没有任何请求侧接入点** ⇒ "新增一个玩法"只能改服务端内核，写不成 mod。
`protocol.request` 补的正是这个口子：

```
客户端一帧报文
  ↓ 网关解密 + 判校验和
  ↓ 【protocol.request 在这里被问一次】←─ 正文与内置 handler 看到的完全一致
  ├─ 返回 false → 放行给内置分发表（一切照旧）
  └─ 返回 true  → 内置分发表全部跳过，由 mod 用 ctx.Reply 自己应答
```

## 为什么它对真实游戏零影响

它只认**带魔数标记**（`D0 0F BE EF`）且 `type=1 id=59999` 的帧，正常客户端永远不会发。
其余每一帧都走"看一眼、放行"，游戏行为与不装时一致。

要改成真玩法，把 `server/mod.go` 里 `onRequest` 的判据换成你自己的业务判据即可。

## 两条写作纪律

1. **只看不改**。`RequestContext` 刻意**不提供**"就地改写正文"的能力。
   改写要重算校验和并重加密，等于把协议改写搬进宿主 —— 协议真源会从
   "服务端代码路径 + IDA/实机取证"退化成"某个 mod 的猜测"。要改行为就**整条接手**。
2. **短路必须能应答**。返回 `handled=true` 之后内置分发不再说话，你不用 `ctx.Reply`
   发应答，客户端就卡在等待态。`ctx.Reply` 走的仍是服务端正规编码（校验和 + 加密），不是裸包。

## 装 / 验

```powershell
# 1) 打包（本 mod 是 server-only，包内没有需要算哈希的文件，所以 zip 只有两个条目）
$pilot = "<仓库根>\mods\examples\request-hook-pilot"
Compress-Archive -Path "$pilot\mod.json","$pilot\server" `
                 -DestinationPath "$pilot\dist\demo.request-hook-pilot-1.0.0.zip" -Force

# 2) 校验（只读）
modkit verify --mod "$pilot\dist\demo.request-hook-pilot-1.0.0.zip"

# 3) 预演（只读；告诉你每一步做什么、哪里会被阻断）
modkit plan --client <客户端根> --mod "$pilot\dist\demo.request-hook-pilot-1.0.0.zip" --root <启动器根>

# 4) 装（会写盘并重写 zz_mods_gen.go；装完**必须重新编译服务端**）
modkit install --client <客户端根> --mod "$pilot\dist\demo.request-hook-pilot-1.0.0.zip" --root <启动器根>
```

本机实测（2026-10-07，带 `protocol.request` 的 modkit）：

```
verify exit 0  清单 OK，层：server，权限：server.hook
plan   exit 0  3 执行 / 2 已就绪 / 0 阻断
               注意：本计划包含 server 层，落地后必须重新编译服务端
```

## 怎么确认它真的生效

| 证据 | 在哪看 |
| --- | --- |
| `Register()` 被调到 | 启动日志 `[mod demo.request-hook-pilot] 已登记 protocol.request 钩子…` |
| 钩子真的挂在分发链上 | 启动日志 `servermod: 已装载 …；request 钩子 1 个`（`internal/servermod` 的 `Description()`） |
| 声明与注册一致 | 启动日志无 `mod 声明与注册不一致`（服务端启动期核对，见 `internal/servermod/declarations.go`） |
| 确实接手了某一帧 | `[mod demo.request-hook-pilot] 接手 CMD59999（连接 …）` |
| 累计计数 | `DFO_SERVERMOD_CONSOLE="demo.request-hook-pilot status"` 后看日志 |

## 边界（别指望它做的事）

- **改不了内容**。内容唯一真源是 PVF（根 `AGENTS.md` §0.2），本模板不碰内容表。
- **不能热摘**。mod 是编译期注册的，装卸都要重新编译服务端 + 重启（会踢在线玩家）。
- **只带脚本的 mod 不需要重编译**：如果只要"发东西"，用 `server.scripts` 的 Lua 规则
  （见 `mods/newchar-kit/`），那条路不碰 Go 编译。
