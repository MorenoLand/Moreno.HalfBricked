package game

import "fmt"

// openHistory lists every undo step, newest first; a click goes back to just before it.
func (a *app) openHistory(x float64) {
	e := a.editor
	p := a.openPopup("History", x, 320, nil)
	p.build = func() []edMenuItem {
		var items []edMenuItem
		if len(e.undo) == 0 && len(e.redo) == 0 {
			return []edMenuItem{{label: "No changes yet", disabled: true}}
		}
		for index := len(e.redo) - 1; index >= 0; index-- {
			// redo steps are drawn dimmed above the present
			step := e.redo[index]
			steps := len(e.redo) - index
			items = append(items, edMenuItem{label: step.label, sub: "undone", keepOpen: true, tip: "Redo up to here", do: func() {
				for i := 0; i < steps; i++ {
					e.doRedo()
				}
			}})
		}
		items = append(items, edMenuItem{label: fmt.Sprintf("Now: %d changes made", len(e.undo)), header: true})
		for index := len(e.undo) - 1; index >= 0; index-- {
			step := e.undo[index]
			steps := len(e.undo) - index
			items = append(items, edMenuItem{label: step.label, sub: fmt.Sprintf("%d back", steps), keepOpen: true, tip: "Go back to before this change", do: func() {
				for i := 0; i < steps; i++ {
					e.doUndo()
				}
			}})
		}
		return items
	}
}
