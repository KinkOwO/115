# -*- coding: utf-8 -*-
"""
arrow_probe.py v3 — 装备栏箭头探针（安全版：只挂函数入口，无中段钩子）

挂钩点（RVA，镜像基址 0x140000000，无 ASLR）：
  0x5CDCA30  Query 函数入口 stub（→0x145CDC8E0）
             onEnter: RCX=manager, EDX=id(0–0x31)
             onLeave: RAX=已装备对象（0=查不到 → 箭头路径）
  0x500A050  SetGridOverlay 入口 — RCX=格子, R8D=kind（1=画↑箭头），仅记录 kind 变化

用法: python arrow_probe.py [pid]
输出: D:\\115us\\analysis\\dumps\\arrow_probe.log
停止: 创建 D:\\115us\\analysis\\dumps\\arrow_probe.log.stop
"""
import sys, time, os
import frida

LOG = r"D:\115us\analysis\dumps\arrow_probe.log"

JS = r"""
const BASE = ptr(0x140000000);
const pQuery = BASE.add(0x5CDCA30);   // Query 入口 stub（静态解出候选1）
const pOverlay = BASE.add(0x500A050); // SetGridOverlay 入口

const lastKind = {};
const lastRet = {};
const tblA = {};   // mgr+0x5B98+id*24  注册表（已装备）
const tblB = {};   // mgr+0x10F60+id*24 当前/展示表
let qCount = 0;
const QCAP = 500000;
const NID = 49;    // id 0..0x31 (49)
let mgrCaptured = false;
let pollTimer = null;

function snapshot(mgr, table, key) {
    const out = {};
    for (let i = 0; i < NID; i++) {
        try {
            out[i] = mgr.add(key + i * 24).readPointer().toString();
        } catch (e) { out[i] = 'ERR'; }
    }
    return out;
}

function diffAndSend(tag, cur, last) {
    const changed = [];
    for (let i = 0; i < NID; i++) {
        if (last[i] !== cur[i]) {
            changed.push(i + ':' + (last[i] || '?') + '->' + cur[i]);
            last[i] = cur[i];
        }
    }
    if (changed.length) send({t:'table', which:tag, chg:changed});
}

function startPolling(mgr) {
    // 初始快照（不作为变化上报）
    const a0 = snapshot(mgr, tblA, 0x5B98);
    for (const k in a0) tblA[k] = a0[k];
    const b0 = snapshot(mgr, tblB, 0x10F60);
    for (const k in b0) tblB[k] = b0[k];
    const emptyB = Object.values(b0).filter(v => v === '0x0').length;
    send({t:'pollstart', mgr: mgr.toString(),
          aNonZero: Object.values(a0).filter(v => v !== '0x0').length,
          bNonZero: NID - emptyB});
    pollTimer = setInterval(function () {
        diffAndSend('A', snapshot(mgr, tblA, 0x5B98), tblA);
        diffAndSend('B', snapshot(mgr, tblB, 0x10F60), tblB);
    }, 200);
}

Interceptor.attach(pQuery, {
    onEnter: function (args) {
        this.mgr = this.context.rcx;
        this.id = this.context.rdx.toInt32() >>> 0;   // x64: 无 edx，取 rdx 低 32 位
    },
    onLeave: function (retval) {
        if (!mgrCaptured && !this.mgr.isNull()) {
            mgrCaptured = true;
            try { startPolling(this.mgr); } catch (e) { send({t:'pollstart', mgr:'ERR '+e}); }
        }
        if (qCount > QCAP) return;
        const ret = retval.toString();
        if (lastRet[this.id] === ret) return;          // 只记录变化
        lastRet[this.id] = ret;
        qCount++;
        send({t:'query', id:this.id, ret:ret, src:'cand1'});
    }
});

Interceptor.attach(pOverlay, {
    onEnter: function (args) {
        const slot = this.context.rcx.toString();
        const kind = this.context.r8.toInt32();
        if (lastKind[slot] === kind) return;
        lastKind[slot] = kind;
        send({t:'overlay', slot:slot, kind:kind});
    }
});
"""

def on_message(msg, data):
    if msg.get('type') != 'send':
        line = "[!] %s" % (msg,)
    else:
        p = msg['payload']
        ts = time.strftime('%H:%M:%S')
        if p.get('t') == 'query':
            line = "[%s] QUERY[%s] id=%-6u -> ret=%s%s" % (
                ts, p.get('src', '?'), p['id'], p['ret'],
                "   (0 => 查不到→箭头)" if p['ret'] in ('0x0', '0') else "")
        elif p.get('t') == 'table':
            line = "[%s] TABLE[%s] %s" % (ts, p['which'], "; ".join(p['chg']))
        elif p.get('t') == 'pollstart':
            line = "[%s] POLLSTART mgr=%s A非零=%s B非零=%s" % (
                ts, p.get('mgr'), p.get('aNonZero'), p.get('bNonZero'))
        else:
            line = "[%s] OVERLAY slot=%s kind=%d%s" % (ts, p['slot'], p['kind'],
                    "   <== 画箭头" if p['kind'] == 1 else "")
    print(line, flush=True)
    with open(LOG, 'a', encoding='utf-8') as f:
        f.write(line + "\n")

def find_dfo_pid():
    try:
        for p in frida.get_local_device().enumerate_processes():
            if p.name.lower() == "dfo.exe":
                return p.pid
    except Exception:
        pass
    return None

def main():
    open(LOG, 'w').close()
    pid = int(sys.argv[1]) if len(sys.argv) > 1 else None
    print("[*] 等待 DFO.exe ...", flush=True)
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
    script.load()
    print("[*] 探针就绪（仅函数入口钩子）。", flush=True)
    stopfile = LOG + ".stop"
    if os.path.exists(stopfile):
        os.remove(stopfile)
    while not os.path.exists(stopfile):
        time.sleep(2)
    print("[*] 收到停止信号。日志在 %s" % LOG, flush=True)

if __name__ == '__main__':
    main()
