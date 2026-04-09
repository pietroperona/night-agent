package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gobwas/glob"
	"gopkg.in/yaml.v3"
)

type MatchType string
type Decision string

const (
	MatchGlob  MatchType = "glob"
	MatchRegex MatchType = "regex"

	DecisionAllow   Decision = "allow"
	DecisionBlock   Decision = "block"
	DecisionAsk     Decision = "ask"
	DecisionSandbox Decision = "sandbox"
)

type Condition struct {
	ActionType     string   `yaml:"action_type"`
	CommandMatches []string `yaml:"command_matches"`
	PathMatches    []string `yaml:"path_matches"`
}

type Rule struct {
	ID        string    `yaml:"id"`
	When      Condition `yaml:"when"`
	MatchType MatchType `yaml:"match_type"`
	Decision  Decision  `yaml:"decision"`
	Reason    string    `yaml:"reason"`
}

type Policy struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

type Action struct {
	Type    string
	Command string
	Path    string
}

type EvalResult struct {
	Decision Decision
	RuleID   string
	Reason   string
}

func Load(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere il file di policy: %w", err)
	}

	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("YAML non valido: %w", err)
	}

	if p.Version == 0 {
		return nil, fmt.Errorf("campo 'version' mancante o zero nella policy")
	}

	return &p, nil
}

func (p *Policy) Evaluate(action Action) EvalResult {
	for _, rule := range p.Rules {
		if rule.When.ActionType != action.Type {
			continue
		}

		if matches(rule, action) {
			return EvalResult{
				Decision: rule.Decision,
				RuleID:   rule.ID,
				Reason:   rule.Reason,
			}
		}
	}

	return EvalResult{Decision: DecisionAllow}
}

func matches(rule Rule, action Action) bool {
	mt := rule.MatchType
	if mt == "" {
		mt = MatchGlob
	}

	// match su command
	if len(rule.When.CommandMatches) > 0 && action.Command != "" {
		for _, pattern := range rule.When.CommandMatches {
			if matchPattern(mt, pattern, action.Command, false) {
				return true
			}
		}
		return false
	}

	// match su path
	if len(rule.When.PathMatches) > 0 && action.Path != "" {
		for _, pattern := range rule.When.PathMatches {
			if matchPattern(mt, pattern, action.Path, true) {
				return true
			}
		}
		return false
	}

	return false
}

// matchPattern valuta un pattern contro un valore.
// Per command_matches usa glob senza separatori (il * matcha spazi e /).
// Per path_matches usa glob con filepath.Separator (il * non matcha /).
func matchPattern(mt MatchType, pattern, value string, isPath bool) bool {
	switch mt {
	case MatchRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default: // glob
		var g glob.Glob
		var err error
		if isPath {
			g, err = glob.Compile(pattern, filepath.Separator)
		} else {
			// per comandi shell: * deve matchare spazi, slash e qualsiasi carattere
			g, err = glob.Compile(pattern)
		}
		if err != nil {
			return false
		}
		return g.Match(value)
	}
}
