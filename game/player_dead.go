package game

// Story mode ends with the TV results screen (Main Menu / Retry) once the last
// life is lost and the death animation has finished.
func (p *playState) storyGameOver(mode int) bool {
	return p != nil && mode == 0 && p.health <= 0 && p.lives <= 0 && p.deathTimer <= 0 && p.allPlayersDown()
}
