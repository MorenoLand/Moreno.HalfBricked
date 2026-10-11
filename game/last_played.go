package game

// Port addition: the story and survival menus open on the level that was played last in that mode (kept in the
// profile as lastPlayed["story"|"survival"]), so picking up again does not mean scrolling from the first card.

func lastPlayedKey(mode int) string {
	if mode == 1 {
		return "survival"
	}
	return "story"
}

// noteLastPlayed records the level being opened.
func (a *app) noteLastPlayed(id string) {
	if id == "" {
		return
	}
	key := lastPlayedKey(a.mode)
	if a.lastPlayed[key] == id {
		return
	}
	if a.lastPlayed == nil {
		a.lastPlayed = map[string]string{}
	}
	a.lastPlayed[key] = id
	_ = a.savePlayerProfile()
}

// restoreLastPlayed points the level select (a.world, a.level) at the remembered level of the current mode; with
// nothing remembered it leaves the first card selected.
func (a *app) restoreLastPlayed() {
	a.world, a.level, a.carouselReady = 0, 0, false
	id := a.lastPlayed[lastPlayedKey(a.mode)]
	if id == "" {
		return
	}
	for world := range a.worlds() {
		a.world = world
		for index, item := range a.filteredLevels() {
			if item.ID == id {
				a.level = index
				return
			}
		}
	}
	a.world, a.level = 0, 0
}
