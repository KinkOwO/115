package database

import (
	"encoding/json"
	"reflect"
	"strings"
)

type adventureDataJSON AdventureData

var adventureDataFields = func() map[string]bool {
	fields := map[string]bool{}
	t := reflect.TypeOf(adventureDataJSON{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" {
			name = f.Name
		}
		if name != "-" {
			fields[name] = true
		}
	}
	return fields
}()

// Unknown top-level account fields survive every existing adventure writer.
// This is necessary when activity progress shares the account document: an old
// shop/EXP/settings save must neither erase activity state nor other future data.
func (d *AdventureData) UnmarshalJSON(raw []byte) error {
	var known adventureDataJSON
	if err := json.Unmarshal(raw, &known); err != nil {
		return err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(raw, &extra); err != nil {
		return err
	}
	for key := range adventureDataFields {
		delete(extra, key)
	}
	*d = AdventureData(known)
	d.extra = extra
	return nil
}

func (d AdventureData) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(adventureDataJSON(d))
	if err != nil {
		return nil, err
	}
	var result map[string]json.RawMessage
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	for key, value := range d.extra {
		if !adventureDataFields[key] {
			result[key] = value
		}
	}
	return json.Marshal(result)
}
