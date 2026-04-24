package mobile

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"hex_globe/hexglobe"
)

func init() {
	mobile.SetGame(hexglobe.NewGame())
}

// Dummy is required by gomobile bind.
func Dummy() {}
