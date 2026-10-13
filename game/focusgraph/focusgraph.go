// Package focusgraph parses the native front end's controller focus graphs (Common0/UserInterface/Controller/*.txt):
// for every screen a set of nodes with Up/Down/Left/Right targets and the IsDefault / IsBack nodes. It is pure
// text-to-data and knows nothing about the game; ui_controller.go in package game loads and caches the files.
package focusgraph

import "strings"

// Graph is a parsed Common0/UserInterface/Controller/<Screen>.txt: the
// focus graph the native front end walks with the d-pad / arrow keys. Lines are
// either `Alias=Component.Path;` or `Node Up:Target Down:Target Left:Target
// Right:Target [IsDefault] [IsBack]`. IsDefault is the initially focused node,
// IsBack the node that is pressed by the back key.
type Graph struct {
	Aliases map[string]string
	Links   map[string]map[string]string
	Default string
	Back    string
}

// Parse reads the text of one controller file.
func Parse(text string) *Graph {
	c := &Graph{Aliases: map[string]string{}, Links: map[string]map[string]string{}}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if at := strings.Index(line, "="); at > 0 && !strings.Contains(line[:at], " ") {
			c.Aliases[line[:at]] = strings.TrimSuffix(strings.TrimSpace(line[at+1:]), ";")
			continue
		}
		fields := strings.Fields(line)
		name := fields[0]
		if c.Links[name] == nil {
			c.Links[name] = map[string]string{}
		}
		for _, field := range fields[1:] {
			field = strings.TrimSuffix(field, ";") // the shipped files end node lines with ';'
			switch field {
			case "IsDefault":
				c.Default = name
			case "IsBack":
				c.Back = name
			default:
				if at := strings.Index(field, ":"); at > 0 {
					c.Links[name][field[:at]] = field[at+1:]
				}
			}
		}
	}
	return c
}

// Neighbour returns the node reached from `from` in direction dir ("Up",
// "Down", "Left", "Right").
func (c *Graph) Neighbour(from, dir string) (string, bool) {
	if c == nil {
		return "", false
	}
	target, ok := c.Links[from][dir]
	return target, ok && target != ""
}
