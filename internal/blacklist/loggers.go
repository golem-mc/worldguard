package blacklist

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ConsoleLogger logs events to the server log. WorldGuard: ConsoleHandler
type ConsoleLogger struct {
	World  string
	Logger *slog.Logger
}

// Log logs "[world] <who> tried to <verb> <name> (comment)".
func (c ConsoleLogger) Log(e Event, comment string, hasComment bool) {
	msg := "[" + c.World + "] " + e.LoggerMessage()
	if hasComment {
		msg += " (" + comment + ")"
	}
	c.Logger.Info(msg)
}

// Close does nothing.
func (ConsoleLogger) Close() {}

// FileLogger appends events to files named by a pattern, keeping up to
// CacheSize of them open. WorldGuard: FileHandler
type FileLogger struct {
	Pattern   string
	CacheSize int
	World     string
	Logger    *slog.Logger

	mu      sync.Mutex
	writers map[string]*logFile
}

type logFile struct {
	f        *os.File
	w        *bufio.Writer
	lastUsed time.Time
}

var (
	patternCode = regexp.MustCompile(`%.`)
	unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_]`)
)

// path fills the pattern's codes; ok is false for %u without a player.
// WorldGuard: FileHandler.buildPath
func (f *FileLogger) path(player string, hasPlayer bool, now time.Time) (string, bool) {
	ok := true
	safe := func(s string) string {
		s = unsafeChars.ReplaceAllString(strings.ToLower(s), "_")
		if len(s) > 32 {
			s = s[:32]
		}
		return s
	}
	hour12 := now.Hour() % 12
	out := patternCode.ReplaceAllStringFunc(f.Pattern, func(code string) string {
		switch code {
		case "%%":
			return "%"
		case "%u":
			if !hasPlayer {
				ok = false
			}
			return safe(player)
		case "%w":
			return safe(f.World)
		case "%Y":
			return fmt.Sprint(now.Year())
		case "%m":
			// Java's Calendar.MONTH, from 00 for January.
			return fmt.Sprintf("%02d", int(now.Month())-1)
		case "%d":
			return fmt.Sprintf("%02d", now.Day())
		case "%W":
			return fmt.Sprintf("%02d", javaWeekOfYear(now))
		case "%H":
			return fmt.Sprintf("%02d", now.Hour())
		case "%h":
			return fmt.Sprintf("%02d", hour12)
		case "%i":
			return fmt.Sprintf("%02d", now.Minute())
		case "%s":
			return fmt.Sprintf("%02d", now.Second())
		}
		return "?"
	})
	return out, ok
}

// javaWeekOfYear is GregorianCalendar's WEEK_OF_YEAR for a US locale:
// weeks start on Sunday, and week 1 holds January 1st.
func javaWeekOfYear(t time.Time) int {
	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	offset := int(jan1.Weekday())
	// The week holding December 31st is the next year's first unless it
	// ends that day (a Saturday).
	if dec31 := time.Date(t.Year(), 12, 31, 0, 0, 0, 0, t.Location()); dec31.Weekday() != time.Saturday &&
		t.YearDay() >= dec31.YearDay()-int(dec31.Weekday()) {
		return 1
	}
	return (t.YearDay()-1+offset)/7 + 1
}

// Log appends "[date] <who>: Tried to <verb> <name> @x,y,z (comment)".
func (f *FileLogger) Log(e Event, comment string, hasComment bool) {
	now := time.Now()
	name := ""
	if e.Player != nil {
		name = e.Player.Name()
	}
	path, ok := f.path(name, e.Player != nil, now)
	if !ok {
		return
	}
	who := "Unknown Source"
	if e.Player != nil {
		who = name
	}
	line := "[" + now.Format("2006-01-02 15:04:05") + "] " + who + ": Tried to " + e.Type.Description() + " " +
		e.Target.plainName() + " " + fmt.Sprintf("@%d,%d,%d", e.Pos.X, e.Pos.Y, e.Pos.Z)
	if hasComment {
		line += " (" + comment + ")"
	}
	line += "\r\n"

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writers == nil {
		f.writers = map[string]*logFile{}
	}
	if w, ok := f.writers[path]; ok {
		if _, err := w.w.WriteString(line); err == nil && w.w.Flush() == nil {
			w.lastUsed = now
			return
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.Logger.Warn("Failed to log blacklist event to '" + path + "': " + err.Error())
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		f.Logger.Warn("Failed to log blacklist event to '" + path + "': " + err.Error())
		return
	}
	w := &logFile{f: file, w: bufio.NewWriter(file), lastUsed: now}
	_, err = w.w.WriteString(line)
	if err == nil {
		err = w.w.Flush()
	}
	if err != nil {
		f.Logger.Warn("Failed to log blacklist event to '" + path + "': " + err.Error())
	}
	if old, ok := f.writers[path]; ok {
		old.f.Close()
	}
	f.writers[path] = w
	if len(f.writers) > max(1, f.CacheSize) {
		// WorldGuard closes files in path order, not by last use.
		paths := make([]string, 0, len(f.writers))
		for p := range f.writers {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if len(f.writers) <= max(1, f.CacheSize) {
				break
			}
			f.writers[p].f.Close()
			delete(f.writers, p)
		}
	}
}

// Close closes the open files.
func (f *FileLogger) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for p, w := range f.writers {
		w.f.Close()
		delete(f.writers, p)
	}
}
