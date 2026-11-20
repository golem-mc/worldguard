package worldguardplugin

import (
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// session is what WorldGuard keeps about an online player.
// WorldGuard: Session
type session struct {
	// bypassDisabled turns the player's region bypass off (/rg bypass).
	bypassDisabled bool
}

// sessionManager holds the online players' sessions. WorldGuard:
// AbstractSessionManager, BukkitSessionManager
type sessionManager struct {
	wg *WorldGuard

	mu       sync.Mutex
	sessions map[plugin.UUID]*session
}

func newSessionManager(wg *WorldGuard) *sessionManager {
	return &sessionManager{wg: wg, sessions: map[plugin.UUID]*session{}}
}

func (m *sessionManager) register() {
	for _, p := range m.wg.Server.Players() {
		m.get(p)
	}
	plugin.Listen(m.wg.Server, func(e *plugin.PlayerJoinEvent) { m.get(e.Player) },
		plugin.ListenOptions{Priority: plugin.Lowest})
	plugin.Listen(m.wg.Server, func(e *plugin.PlayerQuitEvent) {
		m.mu.Lock()
		delete(m.sessions, e.Player.UUID())
		m.mu.Unlock()
	}, plugin.ListenOptions{Priority: plugin.Monitor})
}

// get answers p's session, making it on first use.
func (m *sessionManager) get(p plugin.Player) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[p.UUID()]
	if !ok {
		s = &session{bypassDisabled: m.wg.Config.DisableDefaultBypass}
		m.sessions[p.UUID()] = s
	}
	return s
}

// getIfPresent answers p's session, nil when they have none.
func (m *sessionManager) getIfPresent(p plugin.Player) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[p.UUID()]
}
