// Command server is the golem server with WorldGuard (and WorldEdit) built
// in.
package main

import (
	"github.com/golem-mc/golem/app"
	_ "github.com/golem-mc/worldguard/worldguardplugin"
)

func main() { app.Main() }
