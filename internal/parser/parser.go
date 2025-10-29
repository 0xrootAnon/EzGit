package parser

import "strings"

type VerbParser struct {
	lookup map[string]string
}

func NewVerbParser() *VerbParser {
	p := &VerbParser{lookup: map[string]string{}}
	p.mapSyn("create repo", "init")
	p.mapSyn("init repo", "init")
	p.mapSyn("initialize", "init")
	p.mapSyn("status", "status")
	p.mapSyn("show status", "status")
	p.mapSyn("add", "add")
	p.mapSyn("stage", "add")
	p.mapSyn("commit", "commit")
	p.mapSyn("save", "commit")
	p.mapSyn("push", "push")
	p.mapSyn("publish", "push")
	p.mapSyn("clone", "clone")
	p.mapSyn("undo", "undo")
	p.mapSyn("revert", "undo")
	p.mapSyn("raw git", "raw")
	p.mapSyn("expert", "raw")
	p.mapSyn("branch", "branch")
	p.mapSyn("checkout", "branch")
	return p
}

func (p *VerbParser) mapSyn(k, v string) {
	p.lookup[strings.ToLower(k)] = v
}

func (p *VerbParser) Parse(input string) string {
	in := strings.TrimSpace(strings.ToLower(input))
	if v, ok := p.lookup[in]; ok {
		return v
	}
	for k, v := range p.lookup {
		if strings.Contains(in, k) {
			return v
		}
	}
	words := strings.Fields(in)
	if len(words) > 0 {
		switch words[0] {
		case "init", "clone", "status", "add", "commit", "push", "undo", "raw", "branch":
			return words[0]
		}
	}
	return ""
}
