package gamedata

import (
	"dfolan/internal/activity"
	"fmt"
)

// HoneyNative reads event313's actual indexed US grammar from this Source.
// It is an independent read model; it neither enables a gateway nor projects
// its nested blocks onto the incompatible CN weekday interval executor.
func (s *Source) HoneyNative() (*activity.HoneyNative, error) {
	if s == nil || s.archive == nil {
		return nil, fmt.Errorf("native honey requires selected PVF")
	}
	index, e := s.archive.Tokens("list/event.lst")
	if e != nil {
		return nil, e
	}
	path, e := activity.HoneyNativeEventPath(index)
	if e != nil {
		return nil, e
	}
	tokens, e := s.archive.Tokens(path)
	if e != nil {
		return nil, e
	}
	c, e := activity.ParseHoneyNative(tokens)
	if e != nil {
		return nil, e
	}
	if _, ok := s.archive.FindFile(c.UI); !ok {
		return nil, fmt.Errorf("native honey UI missing: %s", c.UI)
	}
	c.SourceChecksum, c.Path = s.Snapshot().Checksum, path
	return &c, nil
}
