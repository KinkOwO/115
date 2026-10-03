# next79 合并后无法进入作战：派发挂接缺失

用户合并仓库后，建队并开始作战，无法继续选择作战/进图。以
`D:/115us-backup-20261003/115-server` 中已验证版本为对照，未覆盖整个备份或回滚合并。

当前起点a3a2679（63be07e合并重构后补Moon频道配置），工作区起点干净。
实际默认程序D8B136...，源码候选仍ABA2A78...旧版；程序入口也必须同步发布。

## 已确认差异

最新会话 `20261003_191703_976004_next37`：19:19:21 CMD12建队，19:19:25 CMD2043
开战，服务端发2254→2255初始态→ACK2043；之后没有`ispins_info_wait_pushed`，
客户端始终未发CMD2047或2045。不是选图请求已经发出后被拒绝。

备份main.go在CMD2043成功后先快照wait0，再延迟2500ms发送N2255，事件名
`ispins_info_wait_pushed`。合并的新ispins_wiring.go保留了即时三包，却漏了这段
延迟待选状态，客户端因此停在初始态。

另一个差异：新dispatchIspins对所有已校验CMD35都return dispatchHandled，
即使没有次数恢复数据也吞掉请求。备份原循环在附加恢复通知后继续处理待机资格
及普通城镇位置。提前return会阻断后续world handler和同帧pending待机数据。

对照备份，ispins_flow.go、internal/legion/ispins.go、两个Ispins向量文件、
protocol/ispins_party.go及legion_reward115.go逐字节一致；本轮修挂接，不重写已确认协议。

## 修正

- CMD2043即时批次成功后恢复2500ms N2255待选态推送；包体及角色ID先快照，
  延迟回调不读取已经变化的挑战/选角状态。连接关闭信号会取消尚未发送的定时通知。
- 次数恢复通知后继续派发CMD35，允许pending待机数据和城镇位置处理继续执行。
- 原有无限/每周模式、建队、结算、存档及其它重构保留。无schema、玩家数据库、
  客户端资源或DLL改动；只恢复备份中已实机验证的运行路径，没有猜新包。

## 验证与发布

真实net.Pipe连接回归通过：收到2254→2255初始→ACK2043，然后收到2255 wait0；
改变可变挑战/选角状态后，延迟通知仍使用原快照。CMD35普通及pending待机分支
均返回dispatchNext且待机数据送达。
overlay恢复修复前wiring，两项回归分别失败为等待N2255读超时、CMD35被吞掉；
修复后通过。所有备份main.go中的Ispins事件挂接名称在新代码中均存在。
`go test ./...`、`go vet ./...` 全部通过，未再出现旧版4项审计失败。
按项目门禁执行charactercheck，仍在其私有测试schema中失败为缺少
account_unified_options，与server/AGENTS.md已记录的该工具既有问题一致；
本轮只改wireprobe派发文件，未改charactercheck/存储或执行玩家schema迁移。
日志仓库外 `analysis-tools/output/next79-merge-wiring-*` / `next79-pre-merge-wiring-check.log`。

源码与默认程序已同步更新，SHA256：
`d5d298053b4fb36ae3f4ee5c4995ca9f3bfcc970f5f6bc966a142a64c99a3330`。
发布时没有客户端/服务端进程；按用户习惯不自动启动服务端，由用户重启手动验收。

待实机：建队→开始作战后等待选择面板→确认→进入第一图，然后四阶段结算、
最终动画→回城→离队/重复挑战回归。当前候选尚未实机确认，不扩大基线1。
