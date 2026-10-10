package pvf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestWrapOuterRoundTripOnSyntheticClient：用仓库现成的合成客户端做**往返**：
// 外层 → 剥壳 → 重新加壳，必须逐字节还原成原来那份外层。这是 WrapOuter 的自洽判据。
func TestWrapOuterRoundTripOnSyntheticClient(t *testing.T) {
	client := writeSyntheticClient(t, t.TempDir())
	source := filepath.Join(client.dir, "Script.pvf")
	orig, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(t.TempDir(), "Script.inner.pvf")
	if _, err := UnwrapOuter(client.dir, source, inner); err != nil {
		t.Fatalf("UnwrapOuter: %v", err)
	}
	if _, err := os.Stat(inner); err != nil {
		t.Fatal(err)
	}
	rewrapped := filepath.Join(t.TempDir(), "Script.rewrapped.pvf")
	stats, err := WrapOuter(client.dir, inner, rewrapped)
	if err != nil {
		t.Fatalf("WrapOuter: %v", err)
	}
	got, err := os.ReadFile(rewrapped)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(orig) {
		t.Fatalf("长度不符：%d vs %d", len(got), len(orig))
	}
	if d := FirstDifference(got, orig); d >= 0 {
		t.Fatalf("重新加壳与原始外层不同，首个差异在偏移 0x%X", d)
	}
	t.Logf("合成客户端往返逐字节一致：%d 字节 / %d 段 / %d 把密钥", stats.Size, stats.Segments, stats.Keys)
}

// TestWrapOuterReproducesRealClientOuter 是**真档判据**（环境变量开启，默认跳过）：
// 把现网的内层归档重新加壳，应当逐字节还原出现网那份外层 `Script.pvf`。
//
//	PVF_WRAP_CLIENT=<DFO 客户端目录>   （里面要有 DFO.exe / sk.dat / Script.pvf）
//	PVF_WRAP_INNER=<内层归档路径>      （一般是 <root>/server/work/client-build/Script.inner.pvf）
//	PVF_WRAP_OUT=<产物路径>
func TestWrapOuterReproducesRealClientOuter(t *testing.T) {
	clientDir := os.Getenv("PVF_WRAP_CLIENT")
	inner := os.Getenv("PVF_WRAP_INNER")
	out := os.Getenv("PVF_WRAP_OUT")
	if clientDir == "" || inner == "" || out == "" {
		t.Skip("未设 PVF_WRAP_CLIENT / PVF_WRAP_INNER / PVF_WRAP_OUT")
	}
	_ = os.Remove(out)
	stats, err := WrapOuter(clientDir, inner, out)
	if err != nil {
		t.Fatalf("WrapOuter: %v", err)
	}
	want := filepath.Join(clientDir, "Script.pvf")
	orig, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("产 %d 字节 / %d 段（有密钥的 %d，原样尾段 %d）；现网外层 %d 字节",
		stats.Size, stats.Segments, stats.Keys, stats.PlainTailSegments, len(orig))
	if len(got) != len(orig) {
		t.Fatalf("长度不符：%d vs %d", len(got), len(orig))
	}
	d := FirstDifference(got, orig)
	if d >= 0 {
		t.Fatalf("重新加壳与现网外层不同，首个差异在偏移 0x%X（段 %d）", d, d/outerBlockSize)
	}
	if !bytes.Equal(got, orig) {
		t.Fatal("逐字节比对失败")
	}
	t.Logf("** 重新加壳逐字节还原出现网外层 Script.pvf（%d 字节）**", len(got))
}
