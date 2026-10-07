-- giveaway-gold-on-create —— 新角色创建时发一笔金币 + 一个系统邮件通知。
--
-- 由 mod "giveaway.random-equipment" 通过 servermod.RegisterRewardScript 登记，
-- 与内嵌规则集（internal/reward/scripts/*.lua）叠加生效。
--
-- 触发：character_create（角色创建**提交成功后**调用；客户端重试走幂等分支，
--       不会再触发一次，所以不会重复发）。
--
-- ## 为什么发金币而不是发装备
--
-- 目的是**验证 mod 能注册并生效**，所以通道必须"必定成功、不依赖任何内容模板"。
-- 发装备要过"奖励目录"（loaded equipment catalog: 3174 rows —— 注意它不是
-- 42 万条的完整穿戴目录），模板号不在里面就发不出来；这会把"mod 有没有生效"
-- 和"我抄的模板号对不对"两件事搅在一起。
--
-- 而 id 0 是**角色金币堆**（inventory.Bag.Add 的 id==0 分支），
-- 不查目录、不查装备，一定发得出去。本机 newchar.lua 已在多个角色上实证有效。
--
-- ## 幂等
--
-- 发放走 character_events 的稳定键（reward:character_create:<脚本名>:<角色ID>），
-- 所以同一次创建无论重放多少次都只发一次，且**每个角色各发一次**。

local GOLD = 1000000          -- 金币数量（角色身上，不是账号）
local SUBJECT = "mod 生效验证"
local BODY = "这封邮件由 mod（server 层）在角色创建时发出。看到它就说明：" ..
             "mod 已注册 → 规则脚本已进奖励管线 → 事件已触发 → 发放已完成。"

on("character_create", function(ctx)
  -- 1) 发金币：id 0 = 角色金币堆，必定成功
  grant_item(0, GOLD)

  -- 2) 发一封不带附件的系统邮件（无附件就不涉及任何内容模板）
  send_mail(SUBJECT, BODY)
end)
