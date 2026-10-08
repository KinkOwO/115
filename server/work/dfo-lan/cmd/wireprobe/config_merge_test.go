package main

import (
	"io"
	"testing"
)

func TestMergedFeatureDefaultsKeepTheirCLIOverrides(t *testing.T) {
	config, err := loadConfig(nil, func(string) string { return "" }, io.Discard)
	if err != nil || !config.BoostUpEvent || config.VenusFlipGear != "configs/venus-flip-gear.generated.json" {
		t.Fatalf("merged feature defaults missing: %v", err)
	}
	config, err = loadConfig([]string{"-boostup-event=false", "-venus-flip-gear=local-pool.json"}, func(string) string { return "" }, io.Discard)
	if err != nil || config.BoostUpEvent || config.VenusFlipGear != "local-pool.json" {
		t.Fatal("contract update removed operator overrides", err)
	}
}
