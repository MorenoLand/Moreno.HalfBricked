package game

import "time"

// Port addition (HD native screens): a screen that is closed slides out before the page changes, instead of
// vanishing at once. The slide is the screen's own AnimateIn played backwards over the same time (nativeSlide);
// the uiscreens also hold AnimateOut tracks (the 1.2.5 Back callbacks start them), which are not decoded here, so the
// reverse of AnimateIn stands in. Input is ignored while it plays (Update), and the close sounds are played at the
// press (menuFeedbackSnapshot reads the screen as already closed).

// closeMenuAnimated runs next after the slide-out of the screen drawn with nativeSlide(key, ...). Where the screen has
// not been drawn natively (no slide duration known) next runs at once.
func (a *app) closeMenuAnimated(key string, next func() error) error {
	n := &a.native
	if n.closeKey != "" || n.slideDur[key] <= 0 || !a.titleNative() {
		return next()
	}
	n.closeKey, n.closeStart, n.closeNext = key, a.menuTime, next
	return nil
}

// stepMenuClose finishes a slide-out once its time is up.
func (a *app) stepMenuClose() error {
	n := &a.native
	if n.closeKey == "levelselect" {
		a.stepPreload(8 * time.Millisecond) // the slide-out is time the level's files can finish loading in
	}
	if a.menuTime-n.closeStart < n.slideDur[n.closeKey] {
		return nil
	}
	next := n.closeNext
	n.closeKey, n.closeNext = "", nil
	if next == nil {
		return nil
	}
	return next()
}
