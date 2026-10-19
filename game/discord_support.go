package game

import "runtime"

// discordAvailable is whether Discord Rich Presence can work here: it talks to the Discord app over a local pipe or
// socket, which a page in a browser cannot open, so the web build has no DISCORD RPC option at all.
func discordAvailable() bool { return runtime.GOOS != "js" }
