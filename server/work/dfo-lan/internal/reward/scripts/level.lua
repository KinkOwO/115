-- 事件触发奖励规则：角色升级（level_up）。
--
-- 网关把本目录所有 *.lua 载入同一个 Lua 状态，脚本顶层用 on(事件, 处理函数)
-- 注册。角色升级提交成功后调用 level_up 的处理函数，参数 ctx 字段：
-- type / level / quest_id / character_id / account_id / name。
--
-- grant_item(id, count) 直接发放道具到背包；send_mail(subject, body) 发送一封
-- 系统邮件，send_mail(subject, body, attachments) 额外附带多个道具/装备，
-- attachments = { {id = 模板, count = 数量}, ... }（最多 11 件）。两者都走服务端
-- 幂等的角色事件记录，事件重放不会重复发放；背包满时邮件附件会留在邮箱。
--
-- 注意：下面的物品 ID 必须存在于服务端目录；10000100 只是示例占位，请替换。
on("level_up", function(ctx)
  if ctx.level == 10 then
    grant_item(10000100, 5)
    send_mail("升级奖励", "恭喜达到 10 级，奖励已发放。")
  end

  if ctx.level == 20 then
    send_mail("20级奖励", "达到 20 级，奖励见附件。", { {id = 10000100, count = 5} })
  end
end)
