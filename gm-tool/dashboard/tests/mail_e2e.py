# -*- coding: utf-8 -*-
"""邮件系统 e2e：搜索 -> 选择 -> 目标切换(背包/仓库/邮箱) -> 发送 -> 邮件列表/撤销。"""
import asyncio, json, base64, os, sys, urllib.request
import websockets

DEBUG_HTTP = "http://127.0.0.1:9348"
OUT_DIR = r"D:\115us\launcher"
ID = [0]
ERRORS = []

def next_id():
    ID[0] += 1
    return ID[0]

async def cdp(ws, method, params=None):
    mid = next_id()
    await ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
    while True:
        msg = json.loads(await ws.recv())
        if msg.get("id") == mid:
            return msg.get("result", {})
        if msg.get("method") == "Runtime.exceptionThrown":
            ERRORS.append("EXC: " + json.dumps(msg["params"]["exceptionDetails"], ensure_ascii=False)[:300])
        elif msg.get("method") == "Runtime.consoleAPICalled" and msg["params"]["type"] == "error":
            ERRORS.append("CONSOLE: " + json.dumps(msg["params"]["args"], ensure_ascii=False)[:300])

def get_page_ws():
    with urllib.request.urlopen(DEBUG_HTTP + "/json", timeout=5) as r:
        targets = json.loads(r.read().decode("utf-8"))
    for t in targets:
        if t.get("type") == "page":
            return t["webSocketDebuggerUrl"]
    return None

async def eval_js(ws, expr):
    res = await cdp(ws, "Runtime.evaluate", {"expression": expr, "returnByValue": True, "awaitPromise": True})
    if "exceptionDetails" in res:
        return "EXC: " + json.dumps(res["exceptionDetails"], ensure_ascii=False)[:200]
    return res.get("result", {}).get("value")

async def shot(ws, name):
    res = await cdp(ws, "Page.captureScreenshot", {"format": "png"})
    data = base64.b64decode(res["data"])
    path = os.path.join(OUT_DIR, name)
    with open(path, "wb") as f:
        f.write(data)
    return path

async def main():
    ws_url = None
    for _ in range(20):
        ws_url = get_page_ws()
        if ws_url:
            break
        await asyncio.sleep(0.5)
    if not ws_url:
        print("FAIL: no page target via CDP"); sys.exit(1)

    async with websockets.connect(ws_url, max_size=20 * 1024 * 1024) as ws:
        await cdp(ws, "Runtime.enable")
        await cdp(ws, "Page.enable")
        ERRORS.clear()
        cb = str(int(asyncio.get_event_loop().time() * 1000))
        await cdp(ws, "Page.navigate", {"url": "http://127.0.0.1:28081/?cb=" + cb})
        await asyncio.sleep(4)

        # 选择账号角色
        await eval_js(ws, "selectAccount(1,'mmmm',false)")
        await asyncio.sleep(2.5)
        await eval_js(ws, "(function(){ var n=document.querySelector('.rolebar .role'); if(n) n.click(); })()")
        await asyncio.sleep(2.5)

        # 切邮件系统
        await eval_js(ws, "switchView('mail')")
        await asyncio.sleep(1.5)
        tabs = await eval_js(ws, "Array.from(document.querySelectorAll('#tabs .tab')).map(t=>t.textContent).join('|')")
        has_q = await eval_js(ws, "!!document.getElementById('mail_q') && !!document.getElementById('mail_btn')")
        p = await shot(ws, "mail_1_send.png")
        print(f"OK step1 mail view: tabs={tabs} searchbar={has_q} ->", p)

        # 搜索 10300000
        await eval_js(ws, "document.getElementById('mail_q').value='10300000'; mailSearch();")
        await asyncio.sleep(1.5)
        res_rows = await eval_js(ws, "document.querySelectorAll('#mail_res .row2').length")
        flags = await eval_js(ws, """(function(){
          var n=document.querySelector('#mail_res .row2');
          if(!n) return 'no-row';
          return (n.querySelector('.pill.green')?'stackable ':'') + (n.querySelector('.pill.cat')?n.querySelector('.pill.cat').textContent:'no-cat');
        })()""")
        await eval_js(ws, "(function(){ var n=document.querySelector('#mail_res .row2'); if(n) n.click(); })()")
        await asyncio.sleep(0.8)
        sel_ok = await eval_js(ws, "!!document.getElementById('mail_amount') && !!document.getElementById('mail_targets') && !!document.getElementById('mail_send') ? 'selected' : 'not-selected'")
        target_tabs = await eval_js(ws, "Array.from(document.querySelectorAll('#mail_targets .tab')).map(t=>t.textContent).join('|')")
        hint = await eval_js(ws, "document.getElementById('mail_selcard').textContent.slice(0,160)")
        print(f"OK step2 search+select: rows={res_rows} {flags} {sel_ok} targets={target_tabs}")
        print("     hint:", hint.replace("\n", " "))

        # 切到邮箱 -> 发送
        await eval_js(ws, "(function(){ var t=document.querySelector('#mail_targets .tab[data-tg=\"mail\"]'); if(t) t.click(); })()")
        await asyncio.sleep(0.6)
        active = await eval_js(ws, "var a=document.querySelector('#mail_targets .tab.active'); a ? a.textContent : 'none'")
        await eval_js(ws, "document.getElementById('mail_send').click()")
        await asyncio.sleep(2)
        msg1 = await eval_js(ws, "document.getElementById('mail_msg').textContent")
        print(f"OK step3 mail target={active} send-msg={msg1}")

        # 切到背包 -> 发送
        await eval_js(ws, "(function(){ var t=document.querySelector('#mail_targets .tab[data-tg=\"bag\"]'); if(t) t.click(); })()")
        await asyncio.sleep(0.6)
        await eval_js(ws, "document.getElementById('mail_amount').value='2'; document.getElementById('mail_amount').onchange();")
        await eval_js(ws, "document.getElementById('mail_send').click()")
        await asyncio.sleep(2)
        msg2 = await eval_js(ws, "document.getElementById('mail_msg').textContent")
        print("OK step4 bag send:", msg2)

        # 切到仓库 -> 发送
        await eval_js(ws, "(function(){ var t=document.querySelector('#mail_targets .tab[data-tg=\"vault\"]'); if(t) t.click(); })()")
        await asyncio.sleep(0.6)
        await eval_js(ws, "document.getElementById('mail_send').click()")
        await asyncio.sleep(2)
        msg3 = await eval_js(ws, "document.getElementById('mail_msg').textContent")
        print("OK step5 vault send:", msg3)

        # 邮件列表
        await eval_js(ws, "state.subTab=1; renderView();")
        await asyncio.sleep(2)
        rows = await eval_js(ws, "document.querySelectorAll('#content table tbody tr').length")
        first_row = await eval_js(ws, """(function(){
          var tr=document.querySelector('#content table tbody tr');
          return tr ? tr.textContent.slice(0,120).replace(/\\s+/g,' ') : 'none';
        })()""")
        p = await shot(ws, "mail_2_list.png")
        print(f"OK step6 mail list rows={rows} first=[{first_row}] ->", p)

        # 撤销第一条
        await eval_js(ws, "(function(){ var b=document.querySelector('.mail-revoke'); if(b) b.click(); return b?'revoke-clicked':'no-revoke-btn'; })()")
        await asyncio.sleep(2)
        status_now = await eval_js(ws, """(function(){
          var rows=document.querySelectorAll('#content table tbody tr');
          var revoked=0, total=rows.length;
          for(var i=0;i<rows.length;i++){ if(rows[i].textContent.indexOf('revoked')>=0) revoked++; }
          return revoked + '/' + total;
        })()""")
        print("OK step7 revoke status:", status_now)

        print("NO JS ERRORS" if not ERRORS else ("JS ERRORS: " + " | ".join(ERRORS[:5])))

if __name__ == "__main__":
    asyncio.run(main())
