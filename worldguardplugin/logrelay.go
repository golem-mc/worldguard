package worldguardplugin

import (
	"context"
	"log/slog"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// logRelay is the handler behind wg.Log: it writes every record to the
// server's logger and, while a player's /wg reload runs, also relays it to
// that player's chat. WorldGuard: LoggerToChatHandler
type logRelay struct {
	slog.Handler // the server's handler
	shared       *relayTarget
}

type relayTarget struct {
	mu     sync.Mutex
	player plugin.CommandSender
}

func newLogRelay(base slog.Handler) *logRelay {
	return &logRelay{Handler: base, shared: &relayTarget{}}
}

func (r *logRelay) Enabled(ctx context.Context, l slog.Level) bool {
	r.shared.mu.Lock()
	relaying := r.shared.player != nil
	r.shared.mu.Unlock()
	return relaying || r.Handler.Enabled(ctx, l)
}

func (r *logRelay) Handle(ctx context.Context, rec slog.Record) error {
	r.shared.mu.Lock()
	p := r.shared.player
	r.shared.mu.Unlock()
	if p != nil {
		p.SendMessage(plugin.Plain("").Append(
			plugin.Plain(javaLevel(rec.Level)+": ").Color("gray"),
			plugin.Plain(rec.Message).Color("white")))
	}
	if r.Handler.Enabled(ctx, rec.Level) {
		return r.Handler.Handle(ctx, rec)
	}
	return nil
}

func (r *logRelay) WithAttrs(a []slog.Attr) slog.Handler {
	return &logRelay{Handler: r.Handler.WithAttrs(a), shared: r.shared}
}

func (r *logRelay) WithGroup(n string) slog.Handler {
	return &logRelay{Handler: r.Handler.WithGroup(n), shared: r.shared}
}

// relayTo sends the log records to p until the returned function is called.
func (r *logRelay) relayTo(p plugin.CommandSender) (stop func()) {
	r.shared.mu.Lock()
	r.shared.player = p
	r.shared.mu.Unlock()
	return func() {
		r.shared.mu.Lock()
		r.shared.player = nil
		r.shared.mu.Unlock()
	}
}

// javaLevel is the java.util.logging name of a level.
func javaLevel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "SEVERE"
	case l >= slog.LevelWarn:
		return "WARNING"
	case l >= slog.LevelInfo:
		return "INFO"
	}
	return "FINE"
}
