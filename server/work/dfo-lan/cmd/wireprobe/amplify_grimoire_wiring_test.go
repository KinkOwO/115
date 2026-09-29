package main

import (
	"bytes"
	"os"
	"testing"
)

// CMD205（用增幅书打红字）曾经「服务层 + 流程层都在、就是没人调」：
// 客户端发 205 之后服务端既不处理也不回包，实机表现就是「增幅书打了没效果」。
// 这条用例把分派钉住 —— 光有 applyAmplifyGrimoire 这个函数不算接线成功。
func TestAmplifyGrimoireDispatchIsWired(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(src, []byte("frame.ID == 205"))
	if i < 0 {
		t.Fatal("main.go 里没有 frame.ID == 205 的分派：CMD205 会被静默丢弃")
	}
	window := src[i:]
	if j := bytes.IndexByte(window, '\n'); j >= 0 {
		window = window[j+1:]
	}
	if len(window) > 4096 {
		window = window[:4096]
	}
	if !bytes.Contains(window, []byte("applyAmplifyGrimoire(")) {
		t.Fatal("frame.ID == 205 的分支没有调用 applyAmplifyGrimoire")
	}
	if !bytes.Contains(window, []byte("amplifyGrimoireRefusal()")) {
		t.Fatal("frame.ID == 205 的分支没有拒绝路径：客户端会一直停在等待态")
	}
	if !bytes.Contains(window, []byte("protocol.Refusal")) && !bytes.Contains(src, []byte("func amplifyGrimoireRefusal")) {
		t.Fatal("缺少 205 的拒绝包构造")
	}
}
