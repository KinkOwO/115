package main

import "dfolan/internal/catalog"

// Use the complete official gate table. The 20261002 capture includes
// variable-width records and its terminal count; do not clone a whole table
// as though it were the old single-record fixture.
func raidChannelEventInfo(_ map[uint32]uint32, _ *catalog.ChannelDirectory) []byte {
	return eventInfoSnapshot(nil)
}
