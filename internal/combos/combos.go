package combos

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sort"
	"sync"
)

type FlagDef struct {
	Key               string         `json:"key"`
	ParamKey          string         `json:"param_key"`
	Label             string         `json:"label"`
	Type              string         `json:"type"`
	Default           any            `json:"default"`
	ManualOnly        bool           `json:"manualOnly"`
	Advanced          bool           `json:"advanced"`
	Required          bool           `json:"required"`
	InferrableFrom    []string       `json:"inferrableFrom"`
	Validate          map[string]any `json:"validate"`
	Confirmation      string         `json:"confirmation"`
	Example           string         `json:"example"`
	PreviewOrder      int            `json:"previewOrder"`
	MutuallyExclusive []string       `json:"mutuallyExclusive"`
	Implies           []string       `json:"implies"`
}

type CommandSpec struct {
	ActionKey   string    `json:"action_key"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Forms       []string  `json:"forms"`
	Flags       []FlagDef `json:"flags"`
	Notes       string    `json:"notes"`
	Safety      []string  `json:"safety"`
}

type CombosFile struct {
	Commands []CommandSpec `json:"commands"`
}

var (
	mu        sync.RWMutex
	actionMap = map[string]CommandSpec{}
)

func LoadFromFile(path string) (*CombosFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var doc CombosFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	for i := range doc.Commands {
		sort.SliceStable(doc.Commands[i].Flags, func(a, b int) bool {
			return doc.Commands[i].Flags[a].PreviewOrder < doc.Commands[i].Flags[b].PreviewOrder
		})
	}
	return &doc, nil
}

func Register(doc *CombosFile) {
	mu.Lock()
	defer mu.Unlock()
	for _, c := range doc.Commands {
		actionMap[c.ActionKey] = c
	}
}

func Get(actionKey string) (CommandSpec, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := actionMap[actionKey]
	return v, ok
}
