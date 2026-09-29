# -*- coding: utf-8 -*-
"""CDP end-to-end verification for the GM dashboard page (reliable error capture).

Fixes vs previous versions:
- cdp() now collects Runtime.exceptionThrown / console errors instead of dropping them
- navigates with a cache-buster query to force a full page reload
"""
import asyncio, json, base64, os, sys
import urllib.request
import websockets

DEBUG_HTTP = "http://127.0.0.1:9347"
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
            d = msg["params"]["exceptionDetails"]
            ERRORS.append("EXC: " + json.dumps(d, ensure_ascii=False)[:300])
        elif msg.get("method") == "Runtime.consoleAPICalled" and msg["params"]["type"] == "error":
            ERRORS.append("CONSOLE: " + json.dumps(msg["params"]["args"], ensure_ascii=False)[:300])

def get_page_ws():
    req = urllib.request.Request(DEBUG_HTTP + "/json")
    with urllib.request.urlopen(req, timeout=5) as r:
        targets = json.loads(r.read().decode("utf-8"))
    for t in targets:
        if t.get("type") == "page":
            return t["webSocketDebuggerUrl"]
    return None

async def eval_js(ws, expr):
    res = await cdp(ws, "Runtime.evaluate",
                    {"expression": expr, "returnByValue": True, "awaitPromise": True})
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
        print("FAIL: no page target via CDP")
        sys.exit(1)

    async with websockets.connect(ws_url, max_size=20 * 1024 * 1024) as ws:
        await cdp(ws, "Runtime.enable")
        await cdp(ws, "Page.enable")
        ERRORS.clear()
        cb = str(int(asyncio.get_event_loop().time() * 1000))
        await cdp(ws, "Page.navigate", {"url": "http://127.0.0.1:28081/?cb=" + cb})
        await asyncio.sleep(4)

        v = await eval_js(ws, "state.view")
        title = await eval_js(ws, "document.getElementById('pageTitle').textContent")
        p = await shot(ws, "e2e_1_accounts.png")
        print(f"OK step1 view={v} title={title} ->", p)

        # select account 1 -> characters
        await eval_js(ws, "selectAccount(1,'mmmm',false)")
        await asyncio.sleep(2.5)
        title = await eval_js(ws, "document.getElementById('pageTitle').textContent")
        roles = await eval_js(ws, "document.querySelectorAll('.rolebar .role').length")
        print(f"OK step2 title={title} roles={roles}")
        # click role -> five cards
        await eval_js(ws, "(function(){ var n=document.querySelector('.rolebar .role'); if(n) n.click(); })()")
        await asyncio.sleep(2.5)
        cards = await eval_js(ws, "document.querySelectorAll('#content .card').length")
        forms = await eval_js(ws, "[['lv_do','level'],['sk_do','skills'],['bag_do','bag'],['q_do','quests']].every(function(p){ return !!document.getElementById(p[0]); }) ? 'all-forms' : 'missing-form'")
        p = await shot(ws, "e2e_2b_role_cards.png")
        print(f"OK step2b cards={cards} {forms} ->", p)

        # items view
        await eval_js(ws, "switchView('items')")
        await asyncio.sleep(2.5)
        tabs = await eval_js(ws, "Array.from(document.querySelectorAll('#tabs .tab')).map(t=>t.textContent).join('|')")
        rows = await eval_js(ws, "document.querySelectorAll('#res .row2').length")
        print(f"OK step3 tabs={tabs} rows={rows}")

        # search + click result -> jump to characters
        await eval_js(ws, "document.getElementById('globalSearch').value='huahua'; doGlobalSearch();")
        await asyncio.sleep(1)
        sr = await eval_js(ws, "document.querySelectorAll('#searchResults .sr').length")
        print(f"OK step4 search results={sr}")
        res = await eval_js(ws, "(function(){ var n=document.querySelector('#searchResults .sr');"
                                " if(!n) return 'no-sr'; n.click(); return 'clicked'; })()")
        await asyncio.sleep(2.5)
        title2 = await eval_js(ws, "document.getElementById('pageTitle').textContent")
        view2 = await eval_js(ws, "state.view")
        cards2 = await eval_js(ws, "document.querySelectorAll('#content .card').length")
        p = await shot(ws, "e2e_5_after_search_click.png")
        print(f"OK step5 {res} -> view={view2} title={title2} cards={cards2} ->", p)

        # account management + account grant tab
        await eval_js(ws, "switchView('accounts'); state.subTab=1; renderView();")
        await asyncio.sleep(1.5)
        ag = await eval_js(ws, "document.getElementById('ag_do') ? 'acc-grant-form' : 'no-acc-grant'")
        print("OK step6 account grant:", ag)

        # ===== 装备栏管理视图（分类体系） =====
        await eval_js(ws, "switchView('bag')")
        await asyncio.sleep(2)
        g1 = await eval_js(ws, "Array.from(document.querySelectorAll('#bag_g_tabs .tab')).map(t=>t.textContent).join('|')")
        s1 = await eval_js(ws, "Array.from(document.querySelectorAll('#bag_s_tabs .tab')).map(t=>t.textContent).join('|')")
        has_refresh = await eval_js(ws, "!!document.getElementById('bag_refresh') ? 'header-ok' : 'header-missing'")
        add_toggle = await eval_js(ws, "!!document.getElementById('stack_add_toggle') ? 'add-panel-ok' : 'no-add-panel'")
        p = await shot(ws, "e2e_6_bag.png")
        print(f"OK step6b bag: groups={g1} subs={s1} {has_refresh} {add_toggle} ->", p)

        # ===== 分类：切到 物品栏/材料，添加 10300000 x2（服务端自动放材料分类 + 自动堆叠） =====
        await eval_js(ws, "state.bagGroup=0; state.bagSub=2; renderView();")
        await asyncio.sleep(1.2)
        sub_title = await eval_js(ws, "document.querySelector('#content .card h3 .sub') ? document.querySelector('#content .card h3 .sub').textContent : 'no-sub'")
        await eval_js(ws, "(function(){ var t=document.getElementById('stack_add_toggle'); if(t && t.closest('#content')) t.click(); })()")
        await asyncio.sleep(0.4)
        await eval_js(ws, "document.getElementById('stack_q').value='10300000'; stackSearch();")
        await asyncio.sleep(1.5)
        res_rows = await eval_js(ws, "document.querySelectorAll('#stack_res .row2').length")
        cat_mark = await eval_js(ws, "(function(){ var n=document.querySelector('#stack_res .row2'); if(!n) return 'no-row'; var c=n.querySelector('.pill.cat'); return c ? c.textContent : 'no-cat-mark'; })()")
        await eval_js(ws, "(function(){ var n=document.querySelector('#stack_res .row2'); if(n) n.click(); })()")
        await asyncio.sleep(0.3)
        await eval_js(ws, "(function(){ var i=document.querySelector('#stack_chosen input'); if(i){ i.value=2; i.onchange(); } })()")
        await eval_js(ws, "document.getElementById('stack_do').click()")
        await asyncio.sleep(3)
        amt1 = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr[data-sec="items"]');
          return tr ? parseInt(tr.dataset.cur, 10) : -1;
        })()""")
        row_cat = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr');
          return tr ? (tr.dataset.sec + '/' + (tr.querySelector('td:nth-child(2)')||{}).textContent) : 'no-row';
        })()""")
        print(f"OK step6c cat-add: {sub_title} search_rows={res_rows} {cat_mark} grant-clicked amount_after_2={amt1} row={row_cat}")

        # 再发 3 个 -> 应合并为 5（同一格）
        await eval_js(ws, "(function(){ if(typeof stopBagAuto==='function') stopBagAuto(); state.bagAuto=false; })()")
        await eval_js(ws, "document.getElementById('stack_q').value='10300000'; stackSearch();")
        await asyncio.sleep(1.5)
        await eval_js(ws, "(function(){ var n=document.querySelector('#stack_res .row2'); if(n) n.click(); })()")
        await asyncio.sleep(0.3)
        await eval_js(ws, "(function(){ var i=document.querySelector('#stack_chosen input'); if(i){ i.value=3; i.onchange(); } })()")
        await eval_js(ws, "document.getElementById('stack_do').click()")
        await asyncio.sleep(3)
        amt2 = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr[data-sec="items"]');
          return tr ? parseInt(tr.dataset.cur, 10) : -1;
        })()""")
        rows_after = await eval_js(ws, "document.querySelectorAll('#content table.bag tbody tr[data-sec=\"items\"]').length")
        print(f"OK step6d stack-merge: amount_after_2+3={amt2} item_rows={rows_after} (expect 5 / 1 row)")

        # 减少 1 -> 4，再删除 -> 0
        await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr[data-sec="items"]');
          if(tr) tr.querySelector('.bag-dec').click();
        })()""")
        await asyncio.sleep(2.5)
        amt3 = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr[data-sec="items"]');
          return tr ? parseInt(tr.dataset.cur, 10) : -1;
        })()""")
        await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr[data-sec="items"]');
          if(tr){ tr.querySelector('.bag-num').value = 0; tr.querySelector('.bag-set').click(); }
        })()""")
        await asyncio.sleep(2.5)
        items_left = await eval_js(ws, "document.querySelectorAll('#content table.bag tbody tr[data-sec=\"items\"]').length")
        print(f"OK step6e stack-sub-del: amount_after_minus1={amt3} items_left_after_del={items_left} (expect 4 / 0)")

        # 装备分类：切到 物品栏/装备，耐久往返
        await eval_js(ws, "state.bagGroup=0; state.bagSub=0; renderView();")
        await asyncio.sleep(1.5)
        eq_rows = await eval_js(ws, "document.querySelectorAll('#content table.bag tbody tr').length")
        rt = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr');
          if(!tr) return 'no-row';
          var before = parseInt(tr.dataset.cur, 10);
          tr.querySelector('.bag-inc').click();
          return 'inc-clicked before=' + before;
        })()""")
        await asyncio.sleep(2.5)
        after_inc_raw = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr');
          return tr ? { cur: tr.dataset.cur, curInt: parseInt(tr.dataset.cur, 10) } : -1;
        })()""")
        await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr');
          if(tr) tr.querySelector('.bag-dec').click();
        })()""")
        await asyncio.sleep(2.5)
        after_dec_raw = await eval_js(ws, """(function(){
          var tr = document.querySelector('#content table.bag tbody tr');
          return tr ? { cur: tr.dataset.cur, curInt: parseInt(tr.dataset.cur, 10) } : -1;
        })()""")
        ok_rt = (isinstance(after_inc_raw, dict) and isinstance(after_dec_raw, dict)
                 and after_dec_raw.get("curInt") == after_inc_raw.get("curInt") - 1)
        print(f"OK step6f eq-cat roundtrip: {rt} rows={eq_rows} restored={ok_rt}")

        # 装扮/宠物 分类 Tab 切换
        await eval_js(ws, "state.bagGroup=1; state.bagSub=0; renderView();")
        await asyncio.sleep(0.8)
        avatar_empty = await eval_js(ws, "document.querySelector('#content .empty') ? document.querySelector('#content .empty').textContent.slice(0,30) : 'has-rows'")
        await eval_js(ws, "state.bagGroup=2; state.bagSub=0; renderView();")
        await asyncio.sleep(0.8)
        pet_empty = await eval_js(ws, "document.querySelector('#content .empty') ? document.querySelector('#content .empty').textContent.slice(0,30) : 'has-rows'")
        print(f"OK step6g cat-tabs: avatar=[{avatar_empty}] pet=[{pet_empty}]")

        # batch selection bar on equipment tab
        await eval_js(ws, "state.bagGroup=0; state.bagSub=0; renderView();")
        await asyncio.sleep(1.2)
        await eval_js(ws, """(function(){
          var p = document.querySelector('#content table.bag tbody .bagpick');
          if(p){ p.checked = true; p.onchange(); }
        })()""")
        sel = await eval_js(ws, "document.getElementById('bag_sel').textContent")
        print("OK step6h batch select:", sel)

        # settings + server views
        await eval_js(ws, "switchView('server')")
        await asyncio.sleep(0.6)
        sv = await eval_js(ws, "document.getElementById('sv_refresh') ? 'server-view' : 'no-server-view'")
        await eval_js(ws, "switchView('settings')")
        await asyncio.sleep(0.6)
        st = await eval_js(ws, "document.getElementById('copy_token') ? 'settings-view' : 'no-settings-view'")
        print(f"OK step7 {sv} {st}")

        await asyncio.sleep(0.4)

    if ERRORS:
        print("JS ERRORS DETECTED:")
        for e in ERRORS[:20]:
            print("  " + e)
    else:
        print("NO JS ERRORS")

asyncio.run(main())
