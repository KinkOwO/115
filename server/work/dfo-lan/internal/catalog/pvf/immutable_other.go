//go:build !windows

package pvf

import "os"

func openImmutableFile(path string) (*os.File, error) { return os.Open(path) }
