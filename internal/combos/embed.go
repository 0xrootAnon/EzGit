package combos

import _ "embed"

//go:embed combos_updated.json
var embeddedCombosUpdated []byte

//go:embed combos.json
var embeddedCombos []byte

func LoadEmbedded() (*CombosFile, error) {
	if len(embeddedCombosUpdated) > 0 {
		return LoadFromBytes(embeddedCombosUpdated)
	}
	if len(embeddedCombos) > 0 {
		return LoadFromBytes(embeddedCombos)
	}
	return nil, nil
}
