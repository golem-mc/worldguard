// Package worldguard is the API of the WorldGuard port for golem: regions,
// their flags and the queries over them. Other plugins import it to read or
// change regions and to ask what a flag says at a place, through the
// service found with Lookup; importing it registers no plugin. The plugin
// itself is in package worldguardplugin.
//
// A plugin that adds flags registers them with Flags.Register from an init
// function or its Load, before WorldGuard loads the regions.
package worldguard
