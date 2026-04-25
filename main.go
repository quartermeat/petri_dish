//go:build !android

package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"hex_globe/hexglobe"
)

func main() {
	startView := flag.String("view", "", "optional startup view: settings")
	screenshotPath := flag.String("screenshot", "", "optional PNG path to save a screenshot and exit")
	flag.Parse()

	game := hexglobe.NewGame()
	if *startView == "settings" {
		game.OpenSettingsForTesting()
	}
	if *screenshotPath != "" {
		game.ConfigureScreenshot(*screenshotPath, 10)
	}

	ebiten.SetWindowSize(game.ScreenWidth()*2, game.ScreenHeight()*2)
	ebiten.SetWindowTitle("Hex Globe")
	ebiten.SetTPS(ebiten.SyncWithFPS)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
