package worldguardplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/chat"
)

// worldGuardCommands is /worldguard (/wg). WorldGuard: WorldGuardCommands,
// ProtectionCommands.worldGuard
func worldGuardCommands() *commandGroup {
	return &commandGroup{aliases: []string{"worldguard", "wg"}, desc: "WorldGuard commands", subs: []*wgCommand{
		{aliases: []string{"version"}, desc: "Get the WorldGuard version", max: 0, run: wgVersion},
		{aliases: []string{"reload"}, desc: "Reload WorldGuard configuration", max: 0, perm: "worldguard.reload",
			run: wgReload},
		{aliases: []string{"report"}, desc: "Writes a report on WorldGuard", flags: "p", max: 0,
			perm: "worldguard.report", run: wgReport},
		{aliases: []string{"profile"}, usage: "[-p] [-i <interval>] [-t <thread filter>] [<minutes>]",
			desc: "Profile the CPU usage of the server", flags: "t:i:p", max: 1, perm: "worldguard.profile",
			complete: func(*WorldGuard, *cmdArgs, int) []string { return []string{"1", "5", "10"} },
			run:      wgProfile},
		{aliases: []string{"stopprofile"}, desc: "Stop a running profile", max: 0, perm: "worldguard.profile",
			run: wgStopProfile},
		{aliases: []string{"flushstates", "clearstates"}, usage: "[player]", desc: "Flush the state manager",
			max: 1, perm: "worldguard.flushstates", run: wgFlushStates},
		{aliases: []string{"running", "queue"}, desc: "List running tasks", max: 0, perm: "worldguard.running",
			run: func(_ *WorldGuard, a *cmdArgs) error {
				a.sender.SendMessage(chat.Info("There are currently no running tasks."))
				return nil
			}},
	}}
}

// version answers the plugin's module version, "(unknown)" in a build
// without module information.
func (wg *WorldGuard) version() string {
	for _, info := range wg.Server.Plugins() {
		if info.Name == "worldguard" && info.Version != "" {
			return info.Version
		}
	}
	return "(unknown)"
}

func wgVersion(wg *WorldGuard, a *cmdArgs) error {
	a.sender.SendMessage(chat.Info("WorldGuard " + wg.version()))
	a.sender.SendMessage(chat.Info("http://www.enginehub.org"))
	a.sender.SendMessage(chat.Debug("----------- Platforms -----------"))
	name, _ := wg.Server.Version()
	a.sender.SendMessage(chat.Debug(fmt.Sprintf("* golem (%s)", name)))
	return nil
}

// wgReload reads the configuration and every world's regions again.
// WorldGuard: WorldGuardCommands.reload
func wgReload(wg *WorldGuard, a *cmdArgs) error {
	wg.Config.Unload()
	if err := wg.Config.Load(wg.worldNames()); err != nil {
		a.sender.SendMessage(chat.Error("Error while reloading: " + err.Error()))
		return nil
	}
	wg.regions.reload()
	a.sender.SendMessage(chat.Info("WorldGuard configuration reloaded."))
	return nil
}

// report is a plain text report of titled sections of keys and values.
// WorldGuard: ReportList, DataReport
type report struct{ b strings.Builder }

func (r *report) section(title string) {
	line := strings.Repeat("=", 80)
	fmt.Fprintf(&r.b, "%s\n%s\n%s\n\n", line, title, line)
}

func (r *report) kv(key string, value any) {
	fmt.Fprintf(&r.b, "%-30s %v\n", key+":", value)
}

func (r *report) text(s string) {
	r.b.WriteString(strings.TrimRight(s, "\n"))
	r.b.WriteString("\n\n")
}

// wgReport writes report.txt about the server, the configuration and the
// sender's regions. WorldGuard: WorldGuardCommands.report
func wgReport(wg *WorldGuard, a *cmdArgs) error {
	if a.has('p') {
		if !wg.HasPermission(a.sender, "worldguard.report.pastebin") {
			return errPermission
		}
	}
	var r report
	r.section("Server Information")
	name, protocol := wg.Server.Version()
	r.kv("Server", "golem "+name)
	r.kv("Protocol", protocol)
	r.kv("WorldGuard", wg.version())
	r.kv("Online Players", len(wg.Server.Players()))
	r.b.WriteString("\n")
	r.section("Plugins")
	for _, p := range wg.Server.Plugins() {
		r.kv(p.Name, strings.TrimSpace(p.Module+" "+p.Version))
	}
	r.b.WriteString("\n")
	r.section("Worlds")
	for _, w := range wg.Server.Worlds() {
		r.kv(wg.WorldName(w), w.Key().String()+" ("+w.Dimension().String()+")")
	}
	r.b.WriteString("\n")
	r.section("System Information")
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	r.kv("Operating System", runtime.GOOS+" "+runtime.GOARCH)
	r.kv("Go", runtime.Version())
	r.kv("Available Processors", runtime.NumCPU())
	r.kv("Goroutines", runtime.NumGoroutine())
	r.kv("Heap In Use (MB)", mem.HeapInuse>>20)
	r.b.WriteString("\n")
	r.section("WorldGuard Configuration")
	if data, err := os.ReadFile(filepath.Join(wg.Server.DataDir(), "config.yml")); err == nil {
		r.text(string(data))
	}
	for _, w := range wg.Server.Worlds() {
		name := wg.WorldName(w)
		r.section("World: " + name)
		if data, err := os.ReadFile(filepath.Join(wg.Config.WorldsDir(), name, "config.yml")); err == nil {
			r.text(string(data))
		}
		if m, ok := wg.Regions(w); ok {
			r.kv("Region Count", m.Len())
			if g, ok := m.Region(worldguard.GlobalRegionID); ok {
				r.kv("__global__", regionSummary(g))
			} else {
				r.kv("__global__", "<Undefined>")
			}
		} else {
			r.kv("Regions", "<Disabled>")
		}
		r.b.WriteString("\n")
	}
	if p, ok := a.sender.(plugin.Player); ok {
		r.section("Applicable regions")
		pos := p.Pos().Block()
		r.kv("Location", fmt.Sprintf("%s @ (%d, %d, %d)", wg.WorldName(p.World()), pos.X, pos.Y, pos.Z))
		if m, ok := wg.Regions(p.World()); !ok {
			r.kv("Regions", "Disabled for current world")
		} else if set := m.ApplicableRegions(pos, worldguard.ComputeParents); set.Len() == 0 {
			r.kv("Regions", "None")
		} else {
			for _, rg := range set.Regions() {
				id := rg.ID()
				if !rg.Contains(pos) {
					id += "*"
				}
				r.kv(id, regionSummary(rg))
			}
		}
	}
	dest := filepath.Join(wg.Server.DataDir(), "report.txt")
	if abs, err := filepath.Abs(dest); err == nil {
		dest = abs
	}
	if err := os.WriteFile(dest, []byte(r.b.String()), 0o644); err != nil {
		return cmdError("Failed to write report: " + err.Error())
	}
	a.sender.SendMessage(chat.Info("WorldGuard report written to " + dest))
	if a.has('p') {
		a.sender.SendMessage(chat.Error("Pasting reports is not available on golem."))
	}
	return nil
}

// regionSummary is a region's line in a report. WorldGuard: RegionReport
func regionSummary(r *worldguard.Region) string {
	var flags []string
	for f, v := range r.Flags() {
		flags = append(flags, f.Name()+"="+worldguard.FormatFlagValue(f, v))
	}
	sort.Strings(flags)
	return fmt.Sprintf("type=%s priority=%d flags={%s}", r.Type(), r.Priority(), strings.Join(flags, ", "))
}

// profiler is the one CPU profile that may run.
var profiler struct {
	sync.Mutex
	stop chan struct{}
}

// wgProfile records a CPU profile of the whole server for some minutes
// into profile.pprof, which `go tool pprof` reads. WorldGuard:
// WorldGuardCommands.profile, which samples Java threads instead
func wgProfile(wg *WorldGuard, a *cmdArgs) error {
	if a.has('p') && !wg.HasPermission(a.sender, "worldguard.report.pastebin") {
		return errPermission
	}
	minutes := 5
	if a.len() > 0 {
		n, err := a.argInt(0)
		if err != nil {
			return err
		}
		switch {
		case n < 1:
			return cmdError("You must run the profile for at least 1 minute.")
		case n > 10:
			return cmdError("You can profile for, at maximum, 10 minutes.")
		}
		minutes = n
	}
	if v, ok := a.flags['i']; ok {
		var interval int
		if _, err := fmt.Sscan(v, &interval); err != nil || interval < 1 || interval > 100 {
			return cmdError("Interval must be between 1 and 100 (in milliseconds)")
		}
		if interval < 10 {
			a.sender.SendMessage(chat.Debug("Note: A low interval may cause additional slowdown during profiling."))
		}
	}
	dest := filepath.Join(wg.Server.DataDir(), "profile.pprof")
	if abs, err := filepath.Abs(dest); err == nil {
		dest = abs
	}
	profiler.Lock()
	defer profiler.Unlock()
	if profiler.stop != nil {
		return cmdError("A profile is currently in progress! Please use /wg stopprofile to cancel the current profile.")
	}
	f, err := os.Create(dest + ".tmp")
	if err != nil {
		return cmdError("Failed to write CPU profiling data: " + err.Error())
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		os.Remove(f.Name())
		return cmdError("Could not start profiling: " + err.Error())
	}
	stop := make(chan struct{})
	profiler.stop = stop
	a.sender.SendMessage(plugin.Plain("").Append(
		plugin.Plain(fmt.Sprintf("Starting CPU profiling. Results will be available in %d minutes.", minutes)).Color("light_purple"),
		plugin.Plain("\n"), plugin.Plain("Use ").Color("gray"),
		plugin.Plain("/wg stopprofile").Color("aqua").OnClick(plugin.SuggestCommand, "/wg stopprofile"),
		plugin.Plain(" at any time to cancel CPU profiling.").Color("gray")))
	sender := a.sender
	go func() {
		var cancelled bool
		select {
		case <-time.After(time.Duration(minutes) * time.Minute):
		case <-stop:
			cancelled = true
		}
		pprof.StopCPUProfile()
		f.Close()
		profiler.Lock()
		if profiler.stop == stop {
			profiler.stop = nil
		}
		profiler.Unlock()
		if cancelled {
			os.Remove(f.Name())
			return
		}
		wg.Server.Scheduler().RunTask(func() {
			if err := os.Rename(f.Name(), dest); err != nil {
				sender.SendMessage(chat.Error("Failed to write CPU profiling data: " + err.Error()))
				return
			}
			sender.SendMessage(chat.Info("CPU profiling data written to " + dest))
		})
	}()
	return nil
}

func wgStopProfile(_ *WorldGuard, a *cmdArgs) error {
	profiler.Lock()
	defer profiler.Unlock()
	if profiler.stop == nil {
		return cmdError("No CPU profile is currently running.")
	}
	close(profiler.stop)
	profiler.stop = nil
	a.sender.SendMessage(chat.Info("The running CPU profile has been cancelled."))
	return nil
}

func wgFlushStates(wg *WorldGuard, a *cmdArgs) error {
	if a.len() == 0 {
		wg.sessions.resetAll()
		a.sender.SendMessage(chat.Info("Cleared all states."))
		return nil
	}
	p, err := wg.matchSinglePlayer(a.sender, a.args[0])
	if err != nil {
		return err
	}
	if s := wg.sessions.getIfPresent(p); s != nil {
		s.resetState(wg.local(p))
	}
	a.sender.SendMessage(chat.Info("Cleared states for player \"" + p.Name() + "\"."))
	return nil
}

// registerCommands registers the commands outside /rg. WorldGuard:
// WorldGuardPlugin.onEnable's command registration
func (wg *WorldGuard) registerCommands() {
	for _, c := range generalCommands() {
		wg.registerCommand(c)
	}
	for _, c := range toggleCommands() {
		wg.registerCommand(c)
	}
	wg.registerGroup(worldGuardCommands())
}
