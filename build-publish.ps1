# build-publish.ps1 - DFO 115us 离线自用包一键打包（相对路径，可反复运行）
# 用法: 一键打包.cmd                       使用默认发布分支打包（默认 = 离线自用包 offline）
#       一键打包.cmd -Branch main          指定分支（默认 main）
#       一键打包.cmd -OutDir D:\out        指定输出目录（默认仓库上一级）
#       一键打包.cmd -Kind dev             打开发者包（带源码 / GM 工具 / 分析资料；体积大）
#       一键打包.cmd -IncludeGmTools       离线包里额外带上 GM 工具（默认不带：GM 依赖 Python，普通玩家用不到）
#       一键打包.cmd -IncludeServerSource  离线包里额外带上服务端源码（默认不带）
#       一键打包.cmd -IncludeDevDocs       离线包里额外带上开发者文档（默认不带）
#       一键打包.cmd -ListPruned           只列出「会被裁掉的条目」和最终顶层分布，不写 zip（试跑/审计用）
#       一键打包.cmd -VerifyZip <zip 路径>  只校验一个已存在的包（不打新包；抽查/复验用，-Kind 决定按哪套规则校验）
# 流程: git 分支导出 -> 加入 tools/{pg}(排除 pgAdmin 4) ->
#       **复制预编译产物**(服务端程序 / 会话编排 CLI / WFP 探针 / configs) ->
#       launcher.local.json 模板(相对路径) ->
#       **按 -Kind 裁掉非白名单条目**(离线包只留必要项；见下方 $OfflineAllowed) ->
#       Python 标准 zip(正斜杠/UTF-8) -> 逐项校验(必需项在 + 排除项不在 + 条目全在白名单内)
#
# 【定位·2026-10-05 业主定调】**分发给玩家的是单个 exe**（DFO-115US单机一键启动器.exe）：
#   玩家双击它，启动器自动从发布源（主源失败自动回退备用源）取预编译服务端、引导选择 DFO.exe，
#   再点「开始游戏」。**zip 不再发给玩家**。
#   所以本脚本产出的 zip 是 **离线自用包（应急/离线场景）**：给自己备份、或网络差的场景用。
#   它只含跑起来必需的东西（**只含必要项**：不含源码、开发者文档、GM 工具、作者存档）。
#   2026-10-05 之前那一版 zip 有 3239 条目 / 92.9 MB，里面塞了 analysis/**、server/reference/**、
#   gm-tool/**、docs/todo/**、服务端 Go 源码(internal/**、cmd/**)、开发脚本……
#   现在收紧成「白名单优先 + 显式排除」两层：**不在白名单里的一律不进离线包**，
#   打完还要自证「排除项一条都没混进来」（见 Assert-PublishZip 的 (3)(4)(5) 段）。
#   需要带源码/GM工具的开发者包用 `-Kind dev`（同一条流程，只是不裁白名单）。
[CmdletBinding()]
param(
    [string]$Branch = '',
    [string]$OutDir = '',
    [string]$VerifyZip = '',
    [ValidateSet('offline', 'dev')][string]$Kind = 'offline',
    [switch]$IncludeGmTools,
    [switch]$IncludeServerSource,
    [switch]$IncludeDevDocs,
    [switch]$ListPruned
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $Branch) { $Branch = 'main' }
if (-not $OutDir) { $OutDir = Split-Path -Parent $ROOT }

# ==== 预编译产物清单（2026-10-05 业主硬性要求：运行环境绝不编译，包必须自带这些文件）====
#
# 为什么必须在这里复制：`server/work/dfo-lan/bin/`（见该目录的 .gitignore）与 `*.exe`
# （见仓库根 .gitignore）都不入库，`git archive` 导出的树**不带**它们 —— 拿到包一点
# 「开始游戏」就会报"缺少服务端 Go 编排 CLI / 服务端程序"。启动器虽然能按发布仓库的
# manifest 自动补齐（见 internal/prebuilt），但**包本该自带**，别让它为了开玩先下 22 MB。
$PrebuiltFiles = @(
    # 启动器 exe（2026-10-05 追加）：包内的它必须与**当前工作区刚构建的那份**一致 ——
    # git 分支里那份是历史提交（45.9 MB，没有本轮的内置资源释放与受管资源校验），
    # 只从 git 导出会让"整包分发"的玩家拿到旧逻辑，正是这次要修的问题之一。
    'DFO-115US单机一键启动器.exe',
    'server\work\dfo-lan\bin\dfolauncher.exe',   # Go 会话编排 CLI（launch / stop / prepare-inner-pvf / init-storage…）
    'server\work\dfo-lan\bin\wireprobe-pvf.exe', # 服务端程序（PVF 直读默认档 configs\pvf-default.json 指向它）
    'server\work\dfo_probe_tools\probe.exe'      # 客户端宿主回退路径（WFP 回环隔离后拉起客户端）
)
# 备份与半截文件绝不许进包：*.previous-* 是发布 PVF 默认程序时留下的旧版备份（36 MB 一份），
# *.exe~ 是编辑器/收尾工具留下的半成品 —— 它们白占体积，还可能被误当成可用程序。
$PrebuiltForbidden = @('*.previous-*', '*~')
# 服务端运行必需的配置目录（含不入库的大文件，如 configs\equipment-full.data）。
$ConfigsRel = 'server\work\dfo-lan\configs'

# ==== 离线自用包白名单 ===================================================================
#
# 形状：**白名单优先 + 显式排除**两层。
#   · 白名单 = 允许进包的路径（字面量，或带 `*`/`**` 的通配，正斜杠书写、大小写不敏感）。
#     **不在白名单里的一律从包树里删掉** —— 所以白名单必须写全、别指望"默认带上"。
#   · 显式排除 = 白名单内部仍要挖掉的隐私/体积/半成品条目（$ForbiddenRules），单独再验一遍。
#
# 为什么不再用"整棵目录放行"：上一版把 `server/work/**` 整棵放行，于是 1552 个 internal/**
# 服务端源码、155 份开发文档、2107 个 Go/开发文件全进了包；同理 analysis/**、gm-tool/**、
# server/reference/** 也是整棵带进去的。那正是这次要修的。
#
# `**` 必须显式写出来（例如 `scripts/**`）；`scripts/*.cmd` 只匹配一层。
# 匹配用的 Match-Rule 是"按 `/` 分段"的，所以 `dfo-lan/*/testdata/**` 里的 `*` 只吃一段路径。
$OfflineAllowed = @(
    # —— 包根：启动器本体 + 说明 + 许可 ——
    # 启动器 exe（45 MB）：分发给玩家的正式形态就是这个 exe（它自己会去发布源取服务端）；
    # 离线场景下把它放在包根，解压即可直接双击，所以留着最有价值。
    'DFO-115US单机一键启动器.exe',
    'LICENSE',
    'CHANGELOG',                 # 业主的变更记录（纯文本，34 KB）；docs/ 下已无 CHANGELOG.md，这里就是它的位置
    'launcher.settings.json',    # 启动器设置（更新分支/窗口尺寸）；缺失时启动器会自建，但包内带上更省一次自举
    '资源清单.json',              # 包内受管文件的 size+sha256 清单（路 1 自校验的依据，见 build-publish.ps1 第 5.5 步）
    'scripts/README.md',         # 四个入口的用法与踩坑说明（排错最需要的一份文本）

    # —— 启动入口（缺一个就少一条路）——
    # 中文文件名照旧（Explorer / PowerShell 按 UTF-16 处理没问题；内容才是纯 ASCII，见 scripts/README.md）。
    'scripts/启动游戏.cmd',            # 默认档入口（走 storage-route.ps1 game-current）
    'scripts/启动游戏-SQLite.cmd',     # SQLite 档（不需要 PostgreSQL）
    'scripts/启动游戏-PostgreSQL.cmd', # PostgreSQL 档（自动起库）
    'scripts/启动服务端.cmd',          # 只起服务端（默认档）
    'scripts/启动服务端-SQLite.cmd',
    'scripts/启动服务端-PostgreSQL.cmd',
    'scripts/启动游戏-奥德赛.cmd',     # 整档强制奥德赛模式；小且是可选项，留着
    'scripts/停止游戏环境.cmd',
    'scripts/storage-route.ps1',       # 双库双路线切换 + 启动链调用（入口内部调用）
    'scripts/storage-route.cmd',       # 同一件事的手动入口

    # —— 客户端路径模板（本机配置的样板，绝不放作者自己的 launcher.local.json）——
    'server/launcher.example.json',

    # —— 服务端运行必需：预编译程序 + 运行配置 + 网关 fixture 样本 ——
    'server/work/dfo-lan/bin/dfolauncher.exe',
    'server/work/dfo-lan/bin/wireprobe-pvf.exe',
    'server/work/dfo-lan/configs/**',
    # 网关 argv 里明确读取（internal/launcher/gateway.go：-select-probe-config
    # …\cmd\wireprobe\testdata\select-parser-probe.json、-town-entry-probe …town-entry-probe.json、
    # select-world-probe.json），**删了服务端起不来**。testdata 里还有网关 fixture 的回退样本
    # （login-normal22.bin 等），一并留着 —— 整棵 testdata 才 300 KB 出头，不值得为它冒险。
    'server/work/dfo-lan/cmd/wireprobe/testdata/**',
    # 网关 fixture 的 .bin 样本（internal/launcher/fixture.go：login/precheck/characters/name_ok.bin），
    # 共 168 字节。它们与 testdata 同源，缺了会让 fixture 走别的分支，留着最省心。
    'server/work/dfo-lan/runtime/*.bin',
    'server/work/dfo_probe_tools/probe.exe',

    # —— 字体补丁（小、面向客户端；纯 Python 脚本，不依赖仓库根的 tools\，删掉 tools\ 后照跑）——
    'client-patchs/**'
)

# 离线包里的"可选加料"：默认不带，只有显式开关才加回白名单（见参数 -IncludeGmTools / -IncludeDevDocs）。
#   · GM 工具：依赖 Python（Python 已移出 tools\，只存在于包外 ..\gm-tool\python），普通玩家用不到；
#     而且 gm-tool 里还混着 dashboard/backups/_pre_refactor_backup 等开发残留。默认排除。
#   · 开发者文档：docs/todo/**、server/reference/**、server/AGENTS.md、开发对接文档……离线自用不需要，
#     也不该把内部 TODO 跟着包走。默认排除。
$OfflineOptIn = @{
    IncludeGmTools      = @('gm-tool/**', 'scripts/GM.cmd')
    IncludeDevDocs      = @('docs/**', 'server/*.md', 'AGENTS.md')
    IncludeServerSource = @('server/work/dfo-lan/go.mod', 'server/work/dfo-lan/go.sum', 'server/work/dfo-lan/sqlc.yaml',
        'server/work/dfo-lan/internal/**', 'server/work/dfo-lan/cmd/**', 'server/work/dfo-lan/scripts/**',
        'server/work/dfo-lan/docs/**')
}

# ==== 显式排除清单（每一条都写清"为什么"）================================================
#
# 第 1 档 kinds=all：任何包（离线自用包与开发者包）都不许出现的东西 —— 隐私、体积巨物、半成品。
# 第 2 档 kinds=offline：离线包额外要挖掉的开发物（即使白名单误放行也会被这里兜住）。
# 校验时两档都会逐条验"包里 0 条"；离线包的白名单裁剪也会先删掉命中这些规则的条目。
#
# `unless` = 这条规则里的例外（例如"排除 cmd\ 源码、但留下 testdata 里的 fixture"）。
$ForbiddenRules = @(
    # --- all：绝不该进任何发布包 ---
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/dfolan.sqlite3*'
       why = '作者存档（SQLite 存档本体，含作者的角色数据）' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/local.json'
       why = '作者的存储档位选择（应在本机由启动器按默认档位生成）' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/local.*.json'; unless = '**/local.example.json'
       why = '作者的两条路线档（local.sqlite.json / local.postgres.json）；仓库里的样例 local.example.json 不算' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/backups/**'
       why = '作者存档备份' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/pgdata/**'
       why = '作者的 PostgreSQL 数据目录（体积大且是作者存档）' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/storage/*.log'
       why = '运行日志（作者机器上的运行痕迹）' }
    @{ kinds = @('all'); pattern = 'server/work/dfo-lan/runtime/roles_*'
       why = '作者会话的调试追踪记录' }
    @{ kinds = @('all'); pattern = 'server/work/client-build/**'
       why = '内层 PVF（Script.inner.pvf 726 MB），本机侧由 CLI 用自己的 Script.pvf 就地生成' }
    @{ kinds = @('all'); pattern = 'server/runtime/pvf-cache/**'
       why = 'PVF 目录缓存（可重建的派生物）' }
    @{ kinds = @('all'); pattern = 'server/runtime/update-cache/**'
       why = '增量更新缓存（可重建的派生物）' }
    @{ kinds = @('all'); pattern = '.tmp/**'
       why = '打包/调试临时目录' }
    @{ kinds = @('all'); pattern = 'tools/**'
       why = '便携运行环境（PG 等）；启动链只走仓库内 Go 启动器，不需要它（业主也明确要求 tools\ 保留在仓库里、不入包）' }
    @{ kinds = @('all'); pattern = '**/*.previous-*'
       why = '旧版程序备份（*.previous-* 一份 36 MB），白占体积还可能被误当成可用程序' }
    @{ kinds = @('all'); pattern = '**/*~'
       why = '编辑器/收尾工具留下的半成品（*.exe~ 等）' }
    @{ kinds = @('all'); pattern = '**/*.local.json'; unless = '**/local.example.json'
       why = '机器本地配置（含本机口令与绝对路径）；模板 local.example.json 不算' }
    @{ kinds = @('all'); pattern = '**/*.log'
       why = '运行日志' }
    @{ kinds = @('all'); pattern = 'pvf_cache/**'
       why = 'PVF 解包清单（派生物，运行期不需要）' }

    # --- offline：只在离线包里要挖掉的开发物 ---
    @{ kinds = @('offline'); pattern = 'analysis/**'
       why = '开发分析资料（dumps/tasks/tools 等 785 个条目）' }
    @{ kinds = @('offline'); pattern = 'docs/**'
       why = '开发者文档（todo/pvf 计划、协议笔记、归档 CHANGELOG 等）；说明见根 CHANGELOG 与 scripts/README.md' }
    @{ kinds = @('offline'); pattern = 'server/reference/**'
       why = '开发参考（160 条历史分析脚本/原始启动器），不参与运行' }
    @{ kinds = @('offline'); pattern = 'gm-tool/**'
       why = 'GM 工具（依赖 Python，普通玩家用不到）；需要时用 -IncludeGmTools 或直接打 dev 包' }
    @{ kinds = @('offline'); pattern = 'scripts/GM.cmd'
       why = 'GM 工具入口（同上；旧版依赖的 scripts/gm.py 已删，本来也不可用）' }
    @{ kinds = @('offline'); pattern = 'scripts/gm.py'
       why = 'GM 工具的 Python 实现（同上）' }
    @{ kinds = @('offline'); pattern = 'scripts/incremental-package/**'
       why = '增量打包（另一条并行工作的开发者工具）' }
    @{ kinds = @('offline'); pattern = 'scripts/local-fixes/**'
       why = '仓库级一次性修复/移植脚本（开发者工具）' }
    @{ kinds = @('offline'); pattern = 'scripts/build_publish_zip.py'
       why = '打包器自身（包内不需要打包）' }
    @{ kinds = @('offline'); pattern = 'scripts/check-commit-hygiene.ps1'
       why = '提交门禁（开发者工具）' }
    @{ kinds = @('offline'); pattern = 'scripts/一键打包.cmd'
       why = '打包入口（开发者工具）' }
    @{ kinds = @('offline'); pattern = 'scripts/配置环境.cmd'
       why = '旧的 Python 环境配置入口（去 Python 后已无实现）' }
    @{ kinds = @('offline'); pattern = 'scripts/检查环境.cmd'
       why = '开发者自检脚本（走 dev 侧的检查链）' }
    @{ kinds = @('offline'); pattern = 'scripts/移除tools.cmd'
       why = '仓库维护脚本（把 tools/ 挪出仓库）' }
    @{ kinds = @('offline'); pattern = 'scripts/还原tools.cmd'
       why = '仓库维护脚本（同上）' }
    @{ kinds = @('offline'); pattern = 'scripts/打包增量更新.cmd'
       why = '增量打包入口（另一条并行工作）' }
    @{ kinds = @('offline'); pattern = 'scripts/伊斯-*.cmd'
       why = '业主本地的时间线/基线测试入口（伊斯坦线）' }
    @{ kinds = @('offline'); pattern = 'build-publish.ps1'
       why = '打包脚本自身' }
    @{ kinds = @('offline'); pattern = '.gitignore'
       why = '仓库级忽略规则' }
    @{ kinds = @('offline'); pattern = '.gitlab-ci.yml'
       why = 'CI 配置' }
    @{ kinds = @('offline'); pattern = 'AGENTS.md'
       why = 'Agent/开发约定（内部文档）' }
    @{ kinds = @('offline'); pattern = 'server/AGENTS.md'
       why = '同上（服务端侧）' }
    @{ kinds = @('offline'); pattern = 'server/MANIFEST.sha256'
       why = '构建清单（开发侧校验用）' }
    @{ kinds = @('offline'); pattern = 'server/package-manifest.json'
       why = '打包清单（开发侧校验用）' }
    @{ kinds = @('offline'); pattern = 'server/Build-*.ps1'
       why = '服务端构建脚本（运行环境绝不编译）' }
    @{ kinds = @('offline'); pattern = 'server/开发对接文档.md'
       why = '开发者对接文档' }
    @{ kinds = @('offline'); pattern = 'server/修复记录-*.md'
       why = '开发修复记录' }
    @{ kinds = @('offline'); pattern = 'server/退出按钮修复记录-*.md'
       why = '开发修复记录' }
    @{ kinds = @('offline'); pattern = 'server/verification/**'
       why = '打包/启动验证产物（开发证据）' }
    @{ kinds = @('offline'); pattern = 'server/work/*.py'
       why = '仓库级开发小工具（zip_inspect / make_patch_diff 等）' }
    @{ kinds = @('offline'); pattern = 'server/work/patch_diff.json'
       why = '补丁差异数据（开发物）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/go.mod'
       why = 'Go 模块定义（本机侧不编译服务端）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/go.sum'
       why = 'Go 依赖锁定（同上）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/sqlc.yaml'
       why = 'sqlc 代码生成配置（开发物）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/.gitignore'
       why = '仓库级忽略规则' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/meister_laboratory_notes.md'
       why = '开发笔记' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/internal/**'
       why = '服务端 Go 源码（1552 个条目；运行用 bin\wireprobe-pvf.exe，不编译）' }
    # cmd\ 下逐个命令目录点名（**故意不写 `cmd/**`**：那样会把白名单里的
    # cmd/wireprobe/testdata/** 也判成违禁，校验反而误报）。
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/admin/**'
       why = '服务端 Go 命令源码（不编译）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/dfo-tool/**'
       why = '服务端 Go 命令源码（同上）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/dfolauncher/**'
       why = '服务端 Go 命令源码（同上；包内已带编译好的 bin\dfolauncher.exe）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/gmtool/**'
       why = 'GM 工具的 Go 实现' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/wireprobe/**'; unless = 'server/work/dfo-lan/cmd/wireprobe/testdata/**'
       why = '网关 Go 源码（包内已带编译好的 bin\wireprobe-pvf.exe）；testdata 里是网关 argv 硬引用的 fixture，必须留' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/README.md'
       why = '服务端命令目录的开发说明' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/cmd/*.txt'
       why = '调试输出残留（_dump781_out.txt / _tmp_official_pre.txt）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/scripts/**'
       why = '服务端开发脚本（PVF 导出/审计等，16 个条目）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo-lan/docs/**'
       why = '服务端开发文档（155 份）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo_probe_tools/Build-Probe.cmd'
       why = '探针构建脚本（运行环境绝不编译）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo_probe_tools/probe.cpp'
       why = '探针源码（同上）' }
    @{ kinds = @('offline'); pattern = 'server/work/dfo_probe_tools/*.py'
       why = '探针配套开发脚本' }
)

# 包里要预建的空目录（空目录不进 zip，这里只是把"该有的位置"准备好）。
#   · server/work/client-build  —— 内层 PVF 的落地目录，首次启动由 CLI 就地生成；
#   · server/work/dfo-lan/runtime/storage —— 存档目录，启动器会自建。
$OfflineScaffoldDirs = @('server\work\client-build', 'server\work\dfo-lan\runtime\storage')

# Resolve-AllowedRules 把「白名单 + 可选加料 + 排除清单」解析成最终允许/禁止规则集。
# 返回值同时带 Allow（条目必须落在其中之一）与 Forbid（一条都不许出现）。
function Resolve-AllowedRules {
    param([string]$Kind, [switch]$WithGmTools, [switch]$WithServerSource, [switch]$WithDevDocs)

    $allow = @($OfflineAllowed)
    if ($WithGmTools) { $allow += $OfflineOptIn.IncludeGmTools }
    if ($WithServerSource) { $allow += $OfflineOptIn.IncludeServerSource }
    if ($WithDevDocs) { $allow += $OfflineOptIn.IncludeDevDocs }

    $forbid = @()
    foreach ($rule in $ForbiddenRules) {
        if ($rule.kinds -contains 'all' -or $rule.kinds -contains $Kind) { $forbid += $rule }
    }
    return [pscustomobject]@{ Kind = $Kind; Allow = $allow; Forbid = $forbid }
}

# ==== 白名单匹配（通配 -> 正则）===========================================================
# 规则用正斜杠书写；树里的相对路径先把 `\` 换成 `/` 再比。规则里 `*` 只吃一段（不含 `/`），
# `**` 吃任意多段。规则里除了 `*` 一律按字面处理（`[`、`.`、`+` 都转义），所以
# `server/work/dfo_probe_tools/*.py` 不会因为正则元字符而误匹配。
function Convert-RulePattern {
    param([Parameter(Mandatory = $true)][string]$Pattern)
    $rx = ''
    for ($i = 0; $i -lt $Pattern.Length; $i++) {
        $ch = $Pattern[$i]
        if ($ch -eq '*') {
            if (($i + 1) -lt $Pattern.Length -and $Pattern[$i + 1] -eq '*') {
                $rx += '.*'
                $i++
            }
            else { $rx += '[^/]*' }
        }
        else { $rx += [regex]::Escape([string]$ch) }
    }
    return ('^' + $rx + '$')
}

# Match-Rule：相对路径（正斜杠）是否命中规则里的任意一条。
# 注意 `[object[]]` 而不是 `[string[]]`：规则可以是字符串，也可以是 @{pattern=…;unless=…} 的哈希表；
# 声明成 [string[]] 会让 PowerShell **把哈希表悄悄转成字符串**（"System.Collections.Hashtable"），
# 结果一条都匹配不上却看不出错（2026-10-05 实机踩到：排除清单静默失效）。
function Match-Rule {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Rel,
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][object[]]$Rule
    )
    $p = $Rel -replace '\\', '/'
    while ($p.StartsWith('./')) { $p = $p.Substring(2) }
    foreach ($item in @($Rule)) {
        $pat = if ($item -is [string]) { $item } else { [string]$item.pattern }
        if (-not $pat) { continue }
        if ([regex]::IsMatch($p, (Convert-RulePattern $pat), [Text.RegularExpressions.RegexOptions]::IgnoreCase)) { return $true }
    }
    return $false
}

# Get-EntryRuleHits：返回命中的规则列表（用于把"为什么被排除"打进日志）。
# 规则里带 `unless` 时，路径命中 unless 就不算命中这条规则（用于"排除某目录、但留下它的一个子目录"）。
# 同样必须用 [object[]]（见 Match-Rule 的说明）。
function Get-EntryRuleHits {
    param([string]$Rel, [AllowEmptyCollection()][object[]]$Rule)
    $p = $Rel -replace '\\', '/'
    $hits = @()
    foreach ($item in @($Rule)) {
        if ($item -is [string]) {
            if ([regex]::IsMatch($p, (Convert-RulePattern $item), [Text.RegularExpressions.RegexOptions]::IgnoreCase)) { $hits += $item }
            continue
        }
        $pat = [string]$item.pattern
        if (-not $pat) { continue }
        $unless = [string]$item.unless
        if ($unless -and [regex]::IsMatch($p, (Convert-RulePattern $unless), [Text.RegularExpressions.RegexOptions]::IgnoreCase)) { continue }
        if ([regex]::IsMatch($p, (Convert-RulePattern $pat), [Text.RegularExpressions.RegexOptions]::IgnoreCase)) { $hits += $pat }
    }
    return $hits
}

# Get-RulePatterns：把规则数组里每条规则的 pattern 取出来（规则可以是字符串或 @{pattern=..;unless=..}）。
function Get-RulePatterns {
    param([AllowEmptyCollection()][object[]]$Rule)
    $out = @()
    foreach ($item in @($Rule)) {
        if ($item -is [string]) { $out += $item } else { $out += [string]$item.pattern }
    }
    return $out
}

# Keep-OnlyAllowed：把一棵树裁成"只留白名单条目"。自底向上遍历，先删文件再收空目录。
# 目录的留法：只有当"某个白名单规则的前缀就是它"时才留（否则白名单只会放行文件，
# 中间目录先被删掉、后面的文件就没地方待了）。留下的空目录由 Remove-EmptyDirs 收掉。
function Keep-OnlyAllowed {
    param(
        [Parameter(Mandatory = $true)][string]$Tree,
        [Parameter(Mandatory = $true)]$Rules,
        [switch]$Report
    )
    $allow = @($Rules.Allow)
    $all = @(Get-ChildItem -LiteralPath $Tree -Recurse -Force | Sort-Object { $_.FullName.Length } -Descending)
    $summary = @{}
    foreach ($item in $all) {
        $rel = ($item.FullName.Substring($Tree.Length).TrimStart('\', '/')).Replace('\', '/')
        $isDir = $item.PSIsContainer
        $keep = $false
        if (-not $isDir) {
            # 文件：必须在白名单内，且不得命中排除清单（排除优先）。
            if ((Match-Rule -Rel $rel -Rule $allow) -and
                (@(Get-EntryRuleHits -Rel $rel -Rule @($Rules.Forbid)).Count -eq 0)) { $keep = $true }
        }
        else {
            # 目录：是某条白名单规则的前缀就留（`**` 规则天然能覆盖一切，见 dev 形态的 @('**')）。
            foreach ($rule in $allow) {
                $ruleText = [string]$rule
                if ($ruleText.StartsWith($rel + '/')) { $keep = $true; break }
                $prefix = $ruleText
                $star = $prefix.IndexOf('*')
                if ($star -ge 0) { $prefix = $prefix.Substring(0, $star).TrimEnd('/') }
                if ($prefix -and ($rel -eq $prefix -or $rel.StartsWith($prefix + '/'))) { $keep = $true; break }
            }
        }
        if ($keep) { continue }
        if (-not $isDir) {
            $top = ($rel -split '/')[0]
            if (-not $summary.ContainsKey($top)) { $summary[$top] = 0 }
            $summary[$top]++
        }
        if (-not $Report) { Remove-Item -LiteralPath $item.FullName -Force -ErrorAction SilentlyContinue }
    }
    return $summary
}

# Remove-EmptyDirs：收掉裁剪后留下的空目录（真正有内容的目录不动）。
function Remove-EmptyDirs {
    param([Parameter(Mandatory = $true)][string]$Tree)
    $all = @(Get-ChildItem -LiteralPath $Tree -Recurse -Force -Directory | Sort-Object { $_.FullName.Length } -Descending)
    foreach ($d in $all) {
        if (-not (Get-ChildItem -LiteralPath $d.FullName -Force -ErrorAction SilentlyContinue)) {
            Remove-Item -LiteralPath $d.FullName -Force -ErrorAction SilentlyContinue
        }
    }
}

# Assert-PublishZip 逐项校验产物。
#   (1) 必需项必须在（启动入口 / 配置样板 / 预编译产物 / configs 全量比对）；
#   (2) bin\ 下的备份与半截文件（*.previous-*、*~）必须不在；
#   (3) 排除清单必须 0 条（隐私、作者存档、开发物……一条都不许混进来）；
#   (4) 离线包：**每个条目都必须落在白名单内**（白名单是"允许集"，不在里面的都算混入）；
#   (5) 证据：关键条目连大小、configs 条数、顶层目录分布一起列出来。
# 缺任何一个、混进任何一个就 throw 并说清是什么 —— 绝不放行一个"跑不起来/不干净"的包。
function Assert-PublishZip {
    param(
        [Parameter(Mandatory = $true)][string]$ZipPath,
        [string]$Kind = 'offline',
        [switch]$WithGmTools, [switch]$WithServerSource, [switch]$WithDevDocs
    )

    if (-not (Test-Path $ZipPath -PathType Leaf)) { throw "离线自用包不存在: $ZipPath" }
    $rules = Resolve-AllowedRules -Kind $Kind -WithGmTools:$WithGmTools -WithServerSource:$WithServerSource -WithDevDocs:$WithDevDocs
    $z = [IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $entries = @($z.Entries)
        $names = New-Object 'System.Collections.Generic.HashSet[string]'
        foreach ($e in $entries) { [void]$names.Add($e.FullName) }

        # (1) 关键条目：启动入口 + 配置样板 + 预编译产物 + 关键配置
        $need = @(
            'scripts/启动游戏.cmd',
            'scripts/启动游戏-SQLite.cmd',
            'scripts/启动游戏-PostgreSQL.cmd',
            'scripts/启动服务端.cmd',
            'scripts/停止游戏环境.cmd',
            'scripts/storage-route.ps1',
            'server/launcher.example.json',
            'server/work/dfo-lan/configs/pvf-default.json',
            # 网关 argv 硬引用的三个 fixture（internal/launcher/gateway.go）
            'server/work/dfo-lan/cmd/wireprobe/testdata/select-parser-probe.json',
            'server/work/dfo-lan/cmd/wireprobe/testdata/select-world-probe.json',
            'server/work/dfo-lan/cmd/wireprobe/testdata/town-entry-probe.json',
            'server/work/dfo_probe_tools/probe.exe'
        )
        foreach ($rel in $PrebuiltFiles) { $need += ($rel -replace '\\', '/') }
        # 分发给玩家的正式形态就是这个启动器 exe；包里也带上它（解压即可双击），所以列为必需项。
        $need += 'DFO-115US单机一键启动器.exe'
        if (Test-Path (Join-Path $ROOT 'tools\pg')) { $need += 'tools/pg/pgsql/bin/initdb.exe' }

        $miss = @()
        foreach ($n in $need) {
            if (-not $names.Contains($n)) { $miss += $n }
        }

        # (2) configs 全量比对：源目录里（除机器本地配置）有一个算一个，都必须出现在包里。
        #     只验几个代表文件是不够的 —— 少一个 configs 就可能让某个功能哑掉。
        $cfgSrc = Join-Path $ROOT $ConfigsRel
        if (Test-Path $cfgSrc) {
            Get-ChildItem $cfgSrc -Recurse -File | ForEach-Object {
                if ($_.Name -like '*.local.json') { return }   # 机器本地配置（含口令）不进包
                $rel = $_.FullName.Substring($cfgSrc.Length).TrimStart('\').Replace('\', '/')
                $arc = 'server/work/dfo-lan/configs/' + $rel
                if (-not $names.Contains($arc)) { $miss += $arc }
            }
        }

        # (3) bin\ 下的备份与半截文件
        $bad = @()
        foreach ($e in $entries) {
            $n = $e.FullName
            if ($n -notlike 'server/work/dfo-lan/bin/*') { continue }
            foreach ($pat in $PrebuiltForbidden) {
                if ($n -like $pat) { $bad += $n; break }
            }
        }

        # (4) 排除清单：一条都不许出现（隐私/作者存档/体积巨物/开发物）
        #     必须把整条规则（含 unless 例外）传进去：只传 pattern 会把
        #     cmd/wireprobe/testdata/** 这类"排除目录里的例外"也判成违禁，反而误报。
        $leak = @()
        $forbidRules = @($rules.Forbid)
        $forbidPatterns = @(Get-RulePatterns -Rule $forbidRules)
        foreach ($e in $entries) {
            $hits = @(Get-EntryRuleHits -Rel $e.FullName -Rule $forbidRules)
            if ($hits.Count -gt 0) {
                $why = ($forbidRules | Where-Object { $hits -contains $_.pattern } | Select-Object -First 1).why
                $leak += ("{0}  <- [{1}] {2}" -f $e.FullName, ($hits -join ' | '), $why)
            }
        }

        # (5) 离线包：条目必须全在白名单内（这是"不夹带"的正面证据）
        $extra = @()
        if ($Kind -eq 'offline') {
            foreach ($e in $entries) {
                if (-not (Match-Rule -Rel $e.FullName -Rule $rules.Allow)) { $extra += $e.FullName }
            }
        }
        else {
            # dev 包只验"排除清单没混进来"（源码/GM工具/文档本就该在），外加源码与 GM 工具必须在，
            # 否则"开发者包"名不副实。
            foreach ($n in @('server/work/dfo-lan/go.mod', 'gm-tool/README.md')) {
                if (-not $names.Contains($n)) { $miss += $n }
            }
        }

        # 报错顺序刻意如此：先"缺什么/混进什么违禁的"，最后才是"白名单之外的其余条目"。
        # 排除清单（隐私/作者存档/源码）比白名单兜底更具体，先报它能直接说出"为什么不能带"。
        if ($miss.Count -gt 0) {
            throw ("离线自用包缺少关键文件: " + ($miss -join ', ') +
                "`n  这些正是'一点开始游戏就报错'的根因。先在工作区补齐（预编译产物见发布仓库的" +
                " server-bin 预编译包），再重新打包。")
        }
        if ($bad.Count -gt 0) {
            throw ("包里混进了备份/半截文件: " + ($bad -join ', ') +
                "`n  （*.previous-* 是旧程序备份、*.exe~ 是半成品）它们必须排除。")
        }
        if ($leak.Count -gt 0) {
            throw ("包里混进了排除清单条目（{0} 条）:`n  - {1}" -f $leak.Count, ($leak -join "`n  - "))
        }
        if ($extra.Count -gt 0) {
            throw ("离线包里混进了白名单之外的条目（{0} 条，共 {1} 个条目）:`n  - {2}" -f
                $extra.Count, $entries.Count, (($extra | Select-Object -First 40) -join "`n  - "))
        }

        # (6) 证据：把关键条目连大小一起列出来
        Write-Host ("   校验通过: 共 {0} 个条目" -f $entries.Count)
        foreach ($n in $need) {
            $hit = $z.Entries | Where-Object { $_.FullName -eq $n } | Select-Object -First 1
            Write-Host ("     OK  {0}  ({1} bytes)" -f $n, $hit.Length)
        }
        $cfgShipped = @($entries | Where-Object { $_.FullName -like 'server/work/dfo-lan/configs/*' })
        Write-Host ("     OK  server/work/dfo-lan/configs/**  （{0} 个条目）" -f $cfgShipped.Count)
        $testdataShipped = @($entries | Where-Object { $_.FullName -like 'server/work/dfo-lan/cmd/wireprobe/testdata/*' })
        Write-Host ("     OK  server/work/dfo-lan/cmd/wireprobe/testdata/**  （{0} 个条目）" -f $testdataShipped.Count)
        Write-Host ("     OK  排除清单 {0} 条规则 0 命中" -f $forbidPatterns.Count)
        if ($Kind -eq 'offline') {
            Write-Host ("     OK  全部 {0} 个条目都落在白名单内（不夹带）" -f $entries.Count)
        }

        # 顶层目录分布（前后对比用；按条目数降序）
        Write-Host '   包内顶层分布:'
        $entries |
            ForEach-Object { $p = $_.FullName.Split('/'); if ($p.Count -gt 1) { $p[0] + '/' } else { '<根文件>' } } |
            Group-Object | Sort-Object Count -Descending | ForEach-Object {
                Write-Host ("     {0,6}  {1}" -f $_.Count, $_.Name)
            }
    } finally { $z.Dispose() }
}

# -VerifyZip：只校验一个已存在的包（不打新包）。抽查、或事后复验某个已下发的包时用。
if ($VerifyZip) {
    Write-Host "== 校验离线自用包: $VerifyZip （Kind=$Kind）=="
    Assert-PublishZip -ZipPath $VerifyZip -Kind $Kind -WithGmTools:$IncludeGmTools `
        -WithServerSource:$IncludeServerSource -WithDevDocs:$IncludeDevDocs
    Write-Host '[成功] 校验通过'
    return
}

# -ListPruned：只导出 git 树 + 复制预编译产物 + 试算裁剪结果，然后列清单（不写 zip）。
# 用来在改白名单时"先看会删掉什么"，避免瞎删。临时目录照旧清理干净。
$rules = Resolve-AllowedRules -Kind $Kind -WithGmTools:$IncludeGmTools `
    -WithServerSource:$IncludeServerSource -WithDevDocs:$IncludeDevDocs

git -C $ROOT rev-parse --verify --quiet $Branch 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) { throw "分支不存在: $Branch（本地没有这个分支；用 -Branch main 或先建发布分支）" }

$stamp = Get-Date -Format 'yyyyMMdd'
# 产物名带定位：**分发给玩家的是单个 exe，这个 zip 只是离线自用/应急包**，
# 名字里写清楚，免得以后有人误把它当成玩家分发物。
$zipName = if ($Kind -eq 'dev') { "DFO-115US-开发发布包-$stamp.zip" } else { "DFO-115US-发布包-$stamp.zip" }
$zipOut = Join-Path $OutDir $zipName
$work = Join-Path $env:TEMP "df-publish-$stamp-$Kind"
$tar  = Join-Path $env:TEMP "df-publish-$stamp-$Kind.tar.zip"

Write-Host "== 离线自用包打包: 分支 [$Branch]  形态 [$Kind] =="
Write-Host ("   输出: {0}" -f $zipOut)
if ($Kind -ne 'dev') {
    Write-Host '   定位: 分发给玩家的是单个 exe（DFO-115US单机一键启动器.exe）；本包仅作离线自备/应急，不外发。'
}

if (Test-Path $work) { Remove-Item $work -Recurse -Force }
New-Item -ItemType Directory -Force -Path $work | Out-Null
try {
    # 1. 从 git 分支导出发布内容（不依赖工作区状态）
    Write-Host '[1/6] git 分支导出...'
    git -C $ROOT archive --format=zip -o $tar $Branch
    if ($LASTEXITCODE -ne 0) { throw 'git archive 失败' }
    [IO.Compression.ZipFile]::ExtractToDirectory($tar, $work)

    # 2. 加入便携运行环境（pg，按需），排除 pgAdmin 4
    Write-Host '[2/6] 复制便携运行环境...'
    $tools = Join-Path $ROOT 'tools'
    New-Item -ItemType Directory -Force -Path (Join-Path $work 'tools') | Out-Null
    foreach ($d in @('pg')) {
        $src = Join-Path $tools $d
        if (-not (Test-Path $src)) { Write-Warning "缺少运行环境目录 tools\$d (跳过)"; continue }
        if ($Kind -ne 'dev') { Write-Host ("   - 离线包不带 tools\{0}（启动链只走仓库内 Go 启动器；见排除清单）" -f $d); continue }
        Copy-Item $src (Join-Path $work "tools\$d") -Recurse -Force
    }
    $pga = Join-Path $work 'tools\pg\pgsql\pgAdmin 4'
    if (Test-Path $pga) { Remove-Item $pga -Recurse -Force; Write-Host '   已排除 pgAdmin 4' }

    # 3. 复制预编译产物 + configs（git 里没有这些文件，必须从工作区带进包）
    Write-Host '[3/6] 复制预编译产物（服务端程序 / 编排 CLI / 探针 / configs）...'
    $lackPrebuilt = @()
    foreach ($rel in $PrebuiltFiles) {
        $src = Join-Path $ROOT $rel
        if (-not (Test-Path $src -PathType Leaf)) { $lackPrebuilt += $rel; continue }
        $dst = Join-Path $work $rel
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst)  | Out-Null
        Copy-Item $src $dst -Force
        $mb = [math]::Round((Get-Item $src).Length / 1MB, 1)
        Write-Host ("   + {0}  ({1} MB)" -f $rel, $mb)
    }
    if ($lackPrebuilt.Count -gt 0) {
        throw ("工作区里缺少预编译产物: " + ($lackPrebuilt -join ', ') +
            "`n  这些文件在 .gitignore 里（仓库不带），必须先在工作区造出来/取回来再打包：" +
            "`n   · bin\dfolauncher.exe 与 bin\wireprobe-pvf.exe：发布仓库的 server-bin 预编译包" +
            "`n   · dfo_probe_tools\probe.exe：该目录的 Build-Probe.cmd")
    }

    $cfgSrc = Join-Path $ROOT $ConfigsRel
    if (-not (Test-Path $cfgSrc -PathType Container)) {
        throw "工作区里缺少 $ConfigsRel（服务端运行必需的配置目录），无法打包"
    }
    $cfgDst = Join-Path $work $ConfigsRel
    New-Item -ItemType Directory -Force -Path $cfgDst  | Out-Null
    $cfgCount = 0
    $cfgSkipped = @()
    Get-ChildItem $cfgSrc -Recurse -File | ForEach-Object {
        # 机器本地配置（server.local.json / launcher.local.json）含本机口令与绝对路径，绝不进包。
        if ($_.Name -like '*.local.json') { $cfgSkipped += $_.Name; return }
        $rel = $_.FullName.Substring($cfgSrc.Length).TrimStart('\')
        $dst = Join-Path $cfgDst $rel
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst)  | Out-Null
        Copy-Item $_.FullName $dst -Force
        $cfgCount++
    }
    Write-Host ("   + {0}\**  ({1} 个文件)" -f $ConfigsRel, $cfgCount)
    if ($cfgSkipped.Count -gt 0) {
        Write-Host ("   - 已跳过机器本地配置: {0}" -f ($cfgSkipped -join ', '))
    }

    # 4. launcher.local.json 模板（相对路径）
    Write-Host '[4/6] 生成 launcher.local.json 模板...'
    Copy-Item (Join-Path $ROOT 'server\launcher.example.json') (Join-Path $work 'server\launcher.local.json') -Force

    # 5. 按 -Kind 裁剪（离线包只留白名单条目）
    Write-Host '[5/7] 按白名单裁剪（离线包只留必要项）...'
    if ($Kind -ne 'dev') {
        $summary = Keep-OnlyAllowed -Tree $work -Rules $rules
        $removedCount = 0
        foreach ($k in $summary.Keys) { $removedCount += $summary[$k] }
        Remove-EmptyDirs -Tree $work
        foreach ($d in $OfflineScaffoldDirs) {
            $p = Join-Path $work $d
            if (-not (Test-Path $p)) { New-Item -ItemType Directory -Force -Path $p  | Out-Null; Write-Host ("   + 预建空目录 {0}" -f ($d -replace '\\', '/')) }
        }
        Write-Host ("   - 裁掉 {0} 个条目（白名单之外）" -f $removedCount)
        $summary.Keys | Sort-Object { -$summary[$_] } | ForEach-Object {
            Write-Host ("      {0,6}  {1}" -f $summary[$_], $_)
        }
    }
    else {
        Write-Host '   (dev 形态：只做显式排除，不裁源码/文档/GM工具)'
        # dev 形态也要把"任何包都不许有"的那一档挖掉（作者存档/半成品/日志……）
        $summary = Keep-OnlyAllowed -Tree $work -Rules ([pscustomobject]@{
                Allow = @('**')
                Forbid = @($ForbiddenRules | Where-Object { $_.kinds -contains 'all' })
            })
        $removedCount = 0
        foreach ($k in $summary.Keys) { $removedCount += $summary[$k] }
        Remove-EmptyDirs -Tree $work
        Write-Host ("   - 裁掉 {0} 个条目（显式排除档）" -f $removedCount)
    }

    if ($ListPruned) {
        Write-Host '[试跑] -ListPruned：不生成 zip。裁剪后的全量条目如下：'
        Get-ChildItem -LiteralPath $work -Recurse -File -Force |
            ForEach-Object { $_.FullName.Substring($work.Length).TrimStart('\').Replace('\', '/') } |
            Sort-Object | ForEach-Object { Write-Host ("     {0}" -f $_) }
        Write-Host '[试跑] 顶层分布:'
        Get-ChildItem -LiteralPath $work -Recurse -File -Force |
            ForEach-Object { $_.FullName.Substring($work.Length).TrimStart('\').Replace('\', '/').Split('/')[0] } |
            Group-Object | Sort-Object Count -Descending | ForEach-Object { Write-Host ("     {0,6}  {1}" -f $_.Count, $_.Name) }
        return
    }

    # 5.5 资源清单（正式发布包的自校验依据）
    #
    # 业主 2026-10-05 口径：zip 分发要能**自校验**。清单写在包根，逐条记录受管文件的
    # 相对路径 + size + sha256 + 版本；玩家/启动器拿它核对"包内资源完整 / 缺失 / 被改坏 / 过期"。
    # 口径与发布仓库的 tools/manifest.json 一致（那边是包的哈希，这里是包内文件的哈希），
    # 所以同一个校验器（启动器 internal/resources 的 Audit）既能核 zip 包，也能核源链补下来的东西。
    Write-Host '[5.5/7] 写资源清单（资源清单.json）...'
    $manifestEntries = New-Object System.Collections.Generic.List[object]
    $managedGlobs = @(
        'server/work/dfo-lan/configs/**',
        'server/work/dfo-lan/cmd/wireprobe/testdata/**',
        'server/work/dfo-lan/runtime/*.bin',
        'server/work/dfo-lan/bin/*.exe',
        'server/work/dfo_probe_tools/probe.exe'
    )
    foreach ($pattern in $managedGlobs) {
        $full = Join-Path $work ($pattern -replace '/', '\')
        Get-ChildItem -Path $full -File -ErrorAction SilentlyContinue | Sort-Object FullName | ForEach-Object {
            $rel = $_.FullName.Substring($work.Length + 1).Replace('\', '/')
            $sum = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            $manifestEntries.Add([ordered]@{ path = $rel; size = $_.Length; sha256 = $sum })
        }
    }
    # 版本号真源 = **包内启动器 exe 自己的 VERSIONINFO**（115 仓库里没有 version.json，
    # 依赖它会得到 0.0.0）；exe 读不到时才退回 version.json（老包可能带），最后才落 0.0.0。
    $verText = ''
    $exeInPack = Join-Path $work 'DFO-115US单机一键启动器.exe'
    if (Test-Path -LiteralPath $exeInPack) {
        $verText = ([string](Get-Item -LiteralPath $exeInPack).VersionInfo.ProductVersion).Trim()
    }
    if (-not $verText) {
        $verFile = Join-Path $work 'version.json'
        if (Test-Path -LiteralPath $verFile) {
            $verDoc = Get-Content -LiteralPath $verFile -Raw -Encoding UTF8 | ConvertFrom-Json
            if ($verDoc.version) { $verText = [string]$verDoc.version }
        }
    }
    if (-not $verText) { $verText = '0.0.0' }
    $manifestDoc = [ordered]@{
        schema       = 1
        version      = $verText
        generated_at = (Get-Date).ToString('o')
        entries      = $manifestEntries
    }
    $manifestPath = Join-Path $work '资源清单.json'
    [System.IO.File]::WriteAllText($manifestPath, ($manifestDoc | ConvertTo-Json -Depth 5), (New-Object System.Text.UTF8Encoding($false)))
    Write-Host ("     受管文件 {0} 项（版本 {1}）" -f $manifestEntries.Count, $verText)

    # 6. Python 标准 zip 打包（正斜杠分隔符、UTF-8 文件名、无 ./ 前缀）
    Write-Host '[6/7] 压缩为 zip...'
    # Python 是打包/GM 工具专用依赖，已移出 tools\（启动链不需要它）：优先整合包外的 gm-tool\python，其次 PATH 上的 python。
    $py = Join-Path (Split-Path -Parent $ROOT) 'gm-tool\python\python.exe'
    if (-not (Test-Path $py)) {
        $cmdPy = Get-Command python -ErrorAction SilentlyContinue
        if ($cmdPy) { $py = $cmdPy.Source }
        else { throw '找不到 Python 解释器：gm-tool\python\python.exe 不存在，PATH 上也没有 python（Python 是打包/GM 工具专用依赖，已移出 tools\）' }
    }
    $pyScript = Join-Path $ROOT 'scripts\build_publish_zip.py'
    & $py $pyScript $work $zipOut
    if ($LASTEXITCODE -ne 0) { throw 'zip 打包失败' }

    # 7. 校验产物（缺任何一个/混进任何一个就删掉半成品并说清是什么）
    Write-Host '[7/7] 校验包（必需项在 + 排除项不在 + 条目全在白名单内）...'
    try {
        Assert-PublishZip -ZipPath $zipOut -Kind $Kind -WithGmTools:$IncludeGmTools `
            -WithServerSource:$IncludeServerSource -WithDevDocs:$IncludeDevDocs
    } catch {
        Remove-Item $zipOut -Force -ErrorAction SilentlyContinue
        throw
    }

    $mb = [math]::Round((Get-Item $zipOut).Length / 1MB, 1)
    Write-Host ''
    Write-Host ("[成功] 离线自用包已生成: {0}  ({1} MB)" -f $zipOut, $mb)
    if ($Kind -ne 'dev') {
        Write-Host '离线包只含必要项（启动入口 + 预编译服务端 + configs + 说明）：不含源码、开发者文档、GM 工具、作者存档。'
        Write-Host '提醒：分发给玩家的是单个 exe（DFO-115US单机一键启动器.exe）；本包仅作离线自备/应急。'
    }
    else {
        Write-Host '开发者包：含源码/文档/GM 工具，仅供内部使用，不要外发。'
    }
    Write-Host '下一步: 解压后运行 scripts\启动游戏.cmd（首次会自动用你的 Script.pvf 生成内层 PVF）。'
} finally {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $tar -Force -ErrorAction SilentlyContinue
}
