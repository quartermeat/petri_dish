package petridish

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const modalTapCooldown = 1.0 // seconds before OK is tappable (eats stray in-flight taps)

type modalState struct {
	lines     []string
	onConfirm func()
	cooldown  float64
}

func (g *Game) showModal(lines []string, onConfirm func()) {
	g.modal = &modalState{
		lines:     lines,
		onConfirm: onConfirm,
		cooldown:  modalTapCooldown,
	}
}

func (g *Game) modalActive() bool {
	return g.modal != nil
}

func (g *Game) modalCardRect() (float64, float64, float64, float64) {
	w := 320.0
	lineCount := 0
	if g.modal != nil {
		lineCount = len(g.modal.lines)
	}
	if lineCount < 1 {
		lineCount = 1
	}
	h := 24.0 + float64(lineCount)*18.0 + 72.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	y := float64(g.screenHeight)*0.5 - h*0.5
	return x, y, w, h
}

func (g *Game) modalOKRect() (float64, float64, float64, float64) {
	cardX, cardY, cardW, cardH := g.modalCardRect()
	bw := 96.0
	bh := 40.0
	bx := cardX + cardW*0.5 - bw*0.5
	by := cardY + cardH - 16 - bh
	return bx, by, bw, bh
}

func (g *Game) handleModalInput() {
	if g.modal == nil {
		return
	}
	if g.modal.cooldown > 0 {
		g.modal.cooldown -= 1.0 / 60.0
		return
	}
	x, y, tapped := -1, -1, false
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y = ebiten.CursorPosition()
		tapped = true
	}
	if !tapped {
		justTouched := inpututil.AppendJustPressedTouchIDs(nil)
		if len(justTouched) > 0 {
			x, y = ebiten.TouchPosition(justTouched[0])
			tapped = true
		}
	}
	if !tapped {
		return
	}
	bx, by, bw, bh := g.modalOKRect()
	if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
		return
	}
	cb := g.modal.onConfirm
	g.modal = nil
	if cb != nil {
		cb()
	}
}

func (g *Game) drawModal(screen *ebiten.Image) {
	if g.modal == nil {
		return
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{0, 0, 0, 168})

	cardX, cardY, cardW, cardH := g.modalCardRect()
	drawRoundedRect(screen, float32(cardX), float32(cardY), float32(cardW), float32(cardH), 14, color.RGBA{16, 22, 36, 248})
	drawRectOutline(screen, float32(cardX), float32(cardY), float32(cardW), float32(cardH), color.RGBA{146, 196, 230, 255})

	g.drawAlphaDebugTextBlock(screen, cardX+18, cardY+22, g.modal.lines, 1)

	bx, by, bw, bh := g.modalOKRect()
	drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 10, color.RGBA{38, 96, 124, 240})
	drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), color.RGBA{183, 219, 246, 255})
	ebitenutil.DebugPrintAt(screen, "OK", int(bx+bw*0.5)-7, int(by+bh*0.5)-7)
}
