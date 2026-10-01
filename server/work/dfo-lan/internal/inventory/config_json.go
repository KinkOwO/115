package inventory

import (
	"encoding/json"
	"os"
)

// loadOptionalJSON keeps missing optional rule files disabled.
func loadOptionalJSON(path string, target any) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, target)
}
