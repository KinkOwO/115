package main

import (
	"os"
	"strconv"
)

// envStrOr 读一个字符串环境变量；缺失时返回 fallback。
func envStrOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// envByteOr 读一个 0..255 的环境变量；缺失或非法时返回 fallback。
// 装备库制作的应答里有几个单字节开关，做成 env 就能不改代码切换。
func envByteOr(name string, fallback int) int {
	n := envIntOr(name, fallback)
	if n < 0 || n > 255 {
		return fallback
	}
	return n
}

func envIntOr(name string, fallback int) int {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}
	return n
}
