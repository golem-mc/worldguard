package worldguardplugin

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/chat"
)

// flagHelper is the clickable flag table of /rg flags. WorldGuard:
// FlagHelperBox
type flagHelper struct {
	world     string
	region    *worldguard.Region
	perms     *permModel
	monoSpace bool
	page      int
}

// helperFlags answers the flags, built-in ones in their order, then the
// others by name.
func helperFlags() (flags []worldguard.Flag, builtin int) {
	all := worldguard.Flags.All()
	var in, third []worldguard.Flag
	for _, f := range all {
		if worldguard.IsBuiltinFlag(f.Name()) {
			in = append(in, f)
		} else {
			third = append(third, f)
		}
	}
	order := worldguard.BuiltinFlagNames()
	slices.SortFunc(in, func(a, b worldguard.Flag) int {
		return slices.Index(order, a.Name()) - slices.Index(order, b.Name())
	})
	slices.SortFunc(third, func(a, b worldguard.Flag) int { return strings.Compare(a.Name(), b.Name()) })
	return append(in, third...), len(in)
}

// pxLength answers a name's width in pixels of the chat font.
// WorldGuard: FlagHelperBox.FlagFontInfo
func pxLength(s string) int {
	n := 0
	for _, c := range s {
		n += pxWidth(c) + 1
	}
	return n
}

func pxWidth(c rune) int {
	switch c {
	case 'i', ':':
		return 1
	case 'l':
		return 2
	case '*', 't':
		return 3
	case 'f', 'k':
		return 4
	}
	return 5
}

// text answers page of the table.
func (h *flagHelper) text(page int) (plugin.Text, error) {
	h.page = page
	flags, builtin := helperFlags()
	size := len(flags)
	if size != builtin {
		size++
	}
	perPage := 18
	return paginate("Flags for "+h.region.ID(), fmt.Sprintf("/rg flags -w \"%s\" -p %%page%% %s", h.world, h.region.ID()),
		size, perPage, page, func(i int) plugin.Text {
			if i == builtin {
				return centerAndBorder(txt("Third-Party Flags", "aqua"))
			}
			if i > builtin {
				i--
			}
			return h.row(flags[i], i >= builtin)
		})
}

func (h *flagHelper) row(f worldguard.Flag, thirdParty bool) plugin.Text {
	color := "gold"
	if _, unknown := f.(*worldguard.UnknownFlag); unknown {
		color = "gray"
	} else if thirdParty {
		color = "light_purple"
	}
	parts := []plugin.Text{txt(f.Name(), color)}
	length := pxLength(f.Name())
	if h.monoSpace {
		length = len(f.Name())
	}
	star := func(hover string) {
		parts = append(parts, txt("*", "aqua").OnHover(plugin.Plain(hover)))
		if h.monoSpace {
			length++
		} else {
			length += pxWidth('*')
		}
	}
	if f.UsesMembershipAsDefault() {
		star("This is a special flag which defaults to allow for members, and deny for non-members")
	}
	if f == worldguard.FlagPassthrough {
		star("This is a special flag which overrides build checks. (Not movement related!)")
	}
	leftover := 180 - length
	if h.monoSpace {
		leftover = 60 - length
	}
	parts = append(parts, plugin.Plain(" "))
	if leftover -= 4; leftover > 0 {
		n := leftover / 2
		if h.monoSpace {
			n = leftover
		}
		parts = append(parts, txt(strings.Repeat(".", n), "dark_gray"))
	}
	return line(append(parts, h.value(f)...)...)
}

// inherited answers the nearest parent's value of f.
func (h *flagHelper) inherited(f worldguard.Flag) any {
	for p := h.region.Parent(); p != nil; p = p.Parent() {
		if v := p.Flag(f); v != nil {
			return v
		}
	}
	return nil
}

func (h *flagHelper) command(f worldguard.Flag, choice string) string {
	return fmt.Sprintf("/rg flag -w \"%s\" -h %d %s %s %s", h.world, h.page, h.region.ID(), f.Name(), choice)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	s = strings.ToLower(s)
	return strings.ToUpper(s[:1]) + s[1:]
}

// hint answers the tooltip line telling a default or inherited value.
func hint(isDefault, inherited bool) *plugin.Text {
	var t plugin.Text
	switch {
	case inherited && isDefault:
		t = line(plugin.Plain("Inherited & "), plugin.Plain("default").Underlined(), plugin.Plain(" value"))
	case inherited:
		t = plugin.Plain("Inherited value")
	case isDefault:
		t = line(plugin.Plain("Default").Underlined(), plugin.Plain(" value"))
	default:
		return nil
	}
	return &t
}

func joinHover(lines []plugin.Text) plugin.Text {
	var parts []plugin.Text
	for i, l := range lines {
		if i > 0 {
			parts = append(parts, plugin.Plain("\n"))
		}
		parts = append(parts, l)
	}
	return line(parts...)
}

// choices shows each choice, clickable to set or unset it.
// FlagHelperBox.appendValueChoices
func (h *flagHelper) choices(f worldguard.Flag, choices []any, suggest string) []plugin.Text {
	def := f.Default()
	cur := h.region.Flag(f)
	inherited := false
	if cur == nil {
		if cur = h.inherited(f); cur != nil {
			inherited = true
		}
	}
	var out []plugin.Text
	for _, choice := range choices {
		explicit := cur == choice && !inherited
		var value *string
		if !explicit {
			s := fmt.Sprint(choice)
			value = &s
		}
		maySet := h.perms.maySetFlagValue(h.region, f, value)
		color := "dark_gray"
		switch {
		case explicit:
			color = "white"
		case inherited && cur == choice:
			color = "gray"
		}
		label := txt(capitalize(worldguard.FormatFlagValue(f, choice)), color)
		if choice == def {
			label = label.Underlined()
		}
		var hover []plugin.Text
		if maySet {
			switch {
			case explicit:
				hover = append(hover, txt("Click to unset", "gold"))
			case dangerZone(f) && (h.region.ID() != worldguard.GlobalRegionID || f != worldguard.FlagPassthrough):
				hover = append(hover, txt("Setting this flag may have unintended consequences.\n", "red").Append(
					plugin.Plain("Please read the documentation and set this flag manually if you really intend to.\n"+
						"(Hint: You do not need to set this to protect the region!)")))
			default:
				hover = append(hover, txt("Click to set", "gold"))
			}
		}
		if t := hint(choice == def, inherited); t != nil {
			hover = append(hover, *t)
		}
		t := line(label)
		if len(hover) > 0 {
			t = t.OnHover(joinHover(hover))
		}
		if maySet && (explicit || !dangerZone(f)) {
			arg := ""
			if !explicit {
				arg = fmt.Sprint(choice)
			}
			t = t.OnClick(plugin.RunCommand, h.command(f, arg))
		}
		out = append(out, t, plugin.Plain(" "))
	}
	if suggest != "" && h.perms.maySetFlag(h.region, f) {
		out = append(out, txt(suggest, "dark_gray").OnHover(txt("Click to set custom value", "gold")).
			OnClick(plugin.SuggestCommand, h.command(f, "")))
	}
	return out
}

// valueText shows a value, clickable to change it.
// FlagHelperBox.appendValueText
func (h *flagHelper) valueText(f worldguard.Flag, display string, extra *plugin.Text) []plugin.Text {
	def := f.Default()
	cur := h.region.Flag(f)
	inherited := false
	if cur == nil {
		if cur = h.inherited(f); cur != nil {
			inherited = true
		}
	}
	explicit := cur != nil && !inherited
	_, unknown := f.(*worldguard.UnknownFlag)
	maySet := !unknown && h.perms.maySetFlag(h.region, f)
	color := "dark_gray"
	switch {
	case explicit:
		color = "white"
	case inherited:
		color = "gray"
	}
	t := txt(display, color)
	isDefault := fmt.Sprint(cur) == fmt.Sprint(def) && (cur == nil) == (def == nil)
	if isDefault {
		t = t.Underlined()
	}
	var hover []plugin.Text
	if maySet {
		if explicit {
			hover = append(hover, txt("Click to change", "gold"))
		} else {
			hover = append(hover, txt("Click to set", "gold"))
		}
	}
	if x := hint(isDefault, inherited); x != nil {
		hover = append(hover, *x)
	}
	out := line(t)
	if len(hover) > 0 {
		hv := joinHover(hover)
		if extra != nil {
			hv = line(hv, plugin.Plain("\n"), *extra)
		}
		out = out.OnHover(hv)
	}
	if maySet {
		out = out.OnClick(plugin.SuggestCommand, h.command(f, ""))
	}
	return []plugin.Text{out, plugin.Plain(" ")}
}

// shorten cuts a display value to 23 characters.
func shorten(s string) string {
	if len([]rune(s)) > 23 {
		return string([]rune(s)[:20]) + "..."
	}
	return s
}

func (h *flagHelper) current(f worldguard.Flag) any {
	if v := h.region.Flag(f); v != nil {
		return v
	}
	return h.inherited(f)
}

// value shows a flag's value by the flag's kind.
// FlagHelperBox.appendFlagValue
func (h *flagHelper) value(f worldguard.Flag) []plugin.Text {
	switch f := f.(type) {
	case *worldguard.StateFlag:
		return h.choices(f, []any{worldguard.StateAllow, worldguard.StateDeny}, "")
	case *worldguard.BooleanFlag:
		return h.choices(f, []any{true, false}, "")
	case *worldguard.SetFlag:
		sub := strings.TrimSuffix(strings.TrimPrefix(fmt.Sprintf("%T", f.Sub()), "*worldguard."), "Flag")
		if r, ok := f.Sub().(*worldguard.RegistryFlag); ok {
			sub = r.Registry().Name()
		}
		cur, _ := h.current(f).([]any)
		display := sub
		var extra *plugin.Text
		if cur != nil {
			display = fmt.Sprintf("%dx %s", len(cur), sub)
			vals := strings.Trim(worldguard.FormatFlagValue(f, cur), "[]")
			t := plugin.Plain("Current values:\n" + strings.ReplaceAll(vals, ", ", ","))
			extra = &t
		}
		return h.valueText(f, display, extra)
	case *worldguard.RegistryFlag:
		display := f.Registry().Name()
		if cur := h.current(f); cur != nil {
			display = f.Registry().ID(cur)
		}
		return h.valueText(f, display, nil)
	case *worldguard.StringFlag:
		cur, ok := h.current(f).(string)
		label := "Current value:\n"
		if !ok {
			def, _ := f.Default().(string)
			if def == "" {
				return h.valueText(f, "unset string", nil)
			}
			cur, label = def, "Default value:\n"
		}
		extra := line(plugin.Plain(label), chat.Legacy(cur))
		return h.valueText(f, shorten(strings.ReplaceAll(chat.Strip(cur), "\n", "\\n")), &extra)
	case *worldguard.LocationFlag:
		cur, ok := h.current(f).(worldguard.Location)
		if !ok {
			return h.valueText(f, "unset location", nil)
		}
		extra := plugin.Plain("Current value:\n" + cur.String())
		return h.valueText(f, cur.String(), &extra)
	case worldguard.NumberFlag:
		var choices []any
		seen := map[string]bool{}
		addChoice := func(v any) {
			if v != nil && !seen[fmt.Sprint(v)] {
				seen[fmt.Sprint(v)] = true
				choices = append(choices, v)
			}
		}
		addChoice(h.current(f))
		addChoice(f.Default())
		for _, s := range f.Suggestions() {
			if _, isInt := f.(*worldguard.IntegerFlag); isInt {
				n, _ := strconv.Atoi(s)
				addChoice(n)
			} else {
				x, _ := strconv.ParseFloat(s, 64)
				addChoice(x)
			}
		}
		slices.SortFunc(choices, func(a, b any) int {
			x, y := toFloat(a), toFloat(b)
			switch {
			case x < y:
				return -1
			case x > y:
				return 1
			}
			return 0
		})
		suggest := "[custom]"
		if len(choices) == 0 {
			suggest = "unset number"
		}
		return h.choices(f, choices, suggest)
	}
	display := shorten(fmt.Sprint(h.region.Flag(f)))
	if h.region.Flag(f) == nil {
		display = "<nil>"
	}
	return h.valueText(f, display, nil)
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return 0
}
