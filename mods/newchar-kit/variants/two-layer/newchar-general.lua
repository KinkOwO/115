-- ============================================================
-- 新角色出厂补给 · ① 通用层（任何角色都发）
-- mod：newchar.kit    与 ② newchar-class.lua 是两个**独立脚本**
-- ============================================================
--
-- 【为什么拆两个】
--   服务端按**脚本文件名**区分事件与幂等键：
--     reward:character_create:<脚本名>:<角色ID>:item / :mail / :cera
--   所以两个脚本各自注册 on("character_create") 互不影响：谁也不顶掉谁，
--   其中一个出错（服务端只记日志并继续）也不会连累另一个。
--   代价是公共小函数各自留一份 —— 这是刻意的，不靠加载顺序、不跨脚本共享全局。
--
-- 【本脚本发什么】（全部走 grant_item / send_mail / grant_cera）
--   金币 3 亿 + 点券 50 万
--   背包可堆叠物 16 类
--   宠物装备：输出 10 件 / 辅助 3 件
--   毕业宝珠套：输出 12 颗 / 辅助 12 颗
--
-- 【输出 / 辅助怎么判】
--   看 ctx.profession（基础职业）+ ctx.advancement（转职）。
--   同一基础职业里既有输出也有辅助，所以**必须带转职**：
--     例：女神枪手(5) 转职 1 = 漫游枪手（输出） / 转职 5 = 协战师（辅助）。
--   拿不到 profession 时（ctx 字段缺席）按 DEFAULT_WHEN_UNKNOWN 发，**仍然发**（本层"任何角色都发"）。
--
-- 【注意】邮件标题必须纯 ASCII（中文会被服务端转 GBK，客户端渲染成乱码）。

------------------------------------------------------------------------
-- 0) 数据表
------------------------------------------------------------------------

-- 背包可堆叠物 16 项（grant_item）
local BAG = {
  { id = 10335027, n = 15 },   -- 100%+15 装备增幅券
  { id = 1286, n = 15 },       -- 纯净的增幅书
  { id = 10407241, n = 1000 }, -- 秘宝升级材料·无知之梦
  { id = 10413524, n = 1000 }, -- 秘宝升级材料·灾厄之种
  { id = 10404807, n = 1000 }, -- 秘宝升级材料·天帷巨兽的眼泪
  { id = 10404719, n = 1000 }, -- 秘宝升级材料·秘宝玛瑙
  { id = 590723056, n = 20 },  -- 传说升级材料卡 100%
  { id = 10420683, n = 1 },    -- BUFF 称号礼盒
  { id = 10347792, n = 1 },    -- 增益装扮上衣/下装 & BUFF 徽章礼盒
  { id = 10414526, n = 1 },    -- 白金徽章自选礼盒
  { id = 590722960, n = 1 },   -- 稀有克隆武器装扮礼盒
  { id = 590714302, n = 1 },   -- 稀有克隆套装礼盒
  { id = 590722990, n = 1 },   -- 宠物选择礼盒
  { id = 590722989, n = 1 },   -- 夏日狂欢夜 称号选择礼盒
  { id = 590722932, n = 1 },   -- 热带夜之追忆礼盒
  { id = 100360012, n = 1 },   -- 神圣符咒
}

-- 宠物装备·输出 10 件
local PET_GEAR_OUT = {
  500950043, 500950042, 500950041, 500950040, 500950039,
  500960037, 500960036, 500960035, 500960034, 500970014,
}

-- 宠物装备·辅助 3 件
local PET_GEAR_AUX = {
  500950044, 500960038, 500970014,
}

-- 毕业宝珠·输出套 12 颗
local BEADS_OUT = {
  10356186, 10421405, 10401274, 590721849, 590715603, 10407416,
  10413813, 590723048, 590721794, 10416499, 10416501, 590721845,
}

-- 毕业宝珠·辅助套 12 颗
local BEADS_AUX = {
  10406307, 10421407, 10345504, 10407592, 590715604, 10407418,
  10413815, 590723049, 590014785, 10421409, 10416503, 590721847,
}

-- 辅助职业：**必须带转职**（同一基础职业里既有输出也有辅助）
--   例：女神枪手(5) 转职 1 = 漫游枪手（输出） / 转职 5 = 协战师（辅助）
local AUX_CLASSES = {
  [3] = { 5 },
  [4] = { 1 },
  [5] = { 5 },
  [14] = { 1 },
  [16] = { 1 },
}

-- ctx 里拿不到职业时按哪一套发（"out" | "aux"）。
-- 本层是"任何角色都发"，所以必须有个默认值：默认输出向（宠物装备 10 件 + 输出宝珠 12 颗）。
-- 想默认辅助就把这里改成 "aux"。
local DEFAULT_WHEN_UNKNOWN = "out"

------------------------------------------------------------------------
-- 1) 工具
------------------------------------------------------------------------

local function is_aux(prof, adv)
  local t = AUX_CLASSES[prof]
  if not t then return false end
  for _, v in ipairs(t) do
    if v == adv then return true end
  end
  return false
end

-- 按 ctx 判"输出向 / 辅助向"；拿不到职业时用 DEFAULT_WHEN_UNKNOWN。
local function is_aux_ctx(ctx)
  local prof = tonumber(ctx and ctx.profession)
  if prof == nil then
    return DEFAULT_WHEN_UNKNOWN == "aux"
  end
  local adv = tonumber(ctx and ctx.advancement) or 0
  return is_aux(prof, adv)
end

-- 把一串模板按每封 <= 11 件切分发邮件（服务端每封邮件最多 11 个附件）
local function mail_all(prefix, note, tpls)
  local per = 11
  local i, part = 1, 1
  while i <= #tpls do
    local att = {}
    for _ = 1, per do
      if tpls[i] then
        att[#att + 1] = { id = tpls[i], count = 1 }
        i = i + 1
      end
    end
    local total = math.ceil(#tpls / per)
    send_mail(string.format("%s %d/%d", prefix, part, total), note, att)
    part = part + 1
  end
end

------------------------------------------------------------------------
-- 2) 规则：新角色创建（通用层）
------------------------------------------------------------------------

on("character_create", function(ctx)
  local isAux = is_aux_ctx(ctx)

  -- ① 金币（3 亿）+ 点券（50 万）
  grant_item(0, 300000000)
  grant_cera(500000)

  -- ② 背包可堆叠物 16 类
  for _, it in ipairs(BAG) do
    grant_item(it.id, it.n)
  end

  -- ③ 宠物装备（输出 10 件 / 辅助 3 件）
  mail_all("Starter Kit - PetGear", "Pet equipment",
           isAux and PET_GEAR_AUX or PET_GEAR_OUT)

  -- ④ 毕业宝珠套（输出 12 颗 / 辅助 12 颗）
  mail_all("Starter Kit - Beads", "Graduation enchant beads",
           isAux and BEADS_AUX or BEADS_OUT)
end)
