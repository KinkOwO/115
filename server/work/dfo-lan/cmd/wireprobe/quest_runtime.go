package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/quest"
	"fmt"
)

// Native quest preparation must reach the service consumed by CMD1422 and
// event graduation, rather than stopping at the prepared catalog bundle.
func newRuntimeQuestService(service quest.Service, skip *catalog.QuestSkip) (*quest.Service, error) {
	if skip == nil || skip.Source.Checksum != service.Catalog.Source.Checksum {
		return nil, fmt.Errorf("native quest skip rules missing or from another source")
	}
	service.Skip = skip
	return &service, nil
}
