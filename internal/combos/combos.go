package combos

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sort"
	"strings"
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
	ActionKey     string    `json:"action_key"`
	ActionAliases []string  `json:"action_aliases,omitempty"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name,omitempty"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	Forms         []string  `json:"forms"`
	Flags         []FlagDef `json:"flags"`
	Notes         string    `json:"notes"`
	Safety        []string  `json:"safety"`
}

type CombosFile struct {
	Commands []CommandSpec `json:"commands"`
}

var (
	mu        sync.RWMutex
	actionMap = map[string]CommandSpec{}
	aliasMap  = map[string]string{}
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
		for _, al := range c.ActionAliases {
			aliasMap[strings.ToLower(al)] = c.ActionKey
		}
		if c.Name != "" {
			aliasMap[strings.ToLower(c.Name)] = c.ActionKey
		}
	}
}

func Get(actionKey string) (CommandSpec, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if v, ok := actionMap[actionKey]; ok {
		return v, true
	}
	lk := strings.ToLower(actionKey)
	if ak, ok := aliasMap[lk]; ok {
		if v, ok2 := actionMap[ak]; ok2 {
			return v, true
		}
	}
	for k, v := range actionMap {
		if strings.EqualFold(k, actionKey) {
			return v, true
		}
	}
	return CommandSpec{}, false
}

func RegisteredKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(actionMap))
	for k := range actionMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
