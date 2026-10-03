package quest

import "testing"

// 2026-10-01（next146）：直读模式下任务目录源身份必须能从编译期常量切到当次
// 内层 checksum。非 64 位十六进制一律忽略。
func TestSetImageCommunicationSourceValidates(t *testing.T) {
	original := imageCommunicationSourceChecksum
	t.Cleanup(func() { imageCommunicationSourceChecksum = original })

	good := "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
	SetImageCommunicationSource(good)
	if imageCommunicationSourceChecksum != good {
		t.Fatalf("SetImageCommunicationSource did not switch: %s", imageCommunicationSourceChecksum)
	}
	for _, bad := range []string{"", "abc", good[:63], "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		SetImageCommunicationSource(bad)
		if imageCommunicationSourceChecksum != good {
			t.Fatalf("garbage %q changed the token", bad)
		}
	}
}
