package main

import (
	"dfolan/internal/database"
)

// persistentStore 必须由组合根注入的具体存储满足。
//
// 编译期断言：接口与 internal/database 的实现一旦漂移（改签名、删方法），
// 本包立刻编译失败，而不是等到运行期某条操作路径上才炸。
var _ persistentStore = (*database.Store)(nil)
