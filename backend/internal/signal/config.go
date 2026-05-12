package signal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadRulesFile reads and validates rules from a YAML path.
func LoadRulesFile(path string) ([]Rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rf RuleFile
	if err := yaml.Unmarshal(b, &rf); err != nil {
		return nil, err
	}
	if rf.Version != 1 {
		return nil, fmt.Errorf("signal: unsupported rules version %d", rf.Version)
	}
	for _, r := range rf.Rules {
		if r.ID == "" || r.Condition.Type == "" {
			return nil, fmt.Errorf("signal: invalid rule %+v", r)
		}
	}
	return rf.Rules, nil
}
