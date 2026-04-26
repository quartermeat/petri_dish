//go:build !android

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"hex_globe/hexglobe"
)

// Version is overridden at build time via:
//
//	-ldflags "-X main.Version=$(git describe --always --dirty)"
//
// An empty Version means saves are always treated as version-mismatched —
// useful as a default so unsigned builds don't load across unknown binaries.
var Version = ""

func main() {
	startView := flag.String("view", "", "optional startup view: settings")
	screenshotPath := flag.String("screenshot", "", "optional PNG path to save a screenshot and exit")
	flag.Parse()

	game := hexglobe.NewGame()
	game.SetVersion(Version)
	if dir := desktopSaveDir(); dir != "" {
		game.SetSaveDir(dir)
		game.LoadOrInit()
	}
	if *startView == "settings" {
		game.OpenSettingsForTesting()
	}
	if *screenshotPath != "" {
		game.ConfigureScreenshot(*screenshotPath, 10)
	}

	ebiten.SetWindowSize(game.ScreenWidth()*2, game.ScreenHeight()*2)
	ebiten.SetWindowTitle("Helios")
	ebiten.SetTPS(ebiten.SyncWithFPS)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func desktopSaveDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "Helios")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".hex_globe")
	}
	return ""
}
