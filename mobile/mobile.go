package mobile

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"petri_dish/petridish"
)

// Version is overridden at build time via:
//
//	-ldflags "-X petri_dish/mobile.Version=$(git describe --always --dirty)"
//
// Empty version forces every load to be treated as version-mismatched.
var Version = ""

var currentGame *petridish.Game

func init() {
	currentGame = petridish.NewGame()
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
