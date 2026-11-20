package worldguardplugin

import (
	"errors"
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
}

// cmdArgs is a parsed command line. WorldGuard: CommandContext
type cmdArgs struct {
	sender plugin.CommandSender
	// label is the alias the command ran by, with any parent's ("wg debug").
	label string
	args  []string
	flags map[byte]string
}

func (a *cmdArgs) has(flag byte) bool { _, ok := a.flags[flag]; return ok }
func (a *cmdArgs) len() int           { return len(a.args) }

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

// parseCmdArgs splits a line into arguments and flags as the sk89q
// framework does: "-abc" sets flags a, b and c, a flag declared with ':'
// takes the next word, and a negative number is an argument.
func parseCmdArgs(words []string, flagSpec string) (*cmdArgs, error) {
	a := &cmdArgs{flags: map[byte]string{}}
	for i := 0; i < len(words); i++ {
		w := words[i]
		if len(w) < 2 || w[0] != '-' || !isLetters(w[1:]) {
			a.args = append(a.args, w)
			continue
		}
		for j := 1; j < len(w); j++ {
			c := w[j]
			k := strings.IndexByte(flagSpec, c)
			if k < 0 {
				return nil, errUsage{"Unknown flag: " + string(c)}
			}
			if k+1 < len(flagSpec) && flagSpec[k+1] == ':' {
				if i+1 >= len(words) {
					return nil, errUsage{"No value specified for the '-" + string(c) + "' flag."}
				}
				i++
				a.flags[c] = words[i]
				continue
			}
			a.flags[c] = ""
		}
	}
	return a, nil
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
	if c.perm != "" && !wg.HasPermission(sender, c.perm) {
		return errNoPermission
	}
	a, err := parseCmdArgs(words, c.flags)
	if err != nil {
		return err
	}
	a.sender, a.label = sender, label
	switch {
	case len(a.args) < c.min:
		return errUsage{"Too few arguments."}
	case c.max >= 0 && len(a.args) > c.max:
		return errUsage{"Too many arguments."}
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
	if c.perm != "" && !wg.HasPermission(sender, c.perm) {
		return nil
	}
	words := strings.Split(partial, " ")
	last := words[len(words)-1]
	a, err := parseCmdArgs(words[:len(words)-1], c.flags)
	if err != nil || strings.HasPrefix(last, "-") {
		return nil
	}
	a.sender = sender
	i := len(a.args)
	if c.max >= 0 && i >= c.max {
		return nil
	}
	var options []string
	if c.complete != nil {
		options = c.complete(wg, a, i)
	} else {
		for _, p := range wg.Server.Players() {
			options = append(options, p.Name())
		}
	}
	prefix := strings.Join(words[:len(words)-1], " ")
	if prefix != "" {
		prefix += " "
	}
	var out []string
	for _, o := range options {
		if strings.HasPrefix(strings.ToLower(o), strings.ToLower(last)) {
			out = append(out, prefix+o)
		}
	}
	return out
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
func (a *cmdArgs) argInt(i int) (int, error) {
	n, err := strconv.Atoi(a.args[i])
	if err != nil {
		return 0, cmdError("Number expected; string \"" + a.args[i] + "\" given.")
	}
	return n, nil
}
