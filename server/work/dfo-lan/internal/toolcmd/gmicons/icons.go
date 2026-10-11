// Package gmicons reads presentation assets from the current local client.
// Its stdio-only mode has no database, network listener or save writer.
package gmicons

import (
	"bufio"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/toolcmd/gmicons/npkicon"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

func iconField(cells []pvf.Token) (string, int, bool) {
	for i, t := range cells {
		if t.Type != 3 || t.Text != "[icon]" {
			continue
		}
		sprite := ""
		for _, v := range cells[i+1:] {
			if v.Type == 3 {
				break
			}
			if v.Text != "" {
				sprite = v.Text
			}
			if v.Type == 0 && sprite != "" && v.Value >= 0 && v.Value < 1<<20 {
				return sprite, int(v.Value), true
			}
		}
		return "", 0, false
	}
	return "", 0, false
}

func Run(archive, packs string, input io.Reader, output io.Writer) error {
	a, err := pvf.OpenReadOnly(pvf.Options{Path: archive, MaxBytes: 1 << 30}, "")
	if err != nil {
		return err
	}
	defer a.Close()
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		return err
	}
	cache := npkicon.NewCache(packs)
	enc := json.NewEncoder(output)
	if err = enc.Encode(map[string]any{"ready": true, "source": a.Snapshot().Checksum}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 4096)
	for scanner.Scan() {
		var req struct {
			ID uint32 `json:"id"`
		}
		response := map[string]any{}
		err = json.Unmarshal(scanner.Bytes(), &req)
		if err == nil {
			response["id"] = req.ID
			entry, ok := index.Items[req.ID]
			if !ok {
				err = fmt.Errorf("item absent in current client")
			} else {
				var script catalog.ScriptRecord
				script, err = catalog.ResolveScript(a, entry.Path)
				if err == nil {
					sprite, frame, ok := iconField(script.Cells)
					if !ok {
						err = fmt.Errorf("item has no source icon")
					} else {
						var png []byte
						png, err = cache.PNG(sprite, frame)
						if err == nil {
							response["png"] = base64.StdEncoding.EncodeToString(png)
							response["sprite"] = sprite
							response["frame"] = frame
						}
					}
					for i, t := range script.Cells {
						if t.Type == 3 && t.Text == "[rarity]" && i+1 < len(script.Cells) && script.Cells[i+1].Type == 0 {
							response["rarity"] = script.Cells[i+1].Value
							break
						}
					}
				}
			}
		}
		if err != nil {
			response["error"] = err.Error()
		}
		if err = enc.Encode(response); err != nil {
			return err
		}
		a.ReleaseReadCaches()
	}
	return scanner.Err()
}
