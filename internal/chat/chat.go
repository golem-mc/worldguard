// Package chat turns WorldGuard's messages into golem text: legacy §
// formatting codes and the colours of WorldEdit's Actor.print family.
package chat

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

var colors = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua",
	'4': "dark_red", '5': "dark_purple", '6': "gold", '7': "gray",
	'8': "dark_gray", '9': "blue", 'a': "green", 'b': "aqua",
	'c': "red", 'd': "light_purple", 'e': "yellow", 'f': "white",
}

// Legacy parses § formatting codes (not &: WorldGuard turns those into §
// first, so a literal & stays). A colour code resets the formatting, as
// Minecraft's legacy text does.
func Legacy(s string) plugin.Text {
	var parts []plugin.Text
	var cur strings.Builder
	var style plugin.Style
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, plugin.Text{Literal: cur.String(), Style: style})
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "§") && i+2 < len(s) {
			code := s[i+2]
			if code >= 'A' && code <= 'Z' {
				code += 'a' - 'A'
			}
			on := new(true)
			handled := true
			if c, ok := colors[code]; ok {
				flush()
				style = plugin.Style{Color: c}
			} else {
				switch code {
				case 'l':
					flush()
					style.Bold = on
				case 'o':
					flush()
					style.Italic = on
				case 'n':
					flush()
					style.Underlined = on
				case 'm':
					flush()
					style.Strikethrough = on
				case 'k':
					flush()
					style.Obfuscated = on
				case 'r':
					flush()
					style = plugin.Style{}
				default:
					handled = false
				}
			}
			if handled {
				i += 2
				continue
			}
		}
		cur.WriteByte(s[i])
	}
	flush()
	switch len(parts) {
	case 0:
		return plugin.Plain("")
	case 1:
		return parts[0]
	}
	return plugin.Plain("").Append(parts...)
}

// Strip removes § formatting codes.
func Strip(s string) string { return Legacy(s).PlainString() }

// Info is Actor.print's light purple.
func Info(s string) plugin.Text { return plugin.Plain(s).Color("light_purple") }

// Error is Actor.printError's red.
func Error(s string) plugin.Text { return plugin.Plain(s).Color("red") }

// Debug is Actor.printDebug's gray.
func Debug(s string) plugin.Text { return plugin.Plain(s).Color("gray") }

// Notify is WorldGuard's "WG: <cause><description>" notice.
func Notify(cause, description string) plugin.Text {
	return plugin.Plain("").Append(plugin.Plain("WG: ").Color("gray"),
		plugin.Plain(cause).Color("light_purple"), plugin.Plain(description).Color("gold"))
}
