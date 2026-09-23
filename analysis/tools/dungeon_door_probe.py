# -*- coding: utf-8 -*-
"""
dungeon_door_probe.py — 「CMD 2062 直达进图后首图传送门不生效」frida 取证探针（安全版：只挂函数入口）

背景 / 未闭环点：
  server/work/dfo-lan/docs/protocol/next49-odyssey-direct-move.md 的「未闭环」节。
  三次服务端尝试（dfa47bb / 512ca69 / 41fbc1c）后，直达路径的进图帧序列已与城镇选图逐字节一致，
  但客户端在「CMD 2062 直达进入的副本」里仍然不发任何房间门请求。本探针用来判定：
  客户端到底有没有走到 MOVE_MAP(CMD 45) 的发送路径，以及调用者/调用栈是谁。

锚点（analysis/dumps/opcodes.tsv 的 table_slot_va；镜像基址 0x140000000，无 ASLR）：
  0x14ef38fd8  CMD 15   ENUM_CMDPACKET_ENTER_SELECT_DUNGEON
  0x14ef38fe0  CMD 16   ENUM_CMDPACKET_SELECT_DUNGEON
  0x14ef390c8  CMD 45   ENUM_CMDPACKET_MOVE_MAP
  0x14ef397b0  CMD 266  ENUM_CMDPACKET_MOVE_MAP_REPORT
  0x14ef3cfd0  CMD 2062 ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE

  这些槽位由表初始化（0x140069bb0 / 0x140075000）写入，写进去的是包描述解密函数
  0x146e8c7d0 的返回值 —— **不保证是函数指针**。所以本探针自适应处理：
    · 槽位值落在可执行段 → 直接作为函数挂钩；
    · 槽位值落在模块数据段 → 当作包描述结构，dump 96 字节、逐 8 字节标出其中的
      可执行指针，并把这些指针也挂上（标签形如 MOVE_MAP+0x10）。
  只记录调用者与调用栈，**不修改任何寄存器/内存，不阻塞调用**；每个挂钩点只记录前 30 次详情。

用法：
  D:\\115us\\tools\\python\\python.exe analysis\\tools\\dungeon_door_probe.py [pid]
  （不带 pid 时自动等待 DFO.exe 出现）
输出：D:\\115us\\analysis\\dumps\\dungeon_door_probe.log
停止：创建 D:\\115us\\analysis\\dumps\\dungeon_door_probe.log.stop，或 Ctrl+C

建议操作顺序（对照实验见 next49「下一步取证方案」）：
  A) 先只启动客户端到选区/角色界面即可 —— 此时就能看到 SLOT/DUMP/HOOK 段，用来确认锚点长什么样。
  B) 场景对照：从城镇正常选图进副本 → 走到传送门/传送阵过门
     预期：出现 MOVE_MAP 相关 CALL（锚点正确的话，这里能看到发送函数 RVA）。
  C) 问题场景：清关后走「下一个剧情关卡」直达进入 → 站到传送阵上
     关注：是否完全没有 MOVE_MAP 的 CALL；若确实没有，则门判定在更早处被拦住，
           需要沿日志里其它 CALL 的调用栈向上追。
"""
import os
import sys
import time

import frida

LOG = r"D:\115us\analysis\dumps\dungeon_door_probe.log"
STOP = LOG + ".stop"

COUNTS = {}     # label -> 调用次数
HOOKED = {}     # label -> 函数 RVA
TX_HEADS = []   # 每次发包的包头（前 8 字节）

JS = r"""
const SLOTS = [
  { id: 15,   name: 'CMD15_ENTER_SELECT_DUNGEON', va: '0x14ef38fd8', deep: true  },
  { id: 16,   name: 'CMD16_SELECT_DUNGEON',       va: '0x14ef38fe0', deep: true  },
  { id: 45,   name: 'CMD45_MOVE_MAP',             va: '0x14ef390c8', deep: true  },
  { id: 266,  name: 'CMD266_MOVE_MAP_REPORT',     va: '0x14ef397b0', deep: false },
  { id: 2062, name: 'CMD2062_DUNGEON_DIRECT_MOVE', va: '0x14ef3cfd0', deep: true },
];

const HOOK_SLOTS = false;   // 实测这 5 个槽位存的是「包描述结构 RVA」，不是发送函数：
                            // 挂上去全程零触发，且 attach 到非函数入口有破坏代码的风险，故默认不开。

const MOD = (function () {
  const m = Process.findModuleByName('DFO.exe') || Process.enumerateModules()[0];
  return { name: m.name, base: m.base, size: m.size };
})();

function inMod(p) {
  try { return p.compare(MOD.base) >= 0 && p.compare(MOD.base.add(MOD.size)) < 0; }
  catch (e) { return false; }
}
function rva(p) {
  try { return inMod(p) ? 'DFO+0x' + p.sub(MOD.base).toString(16) : p.toString(); }
  catch (e) { return String(p); }
}
function prot(p) {
  try { const r = Process.findRangeByAddress(p); return r ? r.protection : null; }
  catch (e) { return null; }
}
function isExec(p) { const s = prot(p); return !!s && s.indexOf('x') !== -1; }

const hooked = {};
const CAP = 30;

function hookAddr(addr, label, kind) {
  const key = addr.toString();
  if (hooked[key] !== undefined) return;
  hooked[key] = 0;
  try {
    Interceptor.attach(addr, {
      onEnter: function () {
        hooked[key]++;
        const n = hooked[key];
        if (n > CAP) return;
        let bt = [];
        try { bt = Thread.backtrace(this.context, Backtracer.ACCURATE).slice(0, 8).map(rva); }
        catch (e) { bt = ['ERR ' + e]; }
        send({
          t: 'call', label: label, kind: kind, fn: rva(addr), n: n,
          ret: rva(this.returnAddress), bt: bt,
          rcx: this.context.rcx.toString(), rdx: this.context.rdx.toString(),
          r8: this.context.r8.toString(), r9: this.context.r9.toString()
        });
      }
    });
    send({ t: 'hook', label: label, kind: kind, addr: addr.toString(), rva: rva(addr) });
  } catch (e) {
    send({ t: 'err', msg: 'hook ' + label + ' @' + rva(addr) + ' failed: ' + e });
  }
}

function probe(s) {
  const slot = ptr(s.va);
  let v;
  try { v = slot.readPointer(); } catch (e) { send({ t: 'err', msg: s.name + ' read slot: ' + e }); return; }

  if (v.isNull()) { send({ t: 'slot', name: s.name, slot: rva(slot), value: 'NULL' }); return; }

  send({ t: 'slot', name: s.name, slot: rva(slot), value: v.toString() });

  // 槽位里存的可能是绝对 VA，也可能是 RVA（本机实测值形如 0x381ad94，加模块基址才落进模块）。
  const num = parseInt(v.toString(), 16);
  const cands = [{ addr: v, how: 'raw-va' }];
  if (num > 0 && num < MOD.size) cands.push({ addr: MOD.base.add(num), how: 'base+RVA' });

  for (const c of cands) {
    const p = prot(c.addr), exec = isExec(c.addr), inside = inMod(c.addr);
    send({ t: 'cand', name: s.name, how: c.how, addr: c.addr.toString(), rva: rva(c.addr),
           prot: p || '-', inMod: !!inside, exec: !!exec });
    if (!inside) continue;
    if (exec) {
      if (HOOK_SLOTS) hookAddr(c.addr, s.name + ' [' + c.how + ']', 'function');
      else send({ t: 'note', msg: s.name + ' [' + c.how + '] 指向代码 ' + rva(c.addr) + '，按 HOOK_SLOTS=false 跳过挂钩' });
      continue;
    }
    if (!s.deep) continue;

    // 当作包描述/跳转结构：dump 96 字节 + 逐 8 字节找可执行指针
    const ptrs = [];
    for (let off = 0; off < 96; off += 8) {
      let q;
      try { q = c.addr.add(off).readPointer(); } catch (e) { continue; }
      const qp = prot(q), qx = qp && qp.indexOf('x') !== -1;
      ptrs.push('+0x' + off.toString(16) + '=' + rva(q) + (qp ? (qx ? ' [EXEC]' : ' [data]') : ' [outside]'));
      if (qx) hookAddr(q, s.name + ' [' + c.how + ']+0x' + off.toString(16), 'struct-function');
    }
    let raw = '';
    try { raw = hexdump(c.addr, { length: 96, ansi: false }).replace(/\n/g, '\n        '); }
    catch (e) { raw = 'ERR ' + e; }
    send({ t: 'dump', name: s.name, how: c.how, addr: c.addr.toString(), rva: rva(c.addr), ptrs: ptrs, raw: raw });
  }
}

for (const s of SLOTS) {
  try { probe(s); } catch (e) { send({ t: 'err', msg: s.name + ': ' + e }); }
}

// ---- 网络发送锚点：ws2_32 导出函数（可靠锚点）----
// 槽位里那 5 个 RVA 实测不是发送函数（挂上后一个 CALL 都没有），改用系统 DLL 出口：
// 每次发包都会经过它，靠包头识别 opcode，并抓调用栈以定位客户端自己的发送路径。
let txCount = 0;
const TX_CAP = 200;

function txRecord(api, len, head, bt, extra) {
  txCount++;
  const n = txCount;
  if (n > TX_CAP) return;
  send({ t: 'tx', api: api, len: len, head: head, n: n, bt: bt, extra: extra || '' });
}

function head8(p, len) {
  try {
    const n = Math.max(0, Math.min(8, len));
    if (n === 0) return '';
    const a = new Uint8Array(p.readByteArray(n));
    let s = '';
    for (let i = 0; i < a.length; i++) s += (a[i] < 16 ? '0' : '') + a[i].toString(16) + ' ';
    return s.trim();
  } catch (e) { return 'ERR ' + e; }
}

function hookNet() {
  const ws2 = Process.findModuleByName('ws2_32.dll');
  if (!ws2) { send({ t: 'err', msg: 'ws2_32.dll 未找到' }); return; }
  const eSend = ws2.findExportByName('send');
  const eWSASend = ws2.findExportByName('WSASend');
  const eSendTo = ws2.findExportByName('sendto');
  send({ t: 'net', send: eSend ? rva(eSend) : '-', wsasend: eWSASend ? rva(eWSASend) : '-', sendto: eSendTo ? rva(eSendTo) : '-' });

  if (eSend) {
    Interceptor.attach(eSend, {
      onEnter: function (args) { this.buf = args[1]; this.len = args[2].toInt32(); },
      onLeave: function (ret) {
        let bt = [];
        try { bt = Thread.backtrace(this.context, Backtracer.ACCURATE).slice(0, 10).map(rva); } catch (e) {}
        txRecord('send', this.len, head8(this.buf, this.len), bt, 'ret=' + ret.toInt32());
      }
    });
  }
  if (eSendTo) {
    Interceptor.attach(eSendTo, {
      onEnter: function (args) { this.buf = args[1]; this.len = args[2].toInt32(); },
      onLeave: function (ret) {
        let bt = [];
        try { bt = Thread.backtrace(this.context, Backtracer.ACCURATE).slice(0, 10).map(rva); } catch (e) {}
        txRecord('sendto', this.len, head8(this.buf, this.len), bt, 'ret=' + ret.toInt32());
      }
    });
  }
  if (eWSASend) {
    Interceptor.attach(eWSASend, {
      onEnter: function (args) { this.pbuf = args[1]; this.cnt = args[2].toInt32(); },
      onLeave: function (ret) {
        let len = -1, head = '';
        try {
          len = this.pbuf.readU32();
          const p = this.pbuf.add(Process.pointerSize).readPointer();
          head = head8(p, len);
        } catch (e) { head = 'ERR ' + e; }
        let bt = [];
        try { bt = Thread.backtrace(this.context, Backtracer.ACCURATE).slice(0, 10).map(rva); } catch (e) {}
        txRecord('WSASend', len, head, bt, 'bufs=' + this.cnt + ' ret=' + ret.toInt32());
      }
    });
  }
}
hookNet();

send({ t: 'ready', n: Object.keys(hooked).length });
"""


def on_message(msg, data):
    if msg.get('type') != 'send':
        line = "[!] %s" % (msg,)
    else:
        p = msg['payload']
        t = p.get('t')
        ts = time.strftime('%H:%M:%S')
        if t == 'slot':
            line = "[%s] SLOT %-28s slot=%s -> %s" % (ts, p['name'], p['slot'], p.get('value', ''))
        elif t == 'cand':
            line = "[%s] CAND %-28s %-9s %s (%s) prot=%-4s inMod=%-5s exec=%s" % (
                ts, p['name'], p['how'], p['rva'], p['addr'], p['prot'], p['inMod'], p['exec'])
        elif t == 'dump':
            line = "[%s] DUMP %-28s %s struct@%s\n        %s\n        raw:\n        %s" % (
                ts, p['name'], p.get('how', ''), p['rva'], '\n        '.join(p['ptrs']), p['raw'])
        elif t == 'hook':
            HOOKED[p['label']] = p['rva']
            line = "[%s] HOOK %-28s %s (%s)" % (ts, p['label'], p['rva'], p['kind'])
        elif t == 'call':
            COUNTS[p['label']] = COUNTS.get(p['label'], 0) + 1
            line = "[%s] CALL %-28s #%-4d ret=%s\n        bt: %s\n        rcx=%s rdx=%s r8=%s r9=%s" % (
                ts, p['label'], p['n'], p['ret'], ' <- '.join(p['bt']),
                p['rcx'], p['rdx'], p['r8'], p['r9'])
        elif t == 'note':
            line = "[%s] NOTE %s" % (ts, p['msg'])
        elif t == 'net':
            line = "[%s] NET  ws2_32 send=%s WSASend=%s sendto=%s" % (
                ts, p['send'], p['wsasend'], p['sendto'])
        elif t == 'tx':
            TX_HEADS.append(p['head'])
            line = "[%s] TX   #%-3d %-7s len=%-6s head=%s  %s\n        bt: %s" % (
                ts, p['n'], p['api'], p['len'], p['head'], p['extra'], ' <- '.join(p['bt']))
        elif t == 'ready':
            line = "[%s] READY 挂钩点 %s 个" % (ts, p['n'])
        else:
            line = "[%s] %s" % (ts, p)
    print(line, flush=True)
    with open(LOG, 'a', encoding='utf-8') as f:
        f.write(line + "\n")


def find_dfo_pid():
    try:
        for proc in frida.get_local_device().enumerate_processes():
            if proc.name.lower() == 'dfo.exe':
                return proc.pid
    except Exception:
        pass
    return None


def write_summary():
    lines = ["", "===== 调用汇总（本次会话） ====="]
    if not HOOKED:
        summary = ["  没有任何挂钩点 —— 槽位既不是函数、结构里也没有可执行指针，需要换锚点。"]
    else:
        summary = []
        for label, addr in sorted(HOOKED.items()):
            summary.append("  %-34s %s  调用 %d 次" % (label, addr, COUNTS.get(label, 0)))
        n45 = sum(c for l, c in COUNTS.items() if l.startswith('CMD45'))
        summary.append("")
        summary.append("  MOVE_MAP(CMD 45) 相关调用合计 %d 次%s" % (
            n45, "  <- 过门时确实走到了发送路径" if n45 else "  <- 一次都没走到：门判定在更早处被拦住"))
    # 发包统计：C2S 包头第 2 字节即 opcode（实测 CMD 45 = 0x2d）
    if TX_HEADS:
        n45tx = 0
        for h in TX_HEADS:
            parts = h.split()
            if len(parts) >= 2 and parts[1] == '2d':
                n45tx += 1
        summary.append("  发包记录 %d 条，其中包头第 2 字节 = 2d（CMD 45 MOVE_MAP）的有 %d 条" % (
            len(TX_HEADS), n45tx))
    lines += summary
    text = "\n".join(lines)
    print(text, flush=True)
    with open(LOG, 'a', encoding='utf-8') as f:
        f.write(text + "\n")


def main():
    os.makedirs(os.path.dirname(LOG), exist_ok=True)
    if os.path.exists(STOP):
        os.remove(STOP)
    open(LOG, 'w', encoding='utf-8').close()

    pid = int(sys.argv[1]) if len(sys.argv) > 1 else None
    print("[*] 等待 DFO.exe ...（建议先启动服务端与客户端，再运行本探针）", flush=True)
    session = None
    while session is None:
        try:
            pid = pid or find_dfo_pid()
            if pid is not None:
                session = frida.attach(pid)
        except Exception as e:
            print("[!] attach(pid=%s) 失败: %r，重试" % (pid, e), flush=True)
            pid = None
            time.sleep(2)

    print("[*] 已附加 DFO.exe (pid=%s)，安装探针..." % pid, flush=True)
    script = session.create_script(JS)
    script.on('message', on_message)
    session.on('detached', lambda *a: print("[!] 会话已断开（客户端退出或重启？）", flush=True))
    script.load()
    print("[*] 探针就绪。日志: %s" % LOG, flush=True)
    print("[*] 停止: 创建 %s 或 Ctrl+C" % STOP, flush=True)

    try:
        while not os.path.exists(STOP):
            time.sleep(1)
    except KeyboardInterrupt:
        print("[*] Ctrl+C，退出。", flush=True)
    write_summary()
    print("[*] 已停止，日志在 %s" % LOG, flush=True)


if __name__ == '__main__':
    main()
