package inventory

import "fmt"

// GMEquipment is an explicit owner override, bounded by the recovered record
// representation. Normal enhancement costs/probabilities are not changed.
type GMEquipment struct {
	Upgrade      byte `json:"upgrade"`
	Amplify      byte `json:"amplify"`
	AmplifyValue byte `json:"amplify_value"`
	Refine       byte `json:"refine"`
}

func EditGMEquipment(c *EquipmentCatalog, gear BagEquipment, values GMEquipment) (BagEquipment, error) {
	if c == nil {
		return gear, fmt.Errorf("装备来源未准备")
	}
	kind, err := c.EquipmentKind(gear.Template)
	if err != nil {
		return gear, err
	}
	if values.Upgrade > 31 || values.Amplify > 4 || int(values.Refine) > RefineMaxLevel() {
		return gear, fmt.Errorf("打造参数超出范围")
	}
	if kind != "[weapon]" && values.Refine != 0 {
		return gear, fmt.Errorf("只有武器支持锻造")
	}
	if values.Refine != 0 && RefineRecordOffset() < 0 {
		return gear, fmt.Errorf("当前版本未加载已校验锻造布局")
	}
	switch kind {
	case "[weapon]", "[coat]", "[shoulder]", "[pants]", "[shoes]", "[waist]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		if values.Upgrade != 0 || values.Amplify != 0 || values.AmplifyValue != 0 || values.Refine != 0 {
			return gear, fmt.Errorf("该类物品不支持普通装备打造")
		}
	}
	if err = gear.ValidateRecord(); err != nil {
		return gear, err
	}
	row := EquipmentRow(gear)
	setAmplifyLevel(row[:], values.Upgrade)
	row[amplifyTypeOffset] = values.Amplify
	row[amplifyValueOffset] = values.AmplifyValue
	if values.Amplify == 0 {
		row[amplifyValueOffset] = 0
	}
	gear.Record = append([]byte(nil), row[:]...)
	setRefineLevel(&gear, gear.Record, values.Refine)
	return gear, nil
}
func GMEquipmentValues(gear BagEquipment) GMEquipment {
	row := EquipmentRow(gear)
	return GMEquipment{byte(amplifyLevel(row[:])), row[amplifyTypeOffset], row[amplifyValueOffset], refineLevel(gear, row[:])}
}
