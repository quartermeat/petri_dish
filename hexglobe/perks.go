package hexglobe

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hex_globe/core"
)

const (
	perkChoiceCount       = 3
	perkChoiceTapCooldown = 1.5 // seconds before taps register, so heated-tap players can read
	perkChoiceSelectHold  = 0.18
	perkCelebrationLength = 1.35
)

type perkChoiceState struct {
	stageID       string
	perks         []core.PerkDef
	cooldown      float64
	selectedIndex int
	selectedHold  float64
}

// productionMods builds the per-tick multipliers + productive-power
// accumulator from the player's currently-active perks.
func (g *Game) productionMods() *core.ProductionMods {
	mods := &core.ProductionMods{
		OutputMul:         1,
		PowerCostMul:      1,
		SmelterOutputMul:  1,
		SmelterPowerMul:   1,
		GeneratorPowerMul: 1,
		DecayMul:          1,
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
		case core.PerkSmelterOutput:
			mods.SmelterOutputMul *= 1 + def.Magnitude
		case core.PerkSmelterPower:
			mods.SmelterPowerMul *= 1 - def.Magnitude
		case core.PerkGeneratorOutput:
			mods.GeneratorPowerMul *= 1 + def.Magnitude
		case core.PerkBufferDecay:
			mods.DecayMul *= 1 - def.Magnitude
		}
	}
	if mods.PowerCostMul < 0 {
		mods.PowerCostMul = 0
	}
	if mods.SmelterPowerMul < 0 {
		mods.SmelterPowerMul = 0
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

func (g *Game) hasActivePerkKind(kind core.PerkKind) bool {
	for _, perkID := range g.activePerks {
		def, ok := g.perks.Perk(perkID)
		if ok && !def.OneShot && def.Kind == kind {
			return true
		}
	}
	return false
}

// recordProductivePower adds to the current tactical region's power-spent
// meter and triggers a perk choice if a new threshold has been crossed.
func (g *Game) recordProductivePower(power float64) {
	progressKey := g.perkProgressKey()
	if power <= 0 || progressKey == "" {
		return
	}
	if g.stagePowerSpent == nil {
		g.stagePowerSpent = map[string]float64{}
	}
	g.stagePowerSpent[progressKey] += power
	g.maybeTriggerPerkChoice()
}

// maybeTriggerPerkChoice checks the current region-run thresholds against
// (power spent, perks already awarded) and opens the picker if a new
// threshold has been reached.
func (g *Game) maybeTriggerPerkChoice() {
	if g.perkChoice != nil {
		return
	}
	progressKey := g.perkProgressKey()
	if progressKey == "" {
		return
	}
	stage := g.currentStage()
	if len(stage.PerkPowerThresholds) == 0 || len(stage.PerkPool) == 0 {
		return
	}
	awarded := 0
	if g.perksAwarded != nil {
		awarded = g.perksAwarded[progressKey]
	}
	threshold := nextPerkThreshold(stage.PerkPowerThresholds, awarded)
	if g.stagePowerSpent[progressKey] < threshold {
		return
	}
	g.openPerkChoice(stage, progressKey)
}

// openPerkChoice picks up to perkChoiceCount perks from the stage pool.
// Permanent perks can be picked repeatedly and stack cumulatively.
func (g *Game) openPerkChoice(stage core.ProgressStage, progressKey string) {
	pool := make([]core.PerkDef, 0, len(stage.PerkPool))
	if stage.ID == "bootstrap" && !g.gateUplinkUnlocked {
		if def, ok := g.perks.Perk("gate-uplink"); ok {
			pool = append(pool, def)
		}
		g.perkChoice = &perkChoiceState{
			stageID:       progressKey,
			perks:         pool,
			cooldown:      perkChoiceTapCooldown,
			selectedIndex: -1,
		}
		g.perkCelebrationTimer = perkCelebrationLength
		return
	}
	for _, id := range stage.PerkPool {
		def, ok := g.perks.Perk(id)
		if !ok {
			continue
		}
		if def.Kind == core.PerkGateUplink && g.gateUplinkUnlocked {
			continue
		}
		pool = append(pool, def)
	}
	if len(pool) == 0 {
		// Nothing left to offer — bump awarded so we don't busy-loop.
		if g.perksAwarded == nil {
			g.perksAwarded = map[string]int{}
		}
		g.perksAwarded[progressKey]++
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
		stageID:       progressKey,
		perks:         pool,
		cooldown:      perkChoiceTapCooldown,
		selectedIndex: -1,
	}
	g.perkCelebrationTimer = perkCelebrationLength
}

// applyPerk records the picked perk, applies any one-shot effect, and
// dismisses the picker.
func (g *Game) applyPerk(def core.PerkDef) {
	if def.OneShot {
		switch def.Kind {
		case core.PerkResourceGift:
			amount := int(def.Magnitude)
			if amount > 0 && def.Resource != core.ResourceNone {
				inventory := g.inventory
				if !g.gateExportUnlocked() {
					inventory = g.buildResourceInventory()
				}
				if inventory == nil {
					inventory = map[core.ResourceType]int{}
					g.inventory = inventory
				}
				inventory[def.Resource] += amount
				if g.minedTotals == nil {
					g.minedTotals = map[core.ResourceType]int{}
				}
				g.minedTotals[def.Resource] += amount
			}
		case core.PerkGateUplink:
			g.gateUplinkUnlocked = true
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
	g.saveSoon()
	// Another threshold may already be crossed if power flew past two in
	// the same frame — re-check.
	g.maybeTriggerPerkChoice()
}

// perkChoiceActive reports whether the picker is currently blocking input.
func (g *Game) perkChoiceActive() bool {
	return g.perkChoice != nil
}

// perkProgress reports cumulative progress toward the next perk in the
// current tactical region run.
// ok is false when the stage has no thresholds left to award (or no thresholds
// at all) — callers should hide the bar in that case.
func (g *Game) perkProgress() (float64, float64, bool) {
	progressKey := g.perkProgressKey()
	if progressKey == "" {
		return 0, 0, false
	}
	stage := g.currentStage()
	if len(stage.PerkPowerThresholds) == 0 {
		return 0, 0, false
	}
	awarded := 0
	if g.perksAwarded != nil {
		awarded = g.perksAwarded[progressKey]
	}
	spent := 0.0
	if g.stagePowerSpent != nil {
		spent = g.stagePowerSpent[progressKey]
	}
	return spent, nextPerkThreshold(stage.PerkPowerThresholds, awarded), true
}

func (g *Game) perkProgressKey() string {
	if g.tacticalID < 0 {
		return ""
	}
	return fmt.Sprintf("region:%d", g.tacticalID)
}

func nextPerkThreshold(thresholds []float64, awarded int) float64 {
	if len(thresholds) == 0 {
		return 0
	}
	if awarded < len(thresholds) {
		return thresholds[awarded]
	}
	if len(thresholds) == 1 {
		return thresholds[0] * float64(awarded+1)
	}
	last := thresholds[len(thresholds)-1]
	prev := thresholds[len(thresholds)-2]
	step := last - prev
	if step <= 0 {
		step = last
	}
	return last + step*float64(awarded-len(thresholds)+1)
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
	if g.perkChoice.selectedIndex >= 0 {
		g.perkChoice.selectedHold -= 1.0 / 60.0
		if g.perkChoice.selectedHold <= 0 && g.perkChoice.selectedIndex < len(g.perkChoice.perks) {
			g.applyPerk(g.perkChoice.perks[g.perkChoice.selectedIndex])
		}
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
			g.perkChoice.selectedIndex = i
			g.perkChoice.selectedHold = perkChoiceSelectHold
			return
		}
	}
}

func (g *Game) advancePerkCelebration(dt float64) {
	if g.perkCelebrationTimer <= 0 {
		return
	}
	g.perkCelebrationTimer = math.Max(0, g.perkCelebrationTimer-dt)
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
	g.drawPerkCelebration(screen)
	overlayAlpha := uint8(192)
	if g.perkCelebrationTimer > 0 {
		progress := 1 - g.perkCelebrationTimer/perkCelebrationLength
		overlayAlpha = uint8(116 + 76*clampRange(progress, 0, 1))
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{0, 0, 0, overlayAlpha})

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
		text := color.RGBA{232, 240, 248, 255}
		if def.OneShot {
			bg = color.RGBA{34, 28, 20, 244}
			edge = color.RGBA{230, 188, 120, 255}
			text = color.RGBA{248, 234, 206, 255}
		}
		selected := i == g.perkChoice.selectedIndex
		if selected {
			if def.OneShot {
				bg = color.RGBA{92, 62, 20, 252}
				edge = color.RGBA{255, 224, 148, 255}
				text = color.RGBA{255, 244, 222, 255}
			} else {
				bg = color.RGBA{30, 68, 92, 252}
				edge = color.RGBA{182, 238, 255, 255}
				text = color.RGBA{244, 252, 255, 255}
			}
			drawRoundedRect(screen, float32(cx-2), float32(cy-2), float32(cw+4), float32(ch+4), 14, color.RGBA{255, 255, 255, 26})
		}
		drawRoundedRect(screen, float32(cx), float32(cy), float32(cw), float32(ch), 12, bg)
		drawRectOutline(screen, float32(cx), float32(cy), float32(cw), float32(ch), edge)
		g.drawTintedDebugTextBlock(screen, cx+16, cy+12, []string{def.Title}, 1, float32(text.R)/255, float32(text.G)/255, float32(text.B)/255)
		g.drawTintedDebugTextBlock(screen, cx+16, cy+36, []string{def.Description}, 1, float32(text.R)/255, float32(text.G)/255, float32(text.B)/255)
		tag := "permanent"
		if def.OneShot {
			tag = "one-shot"
		}
		if selected {
			tag = "selected"
		}
		g.drawTintedDebugTextBlock(screen, cx+16, cy+58, []string{tag}, 1, float32(text.R)/255, float32(text.G)/255, float32(text.B)/255)
	}
}

func (g *Game) drawPerkCelebration(screen *ebiten.Image) {
	if g.perkCelebrationTimer <= 0 {
		return
	}
	w := float64(g.screenWidth)
	h := float64(g.screenHeight)
	cx := w * 0.5
	cy := h * 0.43
	progress := 1 - g.perkCelebrationTimer/perkCelebrationLength
	progress = clampRange(progress, 0, 1)
	easeOut := 1 - math.Pow(1-progress, 3)
	fade := clampRange(g.perkCelebrationTimer/0.52, 0, 1)

	drawFilledRect(screen, 0, 0, float32(w), float32(h), color.RGBA{14, 31, 42, uint8(170 * fade)})
	maxRadius := math.Hypot(w, h)
	for i := 0; i < 18; i++ {
		angle := float64(i)/18*math.Pi*2 + g.animationTime*0.18
		rayW := maxRadius * 0.045
		inner := 32.0
		outer := maxRadius * (0.28 + 0.62*easeOut)
		points := []screenPoint{
			{x: cx + math.Cos(angle-0.035)*inner, y: cy + math.Sin(angle-0.035)*inner},
			{x: cx + math.Cos(angle+0.035)*inner, y: cy + math.Sin(angle+0.035)*inner},
			{x: cx + math.Cos(angle+rayW/outer)*outer, y: cy + math.Sin(angle+rayW/outer)*outer},
			{x: cx + math.Cos(angle-rayW/outer)*outer, y: cy + math.Sin(angle-rayW/outer)*outer},
		}
		alpha := uint8(32 * fade)
		if i%3 == 0 {
			alpha = uint8(52 * fade)
		}
		drawScreenPolygon(screen, points, color.RGBA{76, 230, 190, alpha})
	}

	for i := 0; i < 4; i++ {
		r := float32(54 + float64(i)*58 + easeOut*150)
		alpha := uint8(math.Max(0, (120-float64(i)*18)*fade))
		vector.StrokeCircle(screen, float32(cx), float32(cy), r, float32(2+i), color.RGBA{255, 214, 112, alpha}, false)
	}
	drawDisc(screen, float32(cx), float32(cy), float32(40+80*(1-progress)), color.RGBA{82, 244, 184, uint8(84 * fade)})

	for i := 0; i < 34; i++ {
		angle := float64(i)*2.399963 + g.animationTime*0.28
		dist := (48 + float64((i*37)%230)) * (0.25 + 0.95*easeOut)
		px := cx + math.Cos(angle)*dist
		py := cy + math.Sin(angle)*dist*0.72
		size := float32(2.2 + float64(i%5)*0.8)
		col := color.RGBA{255, 226, 132, uint8(210 * fade)}
		if i%2 == 0 {
			col = color.RGBA{108, 242, 212, uint8(190 * fade)}
		}
		drawDisc(screen, float32(px), float32(py), size, col)
	}
}
