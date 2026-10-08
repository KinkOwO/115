package gamedata

import (
	"dfolan/internal/catalog"
	"fmt"
)

func (s *Source) BufferRental() (*catalog.BufferRentalRules, error) {
	if s == nil || s.archive == nil {
		return nil, fmt.Errorf("buffer rental requires a native PVF source")
	}
	return catalog.ImportBufferRental(s.archive)
}

// LoadBufferRental never falls back to a JSON projection or a Go content table.
func (c *Catalogs) LoadBufferRental() (*catalog.BufferRentalRules, error) {
	if c == nil || !c.Selected("buffer-rental") {
		return nil, nativeContentRequired("buffer-rental")
	}
	if err := c.RequireSelected("buffer-rental", c.BufferRental != nil); err != nil {
		return nil, err
	}
	return c.BufferRental, nil
}
