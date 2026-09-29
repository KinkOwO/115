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
import pathlib

BACKEND = "http://127.0.0.1:28080"
PORT = 28081
PAGE = None          # 页面内容（首次请求时读取）
PAGE_MTIME = None    # 页面文件修改时间（热更新检测）
PAGE_PATH = None
TOKEN = None
TOKEN_LOCK = threading.Lock()
STACKABLE_IDS = set()   # 服务端 loot 目录中可堆叠的物品 id（发放走堆叠路径）

# ============ 物品分类体系（装备栏管理：物品栏/装扮/宠物） ============
# 数据源：
#   items.index.json      id -> (kind, stack_type)      全量 386K
#   equipment.current37   id -> [equipment type]        装备 67K（含宠物/宠物装备）
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

# ============ 时装套装联动发放（GM-SET-01） ============
AVATAR_SETS = {}          # job_name -> {job_index, groups: [...]}
AVATAR_DETAIL_CACHE = {}  # (job, group_index, set_index) -> 详情缓存
SET_NAME_CACHE = {}       # (job, group_index, set_index) -> 套装名
AVATAR_NAME_MAP = {}      # item_id(str) -> 中文名（从 names.client.json）
AVATAR_LOCK = threading.Lock()

# ============ 装备套装联动发放（EQUIP-GRANT-01） ============
EQUIP_SETS = []           # 装备套装列表
EQUIP_SET_MAP = {}        # set_id -> set_data
EQUIP_LOCK = threading.Lock()
EQUIP_WHITELIST = set()   # 装备白名单（从 equip_whitelist.json 加载）
SET_DISPLAY_NAMES = {}    # 套装显示名映射（从 set_display_names.json 加载）

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


def state_skill_points(payload):
    # 写入角色技能点 SP/TP。直接 UPDATE characters.state 的 skill_points / technique_points。
    cid = int(payload.get("character") or payload.get("character_id") or 0)
    sp = payload.get("skill_points") or [0, 0]
    tp = payload.get("technique_points") or [0, 0]
    recalc = bool(payload.get("recalc_skill_points"))
    restore_locked = bool(payload.get("restore_locked"))
    if not cid:
        raise RuntimeError("缺少角色 ID")
    try:
        sp = [int(sp[0]), int(sp[1])]
        tp = [int(tp[0]), int(tp[1])]
    except Exception:
        raise RuntimeError("skill_points / technique_points 必须是两个整数")
    if recalc:
        sp = [9999, 9999]
    sp_json = "[%d,%d]" % (sp[0], sp[1])
    tp_json = "[%d,%d]" % (tp[0], tp[1])
    psql("CREATE TABLE IF NOT EXISTS gm_state_backup_skill AS SELECT id, state, now() AS backed_up_at FROM characters WHERE id=%d AND NOT EXISTS (SELECT 1 FROM gm_state_backup_skill WHERE id=%d)" % (cid, cid))
    if restore_locked:
        # 把 GM 锁定的 SP/TP 加回到当前值
        row = psql("SELECT state->'gm_locked_sp' ->> 0, state->'gm_locked_sp' ->> 1, state->'gm_locked_tp' ->> 0, state->'gm_locked_tp' ->> 1 FROM characters WHERE id=%d" % cid)
        parts = [x.strip() for x in row.split("|")] if row else ["0","0","0","0"]
        parts = [int(x) if x and x != "" else 0 for x in parts]
        sp = [sp[0] + parts[0], sp[1] + parts[1]]
        tp = [tp[0] + parts[2], tp[1] + parts[3]]
        sp_json = "[%d,%d]" % (sp[0], sp[1])
        tp_json = "[%d,%d]" % (tp[0], tp[1])
    psql("UPDATE characters SET state = jsonb_set(jsonb_set(state, '{skill_points}'::text[], '%s'::jsonb), '{technique_points}'::text[], '%s'::jsonb) WHERE id=%d" % (sp_json, tp_json, cid))
    # 存 GM 锁定值（GM 写入时记录，游戏内初始化后可恢复）
    psql("UPDATE characters SET state = jsonb_set(jsonb_set(state, '{gm_locked_sp}'::text[], '%s'::jsonb), '{gm_locked_tp}'::text[], '%s'::jsonb) WHERE id=%d" % (sp_json, tp_json, cid))
    reset_learned = bool(payload.get("reset_learned"))
    reset_msg = ""
    if reset_learned:
        psql("UPDATE characters SET state = jsonb_set(jsonb_set(jsonb_set(state, '{learned_skills}'::text[], '[null,null]'::jsonb), '{skill_slots}'::text[], '[null,null]'::jsonb), '{skill_variations}'::text[], '[{},{}]'::jsonb) WHERE id=%d" % cid)
        reset_msg = "，已清空已学技能（SP 全额返还）"
    return {"message": "技能点已写入：SP " + sp_json + "，TP " + tp_json + reset_msg, "locked_sp": sp_json, "locked_tp": tp_json}


def clone_account(payload):
    # 克隆账号：复制 accounts 行 + 复制所有角色（含 state 完整数据）
    src_acc = int(payload.get("account") or payload.get("account_id") or 0)
    new_name = (payload.get("new_username") or "").strip()
    if not src_acc:
        raise RuntimeError("缺少源账号 ID")
    if not new_name:
        raise RuntimeError("缺少新账号名")
    # 检查新名是否已存在
    exist = psql("SELECT 1 FROM accounts WHERE username='%s' LIMIT 1" % new_name.replace("'", "''"))
    if exist:
        raise RuntimeError("账号名 %s 已存在" % new_name)
    # 读源账号
    row = psql("SELECT password_hash, development_only FROM accounts WHERE id=%d" % src_acc)
    if not row:
        raise RuntimeError("源账号 %d 不存在" % src_acc)
    parts = row.split("|")
    pwd_hash = parts[0] if len(parts) > 0 else ""
    dev_raw = (parts[1] if len(parts) > 1 else "t").strip()
    dev_only = "true" if dev_raw in ("t", "true", "1") else "false"
    # INSERT 新账号
    psql("INSERT INTO accounts (username, password_hash, development_only, created_at) VALUES ('%s', '%s', %s, now())" % (
        new_name.replace("'", "''"), pwd_hash.replace("'", "''"), dev_only))
    new_acc = int(psql("SELECT id FROM accounts WHERE username='%s' ORDER BY id DESC LIMIT 1" % new_name.replace("'", "''")))
    # 复制角色
    chars = psql("SELECT id FROM characters WHERE account_id=%d AND deleted_at IS NULL" % src_acc)
    char_count = 0
    if chars:
        for line in chars.strip().split(chr(10)):
            line = line.strip()
            if not line:
                continue
            src_char = int(line)
            # 复制角色（state 完整 JSONB 复制）
            psql("INSERT INTO characters (account_id, wire_id, name, profession, create_request, config_version, state, created_at) "
                 "SELECT %d, wire_id, name, profession, create_request, config_version, state, now() "
                 "FROM characters WHERE id=%d" % (new_acc, src_char))
            char_count += 1
    return {"message": "克隆成功：新账号 " + new_name + "（ID " + str(new_acc) + "），复制 " + str(char_count) + " 个角色",
            "new_account_id": new_acc, "character_count": char_count}


def clone_character(payload):
    # 克隆角色：复制 characters 行（新 id），state 完整 JSONB 复制
    src_char = int(payload.get("character") or payload.get("character_id") or 0)
    new_name = (payload.get("new_name") or "").strip()
    target_account = int(payload.get("target_account") or 0)
    if not src_char:
        raise RuntimeError("缺少源角色 ID")
    if not new_name:
        raise RuntimeError("缺少新角色名")
    # 检查新名是否已存在（同账号）
    row = psql("SELECT account_id, wire_id, name, profession, create_request, config_version, state FROM characters WHERE id=%d AND deleted_at IS NULL" % src_char)
    if not row:
        raise RuntimeError("源角色 %d 不存在" % src_char)
    parts = row.split("|")
    src_account_id = int(parts[0])
    if target_account <= 0:
        target_account = src_account_id
    # 检查新名冲突
    exist = psql("SELECT 1 FROM characters WHERE account_id=%d AND name='%s' AND deleted_at IS NULL LIMIT 1" % (target_account, new_name.replace("'", "''")))
    if exist:
        raise RuntimeError("账号下已有同名角色 %s" % new_name)
    # INSERT 新角色（state 完整复制）
    psql("INSERT INTO characters (account_id, wire_id, name, profession, create_request, config_version, state, created_at) "
         "SELECT %d, wire_id, '%s', profession, create_request, config_version, state, now() "
         "FROM characters WHERE id=%d" % (target_account, new_name.replace("'", "''"), src_char))
    new_id = int(psql("SELECT id FROM characters WHERE account_id=%d AND name='%s' ORDER BY id DESC LIMIT 1" % (target_account, new_name.replace("'", "''"))))
    return {"message": "克隆成功：新角色 " + new_name + "（ID " + str(new_id) + "），数据完整复制",
            "new_character_id": new_id}


def delete_account(payload):
    # 删除账号：先软删所有角色，再删账号
    acc = int(payload.get("account") or payload.get("account_id") or 0)
    if not acc:
        raise RuntimeError("缺少账号 ID")
    if acc == 1:
        raise RuntimeError("不能删除主账号 (ID=1)")
    # 软删角色
    psql("UPDATE characters SET deleted_at=now() WHERE account_id=%d AND deleted_at IS NULL" % acc)
    # 删账号
    psql("DELETE FROM accounts WHERE id=%d" % acc)
    return {"message": "账号 " + str(acc) + " 已删除（含其下所有角色）"}


def delete_character(payload):
    # 软删角色
    char_id = int(payload.get("character") or payload.get("character_id") or 0)
    if not char_id:
        raise RuntimeError("缺少角色 ID")
    psql("UPDATE characters SET deleted_at=now() WHERE id=%d" % char_id)
    return {"message": "角色 " + str(char_id) + " 已删除"}


def grant_assets(payload):
    # 发放金币到角色 state.inventory.gold；点券转发 gmweb
    char_id = int(payload.get("character") or 0)
    gold = int(payload.get("gold") or 0)
    cera = int(payload.get("cera") or 0)
    msgs = []
    if gold:
        if not char_id:
            raise RuntimeError("发放金币必须选择角色")
        # 读当前金币，加 gold，写回
        cur = psql("SELECT state->'inventory'->>'gold' FROM characters WHERE id=%d" % char_id)
        cur_gold = int(cur.strip()) if cur and cur.strip().isdigit() else 0
        new_gold = cur_gold + gold
        psql("UPDATE characters SET state = jsonb_set(state, '{inventory,gold}', '%s'::jsonb) WHERE id=%d" % (new_gold, char_id))
        msgs.append("金币 +%d（当前 %d）" % (gold, new_gold))
    if cera:
        # 直接写 account_currency 表
        acc = int(payload.get("account") or 0)
        if not acc:
            raise RuntimeError("发放点券必须选择账号")
        # 读当前点券
        cur = psql("SELECT cera FROM account_currency WHERE account_id=%d" % acc)
        cur_cera = int(cur.strip()) if cur and cur.strip().isdigit() else 0
        new_cera = cur_cera + cera
        if new_cera < 0:
            raise RuntimeError("点券余额不足（当前 %d，不能扣成负数）" % cur_cera)
        # UPSERT
        psql("INSERT INTO account_currency (account_id, cera) VALUES (%d, %d) ON CONFLICT (account_id) DO UPDATE SET cera=%d, updated_at=now()" % (acc, new_cera, new_cera))
        msgs.append("点券 %+d（当前 %d）" % (cera, new_cera))
    return {"message": "，".join(msgs) if msgs else "没有发放内容"}


def change_job(payload):
    # 改角色转职：profession（基础职业）+ advancement（转职阶段）
    char_id = int(payload.get("character") or 0)
    profession = payload.get("profession")
    advancement = payload.get("advancement")
    if not char_id:
        raise RuntimeError("缺少角色 ID")
    sets = []
    if profession is not None and profession != "":
        p = int(profession)
        psql("UPDATE characters SET profession=%d WHERE id=%d" % (p, char_id))
        sets.append("profession=%d" % p)
    if advancement is not None and advancement != "":
        a = int(advancement)
        psql("UPDATE characters SET state = jsonb_set(state, '{advancement}', '%d'::jsonb) WHERE id=%d" % (a, char_id))
        sets.append("advancement=%d" % a)
    if not sets:
        raise RuntimeError("没有要改的字段")
    return {"message": "转职已更新：" + "，".join(sets)}



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


def rename_character(payload):
    import re
    cid = int(payload.get("character_id") or payload.get("id") or 0)
    name = (payload.get("name") or "").strip()
    if not cid:
        raise RuntimeError("缺少角色 ID")
    if not name:
        raise RuntimeError("名字不能为空")
    if len(name) > 32:
        raise RuntimeError("名字最多 32 个字符")
    if not re.match(r"^[\u4e00-\u9fa5a-zA-Z0-9]+$", name):
        raise RuntimeError("名字只能用中文、英文、数字，不能含符号或空格")
    psql("UPDATE characters SET name='%s' WHERE id=%d" % (esc_sql(name), cid))
    return {"message": "改名成功 -> " + name, "name": name}


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



# ============ 任务设为可提交 + 补发物品（GM 工具扩展） ============

QUESTS_JSON = None


def load_quests_catalog():
    """启动时加载 quests.generated.json，供任务所需物品查询。"""
    global QUESTS_JSON
    try:
        base = pathlib.Path(__file__).resolve().parent
        qpath = base.parent.parent / "server" / "work" / "dfo-lan" / "configs" / "quests.generated.json"
        if not qpath.exists():
            qpath = base.parent / "configs" / "quests.generated.json"
        with open(qpath, "r", encoding="utf-8") as f:
            QUESTS_JSON = _json.load(f)
        quests = QUESTS_JSON.get("quests", QUESTS_JSON)
        print("[OK] quests catalog loaded: %d entries from %s" % (len(quests), qpath), flush=True)
    except Exception as exc:
        print("[ERROR] quests catalog load failed: %s" % exc, flush=True)
        import traceback; traceback.print_exc()


def quest_objective_items(qid):
    """从 quests.generated.json 读取任务的所需物品列表。
    [seeking] 类型的 objective_cells 是成对的 {type:0,value:item_id},{type:0,value:amount}。
    """
    if not QUESTS_JSON:
        return []
    quests = QUESTS_JSON.get("quests", QUESTS_JSON)
    q = quests.get(str(qid)) or quests.get(qid)
    if not q:
        return []
    cells = q.get("objective_cells") or []
    items = []
    i = 0
    while i + 1 < len(cells):
        tid = cells[i].get("value")
        amt = cells[i + 1].get("value")
        if tid and amt and tid > 0 and amt > 0:
            items.append((int(tid), int(amt)))
        i += 2
    return items


def grant_to_bag(character_id, template, amount):
    """把物品直接写入 characters.state 的 inventory.items 数组。"""
    out = psql("SELECT state FROM characters WHERE id=%d" % character_id)
    if not out:
        raise RuntimeError("角色 %d 不存在" % character_id)
    state = _json.loads(out)
    inv = state.get("inventory")
    if not isinstance(inv, dict):
        inv = {"version": "ordinary-bag-v1", "items": [], "equipment": [], "worn": []}
        state["inventory"] = inv
    items = inv.get("items")
    if not isinstance(items, list):
        items = []
        inv["items"] = items
    merged = False
    for row in items:
        tmpl = int(row.get("Template") or row.get("template") or 0)
        if tmpl == template:
            old = int(row.get("Amount") or row.get("amount") or 0)
            row["Amount"] = old + amount
            merged = True
            break
    if not merged:
        used = {int(r.get("slot") or -1) for r in items}
        new_slot = None
        for s in range(200):
            if s not in used:
                new_slot = s
                break
        if new_slot is None:
            raise RuntimeError("背包已满，无法放入 template=%d" % template)
        items.append({"slot": new_slot, "Template": template, "Amount": amount})
    new_json = _json.dumps(state, ensure_ascii=False)
    psql("UPDATE characters SET state='%s'::jsonb WHERE id=%d"
         % (esc_sql(new_json), character_id))
    return {"merged": merged}


def remove_equip(payload):
    """从 inventory 的 equipment/worn/items 里删除指定 slot 的装备。"""
    character_id = int(payload.get("character_id") or 0)
    section = payload.get("section") or "equipment"
    slots = payload.get("slots") or []
    if character_id <= 0:
        raise RuntimeError("缺少 character_id")
    if section not in ("equipment", "worn", "items"):
        raise RuntimeError("section 只能是 equipment/worn/items")
    slot_set = {int(s) for s in slots}
    out = psql("SELECT state FROM characters WHERE id=%d" % character_id)
    if not out:
        raise RuntimeError("角色 %d 不存在" % character_id)
    state = _json.loads(out)
    inv = state.get("inventory") or {}
    arr = inv.get(section) or []
    before = len(arr)
    arr = [r for r in arr if int(r.get("slot") or -1) not in slot_set]
    inv[section] = arr
    state["inventory"] = inv
    new_json = _json.dumps(state, ensure_ascii=False)
    psql("UPDATE characters SET state='%s'::jsonb WHERE id=%d"
         % (esc_sql(new_json), character_id))
    return {"removed": before - len(arr), "section": section, "slots": sorted(slot_set)}


def enhance_equip(payload):
    """直接改装备 record 字节，设置强化/增幅/锻造等级。"""
    import base64
    character_id = int(payload.get("character_id") or 0)
    slot = int(payload.get("slot") or -1)
    enchant_upgrade = int(payload.get("enchant_upgrade") or 0)
    amplify_type = int(payload.get("amplify_type") or 0)
    amplify_value = int(payload.get("amplify_value") or 0)
    genuine_upgrade = int(payload.get("genuine_upgrade") or 0)
    section = payload.get("section") or "equipment"
    if character_id <= 0 or slot < 0:
        raise RuntimeError("缺少 character_id 或 slot")
    out = psql("SELECT state FROM characters WHERE id=%d" % character_id)
    if not out:
        raise RuntimeError("角色 %d 不存在" % character_id)
    state = _json.loads(out)
    inv = state.get("inventory") or {}
    arr = inv.get(section) or []
    found = False
    for item in arr:
        if int(item.get("slot") or -1) == slot:
            # 构造 181 字节 record
            # AI-FIX: 直接写 enchant_upgrade 字段到 DB，服务端会填到 record [16:18]
            item["enchant_upgrade"] = enchant_upgrade
            item["amplify_type"] = amplify_type
            item["amplify_value"] = amplify_value
            item["genuine_upgrade"] = genuine_upgrade
            found = True
            break
    if not found:
        raise RuntimeError("槽位 %d 没有装备" % slot)
    inv[section] = arr
    state["inventory"] = inv
    new_json = _json.dumps(state, ensure_ascii=False)
    psql("UPDATE characters SET state='%s'::jsonb WHERE id=%d"
         % (esc_sql(new_json), character_id))
    return {"ok": True, "slot": slot, "enchant_upgrade": enchant_upgrade,
            "amplify_type": amplify_type, "amplify_value": amplify_value,
            "genuine_upgrade": genuine_upgrade}


def quest_ready(payload):
    """把已接取任务的 progress 改为 0（可提交），可选补发物品到背包。"""
    acc = int(payload.get("account") or 0)
    ch = int(payload.get("character") or 0)
    quest_ids = payload.get("quest_ids") or []
    grant = bool(payload.get("grant_items"))
    if ch <= 0:
        raise RuntimeError("character 必须为正数")
    if not quest_ids:
        raise RuntimeError("quest_ids 不能为空")
    exists = psql("SELECT 1 FROM characters WHERE id=%d AND account_id=%d" % (ch, acc))
    if not exists:
        raise RuntimeError("角色 %d 不属于账号 %d" % (ch, acc))
    ready = []
    skipped = []
    for qid in quest_ids:
        qid = int(qid)
        out = psql(
            "UPDATE character_quests SET progress=0 "
            "WHERE character_id=%d AND quest_id=%d AND status='accepted' AND progress<>0 "
            "RETURNING quest_id" % (ch, qid))
        if out.strip():
            ready.append(qid)
        else:
            skipped.append(qid)
    granted_items = []
    if grant and ready:
        for qid in ready:
            for tid, amt in quest_objective_items(qid):
                result = grant_to_bag(ch, tid, amt)
                granted_items.append({"quest_id": qid, "template": tid,
                                      "amount": amt, "merged": result.get("merged")})
    return {"character_id": ch, "ready": ready, "skipped": skipped,
            "granted_items": granted_items,
            "message": "已设为可提交 %d 条；跳过 %d 条%s"
                       % (len(ready), len(skipped),
                          ("；补发物品 %d 件" % len(granted_items)) if grant else "")}




# ============ 任务列表（DB + catalog 合并） ============

ITEM_NAME_MAP = {}  # item_id(str) -> name


def load_item_names():
    """启动时加载 items.index.json 的物品名映射。"""
    global ITEM_NAME_MAP
    try:
        base = pathlib.Path(__file__).resolve().parent
        idx_path = base.parent / "configs" / "items.index.json"
        with open(idx_path, "r", encoding="utf-8") as f:
            data = _json.load(f)
        for it in data.get("items", []):
            ITEM_NAME_MAP[str(it.get("id"))] = it.get("name", "")
        print("[OK] item names loaded: %d entries from %s" % (len(ITEM_NAME_MAP), idx_path), flush=True)
    except Exception as exc:
        print("[ERROR] item names load failed: %s" % exc, flush=True)
        import traceback; traceback.print_exc()


def item_name(tid):
    """根据 item_id 返回英文名（缺失则空）。"""
    return ITEM_NAME_MAP.get(str(tid), "")


def clean_set_name(name):
    """清理套装名末尾的款式后缀，支持多种括号和格式。"""
    import re
    name = name.strip()
    # 匹配：半角方括号/中文方括号/全角圆括号/半角圆括号 + A-D型/a-d型/类型A-D
    patterns = [
        r'\[[A-Da-d]型\]',       # [A型] [a型]
        r'【[A-Da-d]型】',       # 【A型】
        r'\([A-Da-d]型\)',      # (A型)
        r'（[A-Da-d]型）',       # （A型）
        r'\[类型[A-Da-d]\]',     # [类型A]
        r'【类型[A-Da-d]】',     # 【类型A】
        r'\s*$',                # 末尾空格
    ]
    for p in patterns:
        name = re.sub(p, '', name)
    return name.strip()


def load_avatar_sets():
    """加载时装套装数据 + 物品中文名映射。"""
    global AVATAR_SETS, AVATAR_NAME_MAP
    try:
        base = pathlib.Path(__file__).resolve().parent

        # 1. 加载物品中文名映射（从 names.client.json）
        names_path = base.parent / "configs" / "names.client.json"
        with open(names_path, "r", encoding="utf-8") as f:
            names_data = _json.load(f)
        for k, v in names_data.items():
            if k.startswith("name_"):
                item_id = k[5:]  # 去掉 "name_" 前缀
                AVATAR_NAME_MAP[item_id] = v
        print("[OK] avatar names loaded: %d entries" % len(AVATAR_NAME_MAP), flush=True)

        # 2. 加载时装套装数据
        avatar_path = base.parent / "configs" / "avatar_sets.json"
        with open(avatar_path, "r", encoding="utf-8") as f:
            AVATAR_SETS = _json.load(f)

        # 3. 预加载所有套装名
        set_count = 0
        for job_name, job_data in AVATAR_SETS.items():
            for group in job_data.get("groups", []):
                gidx = group.get("group_index", 0)
                for s in group.get("sets", []):
                    sidx = s.get("set_index", 0)
                    types = s.get("types", [])
                    if types:
                        first_coat_id = str(types[0].get("items", {}).get("coat", 0))
                        coat_name = AVATAR_NAME_MAP.get(first_coat_id, "")
                        # 清理套装名后缀
                        set_name = clean_set_name(coat_name)
                        SET_NAME_CACHE[(job_name, gidx, sidx)] = set_name
                    set_count += 1

        print("[OK] avatar sets loaded: %d jobs, %d sets" % (len(AVATAR_SETS), set_count), flush=True)
    except Exception as exc:
        print("[ERROR] avatar sets load failed: %s" % exc, flush=True)


def load_equip_sets():
    """加载装备套装数据。"""
    global EQUIP_SETS, EQUIP_SET_MAP
    try:
        base = pathlib.Path(__file__).resolve().parent
        equip_path = base.parent / "configs" / "set_items.json"
        with open(equip_path, "r", encoding="utf-8") as f:
            EQUIP_SETS = _json.load(f)
        # 建立 set_id 索引
        EQUIP_SET_MAP = {}
        for s in EQUIP_SETS:
            sid = s.get("set_id")
            if sid:
                EQUIP_SET_MAP[sid] = s
        print("[OK] equip sets loaded: %d sets" % len(EQUIP_SETS), flush=True)
    except Exception as exc:
        print("[ERROR] equip sets load failed: %s" % exc, flush=True)


def load_equip_whitelist():
    """加载装备白名单（从 equip_whitelist.json）。"""
    global EQUIP_WHITELIST
    try:
        base = pathlib.Path(__file__).resolve().parent
        whitelist_path = base.parent / "configs" / "equip_whitelist.json"
        with open(whitelist_path, "r", encoding="utf-8") as f:
            ids = _json.load(f)
        EQUIP_WHITELIST = set(str(i) for i in ids)
        print("[OK] equip whitelist loaded: %d items" % len(EQUIP_WHITELIST), flush=True)
    except Exception as exc:
        print("[ERROR] equip whitelist load failed: %s" % exc, flush=True)


def load_set_display_names():
    """加载套装显示名映射表。"""
    global SET_DISPLAY_NAMES
    try:
        base = pathlib.Path(__file__).resolve().parent
        path = base.parent / "configs" / "set_display_names.json"
        with open(path, "r", encoding="utf-8") as f:
            data = _json.load(f)
        SET_DISPLAY_NAMES = data.get("mappings", {})
        print("[OK] set display names loaded: %d sets" % len(SET_DISPLAY_NAMES), flush=True)
    except Exception as exc:
        print("[WARN] set display names load failed: %s" % exc, flush=True)
        SET_DISPLAY_NAMES = {}


def avatar_sets_list(payload):
    """GET /api/avatar-sets?job=xxx"""
    job = payload.get("job", "")

    if not job:
        # 返回所有职业列表
        jobs = []
        for job_name, job_data in AVATAR_SETS.items():
            set_count = sum(len(g.get("sets", [])) for g in job_data.get("groups", []))
            jobs.append({
                "job": job_name,
                "job_index": job_data.get("job_index", 0),
                "set_count": set_count
            })
        return {"jobs": jobs, "total": len(jobs)}

    # 返回指定职业的套装列表
    job_data = AVATAR_SETS.get(job)
    if not job_data:
        raise RuntimeError("职业 %s 不存在" % job)

    sets = []
    for group in job_data.get("groups", []):
        group_index = group.get("group_index", 0)
        for s in group.get("sets", []):
            set_index = s.get("set_index", 0)
            key = (job, group_index, set_index)
            set_name = SET_NAME_CACHE.get(key, "")
            types = s.get("types", [])
            sets.append({
                "job": job,
                "group_index": group_index,
                "set_index": set_index,
                "set_name": set_name,
                "type_count": len(types)
            })

    return {"sets": sets, "total": len(sets)}


def avatar_set_detail(payload):
    """GET /api/avatar-set-detail?job=xxx&group_index=xxx&set_index=xxx"""
    job = payload.get("job", "")
    group_index = int(payload.get("group_index", 0))
    set_index = int(payload.get("set_index", 0))

    cache_key = (job, group_index, set_index)
    with AVATAR_LOCK:
        if cache_key in AVATAR_DETAIL_CACHE:
            return AVATAR_DETAIL_CACHE[cache_key]

    job_data = AVATAR_SETS.get(job)
    if not job_data:
        raise RuntimeError("职业 %s 不存在" % job)

    # 找指定 set
    target_set = None
    for group in job_data.get("groups", []):
        if group.get("group_index") != group_index:
            continue
        for s in group.get("sets", []):
            if s.get("set_index") == set_index:
                target_set = s
                break
        if target_set:
            break

    if not target_set:
        raise RuntimeError("set_index %d 不存在" % set_index)

    # 套装名
    set_name = SET_NAME_CACHE.get(cache_key, "")

    # 构建 types 明细
    parts_order = ["hat", "hair", "face", "neck", "coat", "pants", "belt", "shoes"]
    types = []

    for t in target_set.get("types", []):
        type_index = t.get("type_index", 0)
        items_raw = t.get("items", {})

        # 从本地映射查中文名
        items = []
        for part in parts_order:
            item_id = items_raw.get(part, 0)
            if not item_id:
                continue

            name_cn = AVATAR_NAME_MAP.get(str(item_id), "")
            grantable = True  # 时装默认可发放

            items.append({
                "part": part,
                "id": item_id,
                "name_cn": name_cn,
                "grantable": grantable
            })

        # type label
        label_map = {1: "A型", 2: "B型", 3: "C型", 4: "D型"}
        label = label_map.get(type_index, "类型%d" % type_index)

        types.append({
            "type_index": type_index,
            "label": label,
            "items": items
        })

    result = {
        "job": job,
        "group_index": group_index,
        "set_index": set_index,
        "set_name": set_name,
        "types": types,
        "count": len(types)
    }

    with AVATAR_LOCK:
        AVATAR_DETAIL_CACHE[cache_key] = result

    return result

def avatar_grant(payload):
    """POST /api/avatar/grant - 发放整套时装到装扮槽（special_equipment["1"]）"""
    character_id = int(payload.get("character_id") or 0)
    job = payload.get("job", "")
    group_index = int(payload.get("group_index") or 0)
    set_index = int(payload.get("set_index") or 0)
    type_index = int(payload.get("type_index") or 0)

    if character_id <= 0:
        raise RuntimeError("character_id 必须为正数")
    if not job:
        raise RuntimeError("job 不能为空")

    # 1. 从 avatar_sets.json 读 8 个部位 ID
    job_data = AVATAR_SETS.get(job)
    if not job_data:
        raise RuntimeError("职业 %s 不存在" % job)

    target_set = None
    for group in job_data.get("groups", []):
        if group.get("group_index") != group_index:
            continue
        for s in group.get("sets", []):
            if s.get("set_index") == set_index:
                target_set = s
                break
        if target_set:
            break

    if not target_set:
        raise RuntimeError("set_index %d 不存在" % set_index)

    target_type = None
    for t in target_set.get("types", []):
        if t.get("type_index") == type_index:
            target_type = t
            break

    if not target_type:
        raise RuntimeError("type_index %d 不存在" % type_index)

    items_raw = target_type.get("items", {})

    # 2. 部位 → slot 映射表
    part_slot_map = {
        "hat": 0,
        "hair": 1,
        "face": 2,
        "coat": 3,
        "pants": 4,
        "shoes": 5,
        "belt": 7,
        "neck": 6,
    }

    # 构建要写入的时装列表
    new_items = []
    for part, slot in part_slot_map.items():
        item_id = items_raw.get(part, 0)
        if not item_id:
            continue
        new_items.append({
            "slot": slot,
            "template": item_id,
            "durability": 0,
            "period": 0,
        })

    if not new_items:
        raise RuntimeError("该款式没有可发放的部位")

    # 3. 读角色当前 state
    out = psql("SELECT state FROM characters WHERE id=%d" % character_id)
    if not out:
        raise RuntimeError("角色 %d 不存在" % character_id)
    state = _json.loads(out)

    # 4. 备份 state 到 .ai_backup（固定文件名，覆盖旧备份）
    backup_dir = r"D:\115us\gm-tool\.ai_backup\AVATAR-SERVER-01"
    if not os.path.exists(backup_dir):
        os.makedirs(backup_dir)
    backup_file = os.path.join(backup_dir, "state_backup_latest.json")
    with open(backup_file, "w", encoding="utf-8") as f:
        _json.dump(state, f, ensure_ascii=False, indent=2)

    # 5. 解析 inventory 和 special_equipment
    inv = state.get("inventory")
    if not isinstance(inv, dict):
        inv = {"version": "ordinary-bag-v1", "items": [], "equipment": [], "worn": []}
        state["inventory"] = inv

    special = inv.get("special_equipment")
    if not isinstance(special, dict):
        special = {}
        inv["special_equipment"] = special

    # 6. 往 special_equipment["1"] 追加（同 slot 替换）
    avatar_list = special.get("1")
    if not isinstance(avatar_list, list):
        avatar_list = []
        special["1"] = avatar_list

    # 清理所有 0-11 的 avatar slot（防止旧数据残留）
    AVATAR_SLOTS = set(range(0, 12))
    avatar_list = [item for item in avatar_list if int(item.get("slot", -1)) not in AVATAR_SLOTS]

    # 追加新时装
    avatar_list.extend(new_items)
    special["1"] = avatar_list

    # 7. 写回 DB
    new_json = _json.dumps(state, ensure_ascii=False)
    psql("UPDATE characters SET state='%s'::jsonb WHERE id=%d"
         % (esc_sql(new_json), character_id))

    set_name = SET_NAME_CACHE.get((job, group_index, set_index), "")
    label_map = {1: "A型", 2: "B型", 3: "C型", 4: "D型"}
    type_label = label_map.get(type_index, "类型%d" % type_index)

    return {
        "granted": len(new_items),
        "set_name": set_name,
        "type_label": type_label,
        "message": "发放成功，请退到角色选择界面重新进入游戏查看"
    }


def extract_equip_set_name(set_data):
    """从装备套装数据反查套装名。"""
    coat_items = set_data.get("parts", {}).get("coat", [])
    if not coat_items:
        for part, items in set_data.get("parts", {}).items():
            coat_items = items
            break
    if not coat_items:
        return "套装 %d" % set_data["set_id"]

    first_name = coat_items[0].get("name", "")
    if not first_name:
        return "套装 %d" % set_data["set_id"]

    # 去掉前缀
    prefixes = ["精·", "守 : ", "勇 : ", "护 : ", "魂 : ", "神 : ", "魔 : "]
    for p in prefixes:
        if first_name.startswith(p):
            first_name = first_name[len(p):]

    # 去掉后缀
    suffixes = [
        "辅助装备", "魔法石", "胸铠甲",
        "胸甲", "护肩", "护腿", "腰带", "战靴", "短靴", "皮靴", "长靴",
        "项链", "手镯", "戒指", "耳环", "臂章", "肩甲", "胫甲", "腿甲",
        "长袍", "长裤", "上衣", "下装", "斗篷", "披风", "绑腿",
        "胸铠", "腿铠", "肩铠",
    ]
    suffixes = sorted(set(suffixes), key=len, reverse=True)
    for s in suffixes:
        if first_name.endswith(s):
            first_name = first_name[:-len(s)]
            break

    return first_name if first_name else "套装 %d" % set_data["set_id"]

def get_set_display_info(set_data):
    """获取套装显示名和别名，优先用映射表。"""
    set_id = str(set_data.get("set_id", ""))
    mapping = SET_DISPLAY_NAMES.get(set_id)
    if mapping:
        aliases = list(mapping.get("aliases", []))
        original = set_data.get("name", "")
        if original and original not in aliases:
            aliases.append(original)
        return {"name": mapping["display_name"], "aliases": aliases}
    return {
        "name": set_data.get("name", "套装 #%d" % set_data.get("set_id", 0)),
        "aliases": []
    }



def equip_sets_list(payload):
    """GET /api/equip-sets/list - 装备套装列表"""
    job = payload.get("job", "")
    armor = payload.get("armor", "")
    filter_type = payload.get("filter", "all")

    result = []
    for s in EQUIP_SETS:
        s_job = s.get("job", "")
        s_armor = s.get("armor_type", "")
        is_exclusive = s_job != ""

        # 职业筛选：选了职业就显示该职业 + 通用
        if job:
            if is_exclusive and s_job != job:
                continue
        # 甲种筛选
        if armor and s_armor != armor:
            continue
        # 类型筛选
        if filter_type == "exclusive" and not is_exclusive:
            continue
        if filter_type == "common" and is_exclusive:
            continue

        # 统计装备数
        item_count = 0
        for part, items in s.get("parts", {}).items():
            item_count += len(items)

        _display = get_set_display_info(s)
        result.append({
            "set_id": s["set_id"],
            "name": _display["name"],
            "aliases": _display["aliases"],
            "armor_type": s_armor,
            "job": s_job,
            "is_exclusive": is_exclusive,
            "item_count": item_count,
        })
    return {"sets": result}


def equip_set_detail(payload):
    set_id = int(payload.get("set_id") or 0)
    if set_id <= 0:
        raise RuntimeError("set_id 必须为正数")

    s = EQUIP_SET_MAP.get(set_id)
    if not s:
        raise RuntimeError("套装 %d 不存在" % set_id)

    parts = s.get("parts", {})
    s_job = s.get("job", "")

    _display = get_set_display_info(s)
    return {
        "set_id": set_id,
        "name": _display["name"],
        "aliases": _display["aliases"],
        "armor_type": s.get("armor_type", ""),
        "job": s_job,
        "is_exclusive": s_job != "",
        "parts": parts,
    }
def equip_grant(payload):
    """POST /api/equip-sets/grant - 发放装备套装到背包"""
    character_id = int(payload.get("character_id") or 0)
    set_id = int(payload.get("set_id") or 0)
    version = payload.get("version", "0")
    categories = payload.get("categories", [])

    if character_id <= 0:
        raise RuntimeError("character_id 必须为正数")
    if set_id <= 0:
        raise RuntimeError("set_id 必须为正数")
    if not categories:
        raise RuntimeError("请至少选择一个发放类别")

    # 版本号容错
    try:
        version_idx = int(version)
    except (ValueError, TypeError):
        version_idx = 0

    # 1. 读套装数据
    s = EQUIP_SET_MAP.get(set_id)
    if not s:
        raise RuntimeError("套装 %d 不存在" % set_id)

    parts = s.get("parts", {})

    # 2. 按类别筛选部位
    target_parts = set()
    cat_map = {
        "armor": ["coat", "pants", "shoulder", "belt", "shoes"],
        "accessory": ["neck", "wrist", "ring", "earring"],
        "special": ["support", "magicstone"],
    }
    # 武器部位
    for p in parts.keys():
        if p.startswith("weapon_"):
            cat_map["weapon"] = cat_map.get("weapon", []) + [p]

    for cat in categories:
        target_parts.update(cat_map.get(cat, []))

    # 3. 收集要发放的装备 ID（按版本）
    new_items = []
    for part in target_parts:
        items = parts.get(part, [])
        if not items:
            continue
        idx = version_idx
        if idx >= len(items):
            idx = len(items) - 1  # 回退到最高版本
        item = items[idx]
        if not item.get("id") or int(item["id"]) == 0:
            continue
        new_items.append({
            "template": int(item["id"]),
            "durability": 45,
        })

    if not new_items:
        raise RuntimeError("该类别没有可发放的装备")

    # AI-FIX: 发放目标分支 bag/vault/mail
    target = (payload.get("target") or "bag").lower()
    if target == "mail":
        raise RuntimeError("邮箱系统服务端未修复，暂不可用")
    if target == "vault":
        out = psql("SELECT slots, items FROM character_vaults WHERE character_id=%d" % character_id)
        if not out:
            raise RuntimeError("角色 %d 没有个人仓库（未初始化）" % character_id)
        slots_s, items_raw = out.split("|", 1)
        slots = int(slots_s)
        items = _json.loads(items_raw) if items_raw.strip() else []
        used = {int(r.get("slot", 0) or 0) for r in items}
        written = 0
        for it in new_items:
            new_slot = None
            for sl in range(slots):
                if sl not in used:
                    new_slot = sl
                    used.add(sl)
                    break
            if new_slot is None:
                raise RuntimeError("仓库已满（%d 格）" % slots)
            items.append({"slot": new_slot, "Template": it["template"], "Amount": 1})
            written += 1
        new_json = _json.dumps(items, ensure_ascii=False)
        psql("UPDATE character_vaults SET items='%s'::jsonb, updated_at=now() WHERE character_id=%d"
             % (esc_sql(new_json), character_id))
        return {
            "ok": True,
            "granted": written,
            "set_name": get_set_display_info(s)["name"],
            "target": "vault",
            "message": "已发放到个人仓库，请重新进入游戏查看"
        }
    # target == "bag" 走下方背包逻辑

    # 4. 读角色当前 state
    out = psql("SELECT state FROM characters WHERE id=%d" % character_id)
    if not out:
        raise RuntimeError("角色 %d 不存在" % character_id)
    state = _json.loads(out)

    # 5. 备份 state
    backup_dir = r"D:\115us\gm-tool\.ai_backup\EQUIP-GRANT-01"
    if not os.path.exists(backup_dir):
        os.makedirs(backup_dir)
    backup_file = os.path.join(backup_dir, "equip_state_backup_latest.json")
    with open(backup_file, "w", encoding="utf-8") as f:
        _json.dump(state, f, ensure_ascii=False, indent=2)

    # 6. 找空槽位（9-64）
    inv = state.get("inventory")
    if not isinstance(inv, dict):
        inv = {"version": "ordinary-bag-v1", "items": [], "equipment": [], "worn": []}
        state["inventory"] = inv

    equipment = inv.get("equipment", [])
    occupied_slots = set()
    for item in equipment:
        occupied_slots.add(int(item.get("slot", -1)))

    available_slots = []
    for slot in range(9, 65):
        if slot not in occupied_slots:
            available_slots.append(slot)
            if len(available_slots) == len(new_items):
                break

    if len(available_slots) < len(new_items):
        raise RuntimeError(
            "背包槽位不足：需要 %d 个，当前空 %d 个" %
            (len(new_items), len(available_slots))
        )

    # 7. 写入装备
    for i, item in enumerate(new_items):
        equipment.append({
            "slot": available_slots[i],
            "template": item["template"],
            "durability": item["durability"],
        })
    inv["equipment"] = equipment

    # 8. 写回 DB
    new_json = _json.dumps(state, ensure_ascii=False)
    sql = "UPDATE characters SET state='%s'::jsonb WHERE id=%d" % (esc_sql(new_json), character_id)
    psql(sql)

    set_name = get_set_display_info(s)["name"]
    return {
        "ok": True,
        "granted": len(new_items),
        "set_name": set_name,
        "slots": available_slots,
        "target": "bag",
        "message": "发放成功，请退到角色选择界面重新进入游戏查看"
    }


def quest_list(payload):
    """读取角色所有任务，合并 catalog 信息和物品名。"""
    ch = int(payload.get("character_id") or 0)
    if ch <= 0:
        raise RuntimeError("character_id 必须为正数")

    # 从 DB 读所有任务
    out = psql(
        "SELECT quest_id, status, progress, progress_model "
        "FROM character_quests WHERE character_id=%d ORDER BY quest_id" % ch)
    rows = []
    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        parts = line.split("|")
        if len(parts) < 4:
            continue
        qid = int(parts[0])
        status = parts[1]
        progress = int(parts[2])
        model = parts[3]

        # 从 catalog 补充 kind / level / items
        kind = ""
        level = 0
        need_items = []
        if QUESTS_JSON:
            quests = QUESTS_JSON.get("quests", QUESTS_JSON)
            q = quests.get(str(qid)) or quests.get(qid)
            if q:
                kind = q.get("kind", "")
                level = q.get("minimum_level", 0)
                cells = q.get("objective_cells") or []
                i = 0
                while i + 1 < len(cells):
                    tid = cells[i].get("value")
                    amt = cells[i + 1].get("value")
                    if tid and amt and tid > 0 and amt > 0:
                        need_items.append({
                            "template": int(tid),
                            "amount": int(amt),
                            "name": item_name(int(tid))
                        })
                    i += 2

        # 状态分类
        if status == "completed":
            state_label = "已完成"
        elif progress == 0:
            state_label = "可提交"
        else:
            state_label = "进行中"

        rows.append({
            "quest_id": qid,
            "status": status,
            "progress": progress,
            "progress_model": model,
            "kind": kind,
            "level": level,
            "need_items": need_items,
            "state_label": state_label
        })

    return {"character_id": ch, "quests": rows, "count": len(rows)}


def load_category_maps():
    """启动时加载本地分类映射（index + equipment），全部 id 覆盖。"""
    global INDEX_MAP, EQ_MAP
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
                        filtered_items = []
                        for it in data.get("items", []):
                            it_id = str(it.get("id"))
                            # 判断是不是装备
                            kind, stack_type = INDEX_MAP.get(it_id, ("", ""))
                            is_equip = (kind == "equipment") or (EQ_MAP.get(it_id) is not None)
                            # 如果是装备，检查白名单
                            if is_equip and EQUIP_WHITELIST and it_id not in EQUIP_WHITELIST:
                                continue
                            # 加其他字段
                            it["stackable"] = it_id in STACKABLE_IDS
                            g, s = categorize(it_id)
                            it["category"] = {"group": g, "sub": s}
                            it["restrictions"] = item_restrictions(it)
                            filtered_items.append(it)
                        data["items"] = filtered_items
                        data["total"] = len(filtered_items)
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
        elif path == "/api/quest/list":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = quest_list(payload)
                self._json_ok(data)
            except Exception as exc:
                self._json_err(str(exc))
        elif path == "/api/avatar-sets":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = avatar_sets_list(payload)
                self._json_ok(data)
            except Exception as exc:
                self._json_err(str(exc))
        elif path == "/api/avatar-set-detail":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = avatar_set_detail(payload)
                self._json_ok(data)
            except Exception as exc:
                self._json_err(str(exc))
        elif path == "/api/equip-sets/list":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = equip_sets_list(payload)
                self._json_ok(data)
            except Exception as exc:
                self._json_err(str(exc))
        elif path == "/api/equip-sets/detail":
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            try:
                payload = {k: (v[0] if v else "") for k, v in params.items()}
                data = equip_set_detail(payload)
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
        if path in ("/api/mail/send", "/api/vault/send", "/api/mail/revoke", "/api/quest/ready", "/api/avatar/grant", "/api/equip-sets/grant", "/api/character/rename", "/api/state/skill", "/api/account/clone", "/api/character/clone", "/api/account/delete", "/api/character/delete", "/api/grant", "/api/character/job", "/api/equip/remove", "/api/equip/enhance"):
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
                elif path == "/api/quest/ready":
                    self._json_ok(quest_ready(payload))
                elif path == "/api/avatar/grant":
                    self._json_ok(avatar_grant(payload))
                elif path == "/api/equip-sets/grant":
                    self._json_ok(equip_grant(payload))
                elif path == "/api/character/rename":
                    self._json_ok(rename_character(payload))
                elif path == "/api/state/skill":
                    self._json_ok(state_skill_points(payload))
                elif path == "/api/account/clone":
                    self._json_ok(clone_account(payload))
                elif path == "/api/character/clone":
                    self._json_ok(clone_character(payload))
                elif path == "/api/account/delete":
                    self._json_ok(delete_account(payload))
                elif path == "/api/character/delete":
                    self._json_ok(delete_character(payload))
                elif path == "/api/grant":
                    self._json_ok(grant_assets(payload))
                elif path == "/api/character/job":
                    self._json_ok(change_job(payload))
                elif path == "/api/equip/remove":
                    self._json_ok(remove_equip(payload))
                elif path == "/api/equip/enhance":
                    self._json_ok(enhance_equip(payload))
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
    load_quests_catalog()
    load_item_names()
    load_avatar_sets()
    load_equip_sets()
    load_equip_whitelist()
    load_set_display_names()

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


















