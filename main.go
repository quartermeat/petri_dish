//go:build !android

package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"hex_globe/hexglobe"
)

func main() {
	game := hexglobe.NewGame()

	ebiten.SetWindowSize(game.ScreenWidth()*2, game.ScreenHeight()*2)
	ebiten.SetWindowTitle("Hex Globe")
	ebiten.SetTPS(ebiten.SyncWithFPS)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
