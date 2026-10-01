package inventory

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Client capacities and their save version are independent of the PVF archive.
// Only accountcargo source fields are supplied by the archive.
func ImportVaultRules(a *pvf.Archive, policyPath string) (VaultRules, error) {
	var out VaultRules
	b, err := os.ReadFile(policyPath)
	if err != nil {
		return out, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, err
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF {
		return out, fmt.Errorf("trailing vault policy data")
	}
	if out.Account != nil || len(out.SourceSHA256) != 64 || !out.allows(out.InitialSlots) || (out.InitialSecondarySlots != 0 && !out.allows(out.InitialSecondarySlots)) {
		return out, fmt.Errorf("invalid vault capacity policy or source table in policy")
	}
	if a == nil {
		return out, fmt.Errorf("missing vault PVF archive")
	}
	cells, err := a.Tokens("etc/accountcargo.etc")
	if err != nil {
		return out, err
	}
	account, err := ParseAccountVaultSource(cells)
	if err != nil {
		return out, err
	}
	out.Account = &account
	return out, nil
}

func ParseAccountVaultSource(cells []pvf.Token) (AccountVaultRules, error) {
	var out AccountVaultRules
	f := sourceSections(cells)
	level := f["[required level]"]
	if len(level) != 1 || level[0].Type != 0 || level[0].Value <= 0 || level[0].Value > 65535 {
		return out, fmt.Errorf("invalid account vault required level")
	}
	out.RequiredLevel = uint16(level[0].Value)
	rows := f["[upgrade info]"]
	if len(rows) == 0 || len(rows)%6 != 0 {
		return out, fmt.Errorf("incomplete account vault upgrade table")
	}
	for i := 0; i < len(rows); i += 6 {
		var row [6]int64
		for j, t := range rows[i : i+6] {
			if t.Type != 0 {
				return out, fmt.Errorf("invalid account vault source cell")
			}
			row[j] = int64(t.Value)
		}
		out.Upgrades = append(out.Upgrades, row)
	}
	return out, out.Validate()
}
