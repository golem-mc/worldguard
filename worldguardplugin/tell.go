package worldguardplugin

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard/internal/chat"
)

// tell sends a §-coded message.
func tell(s plugin.CommandSender, msg string) { s.SendMessage(chat.Legacy(msg)) }
