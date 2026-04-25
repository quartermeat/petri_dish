package hexglobe

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hex_globe/core"
)

const (
	perkChoiceCount       = 3
	perkChoiceTapCooldown = 1.5 // seconds before taps register, so heated-tap players can read
)

type perkChoiceState struct {
	stageID  string
	perks    []core.PerkDef
	cooldown float64
}

// productionMods builds the per-tick multipliers + productive-power
// accumulator from the player's currently-active perks.
func (g *Game) productionMods() *core.ProductionMods {
	mods := &core.ProductionMods{
		OutputMul:    1,
		PowerCostMul: 1,
		DecayMul:     1,
	}
	for _, perkID := range g.activePerks {
		def, ok := g.perks.Perk(perkID)
		if !ok || def.OneShot {
			continue
		}
		switch def.Kind {
		case core.PerkMinerOutput:
			mods.OutputMul *= 1 + def.Magnitude
		case core.PerkPowerEfficiency:
			mods.PowerCostMul *= 1 - def.Magnitude
		case core.PerkBufferDecay:
			mods.DecayMul *= 1 - def.Magnitude
		}
	}
	if mods.PowerCostMul < 0 {
		mods.PowerCostMul = 0
	}
	if mods.DecayMul < 0 {
		mods.DecayMul = 0
	}
	var spent float64
	mods.ProductivePower = &spent
	return mods
}

// crankPowerBoost returns the per-tap crank multiplier from active perks.
// Default 1.0; each PerkCrankPower perk multiplies in.
func (g *Game) crankPowerBoost() float64 {
	mul := 1.0
	for _, perkID := range g.activePerks {
		def, ok := g.perks.Perk(perkID)
		if !ok || def.OneShot {
			continue
		}
		if def.Kind == core.PerkCrankPower {
			mul *= 1 + def.Magnitude
		}
	}
	return mul
}

// recordProductivePower adds to the current stage's power-spent meter and
// triggers a perk choice if a new threshold has been crossed.
func (g *Game) recordProductivePower(power float64) {
	if power <= 0 || g.currentStageID == "" {
		return
	}
	if g.stagePowerSpent == nil {
		g.stagePowerSpent = map[string]float64{}
	}
	g.stagePowerSpent[g.currentStageID] += power
	g.maybeTriggerPerkChoice()
}

// maybeTriggerPerkChoice checks the current stage's thresholds against
// (power spent, perks already awarded) and opens the picker if a new
// threshold has been reached.
func (g *Game) maybeTriggerPerkChoice() {
	if g.perkChoice != nil {
		return
	}
	stage := g.currentStage()
	if len(stage.PerkPowerThresholds) == 0 || len(stage.PerkPool) == 0 {
		return
	}
	awarded := 0
	if g.perksAwarded != nil {
		awarded = g.perksAwarded[stage.ID]
	}
	if awarded >= len(stage.PerkPowerThresholds) {
		return
	}
	threshold := stage.PerkPowerThresholds[awarded]
	if g.stagePowerSpent[stage.ID] < threshold {
		return
	}
	g.openPerkChoice(stage)
}

// openPerkChoice picks up to perkChoiceCount perks from the stage pool,
// excluding any already-active permanent perks, and shows the picker.
func (g *Game) openPerkChoice(stage core.ProgressStage) {
	owned := map[string]bool{}
	for _, id := range g.activePerks {
		if def, ok := g.perks.Perk(id); ok && !def.OneShot {
			owned[id] = true
		}
	}
	pool := make([]core.PerkDef, 0, len(stage.PerkPool))
	for _, id := range stage.PerkPool {
		if owned[id] {
			continue
		}
		def, ok := g.perks.Perk(id)
		if !ok {
			continue
		}
		pool = append(pool, def)
	}
	if len(pool) == 0 {
		// Nothing left to offer — bump awarded so we don't busy-loop.
		if g.perksAwarded == nil {
			g.perksAwarded = map[string]int{}
		}
		g.perksAwarded[stage.ID]++
		return
	}
	if g.perkRand != nil {
		g.perkRand.Shuffle(len(pool), func(i, j int) {
			pool[i], pool[j] = pool[j], pool[i]
		})
	}
	if len(pool) > perkChoiceCount {
		pool = pool[:perkChoiceCount]
	}
	g.perkChoice = &perkChoiceState{
		stageID:  stage.ID,
		perks:    pool,
		cooldown: perkChoiceTapCooldown,
	}
}

// applyPerk records the picked perk, applies any one-shot effect, and
// dismisses the picker.
func (g *Game) applyPerk(def core.PerkDef) {
	if def.OneShot {
		switch def.Kind {
		case core.PerkResourceGift:
			amount := int(def.Magnitude)
			if amount > 0 && def.Resource != core.ResourceNone {
				if g.inventory == nil {
					g.inventory = map[core.ResourceType]int{}
				}
				g.inventory[def.Resource] += amount
			}
		}
	} else {
		g.activePerks = append(g.activePerks, def.ID)
	}
	if g.perksAwarded == nil {
		g.perksAwarded = map[string]int{}
	}
	if g.perkChoice != nil {
		g.perksAwarded[g.perkChoice.stageID]++
	}
	g.perkChoice = nil
	g.saveNow()
	// Another threshold may already be crossed if power flew past two in
	// the same frame — re-check.
	g.maybeTriggerPerkChoice()
}

// perkChoiceActive reports whether the picker is currently blocking input.
func (g *Game) perkChoiceActive() bool {
	return g.perkChoice != nil
}

// perkProgress reports (power spent in current stage, next threshold, ok).
// ok is false when the stage has no thresholds left to award (or no thresholds
// at all) — callers should hide the bar in that case.
func (g *Game) perkProgress() (float64, float64, bool) {
	stage := g.currentStage()
	if len(stage.PerkPowerThresholds) == 0 {
		return 0, 0, false
	}
	awarded := 0
	if g.perksAwarded != nil {
		awarded = g.perksAwarded[stage.ID]
	}
	if awarded >= len(stage.PerkPowerThresholds) {
		return 0, 0, false
	}
	spent := 0.0
	if g.stagePowerSpent != nil {
		spent = g.stagePowerSpent[stage.ID]
	}
	return spent, stage.PerkPowerThresholds[awarded], true
}

// drawPerkProgressCard renders a small "PERK" meter at (x, y). Returns the
// rendered height so callers can stack other UI below. Returns 0 if there's
// nothing to show.
func (g *Game) drawPerkProgressCard(screen *ebiten.Image, x, y, w, alpha float64) float64 {
	spent, threshold, ok := g.perkProgress()
	if !ok {
		return 0
	}
	h := 44.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 8, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, uint8(210 * alpha)})

	label := fmt.Sprintf("PERK  %d / %d", int(spent), int(threshold))
	g.drawAlphaDebugTextBlock(screen, x+10, y+8, []string{label}, alpha)

	barX := x + 10
	barY := y + 28
	barW := w - 20
	barH := 8.0
	drawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{18, 28, 44, uint8(220 * alpha)})
	progress := 0.0
	if threshold > 0 {
		progress = spent / threshold
	}
	if progress > 1 {
		progress = 1
	}
	if progress > 0 {
		drawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), perkBarFillColor(progress, alpha))
	}
	return h
}

func perkBarFillColor(progress, alpha float64) color.RGBA {
	// green → amber → bright as it fills, so the player feels it heating up
	r := uint8(80 + 140*progress)
	g := uint8(200 - 60*progress)
	b := uint8(120 - 60*progress)
	return color.RGBA{r, g, b, uint8(240 * alpha)}
}

func (g *Game) handlePerkChoiceInput() {
	if g.perkChoice == nil {
		return
	}
	if g.perkChoice.cooldown > 0 {
		g.perkChoice.cooldown -= 1.0 / 60.0
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
	for i := range g.perkChoice.perks {
		cx, cy, cw, ch := g.perkCardRect(i)
		if g.pointInRect(float64(x), float64(y), cx, cy, cw, ch) {
			def := g.perkChoice.perks[i]
			g.applyPerk(def)
			return
		}
	}
}

func (g *Game) perkCardRect(index int) (float64, float64, float64, float64) {
	w := 320.0
	h := 86.0
	gap := 10.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	totalH := float64(perkChoiceCount)*h + float64(perkChoiceCount-1)*gap
	startY := float64(g.screenHeight)*0.5 - totalH*0.5 + 32
	y := startY + float64(index)*(h+gap)
	return x, y, w, h
}

func (g *Game) drawPerkChoice(screen *ebiten.Image) {
	if g.perkChoice == nil {
		return
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{0, 0, 0, 192})

	headerY := 0.0
	if len(g.perkChoice.perks) > 0 {
		_, firstY, _, _ := g.perkCardRect(0)
		headerY = firstY - 56
	} else {
		headerY = float64(g.screenHeight)*0.5 - 60
	}
	headerX := float64(g.screenWidth)*0.5 - 110
	ebitenutil.DebugPrintAt(screen, "POWER MILESTONE", int(headerX), int(headerY))
	ebitenutil.DebugPrintAt(screen, "Choose a perk", int(headerX), int(headerY)+18)

	for i, def := range g.perkChoice.perks {
		cx, cy, cw, ch := g.perkCardRect(i)
		bg := color.RGBA{18, 30, 48, 244}
		edge := color.RGBA{146, 196, 230, 255}
		if def.OneShot {
			bg = color.RGBA{34, 28, 20, 244}
			edge = color.RGBA{230, 188, 120, 255}
		}
		drawRoundedRect(screen, float32(cx), float32(cy), float32(cw), float32(ch), 12, bg)
		drawRectOutline(screen, float32(cx), float32(cy), float32(cw), float32(ch), edge)
		ebitenutil.DebugPrintAt(screen, def.Title, int(cx)+16, int(cy)+12)
		ebitenutil.DebugPrintAt(screen, def.Description, int(cx)+16, int(cy)+36)
		tag := "permanent"
		if def.OneShot {
			tag = "one-shot"
		}
		ebitenutil.DebugPrintAt(screen, tag, int(cx)+16, int(cy)+58)
	}
}
