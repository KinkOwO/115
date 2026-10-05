package inventory

import (
	"encoding/json"
	"reflect"
	"strings"
)

type equipmentJSON BagEquipment

var equipmentJSONNames = func() []string {
	typ := reflect.TypeOf(equipmentJSON{})
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	return names
}()

func knownEquipmentJSONName(key string) bool {
	for _, name := range equipmentJSONNames {
		if strings.EqualFold(name, key) {
			return true
		}
	}
	return false
}

func (item *BagEquipment) UnmarshalJSON(raw []byte) error {
	var known equipmentJSON
	if err := json.Unmarshal(raw, &known); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	*item = BagEquipment(known)
	for key := range fields {
		if knownEquipmentJSONName(key) {
			delete(fields, key)
		}
	}
	if len(fields) > 0 {
		item.ExtraFields = fields
	}
	return nil
}

func (item BagEquipment) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(equipmentJSON(item))
	if err != nil {
		return nil, err
	}
	if len(item.ExtraFields) == 0 {
		return raw, nil
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for key, value := range item.ExtraFields {
		if !knownEquipmentJSONName(key) {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}
