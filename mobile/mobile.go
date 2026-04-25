package mobile

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"hex_globe/hexglobe"
)

// Version is overridden at build time via:
//   -ldflags "-X hex_globe/mobile.Version=$(git describe --always --dirty)"
// Empty version forces every load to be treated as version-mismatched.
var Version = ""

var currentGame *hexglobe.Game

func init() {
	currentGame = hexglobe.NewGame()
	currentGame.SetVersion(Version)
	mobile.SetGame(currentGame)
}

// SetSaveDir wires the app-private storage path (typically
// Context.getFilesDir()) and immediately attempts to restore prior progress.
// Called from Java in MainActivity.onCreate.
func SetSaveDir(path string) {
	if currentGame == nil {
		return
	}
	currentGame.SetSaveDir(path)
	currentGame.LoadOrInit()
}

// Dummy is required by gomobile bind.
func Dummy() {}
