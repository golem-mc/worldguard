package worldguard

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
)

// UUID identifies a player.
type UUID = plugin.UUID

// State is a state flag's value: allow, deny, or none (unset).
// WorldGuard: StateFlag.State
type State uint8

const (
	StateNone State = iota
	StateAllow
	StateDeny
)

func (s State) String() string {
	switch s {
	case StateAllow:
		return "ALLOW"
	case StateDeny:
		return "DENY"
	}
	return "NONE"
}

// CombineStates answers deny if any state denies, else allow if any allows,
// else none. WorldGuard: StateFlag.combine
func CombineStates(states ...State) State {
	allowed := false
	for _, s := range states {
		if s == StateDeny {
			return StateDeny
		}
		if s == StateAllow {
			allowed = true
		}
	}
	if allowed {
		return StateAllow
	}
	return StateNone
}

// TestStates reports whether the states combine to allow. WorldGuard:
// StateFlag.test
func TestStates(states ...State) bool { return CombineStates(states...) == StateAllow }

// AllowOrNone answers allow for true and none for false.
func AllowOrNone(b bool) State {
	if b {
		return StateAllow
	}
	return StateNone
}

// DenyToNone answers none for deny and s otherwise.
func DenyToNone(s State) State {
	if s == StateDeny {
		return StateNone
	}
	return s
}

func stateOf(v any) State {
	s, _ := v.(State)
	return s
}

// StateFlag is an allow/deny flag. WorldGuard: StateFlag, BuildFlag
type StateFlag struct {
	baseFlag
	def bool
	// build marks the build flag: membership sets it implicitly and decides
	// it by default, allow on the global region is ignored, and a query
	// needs a subject.
	build bool
}

// NewStateFlag answers a state flag that allows by default when def is set.
func NewStateFlag(name string, def bool, opts ...FlagOption) *StateFlag {
	return &StateFlag{baseFlag: base(name, opts), def: def}
}

func (f *StateFlag) Default() any {
	if f.def {
		return StateAllow
	}
	return nil
}

func (f *StateFlag) HasConflictStrategy() bool         { return true }
func (f *StateFlag) ImplicitlySetWithMembership() bool { return f.build }
func (f *StateFlag) UsesMembershipAsDefault() bool     { return f.build }
func (f *StateFlag) RequiresSubject() bool             { return f.build }

// PreventsAllowOnGlobal reports whether allow on the global region is
// ignored.
func (f *StateFlag) PreventsAllowOnGlobal() bool { return f.build }

func (f *StateFlag) Choose(values []any) any {
	if len(values) == 0 {
		return nil
	}
	states := make([]State, len(values))
	for i, v := range values {
		states[i] = stateOf(v)
	}
	if s := CombineStates(states...); s != StateNone {
		return s
	}
	return nil
}

func (f *StateFlag) Parse(ctx *FlagContext) (any, error) {
	switch strings.ToLower(ctx.Input) {
	case "allow":
		return StateAllow, nil
	case "deny":
		return StateDeny, nil
	case "none":
		return nil, nil
	}
	return nil, InvalidFlagFormat("Expected none/allow/deny but got '%s'", ctx.Input)
}

func (f *StateFlag) Unmarshal(raw any) any {
	switch strings.ToLower(fmt.Sprint(raw)) {
	case "allow":
		return StateAllow
	case "deny":
		return StateDeny
	}
	return nil
}

func (f *StateFlag) Marshal(v any) any {
	switch stateOf(v) {
	case StateAllow:
		return "allow"
	case StateDeny:
		return "deny"
	}
	return nil
}

// BooleanFlag is a true/false flag. WorldGuard: BooleanFlag
type BooleanFlag struct{ baseFlag }

// NewBooleanFlag answers a boolean flag.
func NewBooleanFlag(name string, opts ...FlagOption) *BooleanFlag {
	return &BooleanFlag{base(name, opts)}
}

func (f *BooleanFlag) Parse(ctx *FlagContext) (any, error) {
	switch strings.ToLower(ctx.Input) {
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	}
	return nil, InvalidFlagFormat("Not a yes/no value: %s", ctx.Input)
}

func (f *BooleanFlag) Unmarshal(raw any) any {
	if b, ok := raw.(bool); ok {
		return b
	}
	return nil
}

func (f *BooleanFlag) Marshal(v any) any { return v }

// StringFlag is a text flag; its input takes \n for a new line and
// WorldGuard's colour macros. WorldGuard: StringFlag
type StringFlag struct {
	baseFlag
	def string
}

// NewStringFlag answers a string flag with def as its default ("" for none).
func NewStringFlag(name, def string, opts ...FlagOption) *StringFlag {
	return &StringFlag{baseFlag: base(name, opts), def: def}
}

func (f *StringFlag) Default() any {
	if f.def == "" {
		return nil
	}
	return f.def
}

func (f *StringFlag) Parse(ctx *FlagContext) (any, error) {
	return ReplaceColorMacros(unescapeNewlines(ctx.Input)), nil
}

func (f *StringFlag) Unmarshal(raw any) any {
	if s, ok := raw.(string); ok {
		return s
	}
	return nil
}

func (f *StringFlag) Marshal(v any) any { return v }

// unescapeNewlines turns \n into a new line and \\n into \n.
// WorldGuard: StringFlag.parseInput's two replacements
func unescapeNewlines(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], `\\n`):
			b.WriteString(`\n`)
			i += 2
		case strings.HasPrefix(s[i:], `\n`):
			b.WriteByte('\n')
			i++
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// colorMacros are WorldGuard's backtick colour shortcuts.
var colorMacros = strings.NewReplacer(
	"`r", "&c", "`R", "&4", "`y", "&e", "`Y", "&6", "`g", "&a", "`G", "&2",
	"`c", "&b", "`C", "&3", "`b", "&9", "`B", "&1", "`p", "&d", "`P", "&5",
	"`0", "&0", "`1", "&8", "`2", "&7", "`w", "&F", "`k", "&k", "`l", "&l",
	"`m", "&m", "`n", "&n", "`o", "&o", "`x", "&r")

// ReplaceColorMacros turns the backtick macros and &-codes into §-codes.
// WorldGuard: CommandUtils.replaceColorMacros
func ReplaceColorMacros(s string) string {
	s = colorMacros.Replace(s)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '&' && i+1 < len(s) && strings.IndexByte("0123456789abcdefklmnorABCDEFKLMNOR", s[i+1]) >= 0 {
			b.WriteString("§")
			b.WriteByte(strings.ToLower(s[i+1 : i+2])[0])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// CommandStringFlag is a command, stored lower case with its slash.
// WorldGuard: CommandStringFlag
type CommandStringFlag struct{ baseFlag }

// NewCommandStringFlag answers a command flag.
func NewCommandStringFlag(name string, opts ...FlagOption) *CommandStringFlag {
	return &CommandStringFlag{base(name, opts)}
}

func (f *CommandStringFlag) Parse(ctx *FlagContext) (any, error) {
	in := strings.TrimSpace(ctx.Input)
	if !strings.HasPrefix(in, "/") {
		in = "/" + in
	}
	return strings.ToLower(in), nil
}

func (f *CommandStringFlag) Unmarshal(raw any) any {
	if s, ok := raw.(string); ok {
		return s
	}
	return nil
}

func (f *CommandStringFlag) Marshal(v any) any { return v }

// NumberFlag is a flag holding a number, with values /rg flag suggests.
type NumberFlag interface {
	Flag
	Suggestions() []string
}

// IntegerFlag is a whole number flag. WorldGuard: IntegerFlag
type IntegerFlag struct {
	baseFlag
	suggest []string
}

// NewIntegerFlag answers an integer flag.
func NewIntegerFlag(name string, opts ...FlagOption) *IntegerFlag {
	return &IntegerFlag{baseFlag: base(name, opts)}
}

// SetSuggestions sets the values /rg flag suggests.
func (f *IntegerFlag) SetSuggestions(values ...string) *IntegerFlag { f.suggest = values; return f }
func (f *IntegerFlag) Suggestions() []string                        { return f.suggest }

func (f *IntegerFlag) Parse(ctx *FlagContext) (any, error) { return ctx.int() }

func (f *IntegerFlag) Unmarshal(raw any) any {
	switch n := raw.(type) {
	case int:
		return n
	case int64:
		return int(int32(n))
	case uint64:
		return int(int32(n))
	case float64:
		return int(int32(n))
	}
	return nil
}

func (f *IntegerFlag) Marshal(v any) any { return v }

// DoubleFlag is a decimal number flag. WorldGuard: DoubleFlag
type DoubleFlag struct {
	baseFlag
	suggest []string
}

// NewDoubleFlag answers a double flag.
func NewDoubleFlag(name string, opts ...FlagOption) *DoubleFlag {
	return &DoubleFlag{baseFlag: base(name, opts)}
}

// SetSuggestions sets the values /rg flag suggests.
func (f *DoubleFlag) SetSuggestions(values ...string) *DoubleFlag { f.suggest = values; return f }
func (f *DoubleFlag) Suggestions() []string                       { return f.suggest }

func (f *DoubleFlag) Parse(ctx *FlagContext) (any, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(ctx.Input), 64)
	if err != nil {
		return nil, InvalidFlagFormat("Not a number: %s", ctx.Input)
	}
	return v, nil
}

func (f *DoubleFlag) Unmarshal(raw any) any { return toFloat(raw) }

func (f *DoubleFlag) Marshal(v any) any { return v }

func toFloat(raw any) any {
	switch n := raw.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case float32:
		return float64(n)
	}
	return nil
}

func number(raw any) float64 {
	f, _ := toFloat(raw).(float64)
	return f
}

// Location is a location flag's value: a world by name, which need not be
// loaded, and a position. WorldGuard: LazyLocation
type Location struct {
	World      string
	Pos        plugin.Vec3
	Yaw, Pitch float32
}

// String answers "world, x, y, z" (and pitch, yaw when set), as /rg info
// shows it.
func (l Location) String() string {
	parts := []string{l.World, strconv.Itoa(int(l.Pos.X)), strconv.Itoa(int(l.Pos.Y)), strconv.Itoa(int(l.Pos.Z))}
	if l.Pitch != 0 || l.Yaw != 0 {
		parts = append(parts, strconv.Itoa(int(l.Pitch)), strconv.Itoa(int(l.Yaw)))
	}
	return strings.Join(parts, ", ")
}

// LocationFlag is a location flag. WorldGuard: LocationFlag
type LocationFlag struct{ baseFlag }

// NewLocationFlag answers a location flag.
func NewLocationFlag(name string, opts ...FlagOption) *LocationFlag {
	return &LocationFlag{base(name, opts)}
}

func (f *LocationFlag) Parse(ctx *FlagContext) (any, error) {
	p, err := ctx.player()
	if err != nil {
		return nil, err
	}
	var loc *Location
	if strings.EqualFold(ctx.Input, "here") {
		l := p.Location()
		loc = &Location{World: ctx.World, Pos: l.Pos, Yaw: l.Yaw, Pitch: l.Pitch}
	} else if split := strings.Split(ctx.Input, ","); len(split) >= 3 {
		loc = parseLocation(ctx.World, split)
	}
	if loc == nil {
		return nil, InvalidFlagFormat("Expected 'here' or x,y,z.")
	}
	if ctx.Region != nil && ctx.BoundedLocations && !ctx.Region.Contains(loc.Pos.Block()) {
		if !ctx.MayOverrideBounds {
			return nil, InvalidFlagFormat("You can't set that flag outside of the region boundaries.")
		}
		if ctx.Warn != nil {
			ctx.Warn("WARNING: Flag location is outside of region.")
		}
	}
	return *loc, nil
}

// parseLocation reads x,y,z[,yaw[,pitch]]; nil when a number is invalid.
func parseLocation(world string, split []string) *Location {
	var v [5]float64
	for i := range min(len(split), 5) {
		f, err := strconv.ParseFloat(strings.TrimSpace(split[i]), 64)
		if err != nil {
			return nil
		}
		v[i] = f
	}
	return &Location{World: world, Pos: plugin.Vec3{X: v[0], Y: v[1], Z: v[2]}, Yaw: float32(v[3]), Pitch: float32(v[4])}
}

func (f *LocationFlag) Unmarshal(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	for _, k := range []string{"world", "x", "y", "z", "yaw", "pitch"} {
		if m[k] == nil {
			return nil
		}
	}
	return Location{
		World: fmt.Sprint(m["world"]),
		Pos:   plugin.Vec3{X: number(m["x"]), Y: number(m["y"]), Z: number(m["z"])},
		Yaw:   float32(number(m["yaw"])), Pitch: float32(number(m["pitch"])),
	}
}

func (f *LocationFlag) Marshal(v any) any {
	l, ok := v.(Location)
	if !ok {
		return nil
	}
	return map[string]any{"world": l.World, "x": l.Pos.X, "y": l.Pos.Y, "z": l.Pos.Z,
		"yaw": float64(l.Yaw), "pitch": float64(l.Pitch)}
}

// VectorFlag is a position flag. WorldGuard: VectorFlag
type VectorFlag struct{ baseFlag }

// NewVectorFlag answers a vector flag.
func NewVectorFlag(name string, opts ...FlagOption) *VectorFlag {
	return &VectorFlag{base(name, opts)}
}

func (f *VectorFlag) Parse(ctx *FlagContext) (any, error) {
	if strings.EqualFold(ctx.Input, "here") {
		p, err := ctx.player()
		if err != nil {
			return nil, err
		}
		return p.Pos(), nil
	}
	if split := strings.Split(ctx.Input, ","); len(split) == 3 {
		if l := parseLocation("", split); l != nil {
			return l.Pos, nil
		}
	}
	return nil, InvalidFlagFormat("Expected 'here' or x,y,z.")
}

func (f *VectorFlag) Unmarshal(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok || m["x"] == nil || m["y"] == nil || m["z"] == nil {
		return nil
	}
	return plugin.Vec3{X: number(m["x"]), Y: number(m["y"]), Z: number(m["z"])}
}

func (f *VectorFlag) Marshal(v any) any {
	p, ok := v.(plugin.Vec3)
	if !ok {
		return nil
	}
	return map[string]any{"x": p.X, "y": p.Y, "z": p.Z}
}

// SetFlag is a set of another flag's values, held as a []any without
// repeats. WorldGuard: SetFlag
type SetFlag struct {
	baseFlag
	sub Flag
}

// NewSetFlag answers a set of sub's values.
func NewSetFlag(name string, sub Flag, opts ...FlagOption) *SetFlag {
	return &SetFlag{baseFlag: base(name, opts), sub: sub}
}

// Sub answers the flag each element is parsed with.
func (f *SetFlag) Sub() Flag { return f.sub }

func (f *SetFlag) Parse(ctx *FlagContext) (any, error) {
	if ctx.Input == "" {
		return []any{}, nil
	}
	var items []any
	for s := range strings.SplitSeq(ctx.Input, ",") {
		v, err := f.sub.Parse(ctx.With(s))
		if err != nil {
			return nil, err
		}
		items = addUnique(items, v)
	}
	return items, nil
}

func addUnique(items []any, v any) []any {
	if slices.Contains(items, v) {
		return items
	}
	return append(items, v)
}

func (f *SetFlag) Unmarshal(raw any) any {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	items := []any{}
	for _, e := range list {
		if v := f.sub.Unmarshal(e); v != nil {
			items = addUnique(items, v)
		}
	}
	return items
}

func (f *SetFlag) Marshal(v any) any {
	items, _ := v.([]any)
	out := make([]any, 0, len(items))
	for _, e := range items {
		out = append(out, f.sub.Marshal(e))
	}
	return out
}

// MapFlag is a map from one flag's values to another's, held as a
// map[any]any. WorldGuard: MapFlag
type MapFlag struct {
	baseFlag
	key, value Flag
}

// NewMapFlag answers a map flag.
func NewMapFlag(name string, key, value Flag, opts ...FlagOption) *MapFlag {
	return &MapFlag{baseFlag: base(name, opts), key: key, value: value}
}

// Key and Value answer the flags keys and values are parsed with.
func (f *MapFlag) Key() Flag   { return f.key }
func (f *MapFlag) Value() Flag { return f.value }

func (f *MapFlag) Parse(ctx *FlagContext) (any, error) {
	items := map[any]any{}
	if ctx.Input == "" {
		return items, nil
	}
	for str := range strings.SplitSeq(ctx.Input, ",") {
		sep := "="
		if !strings.Contains(str, "=") {
			sep = ":"
		}
		kv := strings.Split(str, sep)
		if len(kv) != 2 {
			return nil, InvalidFlagFormat("Input must be in a 'key:value,key1=value1' format. Either ':' or '=' can be used.")
		}
		k, err := f.key.Parse(ctx.With(kv[0]))
		if err != nil {
			return nil, err
		}
		v, err := f.value.Parse(ctx.With(kv[1]))
		if err != nil {
			return nil, err
		}
		items[k] = v
	}
	return items, nil
}

func (f *MapFlag) Unmarshal(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	items := map[any]any{}
	for k, v := range m {
		kv, vv := f.key.Unmarshal(k), f.value.Unmarshal(v)
		if kv != nil && vv != nil {
			items[kv] = vv
		}
	}
	return items
}

func (f *MapFlag) Marshal(v any) any {
	m, _ := v.(map[any]any)
	out := make(map[string]any, len(m))
	for k, e := range m {
		out[fmt.Sprint(f.key.Marshal(k))] = f.value.Marshal(e)
	}
	return out
}

// Registry is a set of named values a RegistryFlag takes: game modes,
// weather types, entity types. WorldGuard: Registry
type Registry interface {
	// Name names the registry in errors ("game mode").
	Name() string
	// Lookup answers the value a lower-case key names.
	Lookup(key string) (any, bool)
	// ID answers the key a value is stored under.
	ID(v any) string
	// Keys answers every key, for completion.
	Keys() []string
}

// RegistryFlag is a flag holding one value of a Registry. WorldGuard:
// RegistryFlag
type RegistryFlag struct {
	baseFlag
	reg Registry
}

// NewRegistryFlag answers a registry flag.
func NewRegistryFlag(name string, reg Registry, opts ...FlagOption) *RegistryFlag {
	return &RegistryFlag{baseFlag: base(name, opts), reg: reg}
}

// Registry answers the flag's registry.
func (f *RegistryFlag) Registry() Registry { return f.reg }

func (f *RegistryFlag) Parse(ctx *FlagContext) (any, error) {
	key := strings.ToLower(strings.TrimSpace(ctx.Input))
	if v, ok := f.reg.Lookup(key); ok {
		return v, nil
	}
	return nil, InvalidFlagFormat("Unknown %s: %s", f.reg.Name(), key)
}

func (f *RegistryFlag) Unmarshal(raw any) any {
	if v, ok := f.reg.Lookup(strings.ToLower(fmt.Sprint(raw))); ok {
		return v
	}
	return nil
}

func (f *RegistryFlag) Marshal(v any) any { return f.reg.ID(v) }

// EnumFlag is a flag holding one of a fixed list of upper-case names.
// WorldGuard: EnumFlag
type EnumFlag struct {
	baseFlag
	values []string
}

// NewEnumFlag answers an enum flag over values.
func NewEnumFlag(name string, values []string, opts ...FlagOption) *EnumFlag {
	return &EnumFlag{baseFlag: base(name, opts), values: values}
}

// Values answers the names the flag takes.
func (f *EnumFlag) Values() []string { return f.values }

func (f *EnumFlag) Parse(ctx *FlagContext) (any, error) {
	if up := strings.ToUpper(ctx.Input); slices.Contains(f.values, up) {
		return up, nil
	}
	return nil, InvalidFlagFormat("Unknown value '%s' in %s", ctx.Input, f.name)
}

func (f *EnumFlag) Unmarshal(raw any) any {
	if s := fmt.Sprint(raw); slices.Contains(f.values, s) {
		return s
	}
	return nil
}

func (f *EnumFlag) Marshal(v any) any { return v }

// RegionGroupFlag is the "<flag>-group" flag that limits who a flag's
// value applies to. WorldGuard: RegionGroupFlag
type RegionGroupFlag struct {
	baseFlag
	def RegionGroup
}

func (f *RegionGroupFlag) Default() any { return f.def }

// DefaultGroup answers the group used when the region sets none.
func (f *RegionGroupFlag) DefaultGroup() RegionGroup { return f.def }

func (f *RegionGroupFlag) Parse(ctx *FlagContext) (any, error) {
	if g, ok := parseRegionGroup(ctx.Input); ok {
		return g, nil
	}
	return nil, InvalidFlagFormat("Unknown value '%s' in RegionGroup", ctx.Input)
}

func (f *RegionGroupFlag) Unmarshal(raw any) any {
	s := fmt.Sprint(raw)
	for i, n := range regionGroupNames {
		if s == n {
			return RegionGroup(i)
		}
	}
	return nil
}

func (f *RegionGroupFlag) Marshal(v any) any {
	if g, ok := v.(RegionGroup); ok {
		return g.String()
	}
	return nil
}

// UUIDFlag is a UUID flag; "self" is the sender's. WorldGuard: UUIDFlag
type UUIDFlag struct{ baseFlag }

// NewUUIDFlag answers a UUID flag.
func NewUUIDFlag(name string, opts ...FlagOption) *UUIDFlag { return &UUIDFlag{base(name, opts)} }

func (f *UUIDFlag) Parse(ctx *FlagContext) (any, error) {
	if strings.EqualFold(ctx.Input, "self") && ctx.Player != nil {
		return ctx.Player.UUID(), nil
	}
	id, err := uuid.Parse(ctx.Input)
	if err != nil {
		return nil, InvalidFlagFormat("Not a valid uuid: %s", ctx.Input)
	}
	return id, nil
}

func (f *UUIDFlag) Unmarshal(raw any) any {
	s, ok := raw.(string)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return id
}

func (f *UUIDFlag) Marshal(v any) any { return fmt.Sprint(v) }

// TimestampFlag is a point in time; "now" is the present. WorldGuard:
// TimestampFlag
type TimestampFlag struct{ baseFlag }

// NewTimestampFlag answers a timestamp flag.
func NewTimestampFlag(name string, opts ...FlagOption) *TimestampFlag {
	return &TimestampFlag{base(name, opts)}
}

func (f *TimestampFlag) Parse(ctx *FlagContext) (any, error) {
	if strings.EqualFold(ctx.Input, "now") {
		return time.Now().UTC(), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		if t, err := time.Parse(layout, ctx.Input); err == nil {
			return t.UTC(), nil
		}
	}
	return nil, InvalidFlagFormat("Expected 'now' or ISO 8601 formatted input.")
}

func (f *TimestampFlag) Unmarshal(raw any) any {
	s, ok := raw.(string)
	if !ok {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return t.UTC()
}

// Marshal writes Java's ISO_DATE_TIME in UTC: the fraction in groups of
// three digits, none when zero.
func (f *TimestampFlag) Marshal(v any) any {
	t, ok := v.(time.Time)
	if !ok {
		return nil
	}
	t = t.UTC()
	s := t.Format("2006-01-02T15:04:05")
	if ns := t.Nanosecond(); ns != 0 {
		frac := fmt.Sprintf("%09d", ns)
		for strings.HasSuffix(frac, "000") {
			frac = frac[:len(frac)-3]
		}
		s += "." + frac
	}
	return s + "Z"
}

// UnknownFlag holds the value of a flag no installed plugin registered, so
// it is saved back unchanged. WorldGuard: UnknownFlag
type UnknownFlag struct{ baseFlag }

func newUnknownFlag(name string) *UnknownFlag {
	return &UnknownFlag{newBase(name, RegionGroupAll, true)}
}

func (f *UnknownFlag) Parse(*FlagContext) (any, error) {
	return nil, InvalidFlagFormat("The plugin that registered this flag is not currently installed")
}

func (f *UnknownFlag) Unmarshal(raw any) any { return raw }
func (f *UnknownFlag) Marshal(v any) any     { return v }

// FormatFlagValue answers a value as /rg info shows it.
func FormatFlagValue(f Flag, v any) string {
	switch x := v.(type) {
	case State:
		return x.String()
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = formatElement(f, e)
		}
		return "[" + strings.Join(sortedStrings(parts), ", ") + "]"
	case map[any]any:
		parts := make([]string, 0, len(x))
		for _, k := range slices.Sorted(maps.Keys(stringKeys(x))) {
			parts = append(parts, k+"="+fmt.Sprint(stringKeys(x)[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatFloat(x, 'f', 1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case plugin.Vec3:
		return fmt.Sprintf("(%s, %s, %s)", FormatFlagValue(f, x.X), FormatFlagValue(f, x.Y), FormatFlagValue(f, x.Z))
	case time.Time:
		return fmt.Sprint(f.Marshal(x))
	}
	if r, ok := f.(*RegistryFlag); ok {
		return r.reg.ID(v)
	}
	return fmt.Sprint(v)
}

func formatElement(f Flag, e any) string {
	if s, ok := f.(*SetFlag); ok {
		if r, ok := s.sub.(*RegistryFlag); ok {
			return r.reg.ID(e)
		}
	}
	return fmt.Sprint(e)
}

func stringKeys(m map[any]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[fmt.Sprint(k)] = v
	}
	return out
}
