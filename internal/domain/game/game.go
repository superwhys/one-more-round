// Package game owns the group's game catalogue and its local names.
package game

import "strings"

// Game is a game of the group catalogue. The local name is kept apart from the
// original name imported from external sources.
type Game struct {
	ID       string
	Name     string
	Original string
	BGGID    *int
}

// SameName reports whether the game carries the given local name.
func (g *Game) SameName(name string) bool { return strings.EqualFold(g.Name, name) }
