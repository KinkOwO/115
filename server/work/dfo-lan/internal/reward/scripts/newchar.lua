-- 事件触发奖励规则：新角色创建（character_create）。
--
-- 脚本顶层用 on("character_create", 处理函数) 注册；角色创建提交成功后调用，
-- 参数 ctx 字段：type / level / quest_id / character_id / account_id / name。
--
-- grant_item(id, count) 直接发放道具到背包；grant_cera(amount) 发放**账号级点券**；
-- send_mail(subject, body) 发送一封系统邮件，send_mail(subject, body, attachments)
-- 额外附带多个道具/装备，attachments = { {id = 模板, count = 数量}, ... }（最多 11 件）。
-- 三者都走服务端幂等的记录，事件重放不会重复发放；背包满时邮件附件会留在邮箱。
--
-- 本规则对所有新角色无条件生效，只发道具/金币/点券，不做背包/金库扩容、
-- 跳过剧情、复活币或账号材料仓库写入。
--
-- 特殊模板约定：id 0 是**角色金币堆**（不是道具），计入角色身上的金币；
-- 3033~3037、3262 是账号共享材料（晶块类），按普通道具发放到背包。
on("character_create", function(ctx)
  grant_item(0, 10000000)        -- 角色金币
  grant_cera(100000)             -- 账号点券
  grant_item(10418035, 1000)     -- 奥德赛金币
  grant_item(10418036, 1000)     -- 奥德赛银币
  grant_item(3033, 10000)
  grant_item(3034, 10000)
  grant_item(3035, 10000)
  grant_item(3036, 10000)
  grant_item(3037, 10000)
  grant_item(3262, 10000)
end)
