package blacklist

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard/internal/chat"
)

// entry is a heading item's options and actions. WorldGuard: BlacklistEntry
type entry struct {
	b            *Blacklist
	ignoreGroups map[string]bool
	ignorePerms  []string
	actions      map[EventType][]action

	message, comment       string
	hasMessage, hasComment bool
}

// ignores reports whether the entry leaves the player alone.
// WorldGuard: BlacklistEntry.shouldIgnore
func (en *entry) ignores(p Player) bool {
	if p == nil {
		return false
	}
	if en.ignoreGroups != nil {
		for _, g := range p.Groups() {
			if en.ignoreGroups[strings.ToLower(g)] {
				return true
			}
		}
	}
	return slices.ContainsFunc(en.ignorePerms, p.HasPermission)
}

// check runs the entry's actions for the event. WorldGuard: BlacklistEntry.check
func (en *entry) check(e Event, forceRepeat, silent bool) bool {
	if en.ignores(e.Player) {
		return true
	}
	repeating := en.b.repeated(e)
	ok := !en.b.whitelist
	for _, a := range en.actions[e.Type] {
		switch a(en, e, silent, repeating, forceRepeat) {
		case allow:
			ok = true
		case deny:
			ok = false
		case allowOverride:
			return true
		case denyOverride:
			return false
		}
	}
	return ok
}

// result is what an action makes of the event. WorldGuard: ActionResult
type result int

const (
	inherit result = iota
	deny
	allow
	denyOverride
	allowOverride
)

// action is one of an entry's actions. WorldGuard: Action
type action func(en *entry, e Event, silent, repeating, forceRepeat bool) result

var actionNames = map[string]action{
	"allow": func(_ *entry, _ Event, silent, _, _ bool) result {
		if silent {
			return allowOverride
		}
		return allow
	},
	"deny": func(_ *entry, _ Event, silent, _, _ bool) result {
		if silent {
			return denyOverride
		}
		return deny
	},
	"ban": func(en *entry, e Event, silent, _, _ bool) result {
		if !silent && e.Player != nil {
			e.Player.Ban(chat.Legacy("Banned: " + en.reason(e)))
		}
		return inherit
	},
	"kick": func(en *entry, e Event, silent, _, _ bool) result {
		if !silent && e.Player != nil {
			e.Player.Kick(chat.Legacy(en.reason(e)))
		}
		return inherit
	},
	"log": unrepeated(func(en *entry, e Event) {
		for _, l := range en.b.Loggers {
			l.Log(e, en.comment, en.hasComment)
		}
	}),
	"notify": unrepeated(func(en *entry, e Event) {
		if en.b.Notify == nil {
			return
		}
		msg := chat.Notify(e.causeName(), " ("+e.Type.Description()+") ").Append(e.Target.Name)
		if en.hasComment {
			msg = msg.Append(plugin.Plain(" (" + en.comment + ")"))
		}
		en.b.Notify(msg)
	}),
	"tell": unrepeated(func(en *entry, e Event) {
		if e.Player == nil {
			return
		}
		if en.hasMessage {
			msg := strings.ReplaceAll(en.message, `\n`, "\n")
			e.Player.SendMessage(chat.Legacy(format(msg, e.Target.plainName())))
			return
		}
		e.Player.SendMessage(plugin.Plain("").Append(
			plugin.Plain("You're not allowed to "+e.Type.Description()+" "), e.Target.Name, plugin.Plain(".")).Color("red"))
	}),
}

// unrepeated is an action that keeps quiet for a repeated event unless
// forced, and when silent. WorldGuard: RepeatGuardedAction
func unrepeated(f func(en *entry, e Event)) action {
	return func(en *entry, e Event, silent, repeating, forceRepeat bool) result {
		if (!repeating || forceRepeat) && !silent {
			f(en, e)
		}
		return inherit
	}
}

// reason is the kick or ban message: the entry's, or "You can't <verb> <name>".
func (en *entry) reason(e Event) string {
	if en.hasMessage {
		return format(en.message, e.Target.plainName())
	}
	return "You can't " + e.Type.Description() + " " + e.Target.plainName()
}

// format fills a message's %s with the target's name, as Java's
// String.format with one argument: %% is a percent sign.
func format(msg, name string) string {
	var b strings.Builder
	used := false
	for i := 0; i < len(msg); i++ {
		if msg[i] == '%' && i+1 < len(msg) {
			switch msg[i+1] {
			case '%':
				b.WriteByte('%')
				i++
				continue
			case 's':
				if !used {
					b.WriteString(name)
					used = true
					i++
					continue
				}
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			}
		}
		b.WriteByte(msg[i])
	}
	return b.String()
}

// parseActions reads a comma-separated list of action names.
// WorldGuard: Blacklist.parseActions
func parseActions(raw string, warn func(string)) []action {
	var out []action
	for name := range strings.SplitSeq(raw, ",") {
		name = strings.TrimSpace(name)
		a, ok := actionNames[strings.ToLower(name)]
		if !ok {
			warn("Unknown blacklist action: " + name)
			continue
		}
		out = append(out, a)
	}
	return out
}
