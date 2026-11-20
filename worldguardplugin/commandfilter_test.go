// Code translated from WorldGuard's CommandFilterTest.

package worldguardplugin

import (
	"strings"
	"testing"
)

func TestCommandFilter(t *testing.T) {
	var filter commandFilter
	filter = commandFilter{}
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/deny1", true)
	assertSubcommands(t, filter, "/other", true)
	filter = commandFilter{permitted: []string{"/permit1", "/permit2"}}
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/permit2", true)
	assertSubcommands(t, filter, "/other", false)
	filter = commandFilter{denied: []string{"/deny1", "/deny2"}}
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/deny2", false)
	assertSubcommands(t, filter, "/other", true)
	filter = commandFilter{permitted: []string{"/permit1", "/permit2"}, denied: []string{"/deny1", "/deny2"}}
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/deny2", false)
	assertSubcommands(t, filter, "/other", false)
	filter = commandFilter{permitted: []string{"/permit1", "/permit2", "/strange"}, denied: []string{"/deny1", "/deny2", "/strange"}}
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/deny2", false)
	assertSubcommands(t, filter, "/strange", true)
	assertSubcommands(t, filter, "/other", false)
	filter = commandFilter{permitted: []string{"/permit1", "/parent permit1", "/parent between subpermit1"}}
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/parent", false)
	assertSubcommands(t, filter, "/parent permit1", true)
	assertSubcommands(t, filter, "/parent other", false)
	assertSubcommands(t, filter, "/parent between", false)
	assertSubcommands(t, filter, "/parent between subpermit1", true)
	assertSubcommands(t, filter, "/parent between other", false)
	assertSubcommands(t, filter, "/parent between other subpermit1", false)
	assertSubcommands(t, filter, "/other", false)
	assertSubcommands(t, filter, "/other permit1", false)
	assertSubcommands(t, filter, "/other between", false)
	assertSubcommands(t, filter, "/other between subpermit1", false)
	filter = commandFilter{denied: []string{"/deny1", "/parent deny1", "/parent between subdeny1"}}
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/parent", true)
	assertSubcommands(t, filter, "/parent deny1", false)
	assertSubcommands(t, filter, "/parent between", true)
	assertSubcommands(t, filter, "/parent between subdeny1", false)
	assertSubcommands(t, filter, "/parent between else", true)
	assertSubcommands(t, filter, "/parent else", true)
	assertSubcommands(t, filter, "/other", true)
	assertSubcommands(t, filter, "/other deny1", true)
	assertSubcommands(t, filter, "/other between", true)
	assertSubcommands(t, filter, "/other between subdeny1", true)
	assertSubcommands(t, filter, "/other between else", true)
	filter = commandFilter{permitted: []string{"/permit1", "/permitparent permit1", "/permitparent between subpermit1"}, denied: []string{"/deny1", "/denyparent deny1", "/denyparent between subdeny1"}}
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/denyparent", false)
	assertSubcommands(t, filter, "/denyparent deny1", false)
	assertSubcommands(t, filter, "/denyparent else", false)
	assertSubcommands(t, filter, "/denyparent between", false)
	assertSubcommands(t, filter, "/denyparent between subdeny1", false)
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/permitparent", false)
	assertSubcommands(t, filter, "/permitparent permit1", true)
	assertSubcommands(t, filter, "/permitparent else", false)
	assertSubcommands(t, filter, "/permitparent between", false)
	assertSubcommands(t, filter, "/permitparent between subpermit1", true)
	assertSubcommands(t, filter, "/other", false)
	assertSubcommands(t, filter, "/other permit1", false)
	assertSubcommands(t, filter, "/other between", false)
	assertSubcommands(t, filter, "/other between subpermit1", false)
	filter = commandFilter{permitted: []string{"/permit1", "/parent permit1", "/parent between sub", "/parent between strange"}, denied: []string{"/deny1", "/parent deny1", "/parent between subdeny1", "/parent between", "/parent between strange", "/parent between strange sub"}}
	assertSubcommands(t, filter, "/deny1", false)
	assertSubcommands(t, filter, "/parent", false)
	assertSubcommands(t, filter, "/parent deny1", false)
	assertSubcommands(t, filter, "/parent else", false)
	assertSubcommands(t, filter, "/parent permit1", true)
	assertSubcommands(t, filter, "/parent between", false)
	assertSubcommands(t, filter, "/parent between sub", true)
	assertSubcommands(t, filter, "/parent between other", false)
	assertSubcommands(t, filter, "/parent between strange", true)
	assertSubcommands(t, filter, "/parent between strange sub", true)
	assertSubcommands(t, filter, "/parent between strange other", true)
	assertSubcommands(t, filter, "/permit1", true)
	assertSubcommands(t, filter, "/permit1 deny1", true)
	assertSubcommands(t, filter, "/other", false)
	assertSubcommands(t, filter, "/other permit1", false)
	assertSubcommands(t, filter, "/other between", false)
	assertSubcommands(t, filter, "/other between subpermit1", false)
}

func assertSubcommands(t *testing.T, f commandFilter, root string, want bool) {
	t.Helper()
	for _, sep := range []string{" ", "  ", "\t", " \t", "\n", "\r\n"} {
		for _, line := range []string{root, root + " _subcmd", root + " _subcmd _another"} {
			line = strings.ReplaceAll(line, " ", sep)
			if got := f.allows(line); got != want {
				t.Errorf("%+v allows %q = %v, want %v", f, line, got, want)
			}
		}
	}
}
