package main

// indexHTML 是 GM 工具的单页界面。全部本地资源，无外网依赖。
//
// 筛选控件的依据（联网调研后采纳的做法，详见工程报告）：
//  1. 「装备位」按腾讯 DNF 官方新手引导页列出的功能型装备位置命名与排序，
//     并补上 115 版服务端 configs/equipment-wear.current35.json 里多出的 24 号「辅助武器」；
//  2. 品级色标取官方页给出的稀有度配色（普通白/高级蓝/稀有紫/神器粉/史诗黄/传说橙/神话彩），
//     官方页之后新增的 勇者/太初 两级按色阶补色，仅作界面辨识；
//  3. 筛选遵循「部位 → 等级 → 品级 → 关键词」逐层收窄、结果条数实时回显的检索范式；
//  4. 部位是条目的一等字段（有独立的 slot= 参数与独立控件），不是从名字里猜的。
const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>DFO 115 单机服 GM 工具</title>
<style>
*{box-sizing:border-box}
body{margin:0;font:14px/1.6 "Microsoft YaHei",system-ui,sans-serif;background:#14161a;color:#e6e8ec}
header{padding:14px 20px;background:#1c2026;border-bottom:1px solid #2a3038;display:flex;align-items:center;gap:16px}
header h1{font-size:16px;margin:0;font-weight:600}
header .meta{color:#8b93a1;font-size:12px}
main{display:flex;height:calc(100vh - 53px)}
aside{width:280px;background:#181b20;border-right:1px solid #2a3038;overflow:auto;flex:none}
aside h2{font-size:12px;color:#8b93a1;margin:16px 16px 6px;letter-spacing:.08em}
.item{padding:8px 16px;cursor:pointer;border-left:3px solid transparent}
.item:hover{background:#20242b}
.item.active{background:#232a34;border-left-color:#4c8bf5}
.item .sub{color:#8b93a1;font-size:12px}
section{flex:1;overflow:auto;padding:20px}
h3{margin:0 0 12px;font-size:15px}
.card{background:#1c2026;border:1px solid #2a3038;border-radius:8px;padding:16px;margin-bottom:16px}
label{display:block;color:#8b93a1;font-size:12px;margin-bottom:4px}
input,select,textarea{width:100%;padding:8px 10px;background:#111318;border:1px solid #333a45;border-radius:6px;color:#e6e8ec;font:inherit}
input:focus,select:focus{outline:none;border-color:#4c8bf5}
button{padding:9px 16px;background:#4c8bf5;border:0;border-radius:6px;color:#fff;font:inherit;cursor:pointer}
button:hover{background:#3d7ae4}
button.ghost{background:#2a3038}
button.ghost:hover{background:#343c48}
button:disabled{opacity:.5;cursor:default}
.row{display:flex;gap:12px;flex-wrap:wrap}
.row>div{flex:1;min-width:140px}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid #242a33}
th{color:#8b93a1;font-weight:500;font-size:12px}
.kv{display:grid;grid-template-columns:110px 1fr;gap:4px 12px}
.kv div:nth-child(odd){color:#8b93a1}
.pill{display:inline-block;padding:1px 7px;border-radius:10px;background:#2a3038;font-size:12px;color:#a9b2c0;margin-right:6px}
.ok{color:#5ad07f}.err{color:#f0605d}
.hint{color:#8b93a1;font-size:12px;margin-top:8px}
.searchbox{position:relative}
.result{max-height:300px;overflow:auto;border:1px solid #2a3038;border-radius:6px;margin-top:8px}
.result .row2{display:flex;justify-content:space-between;gap:8px;padding:6px 10px;cursor:pointer;border-bottom:1px solid #20242b}
.result .row2:hover{background:#232a34}
.tag{color:#8b93a1;font-size:12px}
.kw{display:inline-block;padding:3px 9px;margin:3px 5px 3px 0;border-radius:12px;background:#1f242c;border:1px solid #2a3038;cursor:pointer;font-size:12px;color:#a9b2c0}
.kw:hover{background:#232a34}
.kw.active{background:#2c3b55;border-color:#4c8bf5;color:#cfe0fb}
.kw .n{color:#8b93a1;margin-left:4px}
.kw.active .n{color:#9dbcf0}
.kw.zero{opacity:.42}
.kw .dot{display:inline-block;width:7px;height:7px;border-radius:50%;margin-right:5px;vertical-align:1px}
.filters{margin-top:12px;padding-top:10px;border-top:1px solid #242a33}
.filters .flabel{color:#8b93a1;font-size:12px;margin:10px 0 4px;display:flex;justify-content:space-between;align-items:baseline}
.filters .flabel b{color:#c8cfda;font-weight:500}
.chosen{display:flex;gap:8px;align-items:center;padding:6px 10px;background:#232a34;border-radius:6px;margin-top:6px}
.chosen input{width:80px}
</style>
</head>
<body>
<header>
  <h1>DFO 115 单机服 · GM 工具</h1>
  <span class="meta" id="meta">载入中…</span>
</header>
<main>
  <aside>
    <h2>账号</h2>
    <div id="accounts"></div>
    <h2>角色</h2>
    <div id="characters"><div class="item sub" style="padding:8px 16px">先选一个账号</div></div>
  </aside>
  <section id="panel"><div class="card">请选择左侧的角色。</div></section>
</main>
<script>
var TOKEN = "__TOKEN__";
var state = { account:0, accountName:"", character:null, cera:0, chars:[],
  itemType:"", slot:"", levelMin:0, levelMax:0, rarity:"", filters:null };

// RARITY_COLORS 只用于界面上区分品级（0..8 与 /api/filters 的品级顺序一致）。
// 白/蓝/紫/粉/黄/橙/彩 取自腾讯官方新手引导页给出的稀有度配色；
// 勇者(5)、太初(8) 是该页之后新增的品级，官方页没有对应说明，这里按色阶补色，
// 仅作界面辨识，不代表游戏内数值。品级中文名一律来自 /api/filters。
var RARITY_COLORS = ["#cfd6e4","#6fc3ff","#b07cf5","#ff8fd0","#ffc85c","#ff9d6b","#ff8a3d","#ff5f5f","#7ef9ff"];

function api(path, opts){
  opts = opts || {};
  var url = path + (path.indexOf("?")>=0 ? "&" : "?") + "token=" + encodeURIComponent(TOKEN);
  if (opts.body) { opts.headers = {"Content-Type":"application/json"}; opts.body = JSON.stringify(opts.body); }
  return fetch(url, opts).then(function(r){
    return r.json().then(function(j){ if(!r.ok) throw new Error(j.error||("HTTP "+r.status)); return j; });
  });
}
function el(id){ return document.getElementById(id); }
function esc(s){ return String(s==null?"":s).replace(/[&<>"]/g, function(c){ return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;"}[c]; }); }

// applyURLFilters 让页面 URL 也能直接带筛选条件，例如
//   /?slot=武器&rarity=史诗&level_min=110&limit=5
//   /?slot=耳环
//   /?rarity=太初&q=短剑
// 这样"装备词典"式的查询可以收藏/转发；条件与界面控件共用同一份 state，
// 因此打开后控件选中态和结果条数都是对的。
function applyURLFilters(){
  var p = new URLSearchParams(location.search);
  state.itemType = p.get("type") || "";
  state.slot = p.get("slot") || "";
  state.rarity = p.get("rarity") || "";
  state.levelMin = parseInt(p.get("level_min")||"0",10) || 0;
  state.levelMax = parseInt(p.get("level_max")||"0",10) || 0;
  state.presetQ = p.get("q") || "";
}
applyURLFilters();

api("/api/overview").then(function(o){
  el("meta").textContent = "物品索引 " + o.items + " 件 · 目录等级上限 " + o.grade_cap + " · 环境 " + o.root;
  el("accounts").innerHTML = o.accounts.map(function(a){
    return "<div class=\"item\" data-acc=\"" + a.id + "\" data-name=\"" + esc(a.username) + "\">" + esc(a.username) + "<div class=\"sub\">账号 ID " + a.id + "</div></div>";
  }).join("") || "<div class=\"item sub\">没有账号</div>";
  Array.prototype.forEach.call(document.querySelectorAll("#accounts .item"), function(node){
    node.onclick = function(){ selectAccount(parseInt(node.dataset.acc,10), node.dataset.name); };
  });
}).catch(function(e){ el("meta").innerHTML = "<span class=\"err\">" + esc(e.message) + "</span>"; });

function selectAccount(id, name){
  state.account = id; state.accountName = name; state.character = null;
  Array.prototype.forEach.call(document.querySelectorAll("#accounts .item"), function(n){ n.classList.toggle("active", parseInt(n.dataset.acc,10)===id); });
  el("characters").innerHTML = "<div class=\"item sub\" style=\"padding:8px 16px\">载入中…</div>";
  api("/api/characters?account=" + id).then(function(o){
    state.cera = o.cera; state.chars = o.characters;
    el("characters").innerHTML = o.characters.map(function(c){
      return "<div class=\"item\" data-cid=\"" + c.id + "\">" + esc(c.name) + "<div class=\"sub\">" + esc(c.className) + " · Lv." + c.level + " · 金币 " + c.gold + "</div></div>";
    }).join("") || "<div class=\"item sub\" style=\"padding:8px 16px\">该账号还没有角色</div>";
    Array.prototype.forEach.call(document.querySelectorAll("#characters .item"), function(n){
      if(!n.dataset.cid) return;
      n.onclick = function(){ selectCharacter(parseInt(n.dataset.cid,10)); };
    });
    renderGrant();
  }).catch(function(e){ el("characters").innerHTML = "<div class=\"item err\">" + esc(e.message) + "</div>"; });
}

function selectCharacter(id){
  Array.prototype.forEach.call(document.querySelectorAll("#characters .item"), function(n){ n.classList.toggle("active", parseInt(n.dataset.cid,10)===id); });
  api("/api/character?id=" + id).then(function(o){
    state.character = o.character; state.cera = o.cera;
    renderCharacter(o); renderGrant(); loadHistory();
  }).catch(function(e){ el("panel").innerHTML = "<div class=\"card err\">" + esc(e.message) + "</div>"; });
}

function renderCharacter(o){
  var c = o.character, s = o.summary;
  var attrs = Object.keys(s.attributes||{}).map(function(k){ return "<div>" + esc(k) + "</div><div>" + s.attributes[k] + "</div>"; }).join("");
  var rows = (o.bag||[]).map(function(b){
    return "<tr><td>" + b.slot + "</td><td>" + b["template"] + "</td><td>" + esc(b.name) + "</td><td>" + b.count + (b.worn?" <span class=\"pill\">已穿戴</span>":"") + "</td></tr>";
  }).join("");
  el("panel").innerHTML =
   "<div class=\"card\"><h3>" + esc(c.name) + " <span class=\"pill\">" + esc(c.className) + "</span> <span class=\"pill\">Lv." + c.level + "</span></h3>" +
   "<div class=\"kv\">" +
     "<div>角色 ID</div><div>" + c.id + "</div>" +
     "<div>账号</div><div>" + esc(state.accountName) + " (ID " + o.account + ")</div>" +
     "<div>经验</div><div>" + c.experience + "</div>" +
     "<div>金币</div><div>" + c.gold + "</div>" +
     "<div>点券</div><div>" + o.cera + "</div>" +
     "<div>觉醒阶段</div><div>" + s.advancement + "</div>" +
     "<div>SP</div><div>" + (s.skill_points||[0,0]).join(" / ") + "</div>" +
     "<div>TP</div><div>" + (s.technique_points||[0,0]).join(" / ") + "</div>" +
     attrs +
   "</div></div>" +
   "<div class=\"card\"><h3>背包与装备</h3>" +
   (rows ? "<table><thead><tr><th>槽位</th><th>ID</th><th>名称</th><th>数量</th></tr></thead><tbody>" + rows + "</tbody></table>"
         : "<div class=\"hint\">背包是空的。</div>") + "</div>";
}

function renderGrant(){
  if(!state.account){ el("panel").innerHTML = "<div class=\"card\">请选择左侧的角色。</div>"; return; }
  var charOpts = "<option value=\"0\">仅点券（账号级，不需要角色）</option>" + state.chars.map(function(c){
    return "<option value=\"" + c.id + "\"" + (state.character && state.character.id===c.id ? " selected" : "") + ">" + esc(c.name) + " (Lv." + c.level + ")</option>";
  }).join("");
  el("panel").innerHTML =
  "<div class=\"card\"><h3>发放资产</h3>" +
    "<div class=\"row\">" +
      "<div><label>目标角色</label><select id=\"g_char\">" + charOpts + "</select></div>" +
      "<div><label>金币</label><input id=\"g_gold\" type=\"number\" min=\"0\" value=\"0\"></div>" +
      "<div><label>点券（可正可负）</label><input id=\"g_cera\" type=\"number\" value=\"0\"></div>" +
    "</div>" +
    "<div class=\"filters\">" +
      "<div class=\"flabel\"><b>物品筛选</b><span><button class=\"ghost\" id=\"clear\" style=\"padding:3px 10px;font-size:12px\">清除筛选</button></span></div>" +
      "<div class=\"flabel\"><b>类型</b><span class=\"tag\">客户端页签 + 装备</span></div>" +
      "<div id=\"types\" class=\"hint\">载入中…</div>" +
      "<div class=\"flabel\"><b>部位</b><span class=\"tag\">装备位，来自 [equipment type]</span></div>" +
      "<div id=\"slots\" class=\"hint\">载入中…</div>" +
      "<div class=\"flabel\"><b>等级</b><span class=\"tag\">装备最低等级 [minimum level]</span></div>" +
      "<div id=\"levels\" class=\"hint\">载入中…</div>" +
      "<div class=\"flabel\"><b>稀有度</b><span class=\"tag\">品级 [rarity]</span></div>" +
      "<div id=\"rarities\" class=\"hint\">载入中…</div>" +
      "<div class=\"hint\" id=\"fnote\"></div>" +
    "</div>" +
    "<div class=\"searchbox\" style=\"margin-top:12px\"><label>物品搜索（中文名 / 英文名 / ID，可与上面全部条件叠加）</label>" +
      "<input id=\"q\" placeholder=\"例如：太初之星、短剑、101001153\"><div class=\"result\" id=\"res\"></div>" +
      "<div class=\"hint\" id=\"rescount\"></div></div>" +
    "<div id=\"chosen\"></div>" +
    "<div style=\"margin-top:12px\"><label>原因（写入审计）</label><input id=\"g_reason\" value=\"GM 工具发放\"></div>" +
    "<div style=\"margin-top:14px\"><button id=\"do\">执行发放</button> <span id=\"msg\" class=\"hint\"></span></div>" +
    "<div class=\"hint\">发放走与服务器相同的背包校验和事务；背包满时会整笔拒绝，不会只发一半。角色在线上时需要重新选择角色才能看到。</div>" +
  "</div>" +
  "<div class=\"card\"><h3>最近发放记录</h3><div id=\"hist\" class=\"hint\">…</div></div>";

  var q = el("q"); var timer = null;
  if(state.presetQ) q.value = state.presetQ;
  q.oninput = function(){ clearTimeout(timer); timer = setTimeout(doSearch, 220); };
  el("do").onclick = doGrant;
  el("clear").onclick = function(){
    state.itemType = ""; state.slot = ""; state.levelMin = 0; state.levelMax = 0; state.rarity = "";
    renderTypes(); renderSlots(); renderLevels(); renderRarities(); doSearch();
  };
  loadTypes();
  loadFilters();
  doSearch();
}

// loadTypes 拉取分类列表与每类数量，渲染成可点选的类型筛选项。
// 装备类会再列出各装备位（key 可直接当 type= 用）。
function loadTypes(){
  var box = el("types"); if(!box) return;
  api("/api/types").then(function(o){
    state.types = o.types || [];
    renderTypes();
  }).catch(function(e){ box.innerHTML = "<span class=\"err\">" + esc(e.message) + "</span>"; });
}
function renderTypes(){
  var box = el("types"); if(!box || !state.types) return;
  var html = kw("type", "", "全部", "", state.itemType, null);
  state.types.forEach(function(t){ html += kw("type", t.key, t.label, t.count, state.itemType, null); });
  state.types.forEach(function(t){
    (t.children||[]).forEach(function(c){ html += kw("type", c.key, c.label, c.count, state.itemType, null, true); });
  });
  box.innerHTML = html;
  bindKws(box, "type", function(v){ state.itemType = v; renderTypes(); doSearch(); });
}

// loadFilters 拉取部位 / 等级 / 稀有度三个维度的可选值与数量。
// 这三个控件与类型、关键词互相叠加，任何一处改动都会重新查询。
function loadFilters(){
  api("/api/filters").then(function(f){
    state.filters = f;
    // URL 里给的稀有度可能是中文名（?rarity=太初），控件选中态用的是序号键，这里归一化。
    if(state.rarity && !/^[0-9]+$/.test(state.rarity)){
      var found = "";
      f.rarities.forEach(function(r){ if(r.label === state.rarity) found = r.key; });
      state.rarity = found;
    }
    renderSlots(); renderLevels(); renderRarities();
    el("fnote").innerHTML = "品级中文名来源：" + esc(f.rarity_source) +
      " · 装备 " + f.equipment + " 件：已归类到具体部位/时装 " + f.slot_covered +
      " 件，落在「其它装备」 " + f.slot_unknown + " 件；有最低等级 " + f.level_known +
      " 件，无等级信息 " + f.level_unknown + " 件（筛选等级时会被排除）";
  }).catch(function(e){ el("fnote").innerHTML = "<span class=\"err\">" + esc(e.message) + "</span>"; });
}

function renderSlots(){
  var box = el("slots"); if(!box || !state.filters) return;
  var html = kw("slot", "", "全部", state.filters.equipment, state.slot, null);
  state.filters.slots.forEach(function(s){ html += kw("slot", s.key, s.label, s.count, state.slot, null); });
  box.innerHTML = html;
  bindKws(box, "slot", function(v){ state.slot = v; renderSlots(); doSearch(); });
}

function renderLevels(){
  var box = el("levels"); if(!box || !state.filters) return;
  var activeKey = (state.levelMin||state.levelMax) ? (state.levelMin + "-" + state.levelMax) : "";
  var html = kw("lv", "", "全部", state.filters.level_known, activeKey, null);
  state.filters.levels.forEach(function(l){
    html += kw("lv", l.min + "-" + l.max, l.label, l.count, activeKey, null,
               false, l.min, l.max);
  });
  box.innerHTML = html;
  bindKws(box, "lv", function(v, n){
    if(!v){ state.levelMin = 0; state.levelMax = 0; }
    else { state.levelMin = parseInt(n.dataset.min,10)||0; state.levelMax = parseInt(n.dataset.max,10)||0; }
    renderLevels(); doSearch();
  });
}

function renderRarities(){
  var box = el("rarities"); if(!box || !state.filters) return;
  var total = 0;
  state.filters.rarities.forEach(function(r){ total += r.count; });
  var html = kw("rarity", "", "全部", total, state.rarity, null);
  state.filters.rarities.forEach(function(r){
    html += kw("rarity", r.key, r.label, r.count, state.rarity, RARITY_COLORS[parseInt(r.key,10)]);
  });
  box.innerHTML = html;
  bindKws(box, "rarity", function(v){ state.rarity = v; renderRarities(); doSearch(); });
}

// kw 渲染一个可点选的筛选标签。activeVal 与 key 相等时为选中态。
// color 非空时在标签前画一个品级色点；min/max 用于等级档。
function kw(attr, key, label, count, activeVal, color, sub, min, max){
  var on = String(activeVal||"") === String(key);
  var attrs = " data-" + attr + "=\"" + esc(key) + "\"";
  if(min !== undefined){ attrs += " data-min=\"" + min + "\""; }
  if(max !== undefined){ attrs += " data-max=\"" + max + "\""; }
  var dot = color ? "<span class=\"dot\" style=\"background:" + color + "\"></span>" : "";
  var n = (count === "" || count === undefined || count === null) ? "" : "<span class=\"n\">" + count + "</span>";
  var cls = "kw" + (on ? " active" : "") + (sub ? " sub" : "") + (count === 0 ? " zero" : "");
  return "<span class=\"" + cls + "\"" + attrs + ">" + dot + esc(label) + n + "</span>";
}
function bindKws(box, attr, onPick){
  Array.prototype.forEach.call(box.querySelectorAll(".kw"), function(n){
    n.onclick = function(){ onPick(n.dataset[attr] || "", n); };
  });
}

// slotTag 返回物品的部位标签（装备）；堆叠物显示客户端页签分类。
function slotTag(it){ return it.slot || it.type_label || (it.kind==="equipment" ? "装备" : "普通"); }

var chosen = [];
function doSearch(){
  var q = el("q"); if(!q) return;
  var url = "/api/items?q=" + encodeURIComponent(q.value) +
    "&type=" + encodeURIComponent(state.itemType) +
    "&slot=" + encodeURIComponent(state.slot) +
    "&level_min=" + (state.levelMin||"") +
    "&level_max=" + (state.levelMax||"") +
    "&rarity=" + encodeURIComponent(state.rarity) +
    "&limit=60";
  api(url).then(function(o){
    el("rescount").textContent = "命中 " + (o.count||0) + " 条" +
      ((o.count||0) >= 60 ? "（只显示前 60 条，可用关键词缩小范围）" : "");
    el("res").innerHTML = o.items.map(function(it){
      var zh = it.name || it.name_en || "(未命名)";
      var color = RARITY_COLORS[it.rarity] || "#e6e8ec";
      var en = (it.name_en && it.name_en !== it.name) ? "<span class=\"tag\">" + esc(it.name_en) + "</span>" : "";
      var parts = ["ID " + it.id];
      if(it.level) parts.push("Lv" + it.level);
      if(it.rarity_label) parts.push(it.rarity_label);
      if(it.grade) parts.push("品级值 " + it.grade);
      return "<div class=\"row2\" data-id=\"" + it.id + "\" data-name=\"" + esc(zh) + "\">" +
        "<span><span style=\"color:" + color + "\">" + esc(zh) + "</span> " + en +
        "<div class=\"tag\">" + esc(parts.join(" · ")) + "</div></span>" +
        "<span class=\"tag\"><span class=\"pill\">" + esc(slotTag(it)) + "</span></span></div>";
    }).join("") || "<div class=\"row2\">没有匹配的物品</div>";
    Array.prototype.forEach.call(el("res").querySelectorAll(".row2"), function(n){
      if(!n.dataset.id) return;
      n.onclick = function(){ addChosen(parseInt(n.dataset.id,10), n.dataset.name); };
    });
  }).catch(function(e){ el("rescount").innerHTML = "<span class=\"err\">" + esc(e.message) + "</span>"; });
}
function addChosen(id, name){
  for(var i=0;i<chosen.length;i++) if(chosen[i].id===id) { chosen[i].amount++; renderChosen(); return; }
  chosen.push({id:id, name:name, amount:1}); renderChosen();
}
function renderChosen(){
  var box = el("chosen"); if(!box) return;
  box.innerHTML = chosen.map(function(c,i){
    return "<div class=\"chosen\"><span style=\"flex:1\">" + esc(c.name) + " <span class=\"tag\">ID " + c.id + "</span></span>" +
      "<input type=\"number\" min=\"1\" value=\"" + c.amount + "\" data-i=\"" + i + "\">" +
      "<button class=\"ghost\" data-del=\"" + i + "\">移除</button></div>";
  }).join("");
  Array.prototype.forEach.call(box.querySelectorAll("input"), function(inp){
    inp.onchange = function(){ chosen[parseInt(inp.dataset.i,10)].amount = Math.max(1, parseInt(inp.value,10)||1); };
  });
  Array.prototype.forEach.call(box.querySelectorAll("button"), function(b){
    b.onclick = function(){ chosen.splice(parseInt(b.dataset.del,10),1); renderChosen(); };
  });
}

function doGrant(){
  var g = el("g_char"), gold = el("g_gold"), cera = el("g_cera"), reason = el("g_reason"), msg = el("msg");
  var body = {
    account: state.account,
    character: parseInt(g.value,10) || 0,
    gold: parseInt(gold.value,10) || 0,
    cera: parseInt(cera.value,10) || 0,
    reason: reason.value || "GM 工具发放",
    items: chosen.map(function(c){ return {template:c.id, amount:c.amount}; })
  };
  if(!body.character && (body.gold || body.items.length)){ msg.innerHTML = "<span class=\"err\">金币和物品必须指定角色。</span>"; return; }
  if(!body.gold && !body.cera && !body.items.length){ msg.innerHTML = "<span class=\"err\">没有可发放的内容。</span>"; return; }
  el("do").disabled = true; msg.textContent = "提交中…";
  api("/api/grant", {method:"POST", body:body}).then(function(o){
    msg.innerHTML = "<span class=\"ok\">" + esc(o.message) + "</span> " + esc(o.note||"");
    chosen = []; renderChosen();
    if(state.character) selectCharacter(state.character.id); else { loadHistory(); }
    if(state.account) selectAccount(state.account, state.accountName);
  }).catch(function(e){ msg.innerHTML = "<span class=\"err\">" + esc(e.message) + "</span>"; })
    .then(function(){ el("do").disabled = false; });
}

function loadHistory(){
  if(!state.account) return;
  api("/api/history?account=" + state.account).then(function(o){
    var h = el("hist"); if(!h) return;
    h.innerHTML = (o.history||[]).map(function(r){
      return "<div>" + esc(r.at) + " · " + esc(r.grant_id) + " · " + esc(r.operator) + " · " + esc(r.reason) + "</div>";
    }).join("") || "还没有发放记录。";
  });
}
</script>
</body>
</html>
`
