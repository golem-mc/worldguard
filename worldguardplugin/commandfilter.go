package worldguardplugin

import (
	"regexp"
	"strings"
)

var commandNamespace = regexp.MustCompile(`^/([^ :]*:)?`)

// commandFilter decides a command line against permitted and denied
// commands, each a command with optional leading arguments; a nil list is
// no rule. WorldGuard: CommandFilter
type commandFilter struct {
	permitted, denied []string
}

// allows reports whether the filter lets command through.
func (f commandFilter) allows(command string) bool {
	command = commandNamespace.ReplaceAllString(strings.ToLower(command), "/")
	result := ""
	used := strings.Fields(command)
	if f.denied != nil {
	denied:
		for _, rule := range f.denied {
			parts := strings.Fields(rule)
			for i := 0; i < len(parts) && i < len(used); i++ {
				if !strings.EqualFold(parts[i], used[i]) {
					continue denied
				}
				if i+1 == len(parts) {
					result = rule
					break denied
				}
				if i+1 == len(used) {
					continue denied
				}
			}
		}
	}
	if f.permitted != nil {
	permitted:
		for _, rule := range f.permitted {
			parts := strings.Fields(rule)
			for i := 0; i < len(parts) && i < len(used); i++ {
				if !strings.EqualFold(parts[i], used[i]) {
					result = command
					continue permitted
				}
				if i+1 == len(parts) {
					result = ""
					break permitted
				}
				if i+1 == len(used) {
					result = command
					continue permitted
				}
			}
		}
	}
	return result == ""
}

// stringList answers a set flag's strings, nil when the flag is unset.
func stringList(v any) []string {
	values, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, x := range values {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
