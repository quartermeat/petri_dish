package hexglobe

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hex_globe/core"
)

const inventoryOverlayCooldown = 0.6

type inventoryOverlayState struct {
	cooldown float64
}

// fullInventoryResources is the canonical resource order shown in the overlay.
var fullInventoryResources = []core.ResourceType{
	core.ResourceStone,
	core.ResourceIronOre,
	core.ResourceCopperOre,
	core.ResourceCoal,
	core.ResourceIronIngot,
	core.ResourceCopperIngot,
	core.ResourceCrystal,
}

var fullInventoryParts = []core.DevicePart{
	core.DevicePartFrame,
	core.DevicePartDrill,
	core.DevicePartMotor,
	core.DevicePartOutput,
	core.DevicePartHandCrank,
}

func (g *Game) inventoryOverlayActive() bool {
	return g.inventoryOverlay != nil
}

func (g *Game) openInventoryOverlay() {
	g.inventoryOverlay = &inventoryOverlayState{cooldown: inventoryOverlayCooldown}
}

// handleInventoryButtonTap consults the inventory ... button rect and opens
// the overlay if hit. Returns true when consumed so callers can stop
// processing other tap targets.
func (g *Game) handleInventoryButtonTap(x, y int) bool {
	if !g.inventoryHasOverflow() {
		return false
	}
	bx, by, bw, bh := g.inventoryMoreButtonRect(16, 16)
	if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
		return false
	}
	g.openInventoryOverlay()
	return true
}

func (g *Game) inventoryOverlayCardRect() (float64, float64, float64, float64) {
	w := 320.0
	h := 380.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	y := float64(g.screenHeight)*0.5 - h*0.5
	return x, y, w, h
}

func (g *Game) inventoryOverlayCloseRect() (float64, float64, float64, float64) {
	cx, cy, _, _ := g.inventoryOverlayCardRect()
	return cx + 12, cy + 12, 38, 30
}

func (g *Game) handleInventoryOverlayInput() {
	if g.inventoryOverlay == nil {
		return
	}
	if g.inventoryOverlay.cooldown > 0 {
		g.inventoryOverlay.cooldown -= 1.0 / 60.0
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
	cx, cy, cw, ch := g.inventoryOverlayCloseRect()
	if g.pointInRect(float64(x), float64(y), cx, cy, cw, ch) {
		g.inventoryOverlay = nil
		return
	}
	// Tap outside the card also closes.
	bx, by, bw, bh := g.inventoryOverlayCardRect()
	if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
		g.inventoryOverlay = nil
	}
}

func (g *Game) drawInventoryOverlay(screen *ebiten.Image) {
	if g.inventoryOverlay == nil {
		return
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{0, 0, 0, 168})

	cx, cy, cw, ch := g.inventoryOverlayCardRect()
	drawRoundedRect(screen, float32(cx), float32(cy), float32(cw), float32(ch), 14, color.RGBA{16, 22, 36, 248})
	drawRectOutline(screen, float32(cx), float32(cy), float32(cw), float32(ch), color.RGBA{146, 196, 230, 255})

	// Close (left arrow) at top-left of card.
	bx, by, bw, bh := g.inventoryOverlayCloseRect()
	drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 6, color.RGBA{40, 56, 74, 236})
	drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), color.RGBA{188, 214, 238, 255})
	g.drawLeftArrow(screen, bx+bw*0.5, by+bh*0.5, 8, color.RGBA{228, 236, 244, 255})

	titleX := cx + 64
	titleY := cy + 16
	ebitenutil.DebugPrintAt(screen, "INVENTORY", int(titleX), int(titleY))

	textX := cx + 24
	rowY := cy + 60
	g.drawInventorySectionHeader(screen, "RESOURCES", textX, rowY)
	rowY += 22
	for _, r := range fullInventoryResources {
		labelX := int(textX) + 18
		if resourceHasMapIcon(r) {
			g.drawInventoryResourceIcon(screen, textX+6, rowY+6, r)
		} else {
			labelX = int(textX)
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%-12s %d", resourceShortLabel(r), g.inventory[r]), labelX, int(rowY))
		rowY += 18
	}

	rowY += 12
	g.drawInventorySectionHeader(screen, "PARTS", textX, rowY)
	rowY += 22
	for _, part := range fullInventoryParts {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%-12s %d", partShortLabel(part), g.partInventory[part]), int(textX), int(rowY))
		rowY += 18
	}
}

func (g *Game) drawInventorySectionHeader(screen *ebiten.Image, label string, x, y float64) {
	ebitenutil.DebugPrintAt(screen, label, int(x), int(y))
	drawFilledRect(screen, float32(x), float32(y+14), 80, 1, color.RGBA{146, 196, 230, 200})
}

func partShortLabel(part core.DevicePart) string {
	switch part {
	case core.DevicePartFrame:
		return "frame"
	case core.DevicePartDrill:
		return "drill"
	case core.DevicePartMotor:
		return "motor"
	case core.DevicePartOutput:
		return "output"
	case core.DevicePartHandCrank:
		return "crank"
	}
	return "part"
}
