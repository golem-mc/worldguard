package worldguard

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Flag is a kind of extra data a region carries: a name, how values are
// parsed from a command and stored in regions.yml, and how competing values
// are resolved. Every flag is registered once with the FlagRegistry. A value
// is held as the flag kind's Go type (State, bool, string, int, float64,
// Location, plugin.Vec3, []any, map[any]any, ...). WorldGuard: Flag
type Flag interface {
	// Name is the flag's name, as /rg flag and regions.yml spell it.
	Name() string
	// GroupFlag is the flag's region group flag ("<name>-group"), nil for
	// a flag with none.
	GroupFlag() *RegionGroupFlag
	// Default is the value used when no region sets one; nil for none.
	Default() any
	// Choose picks one value of several regions' values; nil for none.
	Choose(values []any) any
	// HasConflictStrategy reports whether Choose combines every value of
	// equal priority rather than taking the first.
	HasConflictStrategy() bool
	// ImplicitlySetWithMembership reports whether membership of a region
	// counts as setting the flag (build).
	ImplicitlySetWithMembership() bool
	// UsesMembershipAsDefault reports whether membership decides the value
	// when no region sets it (build).
	UsesMembershipAsDefault() bool
	// RequiresSubject reports whether a query must name a subject.
	RequiresSubject() bool
	// Parse reads a value from a command's input.
	Parse(ctx *FlagContext) (any, error)
	// Unmarshal reads a value from regions.yml; nil for an invalid one.
	Unmarshal(raw any) any
	// Marshal writes a value for regions.yml.
	Marshal(v any) any
}

// ErrInvalidFlagFormat is wrapped by every parse error of a flag value.
// WorldGuard: InvalidFlagFormat
var ErrInvalidFlagFormat = errors.New("invalid flag format")

// InvalidFlagFormat answers a parse error with message.
func InvalidFlagFormat(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidFlagFormat, fmt.Sprintf(format, args...))
}

// FlagErrorMessage answers the message of a parse error, without the
// ErrInvalidFlagFormat prefix.
func FlagErrorMessage(err error) string {
	s := err.Error()
	if after, ok := strings.CutPrefix(s, ErrInvalidFlagFormat.Error()+": "); ok {
		return after
	}
	return s
}

var validFlagName = regexp.MustCompile(`^[:A-Za-z0-9\-]{1,40}$`)

// ValidFlagName reports whether name may name a flag.
func ValidFlagName(name string) bool { return validFlagName.MatchString(name) }

// FlagContext is the input a flag value is parsed from and what it is for.
// WorldGuard: FlagContext
type FlagContext struct {
	// Sender is who set the flag; Player is the sender as a player, nil for
	// the console.
	Sender plugin.CommandSender
	Player plugin.Player
	// Input is the text to parse.
	Input string
	// World is the name of the sender's world, for a location.
	World string
	// Region is the region the flag is set on, nil when unknown.
	Region *Region
	// Extra holds values other plugins attach.
	Extra map[string]any
	// BoundedLocations refuses a location outside Region (the
	// regions.location-flags-only-inside-regions setting); MayOverrideBounds
	// lets the sender do so anyway with a warning.
	BoundedLocations, MayOverrideBounds bool
	// Warn shows a warning to the sender, nil for none.
	Warn func(msg string)
}

// With answers a copy of the context parsing input.
func (c *FlagContext) With(input string) *FlagContext {
	cp := *c
	cp.Input = input
	return &cp
}

func (c *FlagContext) int() (int, error) {
	v, err := strconv.ParseInt(c.Input, 10, 32)
	if err != nil {
		return 0, InvalidFlagFormat("Not a number: %s", c.Input)
	}
	return int(v), nil
}

func (c *FlagContext) player() (plugin.Player, error) {
	if c.Player == nil {
		return nil, InvalidFlagFormat("Not a player")
	}
	return c.Player, nil
}

// baseFlag holds what every flag kind shares.
type baseFlag struct {
	name  string
	group *RegionGroupFlag
}

func newBase(name string, group RegionGroup, hasGroup bool) baseFlag {
	if name != "" && !ValidFlagName(name) {
		panic("worldguard: invalid flag name " + name)
	}
	b := baseFlag{name: name}
	if hasGroup && name != "" {
		b.group = &RegionGroupFlag{name: name + "-group", def: group}
	}
	return b
}

func (b *baseFlag) Name() string                      { return b.name }
func (b *baseFlag) GroupFlag() *RegionGroupFlag       { return b.group }
func (b *baseFlag) Default() any                      { return nil }
func (b *baseFlag) HasConflictStrategy() bool         { return false }
func (b *baseFlag) ImplicitlySetWithMembership() bool { return false }
func (b *baseFlag) UsesMembershipAsDefault() bool     { return false }
func (b *baseFlag) RequiresSubject() bool             { return false }
func (b *baseFlag) Choose(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

// FlagOption sets an option of a flag being made.
type FlagOption func(*flagOptions)

type flagOptions struct {
	group    RegionGroup
	hasGroup bool
}

// WithGroup gives the flag a region group flag defaulting to g; without it a
// flag's group defaults to RegionGroupAll.
func WithGroup(g RegionGroup) FlagOption {
	return func(o *flagOptions) { o.group, o.hasGroup = g, true }
}

// WithoutGroup makes a flag with no region group flag.
func WithoutGroup() FlagOption {
	return func(o *flagOptions) { o.hasGroup = false }
}

func base(name string, opts []FlagOption) baseFlag {
	o := flagOptions{group: RegionGroupAll, hasGroup: true}
	for _, f := range opts {
		f(&o)
	}
	return newBase(name, o.group, o.hasGroup)
}

// sortedStrings answers the strings sorted, for stable output.
func sortedStrings(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}
