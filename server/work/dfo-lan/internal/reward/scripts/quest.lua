-- 事件触发奖励规则：任务完成（quest_complete）。
--
-- 脚本顶层用 on("quest_complete", 处理函数) 注册；任务结算提交成功后调用，
-- 参数 ctx 字段：type / level / quest_id / character_id / account_id / name。
--
-- grant_item(id, count) 直接发放道具到背包；send_mail(subject, body) 发送一封
-- 系统邮件，send_mail(subject, body, attachments) 额外附带多个道具/装备，
-- attachments = { {id = 模板, count = 数量}, ... }（最多 11 件）。两者都走服务端
-- 幂等的角色事件记录，事件重放不会重复发放；背包满时邮件附件会留在邮箱。
--
-- 注意：下面的物品 ID 必须存在于服务端目录；10000100 只是示例占位，请替换。
on("quest_complete", function(ctx)
  if ctx.quest_id == 3149 then
    grant_item(10000100, 1)
  end
end)
