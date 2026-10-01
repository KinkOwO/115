# -*- coding: utf-8 -*-
"""DFO 115us GM 管理台代理服务器。

在 127.0.0.1:28081 提供一个 Dashboard 布局的 GM 管理界面：
  - GET  /          -> 返回 dashboard/index.html（注入后端 token）
  - GET|POST /api/* -> 转发到 gmweb (127.0.0.1:28080)，并注入后端 token

不修改 gmweb 二进制；gmweb 的所有 API（账号/角色/改等级/技能点/背包/任务/
物品发放/记录）原样可用。
"""

import http.server
import json as _json
import os
import re
import socket
import subprocess
import sys
import threading
import urllib.error
import urllib.parse
import urllib.request

BACKEND = "http://127.0.0.1:28080"
PORT = 28081
PAGE = None          # 页面内容（首次请求时读取）
PAGE_MTIME = None    # 页面文件修改时间（热更新检测）
PAGE_PATH = None
TOKEN = None
TOKEN_LOCK = threading.Lock()
STACKABLE_IDS = set()   # 后端实际发放目录中的可堆叠模板
CATALOG_METADATA_LOADED = None

# ============ 物品分类体系（装备栏管理：物品栏/装扮/宠物） ============
# 数据源：
#   gmweb /api/catalog-metadata  后端准备好的源类型与可堆叠集合
#   旧程序404时兼容本地items.index/equipment/loot JSON
#   gmweb /api/items      懒加载补查（时装等不在本地文件中的装备）
INDEX_MAP = {}       # id(str) -> (kind, stack_type)
EQ_MAP = {}          # id(str) -> equipment type
CAT_CACHE = {}       # id(str) -> (group, sub)
CAT_LOCK = threading.Lock()

# stack_type -> (group, sub)
STACK_CATEGORY = {
    "[material]": ("物品栏", "材料"),
    "[enchant waste]": ("物品栏", "材料"),
    "[quest]": ("物品栏", "任务"),
    "[achievement item]": ("物品栏", "任务"),
    "[material expert job]": ("物品栏", "副职业"),
    "[recipe]": ("物品栏", "副职业"),
    "[dungeon and life recipe]": ("物品栏", "副职业"),
    "[avatar emblem]": ("装扮", "徽章"),
    "[feed]": ("宠物", "消耗品"),      # 宠物饲料
    "[creature]": ("宠物", "消耗品"),  # 宠物相关堆叠物（换名卡等）
}
# equipment type -> (group, sub)
EQ_CATEGORY = {
    "[creature]": ("宠物", "宠物"),
    "[artifact red]": ("宠物", "宠物装备"),
    "[artifact blue]": ("宠物", "宠物装备"),
    "[artifact green]": ("宠物", "宠物装备"),
}
UNKNOWN_CAT = ("物品栏", "装备")

# ============ 邮件系统 / 仓库发放（代理直连数据库，psql 子进程） ============
PG = None          # dict: host/port/user/password/db
PSQL = r"D:\115us\tools\pg\pgsql\bin\psql.exe"
STORAGE_JSON = r"D:\115us\server\work\dfo-lan\runtime\storage\local.json"


def load_pg_config():
    """从 storage/local.json 读取 postgres_dsn 并解析连接信息。"""
    global PG
    try:
        with open(STORAGE_JSON, "r", encoding="utf-8") as f:
            cfg = _json.load(f)
        dsn = cfg.get("postgres_dsn", "")
        u = urllib.parse.urlparse(dsn)
        PG = {
            "host": u.hostname or "127.0.0.1",
            "port": u.port or 5432,
            "user": urllib.parse.unquote(u.username or ""),
            "password": urllib.parse.unquote(u.password or ""),
            "db": (u.path.lstrip("/") or "").split("?")[0],
        }
        print("数据库配置已加载：%s:%s/%s" % (PG["host"], PG["port"], PG["db"]))
    except Exception as exc:
        print("加载数据库配置失败：", exc)


def psql(sql, timeout=30):
    """执行一条 psql 查询（SQL 走 stdin，UTF-8 编码，避免 Windows 参数编码损坏中文）。"""
    if not PG:
        raise RuntimeError("数据库配置未加载")
    env = dict(os.environ)
    env["PGPASSWORD"] = PG["password"]
    env["PGCLIENTENCODING"] = "UTF8"
    cmd = [PSQL, "-h", PG["host"], "-p", str(PG["port"]), "-U", PG["user"],
           "-d", PG["db"], "-t", "-A"]
    r = subprocess.run(cmd, input=sql, capture_output=True, text=True,
                       encoding="utf-8", errors="replace", env=env, timeout=timeout)
    if r.returncode != 0:
        raise RuntimeError("psql: " + (r.stderr or "").strip()[:400])
    return r.stdout.strip()


def esc_sql(s):
    """SQL 字符串转义（单引号翻倍）。"""
    return str(s).replace("'", "''")


def ensure_mail_table():
    try:
        psql("CREATE TABLE IF NOT EXISTS gm_mail ("
             "id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,"
             "to_account_id bigint NOT NULL, to_character_id bigint NOT NULL DEFAULT 0,"
             "template bigint NOT NULL CHECK (template > 0),"
             "amount bigint NOT NULL CHECK (amount > 0 AND amount <= 4294967295),"
             "title text NOT NULL DEFAULT '', body text NOT NULL DEFAULT '',"
             "status text NOT NULL DEFAULT 'unread', claimed_at timestamptz, expires_at timestamptz,"
             "created_at timestamptz NOT NULL DEFAULT now(),"
             "CONSTRAINT gm_mail_status_check CHECK (status IN ('unread','read','claimed','expired','revoked')))")
        psql("CREATE INDEX IF NOT EXISTS gm_mail_to_account_idx ON gm_mail(to_account_id, status)")
        psql("CREATE INDEX IF NOT EXISTS gm_mail_to_char_idx ON gm_mail(to_character_id, status)")
        return True
    except Exception as exc:
        print("确保邮件表失败：", exc)
        return False


# ---- 物品发送限制规则（阶段 1：背包/仓库/邮箱可发判定） ----
# 不可直发背包：不在服务端发放目录（grantable=False）
# 不可发邮箱：虚拟类/仅效果类/合同类（无实体，邮箱无意义）
MAIL_BANNED_STACK_TYPES = {"[virtual]", "[only effect]", "[contract]", "[achievement item]"}


def item_restrictions(it):
    """根据 /api/items 单条记录，返回各目标的发送限制。"""
    rules = {"bag": True, "vault": True, "mail": True, "reasons": {}}
    st = it.get("stack_type") or ""
    if not it.get("grantable", True):
        rules["bag"] = False
        rules["reasons"]["bag"] = "不在服务端发放目录（equipment absent from source）"
    if st in MAIL_BANNED_STACK_TYPES:
        rules["mail"] = False
        rules["reasons"]["mail"] = "虚拟/仅效果/合同类物品不可邮寄"
    if st == "[virtual]":
        rules["vault"] = False
        rules["reasons"]["vault"] = "虚拟物品不可放入仓库"
    return rules


def mail_send(payload):
    acc = int(payload.get("to_account_id") or 0)
    ch = int(payload.get("to_character_id") or 0)
    tmpl = int(payload.get("template") or 0)
    amt = int(payload.get("amount") or 0)
    title = payload.get("title") or "GM 邮件"
    body = payload.get("body") or ""
    if acc <= 0 or tmpl <= 0 or amt <= 0:
        raise RuntimeError("to_account_id/template/amount 必须为正数")
    # 校验收件账号存在
    exists = psql("SELECT 1 FROM accounts WHERE id=%d" % acc)
    if not exists:
        raise RuntimeError("收件账号 %d 不存在" % acc)
    if ch > 0:
        exists_c = psql("SELECT 1 FROM characters WHERE id=%d AND account_id=%d" % (ch, acc))
        if not exists_c:
            raise RuntimeError("收件角色 %d 不存在或不属于账号 %d" % (ch, acc))
    sql = ("INSERT INTO gm_mail(to_account_id,to_character_id,template,amount,title,body) "
           "VALUES (%d,%d,%d,%d,'%s','%s') RETURNING id"
           % (acc, ch, tmpl, amt, esc_sql(title), esc_sql(body)))
    rid = psql(sql)
    lines = [ln.strip() for ln in rid.splitlines() if ln.strip()]
    mail_id = int(lines[0]) if lines else 0
    return {"mail_id": mail_id, "to_account_id": acc, "to_character_id": ch,
            "template": tmpl, "amount": amt, "message": "邮件已发送（管理台邮件系统）"}


def mail_list(payload):
    acc = int(payload.get("account") or 0)
    status = payload.get("status") or ""
    where = []
    if acc > 0:
        where.append("to_account_id=%d" % acc)
    if status:
        where.append("status='%s'" % esc_sql(status))
    sql = "SELECT id,to_account_id,to_character_id,template,amount,title,status,created_at FROM gm_mail"
    if where:
        sql += " WHERE " + " AND ".join(where)
    sql += " ORDER BY id DESC LIMIT 200"
    out = psql(sql)
    rows = []
    for line in out.splitlines():
        if not line:
            continue
        parts = line.split("|")
        if len(parts) < 8:
            continue
        rows.append({"id": int(parts[0]), "to_account_id": int(parts[1]),
                     "to_character_id": int(parts[2]), "template": int(parts[3]),
                     "amount": int(parts[4]), "title": parts[5], "status": parts[6],
                     "created_at": parts[7]})
    return {"count": len(rows), "mails": rows}


def mail_revoke(payload):
    mid = int(payload.get("id") or 0)
    if mid <= 0:
        raise RuntimeError("缺少邮件 id")
    out = psql("UPDATE gm_mail SET status='revoked' WHERE id=%d AND status IN ('unread','read') RETURNING id" % mid)
    if not out:
        raise RuntimeError("邮件 %d 不存在或不可撤销" % mid)
    return {"mail_id": mid, "message": "邮件已撤销"}


def vault_send(payload):
    """直写个人仓库 character_vaults（items 为 BagItem 数组：{slot,Template,Amount}）。"""
    acc = int(payload.get("account") or 0)
    ch = int(payload.get("character") or 0)
    tmpl = int(payload.get("template") or 0)
    amt = int(payload.get("amount") or 0)
    if ch <= 0 or tmpl <= 0 or amt <= 0:
        raise RuntimeError("account/character/template/amount 必须为正数")
    # 读仓库
    out = psql("SELECT slots, items FROM character_vaults WHERE character_id=%d" % ch)
    if not out:
        raise RuntimeError("角色 %d 没有个人仓库（未初始化）" % ch)
    slots_s, items_raw = out.split("|", 1)
    slots = int(slots_s)
    items = _json.loads(items_raw) if items_raw.strip() else []
    merged = False
    for row in items:
        if int(row.get("Template") or row.get("template") or 0) == tmpl:
            row["Amount"] = int(row.get("Amount") or row.get("amount") or 0) + amt
            merged = True
            break
    if not merged:
        used = {int(r.get("slot") or 0) for r in items}
        new_slot = None
        for s in range(slots):
            if s not in used:
                new_slot = s
                break
        if new_slot is None:
            raise RuntimeError("仓库已满（%d 格）" % slots)
        items.append({"slot": new_slot, "Template": tmpl, "Amount": amt})
    new_json = _json.dumps(items, ensure_ascii=False)
    psql("UPDATE character_vaults SET items='%s'::jsonb, updated_at=now() WHERE character_id=%d"
         % (esc_sql(new_json), ch))
    return {"character_id": ch, "template": tmpl, "amount": amt,
            "merged": merged, "slots": slots, "message": "已发放到个人仓库"}


def load_catalog_metadata():
    """从后端只读目录取得分类；只有旧程序明确404时使用兼容JSON。"""
    global CATALOG_METADATA_LOADED, INDEX_MAP, EQ_MAP, STACKABLE_IDS
    if CATALOG_METADATA_LOADED is not None:
        return CATALOG_METADATA_LOADED
    url = BACKEND + "/api/catalog-metadata?token=" + urllib.parse.quote(TOKEN or "")
    try:
        with urllib.request.urlopen(url, timeout=30) as response:
            data = _json.load(response)
    except urllib.error.HTTPError as exc:
        if exc.code == 404:
            CATALOG_METADATA_LOADED = False
            return False
        raise
    # Validate the complete response before replacing classification maps.
    items = data["items"]
    stackables = data["stackables"]
    if not isinstance(items, dict) or not isinstance(stackables, list):
        raise ValueError("后端目录元数据格式无效")
    index, equipment = {}, {}
    for key, row in items.items():
        if not isinstance(row, list) or len(row) != 2 or row[0] not in ("stackable", "equipment"):
            raise ValueError("后端物品分类记录无效")
        index[str(key)] = (row[0], row[1] if row[0] == "stackable" else None)
        if row[0] == "equipment" and row[1]:
            equipment[str(key)] = row[1]
    INDEX_MAP, EQ_MAP = index, equipment
    STACKABLE_IDS = {str(key) for key in stackables}
    CATALOG_METADATA_LOADED = True
    print("后端源目录：分类 %d 条，可堆叠 %d 种，source %s" % (len(index), len(STACKABLE_IDS), data.get("source", "")))
    return True


def load_category_maps():
    """启动时加载本地分类映射（index + equipment），全部 id 覆盖。"""
    global INDEX_MAP, EQ_MAP
    if load_catalog_metadata():
        return
    try:
        base = __import__("pathlib").Path(__file__).resolve().parent
        idx_path = base.parent / "configs" / "items.index.json"
        eq_path = base.parent / "configs" / "equipment.current37.json"
        idx = _json.load(open(idx_path, "r", encoding="utf-8"))
        for it in idx.get("items", []):
            INDEX_MAP[str(it.get("id"))] = (it.get("kind"), it.get("stack_type"))
        eq = _json.load(open(eq_path, "r", encoding="utf-8"))
        for r in eq.get("rows", []):
            f = r.get("Fields", {}) or {}
            et = f.get("[equipment type]")
            txt = None
            if et:
                for seg in et:
                    if isinstance(seg, dict) and seg.get("text"):
                        txt = seg["text"]
                        break
            if txt:
                EQ_MAP[str(r.get("ID"))] = txt
        print("分类映射：index %d 条，equipment %d 条" % (len(INDEX_MAP), len(EQ_MAP)))
    except Exception as exc:
        print("载入分类映射失败：", exc)


def classify_api_item(it):
    """根据 gmweb /api/items 单条记录分类。"""
    g = it.get("group")
    if g == "时装":
        return ("装扮", "装扮")
    t = it.get("type") or ""
    tk = it.get("type_key") or ""
    if t == "[creature]":
        return ("宠物", "宠物") if it.get("kind") == "equipment" else ("宠物", "消耗品")
    if tk == "emblem":
        return ("装扮", "徽章")
    if tk == "consumable":
        return ("物品栏", "消耗品")
    if tk == "material":
        return ("物品栏", "材料")
    if tk == "profession":
        return ("物品栏", "副职业")
    if tk == "quest":
        return ("物品栏", "任务")
    if t in ("[artifact red]", "[artifact blue]", "[artifact green]"):
        return ("宠物", "宠物装备")
    if it.get("kind") == "equipment":
        return ("物品栏", "装备")
    return ("物品栏", "消耗品")


def api_lookup_category(key):
    """对本地无法判定的 id（时装等），调 gmweb /api/items 按精确 id 补查分类。"""
    try:
        url = BACKEND + "/api/items?q=" + urllib.parse.quote(key) + "&limit=20&token=" + urllib.parse.quote(TOKEN or "")
        with urllib.request.urlopen(url, timeout=15) as r:
            d = _json.loads(r.read().decode("utf-8"))
        for it in d.get("items", []):
            if str(it.get("id")) == key:
                return classify_api_item(it)
    except Exception:
        pass
    return None


def categorize(tid):
    """返回 (group, sub)；线程安全 + 缓存。"""
    key = str(tid)
    if key in CAT_CACHE:
        return CAT_CACHE[key]
    with CAT_LOCK:
        if key in CAT_CACHE:
            return CAT_CACHE[key]
        cat = _categorize_uncached(key)
        CAT_CACHE[key] = cat
        return cat


def _categorize_uncached(key):
    idx = INDEX_MAP.get(key)
    kind = idx[0] if idx else None
    st = idx[1] if idx else None
    if kind == "stackable":
        return STACK_CATEGORY.get(st, ("物品栏", "消耗品"))
    if kind == "equipment":
        eq = EQ_MAP.get(key)
        if eq:
            return EQ_CATEGORY.get(eq, ("物品栏", "装备"))
        # 本地未覆盖的装备（时装等）→ API 精确补查
        return api_lookup_category(key) or ("物品栏", "装备")
    # index 未知 id → 补查；查不到归未分类
    return api_lookup_category(key) or ("其它", "其它")


def load_stackable_ids():
    """读取服务端 loot.next25.json，得到可堆叠物品 id 集合（这些物品发放时
    服务端会放进背包 items 分区并自动堆叠合并）。"""
    global STACKABLE_IDS
    if load_catalog_metadata():
        return
    try:
        base = __import__("pathlib").Path(__file__).resolve().parent
        loot_path = base.parent / "configs" / "loot.next25.json"
        with open(loot_path, "r", encoding="utf-8") as f:
            loot = _json.load(f)
        items = loot.get("items", {}) or {}
        STACKABLE_IDS = {str(k) for k in items.keys()}
        print("已载入可堆叠物品 %d 种" % len(STACKABLE_IDS))
    except Exception as exc:
        print("载入 loot 目录失败：", exc)


def fetch_token():
    global TOKEN
    try:
        with urllib.request.urlopen(BACKEND + "/", timeout=5) as r:
            html = r.read().decode("utf-8", "replace")
        m = re.search(r'var TOKEN = "([0-9a-f]+)"', html)
        if m:
            TOKEN = m.group(1)
            return True
    except Exception as exc:
        print("获取后端 token 失败：", exc)
    return False


def load_page():
    """读取页面文件；文件变更（mtime 变化）时自动重载，便于热更新。"""
    global PAGE, PAGE_MTIME
    try:
        st = os.stat(PAGE_PATH)
        mtime = st.st_mtime
        if PAGE is None or mtime != PAGE_MTIME:
            with open(PAGE_PATH, "r", encoding="utf-8") as f:
                PAGE = f.read()
            PAGE_MTIME = mtime
    except Exception as exc:
        print("读取页面失败：", exc)
        if PAGE is None:
            PAGE = "<html><body><h1>页面缺失：" + str(exc) + "</h1></body></html>"


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    # ---- 页面 ----
    def _serve_page(self):
        load_page()  # 内部按 mtime 检测，文件变更自动重载（热更新）
        html = PAGE.replace("__TOKEN__", TOKEN or "")
        body = html.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    # ---- API 代理 ----
    def _proxy(self, method, _retried=False):
        if not TOKEN:
            fetch_token()
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
        params = [(k, v) for k, v in params if k != "token"]
        params.append(("token", TOKEN or ""))
        url = BACKEND + parsed.path + "?" + urllib.parse.urlencode(params)
        data = None
        headers = {}
        if method == "POST":
            length = int(self.headers.get("Content-Length", 0) or 0)
            data = self.rfile.read(length) if length > 0 else None
            ct = self.headers.get("Content-Type", "")
            if ct:
                headers["Content-Type"] = ct
        req = urllib.request.Request(url, data=data, method=method, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                body = resp.read()
                ct = resp.headers.get("Content-Type", "application/json; charset=utf-8")
                # 增强 JSON 响应：/api/items 注入 stackable + category；/api/character 的 bag 行注入 category
                if method == "GET" and parsed.path.startswith("/api/items") and STACKABLE_IDS:
                    try:
                        data = _json.loads(body.decode("utf-8"))
                        for it in data.get("items", []):
                            it["stackable"] = str(it.get("id")) in STACKABLE_IDS
                            g, s = categorize(it.get("id"))
                            it["category"] = {"group": g, "sub": s}
                            it["restrictions"] = item_restrictions(it)
                        body = _json.dumps(data, ensure_ascii=False).encode("utf-8")
                    except Exception:
                        pass
                elif method == "GET" and parsed.path.startswith("/api/character"):
                    try:
                        data = _json.loads(body.decode("utf-8"))
                        for row in data.get("bag", []):
                            g, s = categorize(row.get("template"))
                            row["category"] = {"group": g, "sub": s}
                        body = _json.dumps(data, ensure_ascii=False).encode("utf-8")
                    except Exception:
                        pass
                self.send_response(resp.status)
                self.send_header("Content-Type", ct)
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(body)
        except urllib.error.HTTPError as exc:
            # token 失效（gmweb 重启后换新 token）：刷新一次并重试
            if exc.code == 401 and not _retried:
                if fetch_token():
                    print("token 已刷新，重试请求：", parsed.path)
                    return self._proxy(method, _retried=True)
            body = exc.read()
            ct = exc.headers.get("Content-Type", "application/json; charset=utf-8")
            self.send_response(exc.code)
            self.send_header("Content-Type", ct)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        except Exception as exc:
            body = ('{"error":"代理转发失败：%s"}' % str(exc)).encode("utf-8")
            self.send_response(502)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    def do_GET(self):
        path = urllib.parse.urlparse(self.path).path
        if path == "/api/mail/list":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = mail_list(payload)
                self._json_ok(data)
            except Exception as exc:
                self._json_err(str(exc))
        elif path.startswith("/api/"):
            self._proxy("GET")
        elif path in ("/", "/index.html"):
            self._serve_page()
        else:
            self.send_error(404)

    def do_POST(self):
        path = urllib.parse.urlparse(self.path).path
        if path in ("/api/mail/send", "/api/vault/send", "/api/mail/revoke"):
            length = int(self.headers.get("Content-Length", 0) or 0)
            raw = self.rfile.read(length) if length > 0 else b"{}"
            try:
                payload = _json.loads(raw.decode("utf-8"))
            except Exception:
                self._json_err("请求体不是合法 JSON")
                return
            try:
                if path == "/api/mail/send":
                    self._json_ok(mail_send(payload))
                elif path == "/api/vault/send":
                    self._json_ok(vault_send(payload))
                else:
                    self._json_ok(mail_revoke(payload))
            except Exception as exc:
                self._json_err(str(exc))
        elif path.startswith("/api/"):
            self._proxy("POST")
        else:
            self.send_error(404)

    def _json_ok(self, data):
        body = _json.dumps({"ok": True, **data}, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def _json_err(self, msg):
        body = _json.dumps({"ok": False, "error": msg}, ensure_ascii=False).encode("utf-8")
        self.send_response(400)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def port_open(port):
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.4):
            return True
    except OSError:
        return False


def main():
    global PAGE_PATH
    here = __file__
    if getattr(sys, "frozen", False):
        here = sys.executable
    base = __import__("pathlib").Path(here).resolve().parent
    PAGE_PATH = str(base / "index.html")

    if not port_open(28080):
        print("错误：gmweb 未运行（127.0.0.1:28080 无响应）。请先启动 GM 工具（Start-GMWeb.cmd）。")
        sys.exit(2)
    if not fetch_token():
        print("错误：无法从 gmweb 获取 token。")
        sys.exit(3)
    load_stackable_ids()
    load_category_maps()
    load_pg_config()
    ensure_mail_table()

    try:
        srv = http.server.ThreadingHTTPServer(("127.0.0.1", PORT), Handler)
    except OSError as exc:
        print("端口 %d 已被占用：%s（可能管理台已在运行，直接访问 http://127.0.0.1:%d/ 即可）" % (PORT, exc, PORT))
        sys.exit(0)
    print("DFO 115us GM 管理台：http://127.0.0.1:%d/  （后端 %s，token %s…）" % (PORT, BACKEND, TOKEN[:8]))
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
