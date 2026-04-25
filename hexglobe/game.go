package hexglobe

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hex_globe/core"
)

const autoSaveInterval = 60.0

const (
	defaultScreenWidth  = 432
	defaultScreenHeight = 768
	minZoom             = 0.7
	maxZoom             = 5.2
	dragThreshold       = 8
	cameraZ             = 3.1
	minimapWidth        = 138
	minimapHeight       = 86
	globeZoomBoost      = 1.9
	tacticalRadius      = 5
	shadowZoomThreshold = 2.4
	tacticalMinZoom     = 0.55
	tacticalMaxZoom     = 2.4
	statsZoomStart      = 1.2
)

type viewMode int

const (
	modeStrategic viewMode = iota
	modeTactical
	modeBuild
	modeResearch
	modeSettings
)

type Game struct {
	globe                 *core.Globe
	ruleset               core.Ruleset
	screenWidth           int
	screenHeight          int
	mode                  viewMode
	dragging              bool
	dragTouchID           ebiten.TouchID
	dragStartX            int
	dragStartY            int
	dragLastX             int
	dragLastY             int
	dragMoved             bool
	zoom                  float64
	touchIDs              []ebiten.TouchID
	pinching              bool
	pinchTouchA           ebiten.TouchID
	pinchTouchB           ebiten.TouchID
	pinchPrevGap          float64
	tacticalMaps          map[int]*core.TacticalMap
	tacticalID            int
	tacticalTile          int
	tacticalZoom          float64
	tacticalPanX          float64
	tacticalPanY          float64
	buildPart             core.DevicePart
	inventory             map[core.ResourceType]int
	partInventory         map[core.DevicePart]int
	starterMinerCount     int
	starterMinerPlaced    int
	starterMinerRecovered int
	minedTotals           map[core.ResourceType]int
	progression           *core.ProgressionBook
	recipes               *core.RecipeBook
	knownRecipes          map[string]bool
	researchRecipeID      string
	researchLayout        *core.DeviceLayout
	currentStageID        string
	settingsDown          bool
	settingsX             int
	settingsY             int
	settingsTouch         ebiten.TouchID
	screenshotPath        string
	screenshotFrames      int
	screenshotDone        bool
	screenshotErr         error
	saveDir               string
	version               string
	autoSaveTimer         float64
	tutorialLines         []string
	tutorialSeen          map[string]bool
	tutorialDismissTimer  float64
	saveOverlayActive     bool
	saveOverlayStage      saveOverlayStage
	saveOverlayData       *core.SaveData
	saveOverlayBytes      []byte
	saveOverlayTempPath   string
	saveOverlayFinalPath  string
	modal                 *modalState
	lastTapCellID         int
	lastTapTime           time.Time
	inventoryOverlay      *inventoryOverlayState
	perks                 *core.PerkBook
	activePerks           []string
	stagePowerSpent       map[string]float64
	perksAwarded          map[string]int
	perkChoice            *perkChoiceState
	perkRand              *rand.Rand
}

type drawCell struct {
	index   int
	center  core.Vec3
	corners []core.Vec3
	style   core.CellStyle
	depth   float64
}

type screenPoint struct {
	x float64
	y float64
}

type buildPlan struct {
	rawSpend  map[core.ResourceType]int
	partSpend map[core.DevicePart]int
}

type strategicDeviceBadge struct {
	kind    core.DeviceKind
	special bool
}

type saveOverlayStage int

const (
	saveStageSnapshot saveOverlayStage = iota
	saveStageMarshal
	saveStageWriteTemp
	saveStageRename
	saveStageDone
)

var solidPixel = ebiten.NewImage(1, 1)

func init() {
	solidPixel.Fill(color.White)
}

func NewGame() *Game {
	g := &Game{
		screenWidth:   defaultScreenWidth,
		screenHeight:  defaultScreenHeight,
		zoom:          1,
		dragTouchID:   -1,
		pinchTouchA:   -1,
		pinchTouchB:   -1,
		settingsTouch: -1,
		tacticalMaps:  map[int]*core.TacticalMap{},
		tacticalID:    -1,
		tacticalTile:  -1,
		tacticalZoom:  1,
		buildPart:     core.DevicePartFrame,
		progression:     core.DefaultProgressionBook(),
		recipes:         core.DefaultRecipeBook(),
		perks:           core.DefaultPerkBook(),
		knownRecipes:    map[string]bool{},
		stagePowerSpent: map[string]float64{},
		perksAwarded:    map[string]int{},
		perkRand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		lastTapCellID:   -1,
	}
	g.installFreshWorld(time.Now().UnixNano())
	return g
}

// installFreshWorld replaces globe + ruleset + tactical state with a new
// world derived from seed. Inventory is reset to the starter values.
func (g *Game) installFreshWorld(seed int64) {
	globe := core.NewGlobeWithSeed(1, 3, seed)
	rules := core.NewDemoRulesetSeeded(seed)
	rules.Init(globe)
	g.globe = globe
	g.ruleset = rules
	g.tacticalMaps = map[int]*core.TacticalMap{}
	g.tacticalID = -1
	g.tacticalTile = -1
	g.tacticalZoom = 1
	g.tacticalPanX = 0
	g.tacticalPanY = 0
	g.zoom = 1
	g.mode = modeStrategic
	g.dragging = false
	g.dragTouchID = -1
	g.pinching = false
	g.pinchTouchA = -1
	g.pinchTouchB = -1
	g.inventory = map[core.ResourceType]int{}
	g.partInventory = map[core.DevicePart]int{}
	g.starterMinerCount = 1
	g.starterMinerPlaced = 0
	g.starterMinerRecovered = 0
	g.minedTotals = map[core.ResourceType]int{}
	if g.progression == nil {
		g.progression = core.DefaultProgressionBook()
	}
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	g.knownRecipes = map[string]bool{}
	g.researchRecipeID = ""
	g.researchLayout = nil
	g.currentStageID = g.progression.StartStageID
	g.tutorialLines = nil
	g.tutorialSeen = map[string]bool{}
	g.tutorialDismissTimer = 0
	g.activePerks = nil
	g.stagePowerSpent = map[string]float64{}
	g.perksAwarded = map[string]int{}
	g.perkChoice = nil
	if g.perks == nil {
		g.perks = core.DefaultPerkBook()
	}
}

// SetSaveDir tells the game where to read/write save.json. Empty disables
// persistence (in-memory only).
func (g *Game) SetSaveDir(dir string) {
	g.saveDir = dir
}

// SetVersion stamps the build version onto saves. Empty version causes
// every load to be treated as a mismatch.
func (g *Game) SetVersion(version string) {
	g.version = version
}

// LoadOrInit restores prior progress from saveDir. Three outcomes:
//   - no save file → leave fresh world, write a save so future launches resume
//   - save matches version → apply state on top of fresh world
//   - save exists but version doesn't match (or file is corrupt) → show
//     modal informing the player; on OK, wipe with new seed and save
func (g *Game) LoadOrInit() {
	if g.saveDir == "" {
		return
	}
	data, err := core.LoadSave(g.saveDir)
	if err != nil {
		log.Printf("hex_globe: save load failed (%v) — resetting", err)
		g.showModal([]string{
			"Save data couldn't be read.",
			"Resetting to fresh state.",
		}, func() {
			g.wipeAndRestart(time.Now().UnixNano())
		})
		return
	}
	if data == nil {
		// No save yet — write one so seed is preserved next launch.
		g.saveNow()
		return
	}
	if !data.VersionMatches(g.version) {
		g.showModal([]string{
			"Save is from a different build.",
			"Too many changes to load it.",
			"Resetting to fresh state.",
		}, func() {
			g.wipeAndRestart(time.Now().UnixNano())
		})
		return
	}
	g.applySave(data)
}

func (g *Game) applySave(data *core.SaveData) {
	g.installFreshWorld(data.WorldSeed)
	for resource, count := range data.Inventory {
		g.inventory[resource] = count
	}
	g.partInventory = map[core.DevicePart]int{}
	for part, count := range data.PartInventory {
		g.partInventory[part] = count
	}
	switch {
	case data.StarterMinerCount != nil:
		g.starterMinerCount = *data.StarterMinerCount
	case !g.hasPlacedStarterMiner():
		// Migrate pre-starter-miner saves by granting the unit back if it
		// isn't already deployed anywhere in the restored tactical maps.
		g.starterMinerCount = 1
	default:
		g.starterMinerCount = 0
	}
	if len(data.MinedTotals) > 0 {
		g.minedTotals = make(map[core.ResourceType]int, len(data.MinedTotals))
		for resource, count := range data.MinedTotals {
			g.minedTotals[resource] = count
		}
	}
	g.knownRecipes = map[string]bool{}
	for _, recipeID := range data.KnownRecipes {
		g.knownRecipes[recipeID] = true
	}
	g.tutorialSeen = map[string]bool{}
	for _, key := range data.TutorialSeen {
		g.tutorialSeen[key] = true
	}
	legacyBuildDiscovery := data.StarterMinerCount == nil
	if legacyBuildDiscovery {
		// Older saves predate the starter-miner split and could mark the
		// normal miner as "known" from a different progression model.
		delete(g.knownRecipes, "miner")
	}
	g.globe.CameraLon = data.Camera.Lon
	g.globe.CameraLat = data.Camera.Lat
	if data.Camera.Zoom > 0 {
		g.zoom = data.Camera.Zoom
	}
	if data.Selected >= 0 && data.Selected < len(g.globe.Cells) {
		g.globe.SelectedCell = data.Selected
	}
	if data.CurrentStage != "" {
		g.currentStageID = data.CurrentStage
	}
	if len(data.ActivePerks) > 0 {
		g.activePerks = append([]string(nil), data.ActivePerks...)
	}
	if len(data.StagePowerSpent) > 0 {
		g.stagePowerSpent = make(map[string]float64, len(data.StagePowerSpent))
		for stage, power := range data.StagePowerSpent {
			g.stagePowerSpent[stage] = power
		}
	}
	if len(data.PerksAwarded) > 0 {
		g.perksAwarded = make(map[string]int, len(data.PerksAwarded))
		for stage, count := range data.PerksAwarded {
			g.perksAwarded[stage] = count
		}
	}
	for _, entry := range data.Tactical {
		if entry.Map == nil {
			continue
		}
		entry.Map.Rehydrate()
		g.tacticalMaps[entry.CellID] = entry.Map
	}
}

func (g *Game) buildSaveData() *core.SaveData {
	tactical := make([]core.SavedTacticalEntry, 0, len(g.tacticalMaps))
	for cellID, tmap := range g.tacticalMaps {
		tactical = append(tactical, core.SavedTacticalEntry{
			CellID: cellID,
			Map:    tmap,
		})
	}
	inventory := make(map[core.ResourceType]int, len(g.inventory))
	for k, v := range g.inventory {
		inventory[k] = v
	}
	partInventory := make(map[core.DevicePart]int, len(g.partInventory))
	for k, v := range g.partInventory {
		partInventory[k] = v
	}
	minedTotals := make(map[core.ResourceType]int, len(g.minedTotals))
	for k, v := range g.minedTotals {
		minedTotals[k] = v
	}
	knownRecipes := make([]string, 0, len(g.knownRecipes))
	for recipeID, known := range g.knownRecipes {
		if known {
			knownRecipes = append(knownRecipes, recipeID)
		}
	}
	tutorialSeen := make([]string, 0, len(g.tutorialSeen))
	for key, seen := range g.tutorialSeen {
		if seen {
			tutorialSeen = append(tutorialSeen, key)
		}
	}
	starterMinerCount := g.starterMinerCount
	activePerks := append([]string(nil), g.activePerks...)
	stagePowerSpent := make(map[string]float64, len(g.stagePowerSpent))
	for stage, power := range g.stagePowerSpent {
		stagePowerSpent[stage] = power
	}
	perksAwarded := make(map[string]int, len(g.perksAwarded))
	for stage, count := range g.perksAwarded {
		perksAwarded[stage] = count
	}
	return &core.SaveData{
		Version:           g.version,
		WorldSeed:         g.globe.Seed,
		Inventory:         inventory,
		PartInventory:     partInventory,
		StarterMinerCount: &starterMinerCount,
		TutorialSeen:      tutorialSeen,
		CurrentStage:      g.currentStageID,
		KnownRecipes:      knownRecipes,
		MinedTotals:       minedTotals,
		ActivePerks:       activePerks,
		StagePowerSpent:   stagePowerSpent,
		PerksAwarded:      perksAwarded,
		Camera: core.SavedCamera{
			Lon:  g.globe.CameraLon,
			Lat:  g.globe.CameraLat,
			Zoom: g.zoom,
		},
		Selected: g.globe.SelectedCell,
		Tactical: tactical,
	}
}

func (g *Game) saveNow() {
	g.saveNowImmediate()
}

func (g *Game) saveNowImmediate() {
	if g.saveDir == "" {
		return
	}
	if err := g.buildSaveData().Save(g.saveDir); err != nil {
		log.Printf("hex_globe: save write failed: %v", err)
	}
	g.autoSaveTimer = 0
}

func (g *Game) beginSaveOverlay() {
	if g.saveDir == "" || g.saveOverlayActive {
		return
	}
	g.saveOverlayActive = true
	g.saveOverlayStage = saveStageSnapshot
	g.saveOverlayData = nil
	g.saveOverlayBytes = nil
	g.saveOverlayTempPath = ""
	g.saveOverlayFinalPath = ""
}

func (g *Game) advanceSaveOverlay() {
	if !g.saveOverlayActive {
		return
	}
	switch g.saveOverlayStage {
	case saveStageSnapshot:
		g.saveOverlayData = g.buildSaveData()
		if g.saveDir != "" {
			g.saveOverlayFinalPath = filepath.Join(g.saveDir, core.SaveFileName)
			g.saveOverlayTempPath = g.saveOverlayFinalPath + ".tmp"
		}
		g.saveOverlayStage = saveStageMarshal
	case saveStageMarshal:
		if g.saveOverlayData == nil {
			g.finishSaveOverlay()
			return
		}
		bytes, err := json.MarshalIndent(g.saveOverlayData, "", "  ")
		if err != nil {
			log.Printf("hex_globe: save marshal failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		g.saveOverlayBytes = bytes
		g.saveOverlayStage = saveStageWriteTemp
	case saveStageWriteTemp:
		if err := os.MkdirAll(g.saveDir, 0o755); err != nil {
			log.Printf("hex_globe: save mkdir failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		if err := os.WriteFile(g.saveOverlayTempPath, g.saveOverlayBytes, 0o644); err != nil {
			log.Printf("hex_globe: save temp write failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		g.saveOverlayStage = saveStageRename
	case saveStageRename:
		if err := os.Rename(g.saveOverlayTempPath, g.saveOverlayFinalPath); err != nil {
			log.Printf("hex_globe: save rename failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		g.finishSaveOverlay()
	}
}

func (g *Game) finishSaveOverlay() {
	g.saveOverlayActive = false
	g.saveOverlayStage = saveStageDone
	g.saveOverlayData = nil
	g.saveOverlayBytes = nil
	g.saveOverlayTempPath = ""
	g.saveOverlayFinalPath = ""
	g.autoSaveTimer = 0
}

func (g *Game) wipeAndRestart(seed int64) {
	g.installFreshWorld(seed)
	g.saveNow()
}

func (g *Game) requestReset() {
	g.showModal([]string{
		"Wipe all progress and start over.",
		"World stays the same.",
	}, func() {
		g.wipeAndRestart(g.globe.Seed)
	})
}

func (g *Game) requestRegen() {
	g.showModal([]string{
		"Regenerate the world.",
		"Resources and devices will be lost.",
	}, func() {
		g.wipeAndRestart(time.Now().UnixNano())
	})
}

func (g *Game) OpenSettingsForTesting() {
	g.mode = modeSettings
}

func (g *Game) tutorialActive() bool {
	return len(g.tutorialLines) > 0
}

func (g *Game) showTutorialOnce(key string, lines []string) {
	if key == "" || len(lines) == 0 {
		return
	}
	if g.tutorialSeen == nil {
		g.tutorialSeen = map[string]bool{}
	}
	if g.tutorialSeen[key] || g.tutorialActive() {
		return
	}
	g.tutorialSeen[key] = true
	g.tutorialLines = append([]string(nil), lines...)
	g.tutorialDismissTimer = 2
}

func (g *Game) handleTutorialInput() {
	if !g.tutorialActive() {
		return
	}
	if g.tutorialDismissTimer > 0 {
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.tutorialLines = nil
		return
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		g.tutorialLines = nil
	}
}

func (g *Game) drawTutorial(screen *ebiten.Image) {
	if !g.tutorialActive() {
		return
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{0, 0, 0, 132})
	w := 300.0
	h := 36.0 + float64(len(g.tutorialLines))*18.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	y := float64(g.screenHeight)*0.24 - h*0.5
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{20, 28, 44, 248})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{170, 212, 242, 255})
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, g.tutorialLines, 1)
}

func (g *Game) drawSaveOverlay(screen *ebiten.Image) {
	if !g.saveOverlayActive {
		return
	}
	drawFilledRect(screen, 0, 0, float32(g.screenWidth), float32(g.screenHeight), color.RGBA{4, 8, 16, 220})
	w := 280.0
	h := 96.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	y := float64(g.screenHeight)*0.5 - h*0.5
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{18, 24, 38, 248})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{150, 198, 232, 255})
	stageLine, progress := g.saveOverlayStatus()
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, []string{
		"Saving",
		stageLine,
	}, 1)
	barX := x + 18
	barY := y + 58
	barW := w - 36
	barH := 14.0
	drawRoundedRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), 7, color.RGBA{34, 44, 60, 255})
	drawRoundedRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), 7, color.RGBA{104, 196, 232, 255})
}

func (g *Game) saveOverlayStatus() (string, float64) {
	switch g.saveOverlayStage {
	case saveStageSnapshot:
		return "Snapshotting world state", 0.20
	case saveStageMarshal:
		return "Encoding save data", 0.45
	case saveStageWriteTemp:
		return "Writing temp save file", 0.72
	case saveStageRename:
		return "Finalizing save file", 0.92
	default:
		return "Done", 1.0
	}
}

func (g *Game) triggerTutorials() {
	stage := g.currentStage()
	if stage.ID != "bootstrap" {
		return
	}
	stone := g.minedTotals[core.ResourceStone]
	if stone >= 1 {
		g.showTutorialOnce("bootstrap_stone_first", []string{
			"Just found stone!",
			"Unmarked land tiles are your baseline source.",
		})
	}
	if stone >= 5 {
		g.showTutorialOnce("bootstrap_stone_half", []string{
			"Half way there!",
			"Keep moving the starter miner to new deposits.",
		})
	}
}

func (g *Game) ConfigureScreenshot(path string, frames int) {
	g.screenshotPath = path
	g.screenshotFrames = frames
}

func (g *Game) Update() error {
	if g.screenshotDone {
		if g.screenshotErr != nil {
			return g.screenshotErr
		}
		return ebiten.Termination
	}
	dt := 1.0 / 60.0
	if g.modalActive() {
		g.handleModalInput()
		return nil
	}
	if g.perkChoiceActive() {
		g.handlePerkChoiceInput()
		return nil
	}
	if g.inventoryOverlayActive() {
		g.handleInventoryOverlayInput()
		return nil
	}
	if g.saveOverlayActive {
		g.advanceSaveOverlay()
		return nil
	}
	if g.tutorialActive() {
		if g.tutorialDismissTimer > 0 {
			g.tutorialDismissTimer = math.Max(0, g.tutorialDismissTimer-dt)
		}
		g.handleTutorialInput()
		return nil
	}
	mods := g.productionMods()
	for _, tmap := range g.tacticalMaps {
		tmap.Produce(dt, g.inventory, g.minedTotals, mods)
	}
	if mods.ProductivePower != nil && *mods.ProductivePower > 0 {
		g.recordProductivePower(*mods.ProductivePower)
	}
	g.advanceProgression()
	g.triggerTutorials()
	if g.screenshotPath != "" && g.screenshotFrames > 0 {
		g.screenshotFrames--
	}
	g.autoSaveTimer += dt
	if g.autoSaveTimer >= autoSaveInterval {
		g.beginSaveOverlay()
		return nil
	}
	if g.mode == modeResearch {
		g.handleResearchInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeBuild {
		g.handleBuildInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeSettings {
		g.handleSettingsInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeTactical {
		g.handleTacticalInput()
		if tmap := g.currentTacticalMap(); tmap != nil {
			tmap.Update()
		}
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	g.handlePointerInput()
	g.ruleset.Update(g.globe, dt)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.mode == modeResearch {
		g.drawResearchView(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeBuild {
		g.drawBuildView(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeSettings {
		g.drawSettings(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeTactical {
		g.drawTactical(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	screen.Fill(color.RGBA{8, 14, 30, 255})
	g.drawBackdrop(screen)
	g.drawGlobe(screen)
	g.drawMinimap(screen)
	g.drawStrategicSettingsButton(screen)
	g.drawStrategicStats(screen)
	g.drawStrategicStageGoals(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
	invW, invH := g.inventoryCardSize()
	g.drawPerkProgressCard(screen, 16, 16+invH+8, invW, 1)
	if g.strategicDeviceCount() > 0 {
		enterX, enterY, enterW, _ := g.enterButtonRect()
		deviceH := g.strategicDevicesCardHeight()
		deviceX := enterX + enterW - 170
		deviceY := enterY - 12 - deviceH
		g.drawStrategicDevicesCard(screen, deviceX, deviceY, 1)
	}
	g.drawModal(screen)
	g.drawPerkChoice(screen)
	g.drawInventoryOverlay(screen)
	g.drawTutorial(screen)
	g.drawSaveOverlay(screen)
	g.captureScreenshotIfReady(screen)
}

func (g *Game) captureScreenshotIfReady(screen *ebiten.Image) {
	if g.screenshotDone || g.screenshotPath == "" || g.screenshotFrames > 0 {
		return
	}
	g.screenshotErr = saveImagePNG(screen, g.screenshotPath, g.screenWidth, g.screenHeight)
	g.screenshotDone = true
}

func saveImagePNG(screen *ebiten.Image, path string, width, height int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf := make([]byte, 4*width*height)
	screen.ReadPixels(buf)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, buf)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.screenWidth, g.screenHeight
}

func (g *Game) ScreenWidth() int {
	return g.screenWidth
}

func (g *Game) ScreenHeight() int {
	return g.screenHeight
}

func (g *Game) handlePointerInput() {
	g.handleWheelZoom()

	g.touchIDs = ebiten.AppendTouchIDs(g.touchIDs[:0])
	if len(g.touchIDs) >= 2 {
		g.handlePinchZoom(g.touchIDs[0], g.touchIDs[1])
		return
	}
	if g.pinching {
		g.pinching = false
		g.pinchTouchA = -1
		g.pinchTouchB = -1
		g.dragging = false
		g.dragTouchID = -1
		if len(g.touchIDs) == 1 {
			x, y := ebiten.TouchPosition(g.touchIDs[0])
			g.beginDrag(g.touchIDs[0], x, y)
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.beginDrag(-1, x, y)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.dragTouchID == -1 {
		g.finishSelection(g.dragLastX, g.dragLastY)
		g.dragging = false
	}

	if g.dragTouchID == -1 {
		justTouched := inpututil.AppendJustPressedTouchIDs(nil)
		if len(justTouched) > 0 {
			x, y := ebiten.TouchPosition(justTouched[0])
			g.beginDrag(justTouched[0], x, y)
		}
	}

	if g.dragTouchID != -1 {
		ids := ebiten.AppendTouchIDs(nil)
		active := false
		for _, id := range ids {
			if id == g.dragTouchID {
				active = true
				x, y := ebiten.TouchPosition(id)
				g.applyDrag(x, y)
				break
			}
		}
		if !active {
			x, y := inpututil.TouchPositionInPreviousTick(g.dragTouchID)
			g.finishSelection(x, y)
			g.dragTouchID = -1
			g.dragging = false
		}
		return
	}

	if g.dragging {
		x, y := ebiten.CursorPosition()
		g.applyDrag(x, y)
	}
}

func (g *Game) handleTacticalInput() {
	g.handleTacticalZoom()

	g.touchIDs = ebiten.AppendTouchIDs(g.touchIDs[:0])
	if len(g.touchIDs) >= 2 {
		g.handleTacticalPinchZoom(g.touchIDs[0], g.touchIDs[1])
		return
	}
	if g.pinching {
		g.pinching = false
		g.pinchTouchA = -1
		g.pinchTouchB = -1
		g.dragging = false
		g.dragTouchID = -1
		if len(g.touchIDs) == 1 {
			x, y := ebiten.TouchPosition(g.touchIDs[0])
			g.beginDrag(g.touchIDs[0], x, y)
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.beginDrag(-1, x, y)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.dragTouchID == -1 {
		g.finishTacticalPointer(g.dragLastX, g.dragLastY)
		g.dragging = false
	}

	if g.dragTouchID == -1 {
		justTouched := inpututil.AppendJustPressedTouchIDs(nil)
		if len(justTouched) > 0 {
			x, y := ebiten.TouchPosition(justTouched[0])
			g.beginDrag(justTouched[0], x, y)
		}
	}

	if g.dragTouchID != -1 {
		ids := ebiten.AppendTouchIDs(nil)
		active := false
		for _, id := range ids {
			if id == g.dragTouchID {
				active = true
				x, y := ebiten.TouchPosition(id)
				g.applyTacticalDrag(x, y)
				break
			}
		}
		if !active {
			x, y := inpututil.TouchPositionInPreviousTick(g.dragTouchID)
			g.finishTacticalPointer(x, y)
			g.dragTouchID = -1
			g.dragging = false
		}
		return
	}

	if g.dragging {
		x, y := ebiten.CursorPosition()
		g.applyTacticalDrag(x, y)
	}
}

func (g *Game) beginDrag(touchID ebiten.TouchID, x, y int) {
	g.dragging = true
	g.dragTouchID = touchID
	g.dragStartX = x
	g.dragStartY = y
	g.dragLastX = x
	g.dragLastY = y
	g.dragMoved = false
}

const doubleTapWindow = 400 * time.Millisecond

func (g *Game) finishSelection(x, y int) {
	if g.dragMoved {
		return
	}
	if g.handleInventoryButtonTap(x, y) {
		return
	}
	settingsX, settingsY, settingsW, settingsH := g.settingsButtonRect()
	if g.pointInRect(float64(x), float64(y), settingsX, settingsY, settingsW, settingsH) {
		g.mode = modeSettings
		return
	}
	cellID, ok := g.pickCellAt(x, y)
	if !ok {
		return
	}
	now := time.Now()
	if cellID == g.lastTapCellID && cellID == g.globe.SelectedCell && now.Sub(g.lastTapTime) <= doubleTapWindow {
		g.lastTapCellID = -1
		g.enterTactical()
		return
	}
	g.globe.SelectedCell = cellID
	g.lastTapCellID = cellID
	g.lastTapTime = now
}

func (g *Game) handleWheelZoom() {
	_, wheelY := ebiten.Wheel()
	if wheelY == 0 {
		return
	}
	g.setZoom(g.zoom * (1 + wheelY*0.08))
}

func (g *Game) handlePinchZoom(a, b ebiten.TouchID) {
	ax, ay := ebiten.TouchPosition(a)
	bx, by := ebiten.TouchPosition(b)
	gap := touchDistance(ax, ay, bx, by)
	if gap < 1 {
		return
	}

	if !g.pinching || !sameTouchPair(a, b, g.pinchTouchA, g.pinchTouchB) {
		g.pinching = true
		g.pinchTouchA = a
		g.pinchTouchB = b
		g.pinchPrevGap = gap
		g.dragging = false
		g.dragTouchID = -1
		return
	}

	g.setZoom(g.zoom * (gap / g.pinchPrevGap))
	g.pinchPrevGap = gap
}

func (g *Game) applyDrag(x, y int) {
	if !g.dragMoved {
		if absInt(x-g.dragStartX) <= dragThreshold && absInt(y-g.dragStartY) <= dragThreshold {
			g.dragLastX = x
			g.dragLastY = y
			return
		}
		g.dragMoved = true
		g.dragLastX = x
		g.dragLastY = y
		return
	}

	dx := x - g.dragLastX
	dy := y - g.dragLastY
	g.dragLastX = x
	g.dragLastY = y
	g.globe.CameraLon -= float64(dx) * 0.012
	g.globe.CameraLat += float64(dy) * 0.006
	g.clampCamera()
}

func (g *Game) setZoom(zoom float64) {
	g.zoom = math.Max(minZoom, math.Min(maxZoom, zoom))
}

func (g *Game) drawBackdrop(screen *ebiten.Image) {
	cx := float32(g.screenWidth) * 0.5
	cy := float32(g.screenHeight) * 0.42
	for i := 0; i < 5; i++ {
		t := float64(i) / 4
		radius := 220 + t*170
		alpha := uint8(50 - i*8)
		clr := color.RGBA{18, 90, 150, alpha}
		drawDisc(screen, cx, cy, float32(radius), clr)
	}
}

func (g *Game) drawGlobe(screen *ebiten.Image) {
	cells := g.visibleCells()
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].depth < cells[j].depth
	})

	light := core.Vec3{X: -0.5, Y: 0.6, Z: 1}.Normalize()
	for _, cell := range cells {
		if g.zoom >= shadowZoomThreshold {
			shadow := g.globeShadowPoints(cell)
			if len(shadow) >= 3 {
				drawScreenPolygon(screen, shadow, color.RGBA{3, 7, 14, 56})
			}
		}

		points := make([]ebiten.Vertex, 0, len(cell.corners))
		valid := true
		for _, corner := range cell.corners {
			screenX, screenY, ok := g.projectPoint(corner)
			if !ok {
				valid = false
				break
			}
			points = append(points, ebiten.Vertex{DstX: float32(screenX), DstY: float32(screenY), SrcX: 0, SrcY: 0})
		}
		if !valid || len(points) < 3 {
			continue
		}

		normal := cell.center.Normalize()
		shade := 0.55 + 0.45*math.Max(0, normal.Dot(light))
		fill := core.ScaleColor(cell.style.Fill, shade+cell.style.Highlight)
		drawFilledPolygon(screen, points, fill)
		drawPolygonStroke(screen, points, core.ScaleColor(cell.style.Edge, 0.85+cell.style.Highlight))
	}
	g.drawStrategicDeviceBadges(screen, cells)
}

func (g *Game) drawStrategicDeviceBadges(screen *ebiten.Image, cells []drawCell) {
	for _, cell := range cells {
		badges := g.strategicDeviceBadges(cell.index)
		if len(badges) == 0 {
			continue
		}
		centerX, centerY, ok := g.projectPoint(cell.center)
		if !ok {
			continue
		}
		for i, badge := range badges {
			offsetX := (float64(i) - float64(len(badges)-1)*0.5) * 18
			if badge.special {
				g.drawStarterStrategicDeviceBadge(screen, centerX+offsetX, centerY-8, badge.kind)
				continue
			}
			g.drawStrategicDeviceBadge(screen, centerX+offsetX, centerY-8, badge.kind)
		}
	}
}

func (g *Game) drawStrategicDeviceBadge(screen *ebiten.Image, x, y float64, kind core.DeviceKind) {
	drawDisc(screen, float32(x+1.5), float32(y+2.5), 8, color.RGBA{0, 0, 0, 76})
	drawDisc(screen, float32(x), float32(y), 8, color.RGBA{9, 18, 32, 235})
	drawDisc(screen, float32(x), float32(y), 6.5, deviceKindBadgeColor(kind))

	switch kind {
	case core.DeviceKindMiner:
		drawFilledRect(screen, float32(x-1), float32(y-2), 2, 7, color.RGBA{240, 238, 232, 255})
		drawFilledRect(screen, float32(x-4), float32(y-4), 8, 2, color.RGBA{240, 238, 232, 255})
	default:
		drawFilledRect(screen, float32(x-2), float32(y-2), 4, 4, color.RGBA{240, 238, 232, 255})
	}
}

func (g *Game) drawStarterStrategicDeviceBadge(screen *ebiten.Image, x, y float64, kind core.DeviceKind) {
	drawDisc(screen, float32(x+1.5), float32(y+2.5), 8, color.RGBA{0, 0, 0, 76})
	drawDisc(screen, float32(x), float32(y), 8, color.RGBA{42, 30, 12, 235})
	drawDisc(screen, float32(x), float32(y), 6.5, color.RGBA{236, 204, 98, 255})
	switch kind {
	case core.DeviceKindMiner:
		drawFilledRect(screen, float32(x-1), float32(y-2), 2, 7, color.RGBA{68, 48, 10, 255})
		drawFilledRect(screen, float32(x-4), float32(y-4), 8, 2, color.RGBA{68, 48, 10, 255})
	default:
		drawFilledRect(screen, float32(x-2), float32(y-2), 4, 4, color.RGBA{68, 48, 10, 255})
	}
}

func (g *Game) drawMinimap(screen *ebiten.Image) {
	x0 := 16.0
	y0 := float64(g.screenHeight - minimapHeight - 16)
	w := float64(minimapWidth)
	h := float64(minimapHeight)

	drawRoundedRect(screen, float32(x0-6), float32(y0-6), float32(w+12), float32(h+12), 10, color.RGBA{5, 9, 20, 210})
	drawRoundedRect(screen, float32(x0), float32(y0), float32(w), float32(h), 8, color.RGBA{14, 20, 38, 240})

	for i := range g.globe.Cells {
		cell := &g.globe.Cells[i]
		style := g.ruleset.StyleCell(g.globe, cell)
		fill := style.Fill
		if cell.ID == g.globe.SelectedCell {
			fill = core.BlendColor(fill, color.RGBA{245, 248, 255, 255}, 0.35)
		}
		fill = core.ScaleColor(fill, 0.82)
		g.drawMinimapCell(screen, x0, y0, w, h, cell, fill)
	}

	g.drawMinimapView(screen, x0, y0, w, h)

	drawRectOutline(screen, float32(x0), float32(y0), float32(w), float32(h), color.RGBA{96, 120, 166, 255})
	ebitenutil.DebugPrintAt(screen, "WORLD", int(x0)+6, int(y0)+4)
}

func (g *Game) drawStrategicSettingsButton(screen *ebiten.Image) {
	x, y, w, h := g.settingsButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{40, 56, 74, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	g.drawGearIcon(screen, x+w*0.5, y+h*0.5, 10, color.RGBA{228, 236, 244, 255})
}

func (g *Game) drawStrategicStats(screen *ebiten.Image) {
	alpha := g.strategicStatsAlpha()
	if alpha <= 0 || g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		return
	}

	g.drawCellStatsCard(screen, float64(g.screenWidth-186), 16, alpha)
}

func (g *Game) drawTacticalStats(screen *ebiten.Image) {
	if g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		return
	}
	g.drawCellStatsCard(screen, float64(g.screenWidth-186), 16, 1)
}

func (g *Game) drawStrategicStageGoals(screen *ebiten.Image) {
	cardX, cardY, _, _ := g.stageGoalsCardRectForStrategic()
	g.drawStageGoalsText(screen, cardX, cardY, 1, 1, 1, 1)
}

func (g *Game) drawTacticalStageGoals(screen *ebiten.Image) {
	cardX, cardY, _, _ := g.stageGoalsCardRectForTactical()
	stage := g.currentStage()
	lines := g.stageGoalLines(stage)
	if len(lines) == 0 {
		return
	}
	// Two-tone halo: dark outline + bright fill so the text reads against
	// both the dark sky backdrop and the green/blue terrain underneath.
	g.drawHaloedDebugTextBlock(screen, cardX+12, cardY+12, lines, 1,
		1, 0.95, 0.7, // warm cream fill
		0, 0, 0)      // black halo
}

func (g *Game) drawHaloedDebugTextBlock(screen *ebiten.Image, x, y float64, lines []string, alpha float64, fillR, fillG, fillB, haloR, haloG, haloB float32) {
	for _, off := range [][2]float64{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		g.drawTintedDebugTextBlock(screen, x+off[0], y+off[1], lines, alpha, haloR, haloG, haloB)
	}
	g.drawTintedDebugTextBlock(screen, x, y, lines, alpha, fillR, fillG, fillB)
}

func (g *Game) drawCellStatsCard(screen *ebiten.Image, x, y, alpha float64) {
	cell := &g.globe.Cells[g.globe.SelectedCell]
	w := 170.0
	h := 118.0

	panelAlpha := uint8(170 * alpha)
	borderAlpha := uint8(210 * alpha)
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, panelAlpha})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, borderAlpha})

	lines := []string{
		fmt.Sprintf("CELL %d", cell.ID),
		cellBiomeLabel(cell),
		fmt.Sprintf("elev %.0f%%", cell.Elevation*100),
		fmt.Sprintf("moist %.0f%%", cell.Moisture*100),
		fmt.Sprintf("temp %.0f%%", cell.Temperature*100),
		fmt.Sprintf("neighbors %d", len(cell.Neighbors)),
	}

	g.drawAlphaDebugTextBlock(screen, x+12, y+12, lines, alpha)
}

func (g *Game) drawStageGoalsText(screen *ebiten.Image, x, y, alpha float64, r, gC, b float32) {
	stage := g.currentStage()
	lines := g.stageGoalLines(stage)
	if len(lines) == 0 {
		return
	}
	g.drawTintedDebugTextBlock(screen, x+12, y+12, lines, alpha, r, gC, b)
}

func (g *Game) currentStage() core.ProgressStage {
	if g.progression == nil {
		g.progression = core.DefaultProgressionBook()
	}
	stage, ok := g.progression.Stage(g.currentStageID)
	if !ok {
		stage, _ = g.progression.Stage(g.progression.StartStageID)
	}
	return stage
}

func (g *Game) stageGoalLines(stage core.ProgressStage) []string {
	if stage.ID == "" {
		return nil
	}
	lines := []string{stage.Title, "Goals"}
	for _, goal := range stage.Goals {
		cur, target := g.goalProgress(goal)
		mark := "[ ]"
		if cur >= target {
			mark = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%s %s %d/%d", mark, goal.Label, cur, target))
	}
	if g.stageComplete(stage) && stage.NextStageID != "" {
		lines = append(lines, "next: "+stage.NextStageID)
	}
	return lines
}

func (g *Game) stageComplete(stage core.ProgressStage) bool {
	if stage.ID == "" {
		return false
	}
	for _, goal := range stage.Goals {
		cur, target := g.goalProgress(goal)
		if cur < target {
			return false
		}
	}
	return len(stage.Goals) > 0
}

func (g *Game) advanceProgression() {
	for {
		stage := g.currentStage()
		if !g.stageComplete(stage) || stage.NextStageID == "" {
			return
		}
		next, ok := g.progression.Stage(stage.NextStageID)
		if !ok {
			return
		}
		g.currentStageID = next.ID
	}
}

func (g *Game) goalProgress(goal core.ProgressGoal) (int, int) {
	switch goal.Kind {
	case core.GoalMineResource:
		if g.minedTotals == nil {
			return 0, goal.Amount
		}
		return g.minedTotals[goal.Resource], goal.Amount
	case core.GoalDiscoverResource:
		return g.discoveredResourceCount(goal.Resource), goal.Amount
	case core.GoalBuildDevice:
		return g.deviceKindCount(goal.Device), goal.Amount
	case core.GoalPlaceStarterUnit:
		return g.starterMinerPlaced, goal.Amount
	case core.GoalRecoverStarterUnit:
		return g.starterMinerRecovered, goal.Amount
	default:
		return 0, goal.Amount
	}
}

func (g *Game) discoveredResourceCount(resource core.ResourceType) int {
	if resource == core.ResourceNone {
		return 0
	}
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return 0
	}
	for _, tile := range tmap.Tiles {
		if tile.Resource == resource {
			return 1
		}
	}
	return 0
}

func (g *Game) stageVisibleResource(resource core.ResourceType) bool {
	stage := g.currentStage()
	for _, allowed := range stage.VisibleResources {
		if allowed == resource {
			return true
		}
	}
	return false
}

func (g *Game) deviceKindCount(kind core.DeviceKind) int {
	if kind == core.DeviceKindNone {
		return 0
	}
	count := 0
	for _, tmap := range g.tacticalMaps {
		for _, tile := range tmap.Tiles {
			if tile.Device != nil && tile.Device.Kind == kind {
				count++
			}
		}
	}
	return count
}

func (g *Game) drawStrategicDevicesCard(screen *ebiten.Image, x, y, alpha float64) {
	if g.globe.SelectedCell < 0 {
		return
	}
	lines := g.strategicDevicesLines()
	w := 170.0
	h := g.strategicDevicesCardHeight()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, uint8(210 * alpha)})
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, lines, alpha)
}

func (g *Game) strategicDevicesLines() []string {
	miners := g.strategicDeviceCount()
	return []string{
		"REGION DEVICES",
		fmt.Sprintf("miners  %d", miners),
	}
}

func (g *Game) strategicDevicesCardHeight() float64 {
	lines := g.strategicDevicesLines()
	return float64(len(lines)*16 + 24)
}

func (g *Game) strategicDeviceCount() int {
	if g.globe.SelectedCell < 0 {
		return 0
	}
	miners := 0
	if tmap := g.tacticalMapForCell(g.globe.SelectedCell); tmap != nil {
		for _, tile := range tmap.Tiles {
			if tile.Device != nil && tile.Device.Kind == core.DeviceKindMiner {
				miners++
			}
		}
	}
	return miners
}

func (g *Game) strategicDeviceBadges(cellID int) []strategicDeviceBadge {
	tmap := g.tacticalMapForCell(cellID)
	if tmap == nil {
		return nil
	}
	seen := map[string]bool{}
	badges := make([]strategicDeviceBadge, 0, 3)
	for _, tile := range tmap.Tiles {
		if tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
			continue
		}
		key := fmt.Sprintf("%d:%t", tile.Device.Kind, tile.Device.SpecialStarter)
		if seen[key] {
			continue
		}
		seen[key] = true
		badges = append(badges, strategicDeviceBadge{
			kind:    tile.Device.Kind,
			special: tile.Device.SpecialStarter,
		})
	}
	return badges
}

func (g *Game) drawMinimapCell(screen *ebiten.Image, x0, y0, w, h float64, cell *core.Cell, fill color.RGBA) {
	points := make([]screenPoint, 0, len(cell.Corners))
	for _, corner := range cell.Corners {
		points = append(points, minimapPoint(corner, x0, y0, w, h))
	}
	unwrapMinimapPolygon(points, w)

	for _, shift := range []float64{-w, 0, w} {
		vertices := make([]ebiten.Vertex, 0, len(points))
		visible := false
		for _, point := range points {
			px := point.x + shift
			if px >= x0-1 && px <= x0+w+1 {
				visible = true
			}
			vertices = append(vertices, ebiten.Vertex{
				DstX: float32(px),
				DstY: float32(point.y),
				SrcX: 0,
				SrcY: 0,
			})
		}
		if !visible {
			continue
		}
		drawFilledPolygon(screen, vertices, fill)
	}
}

func (g *Game) visibleCells() []drawCell {
	cells := make([]drawCell, 0, len(g.globe.Cells))
	minDot := math.Cos(g.viewAngularRadius())
	for i := range g.globe.Cells {
		cell := &g.globe.Cells[i]
		style := g.ruleset.StyleCell(g.globe, cell)
		height := 1 + style.Height
		center := g.worldToView(cell.Center.Mul(height))
		if center.Normalize().Dot(core.Vec3{Z: 1}) < minDot {
			continue
		}
		viewDir := core.Vec3{Z: cameraZ}.Sub(center).Normalize()
		if center.Normalize().Dot(viewDir) <= 0 {
			continue
		}

		projectedCorners := make([]core.Vec3, 0, len(cell.Corners))
		for _, corner := range cell.Corners {
			projectedCorners = append(projectedCorners, g.worldToView(corner.Mul(height)))
		}
		cells = append(cells, drawCell{
			index:   i,
			center:  center,
			corners: projectedCorners,
			style:   style,
			depth:   center.Z,
		})
	}
	return cells
}

func (g *Game) pickCellAt(x, y int) (int, bool) {
	bestID := -1
	bestDepth := math.Inf(-1)
	for _, cell := range g.visibleCells() {
		points := make([]screenPoint, 0, len(cell.corners))
		valid := true
		for _, corner := range cell.corners {
			screenX, screenY, ok := g.projectPoint(corner)
			if !ok {
				valid = false
				break
			}
			points = append(points, screenPoint{x: screenX, y: screenY})
		}
		if !valid || len(points) < 3 {
			continue
		}
		if pointInPolygon(screenPoint{x: float64(x), y: float64(y)}, points) && cell.depth > bestDepth {
			bestID = cell.index
			bestDepth = cell.depth
		}
	}
	return bestID, bestID >= 0
}

func (g *Game) projectPoint(v core.Vec3) (float64, float64, bool) {
	cx := float64(g.screenWidth) * 0.5
	cy := float64(g.screenHeight) * 0.46
	scale := math.Min(float64(g.screenWidth), float64(g.screenHeight)*0.72) * 0.27 * g.zoom * globeZoomBoost
	return projectPoint(v, cx, cy, scale, cameraZ)
}

func (g *Game) globeShadowPoints(cell drawCell) []screenPoint {
	offsetX := 4.0 + cell.style.Height*60
	offsetY := 8.0 + cell.style.Height*110
	points := make([]screenPoint, 0, len(cell.corners))
	for _, corner := range cell.corners {
		screenX, screenY, ok := g.projectPoint(corner)
		if !ok {
			return nil
		}
		points = append(points, screenPoint{
			x: screenX + offsetX,
			y: screenY + offsetY,
		})
	}
	return points
}

func projectPoint(v core.Vec3, cx, cy, scale, cameraZ float64) (float64, float64, bool) {
	denom := cameraZ - v.Z
	if denom <= 0.01 {
		return 0, 0, false
	}
	perspective := scale / denom
	return cx + v.X*perspective, cy + v.Y*perspective, true
}

func touchDistance(ax, ay, bx, by int) float64 {
	dx := float64(ax - bx)
	dy := float64(ay - by)
	return math.Hypot(dx, dy)
}

func sameTouchPair(a, b, c, d ebiten.TouchID) bool {
	return (a == c && b == d) || (a == d && b == c)
}

func minimapPoint(v core.Vec3, x0, y0, w, h float64) screenPoint {
	n := v.Normalize()
	lon := math.Atan2(n.X, n.Z)
	lat := math.Asin(clampUnit(n.Y))
	return screenPoint{
		x: x0 + (lon+math.Pi)/(math.Pi*2)*w,
		y: y0 + (lat+math.Pi/2)/math.Pi*h,
	}
}

func unwrapMinimapPolygon(points []screenPoint, width float64) {
	if len(points) == 0 {
		return
	}
	for i := 1; i < len(points); i++ {
		dx := points[i].x - points[i-1].x
		if dx > width*0.5 {
			points[i].x -= width
		} else if dx < -width*0.5 {
			points[i].x += width
		}
	}
}

func (g *Game) minimapViewContour(x0, y0, w, h float64) []screenPoint {
	const samples = 48
	center := g.viewCenterDirection()
	center = center.Normalize()

	pole := core.Vec3{Y: 1}
	if math.Abs(center.Dot(pole)) > 0.96 {
		pole = core.Vec3{X: 1}
	}
	tangent := pole.Cross(center).Normalize()
	bitangent := center.Cross(tangent).Normalize()
	radius := g.viewAngularRadius()

	points := make([]screenPoint, 0, samples+1)
	for i := 0; i <= samples; i++ {
		angle := float64(i) / samples * math.Pi * 2
		ring := tangent.Mul(math.Cos(angle)).Add(bitangent.Mul(math.Sin(angle)))
		dir := center.Mul(math.Cos(radius)).Add(ring.Mul(math.Sin(radius))).Normalize()
		points = append(points, minimapPoint(dir, x0, y0, w, h))
	}
	unwrapMinimapPolygon(points, w)
	return points
}

func (g *Game) viewCenterDirection() core.Vec3 {
	return lonLatToVec3(g.globe.CameraLon, g.globe.CameraLat)
}

func (g *Game) viewAngularRadius() float64 {
	base := 1.18
	radius := base / g.zoom
	if radius < 0.42 {
		return 0.42
	}
	if radius > 1.22 {
		return 1.22
	}
	return radius
}

func (g *Game) strategicStatsAlpha() float64 {
	if g.zoom <= statsZoomStart {
		return 0
	}
	t := (g.zoom - statsZoomStart) / (maxZoom - statsZoomStart)
	t = clampRange(t, 0, 1)
	return 0.22 + 0.78*(t*t*(3-2*t))
}

func (g *Game) drawMinimapView(screen *ebiten.Image, x0, y0, w, h float64) {
	center := minimapPoint(lonLatToVec3(g.globe.CameraLon, g.globe.CameraLat), x0, y0, w, h)
	radius := g.viewAngularRadius() / math.Pi * h
	vector.StrokeCircle(screen, float32(center.x), float32(center.y), float32(radius), 2, color.RGBA{240, 243, 255, 190}, false)
}

func (g *Game) drawTactical(screen *ebiten.Image) {
	screen.Fill(color.RGBA{10, 15, 24, 255})
	g.drawBackdrop(screen)
	g.drawTacticalMap(screen)
	g.drawTacticalStageGoals(screen)
	g.drawTacticalBackButton(screen)
	g.drawTacticalDisassembleButton(screen)
	g.drawTacticalPlaceBuildButton(screen)
	g.drawTacticalBuildButton(screen)
	g.drawTacticalStats(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
	invW, invH := g.inventoryCardSize()
	g.drawPerkProgressCard(screen, 16, 16+invH+8, invW, 1)
}

func (g *Game) drawTacticalMap(screen *ebiten.Image) {
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return
	}
	cx, cy := g.tacticalCenter()
	scale := g.tacticalTileScale()
	for _, tile := range tmap.Tiles {
		points := tacticalHexPoints(tile.Center, scale)
		fill := tile.Fill
		if tile.ID == g.tacticalTile {
			fill = core.BlendColor(fill, color.RGBA{246, 249, 255, 255}, 0.33)
		}
		fill = core.ScaleColor(fill, 0.92+tile.Elevation*0.18)
		vertices := make([]ebiten.Vertex, 0, len(points))
		for _, p := range points {
			vertices = append(vertices, ebiten.Vertex{
				DstX: float32(cx + g.tacticalPanX + p.x),
				DstY: float32(cy + g.tacticalPanY + p.y),
				SrcX: 0,
				SrcY: 0,
			})
		}
		drawFilledPolygon(screen, vertices, fill)
		edge := core.ScaleColor(fill, 0.72)
		if tile.ID == g.tacticalTile {
			edge = core.BlendColor(edge, color.RGBA{185, 239, 255, 255}, 0.45)
		}
		drawPolygonStroke(screen, vertices, edge)
		g.drawTacticalTileResourceGlyph(screen, &tile, cx, cy, scale)
		g.drawTacticalTileDevice(screen, &tile, cx, cy, scale)
	}
	g.drawTacticalEntities(screen, tmap, cx, cy, scale)
}

func (g *Game) drawTacticalTileResourceGlyph(screen *ebiten.Image, tile *core.TacticalTile, cx, cy, scale float64) {
	if tile == nil || tile.ResourceRemaining <= 0 {
		return
	}
	switch tile.Resource {
	case core.ResourceNone, core.ResourceStone:
		return
	}
	if !g.stageVisibleResource(tile.Resource) {
		return
	}

	glyphX := cx + g.tacticalPanX + tile.Center.X*scale - scale*0.28
	glyphY := cy + g.tacticalPanY + tile.Center.Y*scale - scale*0.18
	base := core.ResourceColor(tile.Resource)
	shadow := color.RGBA{0, 0, 0, 74}

	drawDisc(screen, float32(glyphX+1.5), float32(glyphY+2.5), float32(scale*0.12), shadow)

	switch tile.Resource {
	case core.ResourceIronOre:
		drawFilledRect(screen, float32(glyphX-scale*0.09), float32(glyphY-scale*0.09), float32(scale*0.18), float32(scale*0.18), base)
	case core.ResourceCopperOre:
		drawDisc(screen, float32(glyphX), float32(glyphY), float32(scale*0.12), base)
	case core.ResourceCoal:
		drawRoundedRect(screen, float32(glyphX-scale*0.12), float32(glyphY-scale*0.08), float32(scale*0.24), float32(scale*0.16), 3, base)
	case core.ResourceCrystal:
		points := []screenPoint{
			{x: glyphX, y: glyphY - scale*0.14},
			{x: glyphX + scale*0.10, y: glyphY},
			{x: glyphX, y: glyphY + scale*0.14},
			{x: glyphX - scale*0.10, y: glyphY},
		}
		drawScreenPolygon(screen, points, base)
	}
}

func (g *Game) drawTacticalTileDevice(screen *ebiten.Image, tile *core.TacticalTile, cx, cy, scale float64) {
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return
	}
	centerX := cx + g.tacticalPanX + tile.Center.X*scale
	centerY := cy + g.tacticalPanY + tile.Center.Y*scale

	switch tile.Device.Kind {
	case core.DeviceKindMiner:
		def := core.DeviceDefinition(tile.Device.Kind)
		shadow := color.RGBA{0, 0, 0, 84}
		body := tacticalDeviceSignalColor(tile)
		drill := color.RGBA{220, 178, 110, 255}
		if tile.Device.SpecialStarter {
			body = color.RGBA{236, 204, 98, 255}
			drill = color.RGBA{92, 68, 18, 255}
		}
		drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.26), shadow)
		drawDisc(screen, float32(centerX), float32(centerY-1), float32(scale*0.22), body)
		drawFilledRect(screen, float32(centerX-scale*0.05), float32(centerY-scale*0.02), float32(scale*0.10), float32(scale*0.27), drill)
		drawFilledRect(screen, float32(centerX-scale*0.16), float32(centerY-scale*0.15), float32(scale*0.32), float32(scale*0.08), body)
		ix, iy := tacticalTileIndicatorAnchor(centerX, centerY, scale, 0)
		g.drawPowerIndicator(screen, ix, iy, scale, tile.PowerBuffer, def.RunPowerCost)
	}
}

func tacticalTileIndicatorAnchor(centerX, centerY, scale float64, slot int) (float64, float64) {
	order := [6]float64{
		-math.Pi / 2,
		-math.Pi / 6,
		math.Pi / 6,
		math.Pi / 2,
		5 * math.Pi / 6,
		7 * math.Pi / 6,
	}
	angle := order[((slot%6)+6)%6]
	radius := scale * 0.82
	return centerX + math.Cos(angle)*radius, centerY + math.Sin(angle)*radius
}

func (g *Game) drawPowerIndicator(screen *ebiten.Image, x, y, scale, powerBuffer, runCost float64) {
	clr := powerIndicatorColor(powerBuffer, runCost)
	drawDisc(screen, float32(x+1), float32(y+1.5), float32(scale*0.09), color.RGBA{0, 0, 0, 84})
	drawDisc(screen, float32(x), float32(y), float32(scale*0.09), clr)
	bolt := []screenPoint{
		{x: x - scale*0.020, y: y - scale*0.085},
		{x: x + scale*0.010, y: y - scale*0.020},
		{x: x - scale*0.006, y: y - scale*0.020},
		{x: x + scale*0.020, y: y + scale*0.080},
		{x: x - scale*0.012, y: y + scale*0.014},
		{x: x + scale*0.006, y: y + scale*0.014},
	}
	drawScreenPolygon(screen, bolt, color.RGBA{18, 20, 24, 240})
}

func powerIndicatorColor(powerBuffer, runCost float64) color.RGBA {
	switch {
	case powerBuffer <= 0.02:
		return color.RGBA{206, 72, 66, 255}
	case runCost <= 0 || powerBuffer >= runCost:
		return color.RGBA{92, 210, 118, 255}
	default:
		return color.RGBA{234, 200, 82, 255}
	}
}

func tacticalDeviceSignalColor(tile *core.TacticalTile) color.RGBA {
	if tile == nil {
		return color.RGBA{118, 186, 210, 255}
	}
	capacity := tile.ResourceRichness * 120
	if capacity <= 0 || tile.ResourceRemaining <= 0 {
		return color.RGBA{196, 78, 70, 255}
	}
	ratio := tile.ResourceRemaining / capacity
	switch {
	case ratio > 0.55:
		return color.RGBA{88, 194, 112, 255}
	case ratio > 0.20:
		return color.RGBA{224, 194, 78, 255}
	default:
		return color.RGBA{196, 78, 70, 255}
	}
}

func (g *Game) drawTacticalBackButton(screen *ebiten.Image) {
	x, y, w, h := g.backButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	g.drawLeftArrow(screen, x+w*0.5, y+h*0.5, 12, color.RGBA{228, 236, 244, 255})
}

// drawLeftArrow renders a filled triangle pointing left, centered at (cx, cy).
// size sets the half-width / half-height of the bounding box.
func (g *Game) drawLeftArrow(screen *ebiten.Image, cx, cy, size float64, clr color.RGBA) {
	verts := []ebiten.Vertex{
		{DstX: float32(cx - size), DstY: float32(cy)},
		{DstX: float32(cx + size*0.6), DstY: float32(cy - size*0.85)},
		{DstX: float32(cx + size*0.6), DstY: float32(cy + size*0.85)},
	}
	drawFilledPolygon(screen, verts, clr)
}

func (g *Game) drawTacticalBuildButton(screen *ebiten.Image) {
	x, y, w, h := g.buildButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "RESEARCH", int(x)+10, int(y)+12)
}

func (g *Game) drawTacticalDisassembleButton(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return
	}
	x, y, w, h := g.disassembleButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{112, 56, 52, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{236, 170, 160, 255})
	ebitenutil.DebugPrintAt(screen, "DISASSEMBLE", int(x)+7, int(y)+12)
}

func (g *Game) drawTacticalPlaceBuildButton(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil {
		return
	}
	if tile.Device.Kind != core.DeviceKindNone {
		return
	}
	x, y, w, h := g.disassembleButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{30, 88, 62, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{176, 230, 192, 255})
	ebitenutil.DebugPrintAt(screen, "BUILD", int(x)+22, int(y)+12)
}

func (g *Game) finishTacticalPointer(x, y int) {
	if g.dragMoved {
		return
	}
	if g.handleInventoryButtonTap(x, y) {
		return
	}
	buttonX, buttonY, buttonW, buttonH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		g.mode = modeStrategic
		return
	}
	buildX, buildY, buildW, buildH := g.buildButtonRect()
	if g.pointInRect(float64(x), float64(y), buildX, buildY, buildW, buildH) {
		g.mode = modeResearch
		return
	}
	disX, disY, disW, disH := g.disassembleButtonRect()
	if g.pointInRect(float64(x), float64(y), disX, disY, disW, disH) {
		tile := g.currentTacticalTile()
		if tile != nil && tile.Device != nil && tile.Device.Kind == core.DeviceKindNone {
			g.mode = modeBuild
			return
		}
		g.disassembleCurrentTacticalDevice()
		return
	}
	if tileID, ok := g.pickTacticalTile(x, y); ok {
		if g.tryCrankTacticalDevice(tileID) {
			g.tacticalTile = tileID
			return
		}
		g.tacticalTile = tileID
	}
}

func (g *Game) tryCrankTacticalDevice(tileID int) bool {
	tmap := g.currentTacticalMap()
	if tmap == nil || tileID < 0 || tileID >= len(tmap.Tiles) {
		return false
	}
	tile := &tmap.Tiles[tileID]
	if tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return false
	}
	layout := tile.Device
	if layout == nil {
		return false
	}
	hasCrank := false
	for _, part := range layout.Parts {
		if part == core.DevicePartHandCrank {
			hasCrank = true
			break
		}
	}
	if !hasCrank {
		return false
	}
	tile.PowerBuffer = math.Min(1, tile.PowerBuffer+0.45*g.crankPowerBoost())
	return true
}

func (g *Game) disassembleCurrentTacticalDevice() bool {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return false
	}
	if tile.Device.SpecialStarter {
		g.starterMinerCount++
		g.starterMinerRecovered++
		tile.Device = core.NewDeviceLayout(tile.Device.Width, tile.Device.Height)
		tile.PowerBuffer = 0
		return true
	}
	g.refundDeviceLayout(tile.Device)
	tile.Device = core.NewDeviceLayout(tile.Device.Width, tile.Device.Height)
	tile.PowerBuffer = 0
	return true
}

func (g *Game) refundDeviceLayout(layout *core.DeviceLayout) {
	if layout == nil {
		return
	}
	for _, part := range layout.Parts {
		if part == core.DevicePartEmpty {
			continue
		}
		g.partInventory[part]++
	}
}

func (g *Game) pickTacticalTile(x, y int) (int, bool) {
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return -1, false
	}
	cx, cy := g.tacticalCenter()
	scale := g.tacticalTileScale()
	p := screenPoint{x: float64(x), y: float64(y)}
	for _, tile := range tmap.Tiles {
		points := tacticalHexPoints(tile.Center, scale)
		poly := make([]screenPoint, 0, len(points))
		for _, point := range points {
			poly = append(poly, screenPoint{x: cx + g.tacticalPanX + point.x, y: cy + g.tacticalPanY + point.y})
		}
		if pointInPolygon(p, poly) {
			return tile.ID, true
		}
	}
	return -1, false
}

func (g *Game) drawTacticalEntities(screen *ebiten.Image, tmap *core.TacticalMap, cx, cy, tileScale float64) {
	microScale := tileScale / 3.2
	for _, entity := range tmap.Entities {
		if entity.MicroCellID < 0 || entity.MicroCellID >= len(tmap.MicroCells) {
			continue
		}
		micro := tmap.MicroCells[entity.MicroCellID]
		centerX := cx + g.tacticalPanX + micro.Center.X*tileScale
		centerY := cy + g.tacticalPanY + micro.Center.Y*tileScale
		drawDisc(screen, float32(centerX+microScale*0.16), float32(centerY+microScale*0.22), float32(microScale*0.46), color.RGBA{0, 0, 0, 70})
		drawDisc(screen, float32(centerX), float32(centerY), float32(microScale*0.42), entity.Fill)
	}
}

func (g *Game) enterTactical() {
	if g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		return
	}
	g.tacticalID = g.globe.SelectedCell
	g.tacticalMapForCell(g.tacticalID)
	g.tacticalTile = -1
	g.tacticalZoom = 1
	g.tacticalPanX = 0
	g.tacticalPanY = 0
	g.mode = modeTactical
}

func (g *Game) currentTacticalMap() *core.TacticalMap {
	if g.tacticalID < 0 {
		return nil
	}
	return g.tacticalMapForCell(g.tacticalID)
}

func (g *Game) currentTacticalTile() *core.TacticalTile {
	tmap := g.currentTacticalMap()
	if tmap == nil || g.tacticalTile < 0 || g.tacticalTile >= len(tmap.Tiles) {
		return nil
	}
	return &tmap.Tiles[g.tacticalTile]
}

func (g *Game) tacticalCenter() (float64, float64) {
	return float64(g.screenWidth) * 0.5, float64(g.screenHeight) * 0.54
}

func (g *Game) tacticalTileScale() float64 {
	return math.Min(float64(g.screenWidth), float64(g.screenHeight)) * 0.07 * g.tacticalZoom
}

func (g *Game) handleTacticalZoom() {
	_, wheelY := ebiten.Wheel()
	if wheelY == 0 {
		return
	}
	g.setTacticalZoom(g.tacticalZoom * (1 + wheelY*0.08))
}

func (g *Game) handleTacticalPinchZoom(a, b ebiten.TouchID) {
	ax, ay := ebiten.TouchPosition(a)
	bx, by := ebiten.TouchPosition(b)
	gap := touchDistance(ax, ay, bx, by)
	if gap < 1 {
		return
	}
	if !g.pinching || !sameTouchPair(a, b, g.pinchTouchA, g.pinchTouchB) {
		g.pinching = true
		g.pinchTouchA = a
		g.pinchTouchB = b
		g.pinchPrevGap = gap
		g.dragging = false
		g.dragTouchID = -1
		return
	}
	g.setTacticalZoom(g.tacticalZoom * (gap / g.pinchPrevGap))
	g.pinchPrevGap = gap
}

func (g *Game) applyTacticalDrag(x, y int) {
	if !g.dragMoved {
		if absInt(x-g.dragStartX) <= dragThreshold && absInt(y-g.dragStartY) <= dragThreshold {
			g.dragLastX = x
			g.dragLastY = y
			return
		}
		g.dragMoved = true
		g.dragLastX = x
		g.dragLastY = y
		return
	}
	dx := x - g.dragLastX
	dy := y - g.dragLastY
	g.dragLastX = x
	g.dragLastY = y
	g.tacticalPanX += float64(dx)
	g.tacticalPanY += float64(dy)
}

func (g *Game) setTacticalZoom(zoom float64) {
	g.tacticalZoom = math.Max(tacticalMinZoom, math.Min(tacticalMaxZoom, zoom))
}

func (g *Game) clampCamera() {
	limitLat := math.Pi/2 - g.viewAngularRadius()
	g.globe.CameraLat = clampRange(g.globe.CameraLat, -limitLat, limitLat)
	g.globe.CameraLon = wrapLongitude(g.globe.CameraLon)
}

func (g *Game) worldToView(v core.Vec3) core.Vec3 {
	return core.RotateX(core.RotateY(v, -g.globe.CameraLon), -g.globe.CameraLat)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampUnit(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampRange(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func wrapLongitude(v float64) float64 {
	for v <= -math.Pi {
		v += math.Pi * 2
	}
	for v > math.Pi {
		v -= math.Pi * 2
	}
	return v
}

func lonLatToVec3(lon, lat float64) core.Vec3 {
	cosLat := math.Cos(lat)
	return core.Vec3{
		X: math.Sin(lon) * cosLat,
		Y: math.Sin(lat),
		Z: math.Cos(lon) * cosLat,
	}
}

func cellBiomeLabel(cell *core.Cell) string {
	switch {
	case cell.Ocean && math.Abs(cell.Center.Normalize().Y) > 0.82:
		return "polar sea"
	case cell.Ocean:
		return "ocean"
	case cell.Tags["coast"]:
		return "coast"
	case math.Abs(cell.Center.Normalize().Y) > 0.78:
		return "ice"
	case cell.Elevation > 0.78:
		return "highland"
	case cell.Moisture < 0.32:
		return "dryland"
	case cell.Moisture > 0.66:
		return "wetland"
	default:
		return "temperate"
	}
}

func pointInPolygon(p screenPoint, polygon []screenPoint) bool {
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		a := polygon[i]
		b := polygon[j]
		intersects := (a.y > p.y) != (b.y > p.y)
		if !intersects {
			continue
		}
		xAtY := (b.x-a.x)*(p.y-a.y)/(b.y-a.y) + a.x
		if p.x < xAtY {
			inside = !inside
		}
	}
	return inside
}

func drawScreenPolygon(screen *ebiten.Image, points []screenPoint, clr color.RGBA) {
	if len(points) < 3 {
		return
	}
	vertices := make([]ebiten.Vertex, 0, len(points))
	for _, point := range points {
		vertices = append(vertices, ebiten.Vertex{
			DstX: float32(point.x),
			DstY: float32(point.y),
			SrcX: 0,
			SrcY: 0,
		})
	}
	drawFilledPolygon(screen, vertices, clr)
}

func (g *Game) drawAlphaDebugTextBlock(screen *ebiten.Image, x, y float64, lines []string, alpha float64) {
	g.drawTintedDebugTextBlock(screen, x, y, lines, alpha, 1, 1, 1)
}

// drawTintedDebugTextBlock renders DebugPrint text scaled by an RGB tint.
// (1,1,1) = stock white; (0,0,0) = black; anything else mixes accordingly.
func (g *Game) drawTintedDebugTextBlock(screen *ebiten.Image, x, y float64, lines []string, alpha float64, r, gC, b float32) {
	if len(lines) == 0 || alpha <= 0 {
		return
	}
	width := 0
	for _, line := range lines {
		if w := len(line)*7 + 4; w > width {
			width = w
		}
	}
	height := len(lines)*16 + 2
	textImage := ebiten.NewImage(width, height)
	textImage.Clear()
	for i, line := range lines {
		ebitenutil.DebugPrintAt(textImage, line, 0, i*16)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(r, gC, b, float32(alpha))
	screen.DrawImage(textImage, op)
}

func (g *Game) enterButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 148), float64(g.screenHeight - 62), 128, 38
}

func (g *Game) backButtonRect() (float64, float64, float64, float64) {
	return 16, float64(g.screenHeight - 62), 88, 38
}

func (g *Game) stageGoalsCardRectForStrategic() (float64, float64, float64, float64) {
	w := 170.0
	h := g.stageGoalsCardHeight()
	x := 16.0
	y0 := float64(g.screenHeight - minimapHeight - 16)
	y := y0 - 12 - h
	return x, y, w, h
}

func (g *Game) stageGoalsCardRectForTactical() (float64, float64, float64, float64) {
	w := 170.0
	h := g.stageGoalsCardHeight()
	x := 16.0
	_, backY, _, _ := g.backButtonRect()
	y := backY - 12 - h
	return x, y, w, h
}

func (g *Game) stageGoalsCardHeight() float64 {
	stage := g.currentStage()
	lines := g.stageGoalLines(stage)
	if len(lines) == 0 {
		return 0
	}
	return float64(len(lines)*16 + 24)
}

func (g *Game) buildButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 104), float64(g.screenHeight - 62), 88, 38
}

func (g *Game) pointInRect(px, py, x, y, w, h float64) bool {
	return px >= x && px <= x+w && py >= y && py <= y+h
}

func tacticalHexPoints(center core.Vec3, scale float64) []screenPoint {
	points := make([]screenPoint, 0, 6)
	for i := 0; i < 6; i++ {
		angle := math.Pi/6 + float64(i)*math.Pi/3
		points = append(points, screenPoint{
			x: center.X*scale + math.Cos(angle)*scale,
			y: center.Y*scale + math.Sin(angle)*scale,
		})
	}
	return points
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return sign + string(buf)
}

// visibleInventoryResources is the fixed set always shown on the compact
// inventory card. Anything else lives behind the ... button.
var visibleInventoryResources = []core.ResourceType{
	core.ResourceStone,
	core.ResourceIronOre,
	core.ResourceCopperOre,
	core.ResourceCoal,
}

func (g *Game) inventoryCardLines() []string {
	lines := []string{"INVENTORY"}
	for _, resource := range visibleInventoryResources {
		lines = append(lines, fmt.Sprintf("%s %d", resourceShortLabel(resource), g.inventory[resource]))
	}
	power := 0.0
	if tile := g.currentTacticalTile(); tile != nil {
		power = tile.PowerBuffer
	}
	lines = append(lines, fmt.Sprintf("power   %.3f", power))
	return lines
}

func resourceShortLabel(r core.ResourceType) string {
	switch r {
	case core.ResourceStone:
		return "stone  "
	case core.ResourceIronOre:
		return "iron  "
	case core.ResourceCopperOre:
		return "copper"
	case core.ResourceCoal:
		return "coal  "
	case core.ResourceIronIngot:
		return "Fe ingot"
	case core.ResourceCopperIngot:
		return "Cu ingot"
	case core.ResourceCrystal:
		return "crystal"
	}
	return string(r)
}

func (g *Game) inventoryCardSize() (float64, float64) {
	return 170.0, 24.0 + float64(len(g.inventoryCardLines()))*16.0
}

// inventoryHasOverflow reports whether anything is held that doesn't fit
// in the visible card — drives whether the ... button shows.
func (g *Game) inventoryHasOverflow() bool {
	visible := map[core.ResourceType]bool{}
	for _, r := range visibleInventoryResources {
		visible[r] = true
	}
	for r, count := range g.inventory {
		if !visible[r] && count > 0 {
			return true
		}
	}
	for _, count := range g.partInventory {
		if count > 0 {
			return true
		}
	}
	return false
}

func (g *Game) inventoryMoreButtonRect(cardX, cardY float64) (float64, float64, float64, float64) {
	cardW, _ := g.inventoryCardSize()
	bw := 28.0
	bh := 22.0
	bx := cardX + cardW - bw - 6
	by := cardY + 6
	return bx, by, bw, bh
}

func (g *Game) drawInventoryCard(screen *ebiten.Image, x, y, alpha float64) {
	lines := g.inventoryCardLines()
	w, h := g.inventoryCardSize()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, uint8(210 * alpha)})
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, lines, alpha)
	if g.inventoryHasOverflow() {
		bx, by, bw, bh := g.inventoryMoreButtonRect(x, y)
		drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 6, color.RGBA{36, 56, 84, uint8(220 * alpha)})
		drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), color.RGBA{146, 196, 230, uint8(220 * alpha)})
		ebitenutil.DebugPrintAt(screen, "...", int(bx)+8, int(by)+5)
	}
}

func (g *Game) drawResearchView(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	if g.researchRecipeID == "" {
		g.drawResearchBackButton(screen)
		g.drawResearchHeader(screen)
		g.drawResearchList(screen)
		g.drawResearchFooter(screen)
		return
	}
	g.drawResearchEditor(screen)
}

func (g *Game) drawBuildView(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawResearchBackButton(screen)
	g.drawBuildHeader(screen)
	g.drawBuildList(screen)
	g.drawBuildFooter(screen)
}

func (g *Game) drawBuildHeader(screen *ebiten.Image) {
	lines := []string{
		"BUILD",
		"Discovered devices",
		"Green can be placed now.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawBuildList(screen *ebiten.Image) {
	ids := g.buildListIDs()
	if len(ids) == 0 {
		g.drawAlphaDebugTextBlock(screen, 22, 112, []string{
			"No buildable devices.",
			"Discover more blueprints in research.",
		}, 1)
		return
	}
	x := 22.0
	y := 112.0
	w := float64(g.screenWidth) - 44
	h := 58.0
	for i, recipeID := range ids {
		ry := y + float64(i)*66
		if ry+h > float64(g.screenHeight)-80 {
			break
		}
		g.drawBuildRecipeCard(screen, x, ry, w, h, recipeID)
	}
}

func (g *Game) drawBuildRecipeCard(screen *ebiten.Image, x, y, w, h float64, recipeID string) {
	title := g.buildRecipeTitle(recipeID)
	costText := g.recipeCostText(recipeID)
	affordable := g.canAffordRecipe(recipeID)
	fill := color.RGBA{72, 34, 38, 230}
	border := color.RGBA{216, 108, 118, 255}
	status := "$"
	if affordable {
		fill = color.RGBA{28, 72, 46, 230}
		border = color.RGBA{128, 226, 160, 255}
		status = "BUILD"
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	g.drawAlphaDebugTextBlock(screen, x+12, y+10, []string{
		title,
		costText,
	}, 1)
	sw := float64(len(status))*7 + 18
	drawRoundedRect(screen, float32(x+w-sw-12), float32(y+18), float32(sw), 20, 8, color.RGBA{8, 18, 32, 220})
	ebitenutil.DebugPrintAt(screen, status, int(x+w-sw-5), int(y)+20)
	if !affordable {
		lineY := y + 28
		drawFilledRect(screen, float32(x+w-sw-7), float32(lineY), float32(sw-10), 2, color.RGBA{236, 170, 170, 255})
	}
}

func (g *Game) drawBuildFooter(screen *ebiten.Image) {
	lines := []string{
		"Tap a green device to spend resources and place it.",
		g.partInventorySummary(),
	}
	g.drawAlphaDebugTextBlock(screen, 18, float64(g.screenHeight-84), lines, 1)
}

func (g *Game) drawResearchHeader(screen *ebiten.Image) {
	stage := g.currentStage()
	lines := []string{
		"RESEARCH",
		stage.Title,
		"Prototype a blueprint to discover it.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawResearchList(screen *ebiten.Image) {
	recipes := g.currentStageRecipes()
	if len(recipes) == 0 {
		return
	}
	x := 22.0
	y := 112.0
	w := float64(g.screenWidth) - 44
	h := 52.0
	for i, recipe := range recipes {
		ry := y + float64(i)*60
		if ry+h > float64(g.screenHeight)-80 {
			break
		}
		g.drawResearchRecipeCard(screen, x, ry, w, h, recipe)
	}
}

func (g *Game) drawResearchRecipeCard(screen *ebiten.Image, x, y, w, h float64, recipe core.RecipeDef) {
	known := g.knownRecipes[recipe.ID]
	fill := color.RGBA{22, 28, 40, 230}
	border := color.RGBA{98, 116, 138, 255}
	if known {
		fill = color.RGBA{24, 40, 30, 230}
		border = color.RGBA{116, 198, 140, 255}
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)

	status := "PROTOTYPE"
	if known {
		status = "KNOWN"
	}
	g.drawAlphaDebugTextBlock(screen, x+12, y+10, []string{
		recipe.Title,
		"blind layout discovery",
	}, 1)
	sw := float64(len(status))*7 + 18
	drawRoundedRect(screen, float32(x+w-sw-12), float32(y+11), float32(sw), 20, 8, color.RGBA{8, 18, 32, 220})
	ebitenutil.DebugPrintAt(screen, status, int(x+w-sw-5), int(y)+13)
}

func (g *Game) drawResearchEditor(screen *ebiten.Image) {
	recipe, ok := g.currentResearchRecipe()
	if !ok {
		g.researchRecipeID = ""
		g.drawResearchList(screen)
		return
	}
	g.drawResearchBackButton(screen)
	g.drawResearchEditorHeader(screen, recipe)
	g.drawResearchEditorPalette(screen, recipe)
	g.drawResearchEditorGrid(screen, recipe)
	g.drawResearchEditorFooter(screen, recipe)
	g.drawResearchDiscoverButton(screen, recipe)
}

func (g *Game) drawResearchEditorHeader(screen *ebiten.Image, recipe core.RecipeDef) {
	known := g.knownRecipes[recipe.ID]
	lines := []string{"PROTOTYPE", recipe.Title}
	if known {
		lines = append(lines, "Known blueprint reference.")
	} else {
		lines = append(lines, "Blind match the layout to discover it.")
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawResearchEditorPalette(screen *ebiten.Image, recipe core.RecipeDef) {
	parts := g.researchEditorParts()
	y := float64(g.screenHeight - 120)
	known := g.knownRecipes[recipe.ID]
	for i, part := range parts {
		x := 12.0 + float64(i)*68
		drawRoundedRect(screen, float32(x), float32(y), 58, 54, 10, color.RGBA{24, 30, 40, 236})
		border := color.RGBA{96, 112, 130, 255}
		if !known && part == g.buildPart {
			border = color.RGBA{184, 228, 250, 255}
		}
		drawRectOutline(screen, float32(x), float32(y), 58, 54, border)
		drawFilledRect(screen, float32(x+18), float32(y+8), 22, 18, core.DevicePartColor(part))
		ebitenutil.DebugPrintAt(screen, core.DevicePartLabel(part), int(x)+6, int(y)+32)
	}
}

func (g *Game) handleResearchEditorTap(x, y int) bool {
	recipe, ok := g.currentResearchRecipe()
	if !ok {
		return false
	}
	if g.knownRecipes[recipe.ID] {
		buttonX, buttonY, buttonW, buttonH := g.createButtonRect()
		return g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH)
	}
	if part, ok := g.pickResearchPalettePart(recipe, x, y); ok {
		g.buildPart = part
		return true
	}
	if gx, gy, ok := g.pickResearchGridCell(x, y); ok {
		layout := g.researchPrototypeLayout()
		layout.SetPart(gx, gy, g.buildPart)
		return true
	}
	buttonX, buttonY, buttonW, buttonH := g.createButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		if g.knownRecipes == nil {
			g.knownRecipes = map[string]bool{}
		}
		if !g.knownRecipes[recipe.ID] && g.researchMatches(recipe) {
			g.researchRecipe(recipe.ID)
		}
		return true
	}
	return false
}

func (g *Game) drawResearchEditorGrid(screen *ebiten.Image, recipe core.RecipeDef) {
	known := g.knownRecipes[recipe.ID]
	layout := g.researchPrototypeLayout()
	x0, y0, cell := g.researchGridMetrics()
	drawRoundedRect(screen, float32(x0-12), float32(y0-12), float32(float64(layout.Width)*cell+24), float32(float64(layout.Height)*cell+24), 12, color.RGBA{16, 20, 26, 236})
	for y := 0; y < layout.Height; y++ {
		for x := 0; x < layout.Width; x++ {
			px := x0 + float64(x)*cell
			py := y0 + float64(y)*cell
			part := layout.PartAt(x, y)
			if known {
				part = recipePatternPartAt(recipe, x, y)
			}
			drawFilledRect(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{30, 35, 44, 255})
			drawRectOutline(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{74, 86, 102, 255})
			if part != core.DevicePartEmpty {
				drawFilledRect(screen, float32(px+6), float32(py+6), float32(cell-14), float32(cell-14), core.DevicePartColor(part))
			}
		}
	}
}

func (g *Game) drawResearchEditorFooter(screen *ebiten.Image, recipe core.RecipeDef) {
	known := g.knownRecipes[recipe.ID]
	match := g.researchMatches(recipe)
	lines := []string{deviceStatusLabelForRecipe(known, match)}
	if known {
		lines = append(lines, "reference blueprint")
	} else {
		lines = append(lines, "no hints on part placement")
	}
	g.drawAlphaDebugTextBlock(screen, 18, float64(g.screenHeight-84), lines, 1)
}

func (g *Game) drawResearchDiscoverButton(screen *ebiten.Image, recipe core.RecipeDef) {
	x, y, w, h := g.createButtonRect()
	known := g.knownRecipes[recipe.ID]
	match := g.researchMatches(recipe)
	fill := color.RGBA{21, 86, 112, 236}
	border := color.RGBA{143, 219, 246, 255}
	label := "DISCOVER"
	if known {
		fill = color.RGBA{54, 62, 76, 228}
		border = color.RGBA{120, 136, 160, 255}
		label = "OWNED"
	} else if !match {
		fill = color.RGBA{54, 62, 76, 228}
		border = color.RGBA{120, 136, 160, 255}
		label = "DISCOVER"
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	ebitenutil.DebugPrintAt(screen, label, int(x)+10, int(y)+12)
}

func (g *Game) drawResearchFooter(screen *ebiten.Image) {
	lines := []string{
		"Tap a recipe card to open its prototype editor.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, float64(g.screenHeight-84), lines, 1)
}

func (g *Game) currentStageRecipes() []core.RecipeDef {
	stage := g.currentStage()
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	recipes := make([]core.RecipeDef, 0, len(stage.KnownRecipes))
	for _, recipeID := range stage.KnownRecipes {
		if recipe, ok := g.recipes.Recipe(recipeID); ok {
			recipes = append(recipes, recipe)
		}
	}
	return recipes
}

func (g *Game) currentBuildRecipes() []core.RecipeDef {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	recipes := make([]core.RecipeDef, 0, len(g.knownRecipes))
	for recipeID, known := range g.knownRecipes {
		if !known {
			continue
		}
		recipe, ok := g.recipes.Recipe(recipeID)
		if !ok || recipe.Kind != core.RecipeDevice || recipe.Device == core.DeviceKindNone {
			continue
		}
		recipes = append(recipes, recipe)
	}
	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].Title < recipes[j].Title
	})
	return recipes
}

const starterMinerRecipeID = "__starter_miner"

func (g *Game) buildListIDs() []string {
	ids := make([]string, 0, len(g.knownRecipes)+1)
	if g.starterMinerCount > 0 {
		ids = append(ids, starterMinerRecipeID)
	}
	for _, recipe := range g.currentBuildRecipes() {
		ids = append(ids, recipe.ID)
	}
	return ids
}

func (g *Game) pickResearchPalettePart(recipe core.RecipeDef, x, y int) (core.DevicePart, bool) {
	parts := g.researchEditorParts()
	py := float64(g.screenHeight - 120)
	for i, part := range parts {
		px := 12.0 + float64(i)*68
		if g.pointInRect(float64(x), float64(y), px, py, 58, 54) {
			return part, true
		}
	}
	return core.DevicePartEmpty, false
}

func (g *Game) pickResearchGridCell(x, y int) (int, int, bool) {
	layout := g.researchPrototypeLayout()
	x0, y0, cell := g.researchGridMetrics()
	if !g.pointInRect(float64(x), float64(y), x0, y0, float64(layout.Width)*cell, float64(layout.Height)*cell) {
		return 0, 0, false
	}
	gx := int((float64(x) - x0) / cell)
	gy := int((float64(y) - y0) / cell)
	if gx < 0 || gy < 0 || gx >= layout.Width || gy >= layout.Height {
		return 0, 0, false
	}
	return gx, gy, true
}

func (g *Game) pickResearchRecipe(x, y int) (string, bool) {
	recipes := g.currentStageRecipes()
	cardX := 22.0
	cardY := 112.0
	cardW := float64(g.screenWidth) - 44
	cardH := 52.0
	for i, recipe := range recipes {
		ry := cardY + float64(i)*60
		if ry+cardH > float64(g.screenHeight)-80 {
			break
		}
		if g.pointInRect(float64(x), float64(y), cardX, ry, cardW, cardH) {
			return recipe.ID, true
		}
	}
	return "", false
}

func (g *Game) pickBuildRecipe(x, y int) (string, bool) {
	ids := g.buildListIDs()
	cardX := 22.0
	cardY := 112.0
	cardW := float64(g.screenWidth) - 44
	cardH := 58.0
	for i, recipeID := range ids {
		ry := cardY + float64(i)*66
		if ry+cardH > float64(g.screenHeight)-80 {
			break
		}
		if g.pointInRect(float64(x), float64(y), cardX, ry, cardW, cardH) {
			return recipeID, true
		}
	}
	return "", false
}

func (g *Game) currentResearchRecipe() (core.RecipeDef, bool) {
	if g.researchRecipeID == "" {
		return core.RecipeDef{}, false
	}
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	recipe, ok := g.recipes.Recipe(g.researchRecipeID)
	return recipe, ok
}

func (g *Game) researchPrototypeLayout() *core.DeviceLayout {
	if g.researchLayout == nil {
		g.researchLayout = core.NewDeviceLayout(5, 5)
	}
	return g.researchLayout
}

func (g *Game) researchGridMetrics() (float64, float64, float64) {
	cell := 42.0
	x0 := float64(g.screenWidth)*0.5 - cell*2.5
	y0 := 138.0
	return x0, y0, cell
}

func (g *Game) researchEditorParts() []core.DevicePart {
	return []core.DevicePart{
		core.DevicePartFrame,
		core.DevicePartDrill,
		core.DevicePartMotor,
		core.DevicePartOutput,
		core.DevicePartHandCrank,
		core.DevicePartEmpty,
	}
}

func (g *Game) researchMatches(recipe core.RecipeDef) bool {
	layout := g.researchPrototypeLayout()
	return layoutMatchesPattern(layout, recipe.Pattern)
}

func layoutMatchesPattern(layout *core.DeviceLayout, pattern []core.RecipeCell) bool {
	if layout == nil || len(pattern) == 0 {
		return false
	}
	minX, minY := pattern[0].X, pattern[0].Y
	maxX, maxY := pattern[0].X, pattern[0].Y
	for _, cell := range pattern[1:] {
		if cell.X < minX {
			minX = cell.X
		}
		if cell.Y < minY {
			minY = cell.Y
		}
		if cell.X > maxX {
			maxX = cell.X
		}
		if cell.Y > maxY {
			maxY = cell.Y
		}
	}
	patternW := maxX - minX + 1
	patternH := maxY - minY + 1
	for offsetY := 0; offsetY <= layout.Height-patternH; offsetY++ {
		for offsetX := 0; offsetX <= layout.Width-patternW; offsetX++ {
			match := true
			for _, cell := range pattern {
				if layout.PartAt(offsetX+cell.X-minX, offsetY+cell.Y-minY) != cell.Part {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			for y := 0; y < layout.Height; y++ {
				for x := 0; x < layout.Width; x++ {
					inPattern := false
					for _, cell := range pattern {
						if offsetX+cell.X-minX == x && offsetY+cell.Y-minY == y {
							inPattern = true
							break
						}
					}
					if inPattern {
						continue
					}
					if layout.PartAt(x, y) != core.DevicePartEmpty {
						match = false
						break
					}
				}
				if !match {
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func deviceStatusLabelForRecipe(known, match bool) string {
	switch {
	case known:
		return "known blueprint"
	case match:
		return "ready to discover"
	default:
		return "prototype incomplete"
	}
}

func recipePatternPartAt(recipe core.RecipeDef, x, y int) core.DevicePart {
	for _, cell := range recipe.Pattern {
		if cell.X == x && cell.Y == y {
			return cell.Part
		}
	}
	return core.DevicePartEmpty
}

func (g *Game) researchRecipe(recipeID string) {
	if g.knownRecipes == nil {
		g.knownRecipes = map[string]bool{}
	}
	if recipeID == "" {
		return
	}
	g.knownRecipes[recipeID] = true
}

func (g *Game) canAffordRecipe(recipeID string) bool {
	if recipeID == starterMinerRecipeID {
		return g.starterMinerCount > 0
	}
	_, ok := g.buildPlanForRecipe(recipeID)
	return ok
}

func (g *Game) recipeCostText(recipeID string) string {
	if recipeID == starterMinerRecipeID {
		return "starter unit"
	}
	plan, ok := g.buildPlanForRecipe(recipeID)
	if !ok {
		return "insufficient materials"
	}
	return buildPlanSummary(plan)
}

func (g *Game) spendBuildPlan(plan buildPlan) {
	for part, amount := range plan.partSpend {
		g.partInventory[part] -= amount
	}
	for resource, amount := range plan.rawSpend {
		g.inventory[resource] -= amount
	}
}

func (g *Game) placeRecipeOnCurrentTile(recipeID string) bool {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindNone {
		return false
	}
	if recipeID == starterMinerRecipeID {
		if g.starterMinerCount <= 0 {
			return false
		}
		g.starterMinerCount--
		g.starterMinerPlaced++
		tile.Device = g.buildStarterMinerLayout()
		tile.PowerBuffer = 0
		return true
	}
	recipe, ok := g.recipes.Recipe(recipeID)
	if !ok || recipe.Kind != core.RecipeDevice || recipe.Device == core.DeviceKindNone {
		return false
	}
	plan, ok := g.buildPlanForRecipe(recipeID)
	if !ok {
		return false
	}
	g.spendBuildPlan(plan)
	layout := core.NewDeviceLayout(5, 5)
	for _, cell := range recipe.Pattern {
		layout.SetPart(cell.X, cell.Y, cell.Part)
	}
	layout.Kind = recipe.Device
	tile.Device = layout
	tile.PowerBuffer = 0
	return true
}

func (g *Game) buildStarterMinerLayout() *core.DeviceLayout {
	layout := core.NewDeviceLayout(5, 5)
	layout.SetPart(2, 1, core.DevicePartMotor)
	layout.SetPart(1, 2, core.DevicePartFrame)
	layout.SetPart(2, 2, core.DevicePartDrill)
	layout.SetPart(3, 2, core.DevicePartFrame)
	layout.SetPart(2, 3, core.DevicePartOutput)
	layout.SetPart(2, 4, core.DevicePartHandCrank)
	layout.Kind = core.DeviceKindMiner
	layout.SpecialStarter = true
	return layout
}

func (g *Game) buildPlanForRecipe(recipeID string) (buildPlan, bool) {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	rawAvail := make(map[core.ResourceType]int, len(g.inventory))
	for resource, amount := range g.inventory {
		rawAvail[resource] = amount
	}
	partAvail := make(map[core.DevicePart]int, len(g.partInventory))
	for part, amount := range g.partInventory {
		partAvail[part] = amount
	}
	plan := buildPlan{
		rawSpend:  map[core.ResourceType]int{},
		partSpend: map[core.DevicePart]int{},
	}
	if !g.resolveRecipeCost(recipeID, 1, rawAvail, partAvail, &plan, map[string]bool{}) {
		return buildPlan{}, false
	}
	return plan, true
}

func (g *Game) resolveRecipeCost(recipeID string, count int, rawAvail map[core.ResourceType]int, partAvail map[core.DevicePart]int, plan *buildPlan, stack map[string]bool) bool {
	if count <= 0 {
		return true
	}
	recipe, ok := g.recipes.Recipe(recipeID)
	if !ok || stack[recipeID] {
		return false
	}
	if recipe.Kind == core.RecipePart && recipe.Part != core.DevicePartEmpty {
		owned := partAvail[recipe.Part]
		if owned > count {
			owned = count
		}
		if owned > 0 {
			partAvail[recipe.Part] -= owned
			plan.partSpend[recipe.Part] += owned
			count -= owned
			if count == 0 {
				return true
			}
		}
	}

	stack[recipeID] = true
	defer delete(stack, recipeID)
	for i := 0; i < count; i++ {
		for _, ing := range recipe.Ingredients {
			if ing.Amount <= 0 {
				continue
			}
			if ing.RecipeID != "" {
				if !g.resolveRecipeCost(ing.RecipeID, ing.Amount, rawAvail, partAvail, plan, stack) {
					return false
				}
				continue
			}
			if ing.Resource == core.ResourceNone || rawAvail[ing.Resource] < ing.Amount {
				return false
			}
			rawAvail[ing.Resource] -= ing.Amount
			plan.rawSpend[ing.Resource] += ing.Amount
		}
	}
	return true
}

func buildPlanSummary(plan buildPlan) string {
	parts := make([]string, 0, len(plan.partSpend))
	for part, amount := range plan.partSpend {
		if amount <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", strings.ToLower(core.DevicePartLabel(part)), amount))
	}
	sort.Strings(parts)
	rawKeys := make([]string, 0, len(plan.rawSpend))
	for resource, amount := range plan.rawSpend {
		if amount <= 0 {
			continue
		}
		rawKeys = append(rawKeys, string(resource))
	}
	sort.Strings(rawKeys)
	raw := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		raw = append(raw, fmt.Sprintf("%s %d", key, plan.rawSpend[core.ResourceType(key)]))
	}
	switch {
	case len(parts) > 0 && len(raw) > 0:
		return "use " + strings.Join(parts, ", ") + " | spend " + strings.Join(raw, ", ")
	case len(parts) > 0:
		return "use " + strings.Join(parts, ", ")
	case len(raw) > 0:
		return "spend " + strings.Join(raw, ", ")
	default:
		return "spend nothing"
	}
}

func (g *Game) partInventorySummary() string {
	parts := make([]string, 0, len(g.partInventory))
	for _, part := range []core.DevicePart{
		core.DevicePartFrame,
		core.DevicePartDrill,
		core.DevicePartMotor,
		core.DevicePartOutput,
		core.DevicePartHandCrank,
	} {
		if g.partInventory[part] <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", strings.ToLower(core.DevicePartLabel(part)), g.partInventory[part]))
	}
	if len(parts) == 0 {
		return "crafted parts: none"
	}
	return "crafted parts: " + strings.Join(parts, ", ")
}

func (g *Game) buildRecipeTitle(recipeID string) string {
	if recipeID == starterMinerRecipeID {
		return fmt.Sprintf("Starter Miner x%d", g.starterMinerCount)
	}
	recipe, ok := g.recipes.Recipe(recipeID)
	if !ok {
		return recipeID
	}
	return recipe.Title
}

func (g *Game) drawSettings(screen *ebiten.Image) {
	screen.Fill(color.RGBA{10, 14, 22, 255})
	g.drawBackdrop(screen)
	g.drawSettingsBackButton(screen)
	g.drawSettingsPanel(screen)
}

func (g *Game) drawSettingsBackButton(screen *ebiten.Image) {
	x, y, w, h := g.backButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "BACK", int(x)+18, int(y)+12)
}

func (g *Game) drawSettingsPanel(screen *ebiten.Image) {
	x := float64(g.screenWidth)*0.5 - 152
	y := 88.0
	w := 304.0
	h := 154.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, 255})
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, []string{
		"SETTINGS",
		"World controls",
		"Research now owns recipe visibility.",
	}, 1)

	rx, ry, rw, rh := g.regenerateButtonRect()
	g.drawAlphaDebugTextBlock(screen, x+18, y+76, []string{
		"Regenerate the world and clear tactical state.",
	}, 1)
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 12, color.RGBA{124, 58, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{240, 190, 170, 255})
	ebitenutil.DebugPrintAt(screen, "REGENERATE MAP", int(rx)+18, int(ry)+14)
}

func (g *Game) handleSettingsInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.settingsDown = true
		g.settingsTouch = -1
		g.settingsX = x
		g.settingsY = y
		g.handleSettingsTap(x, y)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.settingsDown {
		x, y := ebiten.CursorPosition()
		g.finishSettingsGesture(x, y)
		g.settingsDown = false
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		id := justTouched[0]
		x, y := ebiten.TouchPosition(id)
		g.settingsDown = true
		g.settingsTouch = id
		g.settingsX = x
		g.settingsY = y
		g.handleSettingsTap(x, y)
	}
	if g.settingsDown && g.settingsTouch != -1 {
		active := false
		for _, id := range ebiten.AppendTouchIDs(nil) {
			if id == g.settingsTouch {
				active = true
				break
			}
		}
		if !active {
			x, y := inpututil.TouchPositionInPreviousTick(g.settingsTouch)
			g.finishSettingsGesture(x, y)
			g.settingsDown = false
			g.settingsTouch = -1
		}
	}
}

func (g *Game) handleSettingsTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeStrategic
		return
	}
	regX, regY, regW, regH := g.regenerateButtonRect()
	if g.pointInRect(float64(x), float64(y), regX, regY, regW, regH) {
		g.requestRegen()
	}
}

func (g *Game) finishSettingsGesture(x, y int) {
	return
}

func (g *Game) drawResearchBackButton(screen *ebiten.Image) {
	x, y, w, h := g.backButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "BACK", int(x)+18, int(y)+12)
}

func (g *Game) handleResearchInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleResearchTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleResearchTap(x, y)
	}
}

func (g *Game) handleResearchTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		if g.researchRecipeID != "" {
			g.researchRecipeID = ""
			g.researchLayout = nil
			return
		}
		g.mode = modeTactical
		return
	}
	if g.researchRecipeID == "" {
		if recipeID, ok := g.pickResearchRecipe(x, y); ok {
			g.openResearchRecipe(recipeID)
		}
		return
	}
	if g.handleResearchEditorTap(x, y) {
		return
	}
}

func (g *Game) handleBuildInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleBuildTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleBuildTap(x, y)
	}
}

func (g *Game) handleBuildTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeTactical
		return
	}
	recipeID, ok := g.pickBuildRecipe(x, y)
	if !ok || !g.canAffordRecipe(recipeID) {
		return
	}
	if g.placeRecipeOnCurrentTile(recipeID) {
		g.mode = modeTactical
	}
}

func (g *Game) createButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 110), float64(g.screenHeight - 62), 94, 38
}

func (g *Game) disassembleButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 110), float64(g.screenHeight - 108), 94, 38
}

func (g *Game) settingsButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth)*0.5 - 34, float64(g.screenHeight - 62), 68, 38
}

func (g *Game) regenerateButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth)*0.5 - 92, 452, 184, 46
}

func resourceLabel(resource core.ResourceType) string {
	if resource == core.ResourceNone {
		return "none"
	}
	return string(resource)
}

func deviceStatusLabel(device *core.DeviceLayout) string {
	if device == nil {
		return "device none"
	}
	if device.Kind != core.DeviceKindNone {
		return "device " + core.DeviceKindLabel(device.Kind)
	}
	if blueprint := device.FindBlueprint(); blueprint != core.DeviceKindNone {
		return "blueprint " + core.DeviceKindLabel(blueprint)
	}
	return "device idle"
}

func (g *Game) tacticalMapForCell(cellID int) *core.TacticalMap {
	if cellID < 0 || cellID >= len(g.globe.Cells) {
		return nil
	}
	if tmap, ok := g.tacticalMaps[cellID]; ok {
		return tmap
	}
	cell := &g.globe.Cells[cellID]
	tmap := core.NewTacticalMap(cell, tacticalRadius)
	g.tacticalMaps[cellID] = tmap
	return tmap
}

func (g *Game) hasPlacedStarterMiner() bool {
	for _, tmap := range g.tacticalMaps {
		for _, tile := range tmap.Tiles {
			if tile.Device != nil && tile.Device.Kind == core.DeviceKindMiner && tile.Device.SpecialStarter {
				return true
			}
		}
	}
	return false
}

func (g *Game) openResearchRecipe(recipeID string) {
	if recipeID == "" {
		return
	}
	g.researchRecipeID = recipeID
	g.researchLayout = core.NewDeviceLayout(5, 5)
	g.buildPart = core.DevicePartFrame
}

func (g *Game) strategicOrePreview(cellID int) string {
	tmap := g.tacticalMapForCell(cellID)
	if tmap == nil {
		return "unknown"
	}
	type oreStat struct {
		resource core.ResourceType
		score    float64
	}
	stats := make([]oreStat, 0, 5)
	for _, resource := range []core.ResourceType{
		core.ResourceIronOre,
		core.ResourceCopperOre,
		core.ResourceCoal,
		core.ResourceCrystal,
		core.ResourceStone,
	} {
		score := 0.0
		for _, tile := range tmap.Tiles {
			if tile.Resource == resource {
				score += tile.ResourceRichness
			}
		}
		if score > 0 {
			stats = append(stats, oreStat{resource: resource, score: score})
		}
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].score > stats[j].score
	})
	if len(stats) == 0 {
		return "none"
	}
	labels := make([]string, 0, 3)
	threshold := stats[0].score * 0.35
	for _, stat := range stats {
		if len(labels) >= 3 {
			break
		}
		if stat.score < threshold && len(labels) > 0 {
			continue
		}
		labels = append(labels, resourceLabel(stat.resource))
	}
	if len(labels) == 0 {
		return resourceLabel(stats[0].resource)
	}
	result := labels[0]
	for i := 1; i < len(labels); i++ {
		result += "/" + labels[i]
	}
	return result
}

func buildPartCost(part core.DevicePart) (core.ResourceType, int, bool) {
	def := core.PartDefinition(part)
	for resource, amount := range def.Cost {
		return resource, amount, true
	}
	return core.ResourceNone, 0, false
}

func (g *Game) buildPartOverlayCount(part core.DevicePart) (int, bool) {
	resource, cost, ok := buildPartCost(part)
	if !ok || cost <= 0 {
		return 0, false
	}
	return g.inventory[resource] / cost, true
}

func blueprintCost(kind core.DeviceKind) map[core.ResourceType]int {
	costs := map[core.ResourceType]int{}
	switch kind {
	case core.DeviceKindMiner:
		for _, part := range []core.DevicePart{
			core.DevicePartFrame,
			core.DevicePartFrame,
			core.DevicePartDrill,
			core.DevicePartMotor,
			core.DevicePartOutput,
			core.DevicePartHandCrank,
		} {
			resource, amount, ok := buildPartCost(part)
			if ok {
				costs[resource] += amount
			}
		}
	}
	return costs
}

func (g *Game) canCreateCurrentBlueprint() bool {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil {
		return false
	}
	blueprint := tile.Device.FindBlueprint()
	if blueprint == core.DeviceKindNone || tile.Device.Kind == blueprint {
		return false
	}
	for resource, amount := range blueprintCost(blueprint) {
		if g.inventory[resource] < amount {
			return false
		}
	}
	return true
}

func (g *Game) spendBlueprintCost(kind core.DeviceKind) bool {
	costs := blueprintCost(kind)
	for resource, amount := range costs {
		if g.inventory[resource] < amount {
			return false
		}
	}
	for resource, amount := range costs {
		g.inventory[resource] -= amount
	}
	return true
}

func deviceKindBadgeColor(kind core.DeviceKind) color.RGBA {
	switch kind {
	case core.DeviceKindMiner:
		return color.RGBA{198, 150, 86, 255}
	default:
		return color.RGBA{132, 172, 206, 255}
	}
}

func drawFilledRect(screen *ebiten.Image, x, y, w, h float32, clr color.RGBA) {
	vector.DrawFilledRect(screen, x, y, w, h, clr, false)
}

func (g *Game) drawGearIcon(screen *ebiten.Image, cx, cy, radius float64, clr color.RGBA) {
	drawDisc(screen, float32(cx), float32(cy), float32(radius), clr)
	drawDisc(screen, float32(cx), float32(cy), float32(radius*0.42), color.RGBA{40, 56, 74, 255})
	for i := 0; i < 8; i++ {
		angle := float64(i) * math.Pi / 4
		tx := cx + math.Cos(angle)*radius*0.95
		ty := cy + math.Sin(angle)*radius*0.95
		drawFilledRect(screen, float32(tx-1.5), float32(ty-3.5), 3, 7, clr)
	}
}

func drawRoundedRect(screen *ebiten.Image, x, y, w, h, radius float32, clr color.RGBA) {
	vector.DrawFilledRect(screen, x+radius, y, w-radius*2, h, clr, false)
	vector.DrawFilledRect(screen, x, y+radius, radius, h-radius*2, clr, false)
	vector.DrawFilledRect(screen, x+w-radius, y+radius, radius, h-radius*2, clr, false)
	vector.DrawFilledCircle(screen, x+radius, y+radius, radius, clr, false)
	vector.DrawFilledCircle(screen, x+w-radius, y+radius, radius, clr, false)
	vector.DrawFilledCircle(screen, x+radius, y+h-radius, radius, clr, false)
	vector.DrawFilledCircle(screen, x+w-radius, y+h-radius, radius, clr, false)
}

func drawRectOutline(screen *ebiten.Image, x, y, w, h float32, clr color.RGBA) {
	vector.StrokeLine(screen, x, y, x+w, y, 1.5, clr, false)
	vector.StrokeLine(screen, x+w, y, x+w, y+h, 1.5, clr, false)
	vector.StrokeLine(screen, x+w, y+h, x, y+h, 1.5, clr, false)
	vector.StrokeLine(screen, x, y+h, x, y, 1.5, clr, false)
}

func drawFilledPolygon(screen *ebiten.Image, points []ebiten.Vertex, clr color.RGBA) {
	r := float32(clr.R) / 255
	g := float32(clr.G) / 255
	b := float32(clr.B) / 255
	a := float32(clr.A) / 255
	for i := range points {
		points[i].ColorR = r
		points[i].ColorG = g
		points[i].ColorB = b
		points[i].ColorA = a
	}

	indices := make([]uint16, 0, (len(points)-2)*3)
	for i := 1; i < len(points)-1; i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	screen.DrawTriangles(points, indices, solidPixel, nil)
}

func drawPolygonStroke(screen *ebiten.Image, points []ebiten.Vertex, clr color.RGBA) {
	for i := range points {
		a := points[i]
		b := points[(i+1)%len(points)]
		vector.StrokeLine(screen, a.DstX, a.DstY, b.DstX, b.DstY, 1.4, clr, false)
	}
}

func drawDisc(screen *ebiten.Image, cx, cy, radius float32, clr color.RGBA) {
	const segments = 48
	vertices := make([]ebiten.Vertex, 0, segments+1)
	vertices = append(vertices, ebiten.Vertex{
		DstX:   cx,
		DstY:   cy,
		SrcX:   0,
		SrcY:   0,
		ColorR: float32(clr.R) / 255,
		ColorG: float32(clr.G) / 255,
		ColorB: float32(clr.B) / 255,
		ColorA: float32(clr.A) / 255,
	})
	for i := 0; i <= segments; i++ {
		angle := float64(i) / segments * math.Pi * 2
		vertices = append(vertices, ebiten.Vertex{
			DstX:   cx + radius*float32(math.Cos(angle)),
			DstY:   cy + radius*float32(math.Sin(angle)),
			SrcX:   0,
			SrcY:   0,
			ColorR: float32(clr.R) / 255,
			ColorG: float32(clr.G) / 255,
			ColorB: float32(clr.B) / 255,
			ColorA: float32(clr.A) / 255,
		})
	}
	indices := make([]uint16, 0, segments*3)
	for i := 1; i < len(vertices)-1; i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	screen.DrawTriangles(vertices, indices, solidPixel, nil)
}
