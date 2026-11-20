package worldguardplugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard/internal/chat"
)

// wgCommand is a command in the style of WorldGuard's command framework:
// whitespace separated arguments with single-letter flags. WorldGuard:
// sk89q's @Command and @CommandPermissions
type wgCommand struct {
	aliases []string
	usage   string
	desc    string
	// flags are the flag letters; a letter followed by ':' takes a value.
	flags    string
	min, max int // max -1 for no limit
	// perm is checked before running; empty for none.
	perm string
	run  func(wg *WorldGuard, a *cmdArgs) error
	// complete suggests the argument at index i, nil for player names.
	complete func(wg *WorldGuard, a *cmdArgs, i int) []string
	// group, when set, holds nested sub-commands that take the words.
	group *commandGroup
}

// cmdArgs is a parsed command line. WorldGuard: CommandContext
type cmdArgs struct {
	wg     *WorldGuard
	sender plugin.CommandSender
	// label is the alias the command ran by, with any parent's ("wg debug").
	label string
	args  []string
	flags map[byte]string
}

func (a *cmdArgs) has(flag byte) bool { _, ok := a.flags[flag]; return ok }
func (a *cmdArgs) len() int           { return len(a.args) }

// value answers a value flag, false when not given.
func (a *cmdArgs) value(flag byte) (string, bool) { v, ok := a.flags[flag]; return v, ok }

// arg answers argument i, "" when there are fewer.
func (a *cmdArgs) arg(i int) string {
	if i < len(a.args) {
		return a.args[i]
	}
	return ""
}

// joined answers the arguments from i on, joined by spaces.
func (a *cmdArgs) joined(i int) string { return strings.Join(a.args[i:], " ") }

// errUsage is a usage failure; the usage line follows its message.
type errUsage struct{ msg string }

func (e errUsage) Error() string { return e.msg }

// errNoPermission is the framework's answer to a missing
// @CommandPermissions node.
var errNoPermission error = cmdMsg("You don't have permission.")

// errPermission is checkPermission's answer inside a command.
var errPermission error = cmdMsg("You don't have permission to do that.")

// cmdMsg is a CommandException: a message shown to the sender in red.
type cmdMsg string

func (e cmdMsg) Error() string { return string(e) }

func cmdError(msg string) error { return cmdMsg(msg) }

func errorf(format string, args ...any) error { return cmdMsg(fmt.Sprintf(format, args...)) }

// parseCmdArgs splits a line into arguments and flags as the sk89q
// framework does: quoted words are joined, "-abc" sets flags a, b and c, a
// flag declared with ':' takes the next word, and a negative number is an
// argument. Undeclared flags are kept for execute to refuse.
// WorldGuard: CommandContext
func parseCmdArgs(words []string, flagSpec string) (*cmdArgs, error) {
	a := &cmdArgs{flags: map[byte]string{}}
	words = joinQuoted(words)
	for i := 0; i < len(words); i++ {
		w := words[i]
		if len(w) < 2 || w[0] != '-' || !isLetters(w[1:]) {
			a.args = append(a.args, w)
			continue
		}
		for j := 1; j < len(w); j++ {
			c := w[j]
			k := strings.IndexByte(flagSpec, c)
			if k < 0 || k+1 >= len(flagSpec) || flagSpec[k+1] != ':' {
				a.flags[c] = ""
				continue
			}
			if _, dup := a.flags[c]; dup {
				return nil, cmdError("Value flag '" + string(c) + "' already given")
			}
			if i+1 >= len(words) {
				return nil, cmdError("No value specified for the '-" + string(c) + "' flag.")
			}
			i++
			a.flags[c] = words[i]
		}
	}
	return a, nil
}

// joinQuoted joins words from one starting with " or ' to one ending with
// it. WorldGuard: CommandContext
func joinQuoted(words []string) []string {
	var out []string
	for i := 0; i < len(words); i++ {
		arg := words[i]
		if arg == "" {
			continue
		}
		if q := arg[0]; q == '"' || q == '\'' {
			var b strings.Builder
			end := i
			for ; end < len(words); end++ {
				w := words[end]
				if len(w) > 1 && w[len(w)-1] == q {
					start := 0
					if end == i {
						start = 1
					} else {
						b.WriteByte(' ')
					}
					b.WriteString(w[start : len(w)-1])
					break
				}
				if end == i {
					b.WriteString(w[1:])
				} else {
					b.WriteByte(' ')
					b.WriteString(w)
				}
			}
			if end < len(words) {
				arg, i = b.String(), end
			}
			if arg == "" {
				continue
			}
		}
		out = append(out, arg)
	}
	return out
}

func isLetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < 'a' || s[i] > 'z') && (s[i] < 'A' || s[i] > 'Z') {
			return false
		}
	}
	return true
}

// usageLine answers "/label [-flags] usage".
func (c *wgCommand) usageLine(label string) string {
	var plain []byte
	for i := 0; i < len(c.flags); i++ {
		if c.flags[i] == ':' {
			continue
		}
		if i+1 < len(c.flags) && c.flags[i+1] == ':' {
			continue
		}
		plain = append(plain, c.flags[i])
	}
	out := "/" + label
	if len(plain) > 0 {
		out += " [-" + string(plain) + "]"
	}
	if c.usage != "" {
		out += " " + c.usage
	}
	return out
}

// execute runs the command for sender with the words after label.
func (c *wgCommand) execute(wg *WorldGuard, sender plugin.CommandSender, label string, words []string) error {
	if c.group != nil {
		c.group.execute(wg, sender, label, words)
		return nil
	}
	if c.perm != "" && !wg.HasPermission(sender, c.perm) {
		return errNoPermission
	}
	a, err := parseCmdArgs(words, c.flags)
	if err != nil {
		return err
	}
	a.wg, a.sender, a.label = wg, sender, label
	switch {
	case len(a.args) < c.min:
		return errUsage{"Too few arguments."}
	case c.max >= 0 && len(a.args) > c.max:
		return errUsage{"Too many arguments."}
	}
	for _, f := range slices.Sorted(maps.Keys(a.flags)) {
		if strings.IndexByte(c.flags, f) < 0 {
			return errUsage{"Unknown flag: " + string(f)}
		}
	}
	return c.run(wg, a)
}

// report shows err to the sender as WorldGuardPlugin.onCommand does.
func (c *wgCommand) report(sender plugin.CommandSender, label string, err error) {
	if err == nil {
		return
	}
	sender.SendMessage(chat.Error(err.Error()))
	if _, ok := errors.AsType[errUsage](err); ok {
		sender.SendMessage(chat.Error(c.usageLine(label)))
	}
}

// node answers the golem command node that runs c under the root name.
func (c *wgCommand) runner(wg *WorldGuard, label string) func(ctx *plugin.CommandContext) (int, error) {
	return func(ctx *plugin.CommandContext) (int, error) {
		err := c.execute(wg, ctx.Sender, label, strings.Fields(plugin.Arg[string](ctx, "args")))
		c.report(ctx.Sender, label, err)
		if err != nil {
			return 0, plugin.ErrHandled
		}
		return 1, nil
	}
}

// suggest completes the last word of a partial argument line.
func (c *wgCommand) suggest(wg *WorldGuard, sender plugin.CommandSender, partial string) []string {
	if c.group != nil {
		return c.group.suggest(wg, sender, partial)
	}
	if c.perm != "" && !wg.HasPermission(sender, c.perm) {
		return nil
	}
	words := strings.Split(partial, " ")
	last := words[len(words)-1]
	a, err := parseCmdArgs(words[:len(words)-1], c.flags)
	if err != nil {
		return nil
	}
	a.wg, a.sender = wg, sender
	prefix := strings.Join(words[:len(words)-1], " ")
	if prefix != "" {
		prefix += " "
	}
	var out []string
	for _, o := range c.options(wg, a, words) {
		if strings.HasPrefix(strings.ToLower(o), strings.ToLower(last)) {
			out = append(out, prefix+o)
		}
	}
	return out
}

// options answers what the last of words may be: a value flag's values,
// the flags, or the next argument.
func (c *wgCommand) options(wg *WorldGuard, a *cmdArgs, words []string) []string {
	if n := len(words); n > 1 {
		if prev := words[n-2]; len(prev) == 2 && prev[0] == '-' {
			if k := strings.IndexByte(c.flags, prev[1]); k >= 0 && k+1 < len(c.flags) && c.flags[k+1] == ':' {
				return wg.flagValueOptions(prev[1])
			}
		}
	}
	if strings.HasPrefix(words[len(words)-1], "-") {
		var out []string
		for j := 0; j < len(c.flags); j++ {
			if c.flags[j] != ':' {
				out = append(out, "-"+string(c.flags[j]))
			}
		}
		return out
	}
	i := len(a.args)
	if c.max >= 0 && i >= c.max {
		return nil
	}
	if c.complete != nil {
		return c.complete(wg, a, i)
	}
	return wg.playerNames()
}

// registerCommand registers c under each alias; a name another plugin
// holds is skipped with a warning.
func (wg *WorldGuard) registerCommand(c *wgCommand) {
	for _, name := range c.aliases {
		run := c.runner(wg, name)
		err := wg.Server.Commands().Register(plugin.Command{
			Name: name, Description: c.desc, Usage: c.usageLine(name),
			Run: run, Children: []plugin.Node{{
				Arg: plugin.GreedyArg("args"), Run: run,
				Suggest: func(ctx *plugin.CommandContext, partial string) []string {
					return c.suggest(wg, ctx.Sender, partial)
				},
			}},
		})
		if err != nil {
			wg.Log.Warn("Could not register /" + name + ": " + err.Error())
		}
	}
}

// commandGroup is a command whose first word picks a subcommand, as /wg.
// WorldGuard: @NestedCommand
type commandGroup struct {
	aliases []string
	desc    string
	subs    []*wgCommand
}

func (g *commandGroup) find(name string) *wgCommand {
	for _, c := range g.subs {
		for _, a := range c.aliases {
			if strings.EqualFold(a, name) {
				return c
			}
		}
	}
	return nil
}

// usage answers "/wg <version|reload|...>".
func (g *commandGroup) usage(label string) string {
	var names []string
	for _, c := range g.subs {
		names = append(names, c.aliases[0])
	}
	return "/" + label + " <" + strings.Join(names, "|") + ">"
}

func (g *commandGroup) execute(wg *WorldGuard, sender plugin.CommandSender, label string, words []string) {
	if len(words) == 0 {
		sender.SendMessage(chat.Error(g.usage(label)))
		return
	}
	c := g.find(words[0])
	if c == nil {
		sender.SendMessage(chat.Error(g.usage(label)))
		return
	}
	sub := label + " " + strings.ToLower(words[0])
	c.report(sender, sub, c.execute(wg, sender, sub, words[1:]))
}

func (g *commandGroup) suggest(wg *WorldGuard, sender plugin.CommandSender, partial string) []string {
	first, rest, more := strings.Cut(partial, " ")
	if !more {
		var out []string
		for _, c := range g.subs {
			if (c.perm == "" || wg.HasPermission(sender, c.perm)) && strings.HasPrefix(c.aliases[0], strings.ToLower(first)) {
				out = append(out, c.aliases[0])
			}
		}
		return out
	}
	c := g.find(first)
	if c == nil {
		return nil
	}
	var out []string
	for _, s := range c.suggest(wg, sender, rest) {
		out = append(out, first+" "+s)
	}
	return out
}

func (wg *WorldGuard) registerGroup(g *commandGroup) {
	for _, name := range g.aliases {
		label := name
		run := func(ctx *plugin.CommandContext) (int, error) {
			g.execute(wg, ctx.Sender, label, strings.Fields(plugin.Arg[string](ctx, "args")))
			return 1, nil
		}
		err := wg.Server.Commands().Register(plugin.Command{
			Name: name, Description: g.desc, Usage: g.usage(name),
			Run: run, Children: []plugin.Node{{
				Arg: plugin.GreedyArg("args"), Run: run,
				Suggest: func(ctx *plugin.CommandContext, partial string) []string {
					return g.suggest(wg, ctx.Sender, partial)
				},
			}},
		})
		if err != nil {
			wg.Log.Warn("Could not register /" + name + ": " + err.Error())
		}
	}
}

// checkPlayer answers the sender as a player. WorldGuard:
// WorldGuard.checkPlayer
func checkPlayer(s plugin.CommandSender) (plugin.Player, error) {
	if p, ok := s.(plugin.Player); ok {
		return p, nil
	}
	return nil, cmdError("A player is expected.")
}

// displayName answers the name a sender is called by in messages.
func displayName(s plugin.CommandSender) string {
	if p, ok := s.(plugin.Player); ok {
		return p.DisplayName().PlainString()
	}
	return s.Name()
}

// argInt reads argument i as an integer. WorldGuard:
// CommandContext.getInteger
func (a *cmdArgs) argInt(i int) (int, error) { return parseInt(a.args[i]) }

// parseInt reads a number argument or flag value, with
// WorldGuardExceptionConverter's message for anything else.
func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, cmdError("Number expected; string \"" + s + "\" given.")
	}
	return n, nil
}

// flagValueOptions completes a value flag: worlds for -w, region groups for
// -g, players for -p.
func (wg *WorldGuard) flagValueOptions(flag byte) []string {
	switch flag {
	case 'w':
		return wg.worldNamesFor()
	case 'g':
		return []string{"members", "owners", "nonmembers", "nonowners", "everyone", "none"}
	case 'p':
		return wg.playerNames()
	}
	return nil
}
