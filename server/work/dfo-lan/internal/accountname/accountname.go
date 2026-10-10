// Package accountname owns the one rule for a development account name: which
// characters are legal, and which name a session falls back to when nobody picks one.
//
// The rule is shared because the same string crosses three boundaries: it is written
// into the accounts table by wireprobe, it becomes a segment of the "?"-separated
// client connect payload built by the launcher, and the GUI launcher (a separate Go
// module) has to pre-check what the owner types. A name containing "?", "=", a space
// or a CJK character would silently shift the payload apart, so every caller refuses
// it before any process starts.
package accountname

// Default is the account a session logs in with when neither --account nor
// DFO_ACCOUNT says otherwise. Every existing save was created under this identity,
// so changing it is a data decision, not a code cleanup.
const Default = "probe"

// MaxLength bounds a name so it stays readable in the client's login dialog and
// inside the payload line the probe prints.
const MaxLength = 32

// Valid reports whether name may be used as a development account name: 1..MaxLength
// bytes of ASCII letters, digits, '_' and '-'.
func Valid(name string) bool {
	if len(name) == 0 || len(name) > MaxLength {
		return false
	}
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
