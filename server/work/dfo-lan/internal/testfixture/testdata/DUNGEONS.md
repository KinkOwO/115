# 历史副本图测试快照

六份 `dungeons.*.json.gz` 来自退休导出的完整原始字节，合计 39,181,151 字节压缩为 2,051,913 字节。保留 generated、next28、odyssey candidate/release/scenes-release 和 skycastle candidate 的历史阶段差异，原有全图准入、房间、怪物、场景路由与通关流程测试不裁剪。

`testfixture.DungeonPath` 只由测试调用，将快照解压到测试私有临时目录并核对每份原 SHA256。缺失或字节变化直接失败，不跳过也不回退运行目录。源码运行内容仍只从活动 PVF 准备，不加载这些快照；完整当前原生投影的 SHA256 由 gamedata 的本地归档回归独立钉住。
