package petridish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "embed"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"petri_dish/core"
)

const (
	projectName      = "Petri Dish"
	autoSaveInterval = 180.0
)

const (
	defaultScreenWidth  = 432
	defaultScreenHeight = 768
	minZoom             = 0.7
	maxZoom             = 5.2
	dragThreshold       = 8
	starterHoldPower    = 1.8 / 60.0
	mugMoveStepPower    = 0.12
	mugMoveTilesPerSec  = 3.2
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
	modeTech
	modeSettings
	modeDish
	modeSmelterConfig
	modeAssemblerConfig
	modeGeneratorConfig
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
	inventory             map[core.ResourceType]int
	partInventory         map[core.DevicePart]int
	starterMinerCount     int
	starterGateCount      int
	starterMinerPlaced    int
	starterMinerRecovered int
	minedTotals           map[core.ResourceType]int
	progression           *core.ProgressionBook
	recipes               *core.RecipeBook
	knownRecipes          map[string]bool
	pendingBuildRecipeID  string
	currentStageID        string
	dishTileID            int
	dishCells             []dishCell
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
	animationTime         float64
	fieldDataPopups       []fieldDataPopup
	fieldDataScanCarry    map[fieldDataTileKey]float64
	uplinkPackets         []uplinkPacket
	perkCelebrationTimer  float64
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
	lastTapTacticalTileID int
	lastTapTacticalTime   time.Time
	inventoryOverlay      *inventoryOverlayState
	expandedTechRecipeID  string
	expandedBuildRecipeID string
	perks                 *core.PerkBook
	activePerks           []string
	stagePowerSpent       map[string]float64
	perksAwarded          map[string]int
	perkChoice            *perkChoiceState
	perkOfferCooldown     float64
	perkRand              *rand.Rand
	configTileID          int
	mugDragActive         bool
	mugDragStartTile      int
	mugDragTargetTile     int
	mugMoves              map[int]*mugMoveState
	creaturesEnabled      bool
	gateUplinkUnlocked    bool
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

type dishCell struct {
	ID        int
	Q         int
	R         int
	Center    core.Vec3
	Kind      int
	Influence int
	Phase     float64
}

type mugMoveState struct {
	path     []int
	progress float64
}

type fieldDataPopup struct {
	x      float64
	y      float64
	amount int
	timer  float64
}

type fieldDataTileKey struct {
	cellID int
	tileID int
}

type uplinkPacket struct {
	x        float64
	y        float64
	offsetX  float64
	delay    float64
	timer    float64
	duration float64
	resource core.ResourceType
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

//go:embed assets/miner_sprite_sheet.png
var minerSpriteSheetPNG []byte

//go:embed assets/auto_miner_sprite_sheet.png
var autoMinerSpriteSheetPNG []byte

//go:embed assets/resource_sprite_sheet.png
var resourceSpriteSheetPNG []byte

//go:embed assets/friendly_organism_sheet.png
var friendlyOrganismSheetPNG []byte

//go:embed assets/danger_organism_sheet.png
var dangerOrganismSheetPNG []byte

//go:embed assets/purple_caveman_sheet.png
var purpleCavemanSheetPNG []byte

//go:embed assets/tactical_texture_atlas.png
var tacticalTextureAtlasPNG []byte

//go:embed assets/power_indicator_sheet.png
var powerIndicatorSpriteSheetPNG []byte

//go:embed assets/device_sprite_sheet.png
var deviceSpriteSheetPNG []byte

//go:embed assets/gate_uplink_sprite.png
var gateUplinkSpritePNG []byte

//go:embed assets/assembler_sprite_sheet.png
var assemblerSpriteSheetPNG []byte

var minerSprites [4]*ebiten.Image
var autoMinerSprites [4]*ebiten.Image
var resourceSprites [4]*ebiten.Image
var friendlyOrganismSprites [4]*ebiten.Image
var dangerOrganismSprites [4]*ebiten.Image
var purpleCavemanSprites [4]*ebiten.Image
var tacticalTextures [5]*ebiten.Image
var powerIndicatorSprites [3]*ebiten.Image
var deviceSprites [6]*ebiten.Image
var gateUplinkSprite *ebiten.Image
var assemblerSprites [4]*ebiten.Image

func init() {
	solidPixel.Fill(color.White)
	initMinerSprites()
	initAutoMinerSprites()
	initResourceSprites()
	initOrganismSprites()
	initTacticalTextures()
	initPowerIndicatorSprites()
	initDeviceSprites()
	initGateUplinkSprite()
	initAssemblerSprites()
}

func NewGame() *Game {
	g := &Game{
		screenWidth:           defaultScreenWidth,
		screenHeight:          defaultScreenHeight,
		zoom:                  1,
		dragTouchID:           -1,
		pinchTouchA:           -1,
		pinchTouchB:           -1,
		settingsTouch:         -1,
		tacticalMaps:          map[int]*core.TacticalMap{},
		tacticalID:            -1,
		tacticalTile:          -1,
		tacticalZoom:          1,
		progression:           core.DefaultProgressionBook(),
		recipes:               core.DefaultRecipeBook(),
		perks:                 core.DefaultPerkBook(),
		knownRecipes:          map[string]bool{},
		stagePowerSpent:       map[string]float64{},
		perksAwarded:          map[string]int{},
		perkRand:              rand.New(rand.NewSource(time.Now().UnixNano())),
		lastTapCellID:         -1,
		lastTapTacticalTileID: -1,
		dishTileID:            -1,
		configTileID:          -1,
		mugMoves:              map[int]*mugMoveState{},
		fieldDataScanCarry:    map[fieldDataTileKey]float64{},
		creaturesEnabled:      false,
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
	g.dishTileID = -1
	g.dishCells = nil
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
	g.starterGateCount = 1
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
	g.currentStageID = g.progression.StartStageID
	g.tutorialLines = nil
	g.tutorialSeen = map[string]bool{}
	g.tutorialDismissTimer = 0
	g.activePerks = nil
	g.stagePowerSpent = map[string]float64{}
	g.perksAwarded = map[string]int{}
	g.perkChoice = nil
	g.perkOfferCooldown = 0
	g.configTileID = -1
	g.mugDragActive = false
	g.mugDragStartTile = -1
	g.mugDragTargetTile = -1
	g.mugMoves = map[int]*mugMoveState{}
	g.lastTapTacticalTileID = -1
	g.fieldDataPopups = nil
	g.fieldDataScanCarry = map[fieldDataTileKey]float64{}
	g.uplinkPackets = nil
	g.creaturesEnabled = false
	g.gateUplinkUnlocked = false
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

func (g *Game) displayVersion() string {
	if g.version == "" {
		return "dev"
	}
	return g.version
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
		log.Printf("petri_dish: save load failed (%v) — resetting", err)
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
	switch {
	case data.StarterGateCount != nil:
		g.starterGateCount = *data.StarterGateCount
	case !g.hasPlacedStarterGate():
		g.starterGateCount = 1
	default:
		g.starterGateCount = 0
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
	g.fieldDataPopups = nil
	g.fieldDataScanCarry = map[fieldDataTileKey]float64{}
	g.uplinkPackets = nil
	g.creaturesEnabled = data.CreaturesEnabled
	g.gateUplinkUnlocked = data.GateUplinkUnlocked
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
		if !tacticalMapNeedsSave(tmap) {
			continue
		}
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
	starterGateCount := g.starterGateCount
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
		Version:            g.version,
		WorldSeed:          g.globe.Seed,
		Inventory:          inventory,
		PartInventory:      partInventory,
		StarterMinerCount:  &starterMinerCount,
		StarterGateCount:   &starterGateCount,
		TutorialSeen:       tutorialSeen,
		CurrentStage:       g.currentStageID,
		KnownRecipes:       knownRecipes,
		MinedTotals:        minedTotals,
		ActivePerks:        activePerks,
		StagePowerSpent:    stagePowerSpent,
		PerksAwarded:       perksAwarded,
		CreaturesEnabled:   g.creaturesEnabled,
		GateUplinkUnlocked: g.gateUplinkUnlocked,
		Camera: core.SavedCamera{
			Lon:  g.globe.CameraLon,
			Lat:  g.globe.CameraLat,
			Zoom: g.zoom,
		},
		Selected: g.globe.SelectedCell,
		Tactical: tactical,
	}
}

func tacticalMapNeedsSave(tmap *core.TacticalMap) bool {
	if tmap == nil {
		return false
	}
	for _, amount := range tmap.Supply {
		if amount != 0 {
			return true
		}
	}
	if len(tmap.Entities) > 0 || tmap.CreatureSpawnRemaining > 0 {
		return true
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		if tile.PowerBuffer > 0 || tile.ResourceCarry > 0 {
			return true
		}
		if tile.ResourceRichness > 0 && tile.ResourceRemaining < core.ResourceCapacity(tile.Resource, tile.ResourceRichness) {
			return true
		}
		if tile.Device != nil && tile.Device.Kind != core.DeviceKindNone {
			return true
		}
	}
	return false
}

func (g *Game) saveNow() {
	g.saveNowImmediate()
}

func (g *Game) saveSoon() {
	if g.saveDir == "" {
		return
	}
	g.beginSaveOverlay()
}

func (g *Game) saveNowImmediate() {
	if g.saveDir == "" {
		return
	}
	if err := g.buildSaveData().Save(g.saveDir); err != nil {
		log.Printf("petri_dish: save write failed: %v", err)
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
		bytes, err := json.Marshal(g.saveOverlayData)
		if err != nil {
			log.Printf("petri_dish: save marshal failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		g.saveOverlayBytes = bytes
		g.saveOverlayStage = saveStageWriteTemp
	case saveStageWriteTemp:
		if err := os.MkdirAll(g.saveDir, 0o755); err != nil {
			log.Printf("petri_dish: save mkdir failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		if err := os.WriteFile(g.saveOverlayTempPath, g.saveOverlayBytes, 0o644); err != nil {
			log.Printf("petri_dish: save temp write failed: %v", err)
			g.finishSaveOverlay()
			return
		}
		g.saveOverlayStage = saveStageRename
	case saveStageRename:
		if err := os.Rename(g.saveOverlayTempPath, g.saveOverlayFinalPath); err != nil {
			log.Printf("petri_dish: save rename failed: %v", err)
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

func (g *Game) OpenTacticalForTesting() {
	if g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		g.globe.SelectedCell = 0
	}
	g.enterTactical()
	if tmap := g.currentTacticalMap(); tmap != nil && len(tmap.Tiles) > 0 {
		g.tacticalTile = len(tmap.Tiles) / 2
	}
}

func (g *Game) OpenDishForTesting() {
	g.OpenTacticalForTesting()
	if tmap := g.currentTacticalMap(); tmap != nil && len(tmap.Tiles) > 0 {
		g.enterDish(g.tacticalTile)
	}
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
	maxLineWidth := 0
	for _, line := range g.tutorialLines {
		if width := len(line)*7 + 4; width > maxLineWidth {
			maxLineWidth = width
		}
	}
	w := float64(maxLineWidth + 36)
	if w < 220 {
		w = 220
	}
	maxW := float64(g.screenWidth) - 40
	if w > maxW {
		w = maxW
	}
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
	if g.minedTotals[core.ResourceIronOre] <= 0 &&
		g.minedTotals[core.ResourceCopperOre] <= 0 &&
		g.minedTotals[core.ResourceCoal] <= 0 &&
		g.minedTotals[core.ResourceCrystal] <= 0 {
		return
	}
	g.showTutorialOnce("bootstrap_stone_plain_tile", []string{
		"Stone has no ore marker.",
		"Mine a plain tile to collect stone.",
	})
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
	g.animationTime += dt
	g.advanceFieldDataPopups(dt)
	g.advanceUplinkPackets(dt)
	if g.tacticalPerkSystemActive() {
		g.advancePerkOfferCooldown(dt)
		g.advancePerkCelebration(dt)
	}
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
		before := fieldDataMiningSnapshot(tmap)
		tmap.Produce(dt, g.inventory, g.minedTotals, mods)
		g.collectFieldDataFromMining(tmap, before)
	}
	g.advanceTacticalDeviceMotion(dt)
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
	if g.mode == modeBuild {
		g.handleBuildInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeTech {
		g.handleTechInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeSettings {
		g.handleSettingsInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeDish {
		g.handleDishInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeSmelterConfig {
		g.handleSmelterConfigInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeAssemblerConfig {
		g.handleAssemblerConfigInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeGeneratorConfig {
		g.handleGeneratorConfigInput()
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	if g.mode == modeTactical {
		g.handleTacticalInput()
		if tmap := g.currentTacticalMap(); tmap != nil && g.creaturesEnabled {
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
	if g.mode == modeTech {
		g.drawTechView(screen)
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
	if g.mode == modeDish {
		g.drawDish(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeSmelterConfig {
		g.drawSmelterConfig(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeAssemblerConfig {
		g.drawAssemblerConfig(screen)
		g.drawModal(screen)
		g.drawPerkChoice(screen)
		g.drawInventoryOverlay(screen)
		g.drawTutorial(screen)
		g.drawSaveOverlay(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeGeneratorConfig {
		g.drawGeneratorConfig(screen)
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
	g.drawStrategicTechButton(screen)
	g.drawStrategicStats(screen)
	g.drawStrategicStageGoals(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
	if g.strategicDeviceCount(core.DeviceKindMiner)+g.strategicDeviceCount(core.DeviceKindSmelter)+g.strategicDeviceCount(core.DeviceKindGate)+g.strategicDeviceCount(core.DeviceKindGenerator)+g.strategicDeviceCount(core.DeviceKindAssembler) > 0 {
		techX, techY, techW, _ := g.techButtonRect()
		deviceH := g.strategicDevicesCardHeight()
		deviceX := techX + techW - 170
		deviceY := techY - 12 - deviceH
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
		g.beginTacticalMUGDrag(x, y)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.dragTouchID == -1 {
		x, y := ebiten.CursorPosition()
		g.finishTacticalPointer(x, y)
		g.dragging = false
	}

	if g.dragTouchID == -1 {
		justTouched := inpututil.AppendJustPressedTouchIDs(nil)
		if len(justTouched) > 0 {
			x, y := ebiten.TouchPosition(justTouched[0])
			g.beginDrag(justTouched[0], x, y)
			g.beginTacticalMUGDrag(x, y)
		}
	}

	if g.dragTouchID != -1 {
		ids := ebiten.AppendTouchIDs(nil)
		active := false
		for _, id := range ids {
			if id == g.dragTouchID {
				active = true
				x, y := ebiten.TouchPosition(id)
				g.tryHoldPowerStarterMiner(x, y)
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
		g.tryHoldPowerStarterMiner(x, y)
		g.applyTacticalDrag(x, y)
	}
}

func (g *Game) handleDishInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleDishTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleDishTap(x, y)
	}
}

func (g *Game) handleDishTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeTactical
		return
	}
	if id, ok := g.pickDishCell(x, y); ok {
		for i := range g.dishCells {
			if g.dishCells[i].ID == id {
				g.dishCells[i].Influence = (g.dishCells[i].Influence + 1) % 4
				return
			}
		}
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

const doubleTapWindow = 900 * time.Millisecond

func (g *Game) finishSelection(x, y int) {
	if g.dragMoved {
		return
	}
	if g.handleInventoryButtonTap(x, y) {
		return
	}
	techX, techY, techW, techH := g.techButtonRect()
	if g.pointInRect(float64(x), float64(y), techX, techY, techW, techH) {
		g.mode = modeTech
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
	panScale := g.strategicPanScale()
	g.globe.CameraLon -= float64(dx) * 0.012 * panScale
	g.globe.CameraLat += float64(dy) * 0.006 * panScale
	g.clampCamera()
}

func (g *Game) strategicPanScale() float64 {
	if g.zoom <= 0 {
		return 1
	}
	return clampRange(1/math.Pow(g.zoom, 0.85), 0.22, 1.25)
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
	g.drawStrategicMagnifierHint(screen, cells)
	g.drawStrategicDeviceBadges(screen, cells)
}

func (g *Game) drawStrategicMagnifierHint(screen *ebiten.Image, cells []drawCell) {
	alpha := g.strategicMagnifierAlpha()
	if alpha <= 0 || g.globe.SelectedCell < 0 {
		return
	}
	for _, cell := range cells {
		if cell.index != g.globe.SelectedCell {
			continue
		}
		centerX, centerY, ok := g.projectPoint(cell.center)
		if !ok {
			return
		}
		radius := math.Inf(1)
		for _, corner := range cell.corners {
			x, y, ok := g.projectPoint(corner)
			if !ok {
				return
			}
			radius = math.Min(radius, math.Hypot(x-centerX, y-centerY))
		}
		if math.IsInf(radius, 0) || math.IsNaN(radius) || radius <= 0 {
			return
		}
		g.drawMagnifierIcon(screen, centerX, centerY, radius*0.72, uint8(118*alpha))
		return
	}
}

func (g *Game) strategicMagnifierAlpha() float64 {
	if g.lastTapCellID != g.globe.SelectedCell || g.lastTapTime.IsZero() {
		return 0
	}
	elapsed := time.Since(g.lastTapTime)
	if elapsed < 0 || elapsed > doubleTapWindow {
		return 0
	}
	t := float64(elapsed) / float64(doubleTapWindow)
	if t < 0.28 {
		return 1
	}
	return clampRange(1-(t-0.28)/0.72, 0, 1)
}

func (g *Game) drawMagnifierIcon(screen *ebiten.Image, centerX, centerY, size float64, alpha uint8) {
	if alpha == 0 || size <= 0 {
		return
	}
	lensR := size * 0.30
	stroke := float32(math.Max(2, size*0.055))
	clr := color.RGBA{0, 0, 0, alpha}
	shadow := color.RGBA{240, 248, 255, uint8(float64(alpha) * 0.42)}
	lensX := centerX - size*0.08
	lensY := centerY - size*0.07
	handleAX := lensX + lensR*0.62
	handleAY := lensY + lensR*0.62
	handleBX := centerX + size*0.34
	handleBY := centerY + size*0.35
	drawDisc(screen, float32(lensX), float32(lensY), float32(lensR*0.82), color.RGBA{255, 255, 255, uint8(float64(alpha) * 0.16)})
	vector.StrokeCircle(screen, float32(lensX+size*0.025), float32(lensY+size*0.025), float32(lensR), stroke, shadow, false)
	vector.StrokeLine(screen, float32(handleAX+size*0.025), float32(handleAY+size*0.025), float32(handleBX+size*0.025), float32(handleBY+size*0.025), stroke*1.25, shadow, false)
	vector.StrokeCircle(screen, float32(lensX), float32(lensY), float32(lensR), stroke, clr, false)
	vector.StrokeLine(screen, float32(handleAX), float32(handleAY), float32(handleBX), float32(handleBY), stroke*1.25, clr, false)
}

func (g *Game) drawStrategicDeviceBadges(screen *ebiten.Image, cells []drawCell) {
	for _, cell := range cells {
		if !g.strategicCellHasGate(cell.index) {
			continue
		}
		centerX, centerY, ok := g.projectPoint(cell.center)
		if !ok {
			continue
		}
		g.drawStrategicGateMarker(screen, centerX, centerY-8)
	}
}

func (g *Game) drawStrategicGateMarker(screen *ebiten.Image, x, y float64) {
	sprite := gateSprite()
	if g.gateUplinkUnlocked && gateUplinkSprite != nil {
		sprite = gateUplinkSprite
	}
	if sprite != nil {
		drawCenteredSprite(screen, sprite, x, y, 24, 1.5, 2.5, 0.28, color.RGBA{})
		return
	}
	drawDisc(screen, float32(x+1.5), float32(y+2.5), 8, color.RGBA{0, 0, 0, 76})
	drawDisc(screen, float32(x), float32(y), 8, color.RGBA{9, 18, 32, 235})
	drawDisc(screen, float32(x), float32(y), 6.5, deviceKindBadgeColor(core.DeviceKindGate))
	drawFilledRect(screen, float32(x-5), float32(y-1), 10, 2, color.RGBA{220, 246, 250, 255})
	drawFilledRect(screen, float32(x-1), float32(y-5), 2, 10, color.RGBA{220, 246, 250, 255})
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

func (g *Game) drawStrategicTechButton(screen *ebiten.Image) {
	x, y, w, h := g.techButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{42, 62, 82, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "TECH", int(x)+18, int(y)+12)
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

func (g *Game) drawTacticalSelectionPopup(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil {
		return
	}
	x, y, w, h := g.tacticalSelectionPopupRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 8, color.RGBA{8, 18, 32, 220})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, 240})
	g.drawAlphaDebugTextBlock(screen, x+10, y+10, g.tacticalSelectionLines(tile), 1)
}

func (g *Game) tacticalSelectionLines(tile *core.TacticalTile) []string {
	lines := []string{fmt.Sprintf("TILE %d", tile.ID)}
	if tile.Ocean {
		lines = append(lines, "terrain ocean")
	} else {
		lines = append(lines, "terrain land")
	}
	if tile.Resource != core.ResourceNone && tile.ResourceRemaining > 0 {
		lines = append(lines, fmt.Sprintf("%s %.0f%%", resourceLabel(tile.Resource), tile.ResourceRichness*100))
	} else {
		lines = append(lines, "resource none")
	}
	if tile.Device != nil && tile.Device.Kind != core.DeviceKindNone {
		lines = append(lines, "device "+core.DeviceKindLabel(tile.Device.Kind))
	} else {
		lines = append(lines, "device none")
	}
	lines = append(lines, "double tap: dish")
	return lines
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
		0, 0, 0) // black halo
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
	case core.GoalProduceResource:
		if g.minedTotals == nil {
			return 0, goal.Amount
		}
		return g.minedTotals[goal.Resource], goal.Amount
	case core.GoalExportResource:
		if g.inventory == nil {
			return 0, goal.Amount
		}
		return g.inventory[goal.Resource], goal.Amount
	case core.GoalDiscoverResource:
		return g.discoveredResourceCount(goal.Resource), goal.Amount
	case core.GoalDiscoverRecipe:
		if g.knownRecipes[goal.RecipeID] {
			return 1, goal.Amount
		}
		return 0, goal.Amount
	case core.GoalBuildDevice:
		return g.deviceKindCount(goal.Device), goal.Amount
	case core.GoalPlaceStarterUnit:
		if goal.Device == core.DeviceKindGate {
			return g.deviceKindCount(core.DeviceKindGate), goal.Amount
		}
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
	miners := g.strategicDeviceCount(core.DeviceKindMiner)
	smelters := g.strategicDeviceCount(core.DeviceKindSmelter)
	gates := g.strategicDeviceCount(core.DeviceKindGate)
	generators := g.strategicDeviceCount(core.DeviceKindGenerator)
	assemblers := g.strategicDeviceCount(core.DeviceKindAssembler)
	return []string{
		"REGION DEVICES",
		fmt.Sprintf("miners  %d", miners),
		fmt.Sprintf("smelters %d", smelters),
		fmt.Sprintf("gates   %d", gates),
		fmt.Sprintf("gens    %d", generators),
		fmt.Sprintf("asmblrs %d", assemblers),
	}
}

func (g *Game) strategicDevicesCardHeight() float64 {
	lines := g.strategicDevicesLines()
	return float64(len(lines)*16 + 24)
}

func (g *Game) strategicDeviceCount(kind core.DeviceKind) int {
	if g.globe.SelectedCell < 0 {
		return 0
	}
	count := 0
	if tmap := g.tacticalMapForCell(g.globe.SelectedCell); tmap != nil {
		for _, tile := range tmap.Tiles {
			if tile.Device != nil && tile.Device.Kind == kind {
				count++
			}
		}
	}
	return count
}

func (g *Game) strategicCellHasGate(cellID int) bool {
	tmap := g.tacticalMapForCell(cellID)
	if tmap == nil {
		return false
	}
	for _, tile := range tmap.Tiles {
		if tile.Device != nil && tile.Device.Kind == core.DeviceKindGate {
			return true
		}
	}
	return false
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
	g.drawFieldDataPopups(screen)
	g.drawUplinkPackets(screen)
	g.drawTacticalStageGoals(screen)
	g.drawTacticalBackButton(screen)
	g.drawTacticalDepositButton(screen)
	g.drawTacticalDisassembleButton(screen)
	g.drawTacticalPlaceBuildButton(screen)
	g.drawTacticalBuildButton(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
	g.drawTacticalHudMeters(screen)
	g.drawTacticalSelectionPopup(screen)
	g.drawPendingBuildCard(screen)
}

func (g *Game) drawDish(screen *ebiten.Image) {
	screen.Fill(color.RGBA{5, 12, 18, 255})
	g.drawDishBackdrop(screen)
	g.drawArrowBackButton(screen)
	g.drawDishCells(screen)
	g.drawDishHud(screen)
}

func (g *Game) drawDishBackdrop(screen *ebiten.Image) {
	cx, cy := g.dishCenter()
	radius := g.dishRadius()
	drawDisc(screen, float32(cx), float32(cy), float32(radius*1.12), color.RGBA{16, 42, 48, 120})
	drawDisc(screen, float32(cx), float32(cy), float32(radius*1.02), color.RGBA{28, 76, 72, 135})
	drawDisc(screen, float32(cx), float32(cy), float32(radius*0.94), color.RGBA{12, 34, 40, 235})
	vector.StrokeCircle(screen, float32(cx), float32(cy), float32(radius*1.02), 3, color.RGBA{174, 230, 232, 205}, false)
	vector.StrokeCircle(screen, float32(cx), float32(cy), float32(radius*0.92), 1.4, color.RGBA{90, 180, 172, 130}, false)
	drawDisc(screen, float32(cx-radius*0.34), float32(cy-radius*0.42), float32(radius*0.18), color.RGBA{232, 255, 250, 24})
}

func (g *Game) drawDishCells(screen *ebiten.Image) {
	cells := g.currentDishCells()
	if len(cells) == 0 {
		return
	}
	cx, cy := g.dishCenter()
	scale := g.dishCellScale()
	for i := range cells {
		cell := &cells[i]
		x := cx + cell.Center.X*scale
		y := cy + cell.Center.Y*scale
		fill := dishCellColor(cell.Kind)
		pulse := 0.5 + 0.5*math.Sin(g.animationTime*(1.2+float64(cell.Kind)*0.18)+cell.Phase)
		fill = core.BlendColor(fill, color.RGBA{226, 255, 212, 255}, 0.08*pulse)
		if cell.Influence > 0 {
			fill = core.BlendColor(fill, dishInfluenceColor(cell.Influence), 0.34)
		}
		points := dishHexPoints(x, y, scale*0.52)
		verts := make([]ebiten.Vertex, 0, len(points))
		for _, p := range points {
			verts = append(verts, ebiten.Vertex{DstX: float32(p.x), DstY: float32(p.y), SrcX: 0, SrcY: 0})
		}
		drawFilledPolygon(screen, verts, fill)
		edge := core.ScaleColor(fill, 0.68)
		if cell.Influence > 0 {
			edge = dishInfluenceColor(cell.Influence)
		}
		drawPolygonStroke(screen, verts, edge)
		drawDisc(screen, float32(x+scale*0.08*math.Sin(cell.Phase)), float32(y-scale*0.06), float32(scale*(0.10+0.035*pulse)), core.ScaleColor(fill, 1.34))
	}
}

func (g *Game) drawDishHud(screen *ebiten.Image) {
	title := "PETRI DISH"
	if g.dishTileID >= 0 {
		title = fmt.Sprintf("PETRI DISH %d", g.dishTileID)
	}
	g.drawAlphaDebugTextBlock(screen, 24, 22, []string{
		title,
		"automated culture",
		"tap cells to poke/prod",
	}, 1)
	x := 24.0
	y := float64(g.screenHeight) - 154
	w := float64(g.screenWidth) - 48
	h := 74.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 8, color.RGBA{8, 18, 24, 218})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{122, 214, 190, 220})
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, []string{
		"Culture output: latent perk spores",
		"Influence cycle: feed / stress / isolate / clear",
		"Battle perks will hook in here next.",
	}, 1)
}

func (g *Game) drawTacticalHudMeters(screen *ebiten.Image) {
	w := 170.0
	x := float64(g.screenWidth) - w - 16
	stackY := 16.0
	if perkH := g.drawPerkProgressCard(screen, x, stackY, w, 1); perkH > 0 {
		stackY += perkH + 8
	}
	if powerH := g.drawPowerProgressCard(screen, x, stackY, w, 1); powerH > 0 {
		stackY += powerH + 8
	}
}

func (g *Game) drawTacticalMap(screen *ebiten.Image) {
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return
	}
	cx, cy := g.tacticalCenter()
	scale := g.tacticalTileScale()
	generatorPowerHighlight := g.selectedGeneratorPowerHighlight(tmap)
	for _, tile := range tmap.Tiles {
		points := tacticalHexPoints(tile.Center, scale)
		fill := tile.Fill
		if clr, ok := generatorPowerHighlight[tile.ID]; ok {
			fill = core.BlendColor(fill, clr, 0.46)
		}
		if tile.ID == g.tacticalTile {
			fill = core.BlendColor(fill, color.RGBA{246, 249, 255, 255}, 0.33)
		}
		if g.pendingBuildRecipeID != "" && g.canPlacePendingBuildOnTile(tile.ID) {
			fill = core.BlendColor(fill, color.RGBA{128, 226, 160, 255}, 0.26)
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
		g.drawTacticalTileTexture(screen, &tile, vertices, points, scale)
		edge := core.ScaleColor(fill, 0.72)
		if clr, ok := generatorPowerHighlight[tile.ID]; ok {
			edge = clr
		}
		if tile.ID == g.tacticalTile {
			edge = core.BlendColor(edge, color.RGBA{185, 239, 255, 255}, 0.45)
		}
		if g.pendingBuildRecipeID != "" && g.canPlacePendingBuildOnTile(tile.ID) {
			edge = color.RGBA{128, 226, 160, 255}
		}
		drawPolygonStroke(screen, vertices, edge)
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		g.drawTacticalTileResourceGlyph(screen, tile, cx, cy, scale)
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		g.drawTacticalTileDevice(screen, tmap, tile, cx, cy, scale)
	}
	if g.creaturesEnabled {
		g.drawTacticalEntities(screen, tmap, cx, cy, scale)
	}
	g.drawMUGDragPath(screen, tmap, cx, cy, scale)
	g.drawTacticalTileIndicators(screen, tmap, cx, cy, scale)
}

func (g *Game) drawTacticalTileTexture(screen *ebiten.Image, tile *core.TacticalTile, vertices []ebiten.Vertex, localPoints []screenPoint, scale float64) {
	texture := tacticalTextureForTile(tile)
	if texture == nil || len(vertices) < 3 || len(localPoints) != len(vertices) || scale <= 0 {
		return
	}

	bounds := texture.Bounds()
	texW := float64(bounds.Dx())
	texH := float64(bounds.Dy())
	if texW == 0 || texH == 0 {
		return
	}

	overlay := make([]ebiten.Vertex, len(vertices))
	copy(overlay, vertices)
	srcScale := scale * 1.15
	texDX := bounds.Dx()
	texDY := bounds.Dy()
	if texDX <= 0 {
		texDX = 1
	}
	if texDY <= 0 {
		texDY = 1
	}
	offsetX := float64((tile.Q*29 + tile.R*17) % texDX)
	offsetY := float64((tile.R*31 - tile.Q*13) % texDY)
	alpha := tacticalTextureAlpha(tile)
	for i := range overlay {
		p := localPoints[i]
		overlay[i].SrcX = float32((p.x/srcScale+0.5)*texW + offsetX)
		overlay[i].SrcY = float32((p.y/srcScale+0.5)*texH + offsetY)
		overlay[i].ColorR = 1
		overlay[i].ColorG = 1
		overlay[i].ColorB = 1
		overlay[i].ColorA = alpha
	}

	indices := make([]uint16, 0, (len(overlay)-2)*3)
	for i := 1; i < len(overlay)-1; i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	opts := &ebiten.DrawTrianglesOptions{Address: ebiten.AddressRepeat, Filter: ebiten.FilterNearest}
	screen.DrawTriangles(overlay, indices, texture, opts)
}

func (g *Game) selectedGeneratorPowerHighlight(tmap *core.TacticalMap) map[int]color.RGBA {
	if tmap == nil || g.tacticalTile < 0 || g.tacticalTile >= len(tmap.Tiles) {
		return nil
	}
	generator := &tmap.Tiles[g.tacticalTile]
	if generator.Device == nil || generator.Device.Kind != core.DeviceKindGenerator {
		return nil
	}
	clr := color.RGBA{208, 76, 68, 255}
	if g.generatorWorking(generator) {
		clr = color.RGBA{76, 220, 118, 255}
	}
	out := map[int]color.RGBA{}
	for _, neighbor := range tmap.AdjacentTiles(generator) {
		out[neighbor.ID] = clr
	}
	return out
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

	if sprite := tacticalResourceSprite(tile.Resource); sprite != nil {
		drawCenteredSprite(screen, sprite, glyphX, glyphY, scale*0.54, scale*0.03, scale*0.05, 0.32, color.RGBA{})
		return
	}

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

func (g *Game) drawTacticalTileDevice(screen *ebiten.Image, tmap *core.TacticalMap, tile *core.TacticalTile, cx, cy, scale float64) {
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return
	}
	center := tile.Center
	if tile.Device.Kind == core.DeviceKindMiner && tile.Device.SpecialStarter {
		center = g.movingMUGCenter(tmap, tile)
	}
	centerX := cx + g.tacticalPanX + center.X*scale
	centerY := cy + g.tacticalPanY + center.Y*scale
	deviceScale := scale
	if tile.Device.SpecialStarter && tile.Device.DeployTimer > 0 {
		t := clampRange(tile.Device.DeployTimer, 0, 1)
		centerY -= scale * 1.8 * t * t
		deviceScale *= 1 + 0.10*math.Sin((1-t)*math.Pi)
		if t < 0.28 {
			impact := 1 - t/0.28
			drawDisc(screen, float32(centerX), float32(cy+g.tacticalPanY+tile.Center.Y*scale+scale*0.12), float32(scale*(0.18+impact*0.22)), color.RGBA{218, 184, 112, uint8(76 * impact)})
		}
	}

	switch tile.Device.Kind {
	case core.DeviceKindMiner:
		g.drawMinerSprite(screen, tile, centerX, centerY, deviceScale)
	case core.DeviceKindSmelter:
		g.drawSmelter(screen, tile, centerX, centerY, deviceScale)
	case core.DeviceKindGate:
		g.drawGate(screen, centerX, centerY, deviceScale)
	case core.DeviceKindGenerator:
		g.drawGenerator(screen, tile, centerX, centerY, deviceScale)
	case core.DeviceKindAssembler:
		g.drawAssembler(screen, tile, centerX, centerY, deviceScale)
	}
}

func (g *Game) movingMUGCenter(tmap *core.TacticalMap, tile *core.TacticalTile) core.Vec3 {
	if tmap == nil || tile == nil {
		return core.Vec3{}
	}
	move := g.mugMoves[tmap.CellID]
	if move == nil || len(move.path) < 2 || move.path[0] != tile.ID {
		return tile.Center
	}
	nextID := move.path[1]
	if nextID < 0 || nextID >= len(tmap.Tiles) {
		return tile.Center
	}
	t := clampRange(move.progress, 0, 1)
	t = t * t * (3 - 2*t)
	return tile.Center.Mul(1 - t).Add(tmap.Tiles[nextID].Center.Mul(t))
}

func (g *Game) drawTacticalTileIndicators(screen *ebiten.Image, tmap *core.TacticalMap, cx, cy, scale float64) {
	if tmap == nil {
		return
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		if tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
			continue
		}
		centerX := cx + g.tacticalPanX + tile.Center.X*scale
		centerY := cy + g.tacticalPanY + tile.Center.Y*scale
		ix, iy := tacticalTileIndicatorAnchor(centerX, centerY, scale, 0)
		runCost := core.DeviceDefinition(tile.Device.Kind).RunPowerCost
		if runCost > 0 && tile.Device.Kind != core.DeviceKindGenerator {
			g.drawPowerIndicator(screen, ix, iy, scale*0.56, tile.PowerBuffer, runCost)
		}
		if tile.Device.Kind != core.DeviceKindMiner {
			continue
		}
		rx, ry := tacticalTileIndicatorAnchor(centerX, centerY, scale, 3)
		g.drawResourceIndicator(screen, rx, ry, scale, tile)
	}
}

func (g *Game) drawMUGDragPath(screen *ebiten.Image, tmap *core.TacticalMap, cx, cy, scale float64) {
	var path []int
	if g.mugDragActive && g.mugDragStartTile >= 0 && g.mugDragTargetTile >= 0 {
		path = tmap.TilePath(g.mugDragStartTile, g.mugDragTargetTile)
	} else if move := g.mugMoves[tmap.CellID]; move != nil {
		path = move.path
	}
	if len(path) < 2 {
		return
	}
	for i := 0; i < len(path)-1; i++ {
		if !tacticalTileInRange(tmap, path[i]) || !tacticalTileInRange(tmap, path[i+1]) {
			continue
		}
		a := tmap.Tiles[path[i]]
		b := tmap.Tiles[path[i+1]]
		ax := cx + g.tacticalPanX + a.Center.X*scale
		ay := cy + g.tacticalPanY + a.Center.Y*scale
		bx := cx + g.tacticalPanX + b.Center.X*scale
		by := cy + g.tacticalPanY + b.Center.Y*scale
		vector.StrokeLine(screen, float32(ax), float32(ay), float32(bx), float32(by), 3, color.RGBA{126, 228, 168, 210}, false)
		drawDisc(screen, float32(bx), float32(by), float32(scale*0.055), color.RGBA{126, 228, 168, 170})
	}
}

func (g *Game) drawSmelter(screen *ebiten.Image, tile *core.TacticalTile, centerX, centerY, scale float64) {
	if sprite := smelterSpriteForTile(tile, g.animationTime, g.smelterWorking(tile)); sprite != nil {
		drawCenteredSprite(screen, sprite, centerX, centerY-1, scale*1.02, scale*0.06, scale*0.09, 0.34, color.RGBA{})
		return
	}
	body := color.RGBA{92, 82, 76, 255}
	glow := color.RGBA{84, 92, 96, 160}
	if g.smelterWorking(tile) {
		body = color.RGBA{116, 82, 64, 255}
		glow = color.RGBA{244, 132, 62, 180}
		drawDisc(screen, float32(centerX), float32(centerY+scale*0.04), float32(scale*0.30), color.RGBA{244, 116, 42, 70})
	}
	drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.25), color.RGBA{0, 0, 0, 78})
	drawRoundedRect(screen, float32(centerX-scale*0.20), float32(centerY-scale*0.16), float32(scale*0.40), float32(scale*0.34), 5, body)
	drawRectOutline(screen, float32(centerX-scale*0.20), float32(centerY-scale*0.16), float32(scale*0.40), float32(scale*0.34), color.RGBA{210, 188, 160, 220})
	drawFilledRect(screen, float32(centerX-scale*0.09), float32(centerY-scale*0.26), float32(scale*0.18), float32(scale*0.10), color.RGBA{82, 78, 76, 255})
	drawDisc(screen, float32(centerX), float32(centerY+scale*0.02), float32(scale*0.09), glow)
}

func (g *Game) drawGate(screen *ebiten.Image, centerX, centerY, scale float64) {
	if g.gateUplinkUnlocked && gateUplinkSprite != nil {
		drawCenteredSprite(screen, gateUplinkSprite, centerX, centerY-scale*0.08, scale*0.98, scale*0.06, scale*0.09, 0.32, color.RGBA{})
		return
	}
	if sprite := gateSprite(); sprite != nil {
		drawCenteredSprite(screen, sprite, centerX, centerY-1, scale*0.94, scale*0.05, scale*0.08, 0.32, color.RGBA{})
		if g.gateUplinkUnlocked {
			g.drawGateUplinkOverlay(screen, centerX, centerY, scale)
		}
		return
	}
	drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.24), color.RGBA{0, 0, 0, 78})
	body := color.RGBA{42, 76, 94, 245}
	outline := color.RGBA{144, 220, 236, 255}
	if g.gateUplinkUnlocked {
		body = color.RGBA{38, 92, 72, 245}
		outline = color.RGBA{122, 240, 168, 255}
	}
	drawRoundedRect(screen, float32(centerX-scale*0.20), float32(centerY-scale*0.20), float32(scale*0.40), float32(scale*0.40), 6, body)
	drawRectOutline(screen, float32(centerX-scale*0.20), float32(centerY-scale*0.20), float32(scale*0.40), float32(scale*0.40), outline)
	drawFilledRect(screen, float32(centerX-scale*0.12), float32(centerY-scale*0.03), float32(scale*0.24), float32(scale*0.06), color.RGBA{190, 238, 244, 255})
	drawFilledRect(screen, float32(centerX-scale*0.03), float32(centerY-scale*0.12), float32(scale*0.06), float32(scale*0.24), color.RGBA{190, 238, 244, 255})
	if g.gateUplinkUnlocked {
		g.drawGateUplinkOverlay(screen, centerX, centerY, scale)
	}
}

func (g *Game) drawGateUplinkOverlay(screen *ebiten.Image, centerX, centerY, scale float64) {
	drawDisc(screen, float32(centerX), float32(centerY), float32(scale*0.31), color.RGBA{78, 232, 132, 54})
	vector.StrokeCircle(screen, float32(centerX), float32(centerY), float32(scale*0.26), float32(math.Max(1.2, scale*0.025)), color.RGBA{122, 255, 170, 210}, false)
	drawFilledRect(screen, float32(centerX-scale*0.03), float32(centerY-scale*0.36), float32(scale*0.06), float32(scale*0.20), color.RGBA{102, 240, 150, 230})
	drawDisc(screen, float32(centerX), float32(centerY-scale*0.39), float32(scale*0.055), color.RGBA{146, 255, 184, 240})
}

func (g *Game) drawGenerator(screen *ebiten.Image, tile *core.TacticalTile, centerX, centerY, scale float64) {
	working := g.generatorWorking(tile)
	if sprite := generatorSprite(); sprite != nil {
		if working {
			drawDisc(screen, float32(centerX), float32(centerY+scale*0.02), float32(scale*0.30), color.RGBA{92, 220, 138, 62})
			drawCenteredSprite(screen, sprite, centerX, centerY-1, scale*1.03, scale*0.06, scale*0.09, 0.34, color.RGBA{})
			return
		}
		drawCenteredSprite(screen, sprite, centerX, centerY-1, scale*1.03, scale*0.06, scale*0.09, 0.34, color.RGBA{150, 158, 150, 255})
		return
	}
	body := color.RGBA{72, 78, 76, 255}
	glow := color.RGBA{94, 132, 150, 160}
	if working {
		body = color.RGBA{82, 92, 84, 255}
		glow = color.RGBA{130, 220, 168, 190}
		drawDisc(screen, float32(centerX), float32(centerY+scale*0.02), float32(scale*0.28), color.RGBA{92, 220, 138, 62})
	}
	drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.25), color.RGBA{0, 0, 0, 78})
	drawRoundedRect(screen, float32(centerX-scale*0.22), float32(centerY-scale*0.15), float32(scale*0.44), float32(scale*0.30), 5, body)
	drawRectOutline(screen, float32(centerX-scale*0.22), float32(centerY-scale*0.15), float32(scale*0.44), float32(scale*0.30), color.RGBA{190, 214, 188, 230})
	drawFilledRect(screen, float32(centerX-scale*0.08), float32(centerY-scale*0.25), float32(scale*0.16), float32(scale*0.10), color.RGBA{74, 74, 70, 255})
	drawDisc(screen, float32(centerX), float32(centerY), float32(scale*0.08), glow)
	drawFilledRect(screen, float32(centerX-scale*0.13), float32(centerY+scale*0.10), float32(scale*0.26), float32(scale*0.04), color.RGBA{220, 238, 210, 230})
}

func (g *Game) drawAssembler(screen *ebiten.Image, tile *core.TacticalTile, centerX, centerY, scale float64) {
	working := g.assemblerWorking(tile)
	configured := false
	if tile != nil && tile.Device != nil {
		_, configured = core.AssemblerOutputForConfig(tile.Device.ConfigInput)
	}
	if sprite := assemblerSpriteForTile(tile, g.animationTime, working); sprite != nil {
		if working {
			drawDisc(screen, float32(centerX), float32(centerY+scale*0.02), float32(scale*0.34), color.RGBA{238, 210, 104, 64})
			drawCenteredSprite(screen, sprite, centerX, centerY-scale*0.02, scale*1.12, scale*0.06, scale*0.09, 0.30, color.RGBA{})
			return
		}
		tint := color.RGBA{}
		if !configured {
			tint = color.RGBA{150, 154, 148, 255}
		}
		drawCenteredSprite(screen, sprite, centerX, centerY-scale*0.02, scale*1.12, scale*0.06, scale*0.09, 0.30, tint)
		return
	}
	body := color.RGBA{78, 84, 86, 255}
	edge := color.RGBA{172, 188, 184, 230}
	glow := color.RGBA{92, 110, 116, 140}
	belt := color.RGBA{96, 104, 106, 235}
	if configured {
		body = color.RGBA{86, 94, 82, 255}
		edge = color.RGBA{214, 198, 138, 235}
		glow = color.RGBA{150, 132, 72, 150}
		belt = color.RGBA{142, 126, 76, 240}
	}
	if working {
		body = color.RGBA{112, 102, 72, 255}
		edge = color.RGBA{244, 222, 128, 255}
		glow = color.RGBA{248, 218, 92, 205}
		belt = color.RGBA{238, 210, 104, 245}
		drawDisc(screen, float32(centerX), float32(centerY+scale*0.02), float32(scale*0.28), color.RGBA{238, 210, 104, 58})
	}
	phase := 0.0
	gearSpin := 0.0
	stamp := 0.0
	if !working {
		gearSpin = math.Pi / 8
	} else {
		phase = math.Sin(g.animationTime*8) * scale * 0.012
		gearSpin = g.animationTime * 6.8
		stamp = (0.5 + 0.5*math.Sin(g.animationTime*10)) * scale * 0.055
	}
	drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.29), color.RGBA{0, 0, 0, 82})
	drawFilledRect(screen, float32(centerX-scale*0.33), float32(centerY-scale*0.08), float32(scale*0.66), float32(scale*0.15), color.RGBA{38, 44, 46, 245})
	drawRoundedRect(screen, float32(centerX-scale*0.25), float32(centerY-scale*0.22), float32(scale*0.50), float32(scale*0.42), 5, body)
	drawRectOutline(screen, float32(centerX-scale*0.25), float32(centerY-scale*0.22), float32(scale*0.50), float32(scale*0.42), edge)
	drawAssemblerGearIcon(screen, centerX-scale*0.14, centerY-scale*0.08, scale*0.095, gearSpin, glow)
	drawAssemblerGearIcon(screen, centerX+scale*0.14, centerY-scale*0.08, scale*0.095, -gearSpin+math.Pi/7, glow)
	drawFilledRect(screen, float32(centerX-scale*0.28), float32(centerY-scale*0.02), float32(scale*0.56), float32(scale*0.05), belt)
	for i := -2; i <= 2; i++ {
		tickX := centerX + float64(i)*scale*0.11 + phase*2.2
		if tickX < centerX-scale*0.25 {
			tickX += scale * 0.55
		}
		if tickX > centerX+scale*0.25 {
			tickX -= scale * 0.55
		}
		drawFilledRect(screen, float32(tickX-scale*0.012), float32(centerY-scale*0.018), float32(scale*0.024), float32(scale*0.046), color.RGBA{248, 238, 178, 185})
	}
	drawDisc(screen, float32(centerX-scale*0.10+phase), float32(centerY-scale*0.005), float32(scale*0.058), glow)
	drawDisc(screen, float32(centerX+scale*0.10-phase), float32(centerY-scale*0.005), float32(scale*0.058), glow)
	drawRectOutline(screen, float32(centerX-scale*0.17), float32(centerY+scale*0.085), float32(scale*0.34), float32(scale*0.07), color.RGBA{238, 226, 170, 220})
	drawFilledRect(screen, float32(centerX-scale*0.045), float32(centerY-scale*0.31+stamp), float32(scale*0.09), float32(scale*0.12), color.RGBA{68, 76, 78, 255})
	drawFilledRect(screen, float32(centerX-scale*0.10), float32(centerY-scale*0.19+stamp), float32(scale*0.20), float32(scale*0.035), edge)
	if configured {
		drawAssemblerGearIcon(screen, centerX, centerY+scale*0.12, scale*0.045, gearSpin*0.8, core.ResourceColor(core.ResourceGear))
	}
}

func assemblerSpriteForTile(tile *core.TacticalTile, animationTime float64, working bool) *ebiten.Image {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindAssembler {
		return nil
	}
	frame := 0
	if _, ok := core.AssemblerOutputForConfig(tile.Device.ConfigInput); ok {
		frame = 1
	}
	if working {
		frame = 2 + int(animationTime*7)%2
	}
	if frame < 0 || frame >= len(assemblerSprites) {
		return nil
	}
	return assemblerSprites[frame]
}

func gateSprite() *ebiten.Image {
	return deviceSprites[0]
}

func generatorSprite() *ebiten.Image {
	return deviceSprites[5]
}

func smelterSpriteForTile(tile *core.TacticalTile, animationTime float64, working bool) *ebiten.Image {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindSmelter {
		return nil
	}
	frame := 1
	if working {
		frame = 1 + int(animationTime*7)%4
	}
	if frame < 1 || frame >= len(deviceSprites) {
		return nil
	}
	return deviceSprites[frame]
}

func (g *Game) drawMinerSprite(screen *ebiten.Image, tile *core.TacticalTile, centerX, centerY, scale float64) {
	sprite := minerSpriteForTile(tile, g.animationTime)
	if sprite == nil {
		drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.26), color.RGBA{0, 0, 0, 84})
		drawDisc(screen, float32(centerX), float32(centerY-1), float32(scale*0.22), tacticalDeviceSignalColor(tile))
		drawFilledRect(screen, float32(centerX-scale*0.05), float32(centerY-scale*0.02), float32(scale*0.10), float32(scale*0.27), color.RGBA{220, 178, 110, 255})
		drawFilledRect(screen, float32(centerX-scale*0.16), float32(centerY-scale*0.15), float32(scale*0.32), float32(scale*0.08), tacticalDeviceSignalColor(tile))
		return
	}

	targetHeight := scale * 1.18
	offsetX := scale * 0.12
	if tile != nil && tile.Device != nil && !tile.Device.SpecialStarter {
		targetHeight = scale * 1.10
		offsetX = scale * 0.03
	}
	if tile != nil && tile.Device != nil && tile.Device.SpecialStarter {
		drawDisc(screen, float32(centerX), float32(centerY+scale*0.10), float32(scale*0.32), color.RGBA{236, 204, 98, 92})
	}
	tint := color.RGBA{}
	if tile != nil && tile.Device != nil && tile.Device.SpecialStarter {
		tint = color.RGBA{255, 240, 194, 255}
	}
	drawCenteredSprite(screen, sprite, centerX-offsetX, centerY-1, targetHeight, scale*0.06, scale*0.09, 0.34, tint)
	if tile != nil && tile.Device != nil && !tile.Device.SpecialStarter {
		g.drawResourceIndicator(screen, centerX+scale*0.24, centerY-scale*0.28, scale, tile)
	}
}

func minerSpriteForTile(tile *core.TacticalTile, animationTime float64) *ebiten.Image {
	sprites := minerSprites[:]
	if tile != nil && tile.Device != nil && !tile.Device.SpecialStarter {
		sprites = autoMinerSprites[:]
	}
	frame := 0
	if minerWorking(tile) {
		frame = 1 + int(animationTime*10)%3
	}
	if frame < 0 || frame >= len(sprites) {
		return nil
	}
	return sprites[frame]
}

func minerWorking(tile *core.TacticalTile) bool {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindMiner {
		return false
	}
	if tile.Resource == core.ResourceNone || tile.ResourceRemaining <= 0 {
		return false
	}
	runCost := core.DeviceDefinition(tile.Device.Kind).RunPowerCost
	return runCost > 0 && tile.PowerBuffer >= runCost
}

func (g *Game) smelterWorking(tile *core.TacticalTile) bool {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindSmelter {
		return false
	}
	runCost := core.DeviceDefinition(tile.Device.Kind).RunPowerCost
	if runCost <= 0 || tile.PowerBuffer < runCost {
		return false
	}
	tmap := g.currentTacticalMap()
	if tmap == nil || !tmap.HasAdjacentDevice(tile, core.DeviceKindGate) {
		return false
	}
	input := tile.Device.ConfigInput
	if _, ok := core.SmelterOutputForInput(input); !ok {
		return false
	}
	return tmap.Supply != nil && tmap.Supply[input] > 0 && tmap.Supply[core.ResourceCoal] > 0
}

func (g *Game) generatorWorking(tile *core.TacticalTile) bool {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindGenerator {
		return false
	}
	tmap := g.currentTacticalMap()
	if tile.Device.ConfigMode == core.DeviceModeSolar {
		return tmap != nil && g.solarRetrofitUnlocked()
	}
	return tmap != nil && tmap.Supply[core.ResourceCoal] > 0
}

func (g *Game) solarRetrofitUnlocked() bool {
	return g.knownRecipes != nil && g.knownRecipes["solar-retrofit"]
}

func (g *Game) assemblerWorking(tile *core.TacticalTile) bool {
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindAssembler {
		return false
	}
	if _, ok := core.AssemblerOutputForConfig(tile.Device.ConfigInput); !ok {
		return false
	}
	runCost := core.DeviceDefinition(tile.Device.Kind).RunPowerCost
	if runCost <= 0 || tile.PowerBuffer < runCost {
		return false
	}
	tmap := g.currentTacticalMap()
	return tmap != nil &&
		tmap.HasAdjacentSmelterOutput(tile, core.ResourceIronIngot) &&
		tmap.HasAdjacentSmelterOutput(tile, core.ResourceCopperIngot) &&
		tmap.Supply[core.ResourceIronIngot] >= 2 &&
		tmap.Supply[core.ResourceCopperIngot] > 0
}

func (g *Game) gateExportUnlocked() bool {
	tmap := g.currentTacticalMap()
	return tmap != nil && tmap.HasDevice(core.DeviceKindGate)
}

func (g *Game) currentRegionHasDepositableSupply() bool {
	if !g.gateUplinkUnlocked {
		return false
	}
	tmap := g.currentTacticalMap()
	if tmap == nil || !tmap.HasDevice(core.DeviceKindGate) || len(tmap.Supply) == 0 {
		return false
	}
	for resource, amount := range tmap.Supply {
		if globalTransferResource(resource) && amount > 0 {
			return true
		}
	}
	return false
}

func (g *Game) exportCurrentRegionToGlobal() bool {
	if !g.gateExportUnlocked() {
		return false
	}
	tmap := g.currentTacticalMap()
	if tmap == nil || !tmap.HasDevice(core.DeviceKindGate) || len(tmap.Supply) == 0 {
		return false
	}
	if g.inventory == nil {
		g.inventory = map[core.ResourceType]int{}
	}
	exported := false
	for resource, amount := range tmap.Supply {
		if !globalTransferResource(resource) || amount <= 0 {
			continue
		}
		g.inventory[resource] += amount
		tmap.Supply[resource] = 0
		exported = true
	}
	if exported {
		g.saveSoon()
	}
	return exported
}

func globalTransferResource(resource core.ResourceType) bool {
	switch resource {
	case core.ResourceNone, core.ResourceCoal:
		return false
	default:
		return true
	}
}

func (g *Game) depositCurrentRegionToGlobal() bool {
	if !g.gateUplinkUnlocked {
		return false
	}
	if !g.currentRegionHasDepositableSupply() {
		return false
	}
	g.spawnUplinkPackets()
	if !g.exportCurrentRegionToGlobal() {
		return false
	}
	return true
}

func (g *Game) spawnUplinkPackets() {
	tmap := g.currentTacticalMap()
	if tmap == nil || len(tmap.Supply) == 0 {
		return
	}
	gate := tacticalGateTile(tmap)
	if gate == nil {
		return
	}
	cx, cy := g.tacticalCenter()
	scale := g.tacticalTileScale()
	x := cx + g.tacticalPanX + gate.Center.X*scale
	y := cy + g.tacticalPanY + gate.Center.Y*scale - scale*0.18
	resources := make([]core.ResourceType, 0, len(tmap.Supply))
	totalItems := 0
	for resource, amount := range tmap.Supply {
		if globalTransferResource(resource) && amount > 0 {
			resources = append(resources, resource)
			totalItems += amount
		}
	}
	if totalItems <= 0 {
		return
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i] < resources[j] })
	const totalDuration = 1.45
	travelDuration := clampRange(0.98-float64(totalItems)*0.012, 0.42, 0.98)
	emitWindow := math.Max(0, totalDuration-travelDuration)
	delayStep := 0.0
	if totalItems > 1 {
		delayStep = emitWindow / float64(totalItems-1)
	}
	packetIndex := 0
	for _, resource := range resources {
		amount := tmap.Supply[resource]
		for i := 0; i < amount; i++ {
			lane := float64((packetIndex%5)-2) * 4
			g.uplinkPackets = append(g.uplinkPackets, uplinkPacket{
				x:        x,
				y:        y,
				offsetX:  lane,
				delay:    float64(packetIndex) * delayStep,
				timer:    travelDuration,
				duration: travelDuration,
				resource: resource,
			})
			packetIndex++
		}
	}
}

func tacticalGateTile(tmap *core.TacticalMap) *core.TacticalTile {
	if tmap == nil {
		return nil
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		if tile.Device != nil && tile.Device.Kind == core.DeviceKindGate {
			return tile
		}
	}
	return nil
}

func (g *Game) advanceUplinkPackets(dt float64) {
	if len(g.uplinkPackets) == 0 {
		return
	}
	out := g.uplinkPackets[:0]
	for _, packet := range g.uplinkPackets {
		if packet.delay > 0 {
			packet.delay -= dt
			out = append(out, packet)
			continue
		}
		packet.timer -= dt
		if packet.timer > 0 {
			out = append(out, packet)
		}
	}
	g.uplinkPackets = out
}

func (g *Game) drawUplinkPackets(screen *ebiten.Image) {
	for _, packet := range g.uplinkPackets {
		if packet.delay > 0 {
			continue
		}
		g.drawUplinkPacket(screen, packet)
	}
}

func (g *Game) drawUplinkPacket(screen *ebiten.Image, packet uplinkPacket) {
	duration := packet.duration
	if duration <= 0 {
		duration = 0.8
	}
	progress := clampRange(1-packet.timer/duration, 0, 1)
	fade := clampRange(packet.timer/0.22, 0, 1)
	ease := progress * progress * (3 - 2*progress)
	startY := packet.y
	endY := -24.0
	px := packet.x + packet.offsetX
	py := startY + (endY-startY)*ease
	col := core.ResourceColor(packet.resource)
	col.A = uint8(72 * fade)
	vector.StrokeLine(screen, float32(px), float32(startY), float32(px), float32(py), 2, col, false)
	g.drawUplinkItemIcon(screen, px, py, packet.resource, fade)
}

func (g *Game) drawUplinkItemIcon(screen *ebiten.Image, x, y float64, resource core.ResourceType, alpha float64) {
	if resource == core.ResourceFieldData {
		g.drawFloppyIcon(screen, x, y, 14, alpha)
		return
	}
	if resource != core.ResourceStone {
		if sprite := tacticalResourceSprite(resource); sprite != nil {
			drawCenteredSprite(screen, sprite, x, y, 14, 1, 1, 0.18*alpha, color.RGBA{})
			return
		}
	}
	col := core.ResourceColor(resource)
	col.A = uint8(230 * alpha)
	switch resource {
	case core.ResourceStone:
		points := []screenPoint{
			{x: x, y: y - 5},
			{x: x + 5, y: y - 1},
			{x: x + 3, y: y + 5},
			{x: x - 4, y: y + 4},
			{x: x - 6, y: y - 2},
		}
		drawScreenPolygon(screen, points, col)
	case core.ResourceIronIngot, core.ResourceCopperIngot:
		drawRoundedRect(screen, float32(x-6), float32(y-3), 12, 6, 2, col)
	case core.ResourceGear:
		drawDisc(screen, float32(x), float32(y), 5, col)
		drawDisc(screen, float32(x), float32(y), 2, color.RGBA{10, 15, 24, uint8(255 * alpha)})
	default:
		drawDisc(screen, float32(x), float32(y), 4, col)
	}
}

const fieldDataUnitsPerSample = 3.0

func fieldDataMiningSnapshot(tmap *core.TacticalMap) map[int]float64 {
	if tmap == nil {
		return nil
	}
	before := map[int]float64{}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		if tile.Device == nil || tile.Device.Kind != core.DeviceKindMiner || tile.Resource == core.ResourceNone {
			continue
		}
		before[tile.ID] = tile.ResourceRemaining
	}
	return before
}

func (g *Game) collectFieldDataFromMining(tmap *core.TacticalMap, before map[int]float64) {
	if tmap == nil || len(before) == 0 || !tmap.HasDevice(core.DeviceKindGate) {
		return
	}
	if g.fieldDataScanCarry == nil {
		g.fieldDataScanCarry = map[fieldDataTileKey]float64{}
	}
	for i := range tmap.Tiles {
		tile := &tmap.Tiles[i]
		prev, ok := before[tile.ID]
		if !ok || tile.Resource == core.ResourceNone {
			continue
		}
		mined := prev - tile.ResourceRemaining
		if mined <= 0 {
			continue
		}
		key := fieldDataTileKey{cellID: tmap.CellID, tileID: tile.ID}
		g.fieldDataScanCarry[key] += mined
		award := int(g.fieldDataScanCarry[key] / fieldDataUnitsPerSample)
		if award <= 0 {
			continue
		}
		g.fieldDataScanCarry[key] -= float64(award) * fieldDataUnitsPerSample
		if tmap.Supply == nil {
			tmap.Supply = map[core.ResourceType]int{}
		}
		tmap.Supply[core.ResourceFieldData] += award
		if g.minedTotals == nil {
			g.minedTotals = map[core.ResourceType]int{}
		}
		g.minedTotals[core.ResourceFieldData] += award
		if tmap.CellID == g.tacticalID {
			g.spawnFieldDataPopup(tmap, tile, award)
		}
	}
}

func (g *Game) spawnFieldDataPopup(tmap *core.TacticalMap, tile *core.TacticalTile, amount int) {
	if tmap == nil || tile == nil || amount <= 0 {
		return
	}
	cx, cy := g.tacticalCenter()
	scale := g.tacticalTileScale()
	x := cx + g.tacticalPanX + tile.Center.X*scale
	y := cy + g.tacticalPanY + tile.Center.Y*scale - scale*0.16
	g.fieldDataPopups = append(g.fieldDataPopups, fieldDataPopup{x: x, y: y, amount: amount, timer: 1.25})
}

func (g *Game) advanceFieldDataPopups(dt float64) {
	if len(g.fieldDataPopups) == 0 {
		return
	}
	out := g.fieldDataPopups[:0]
	for _, popup := range g.fieldDataPopups {
		popup.timer -= dt
		if popup.timer > 0 {
			out = append(out, popup)
		}
	}
	g.fieldDataPopups = out
}

func (g *Game) drawFieldDataPopups(screen *ebiten.Image) {
	for _, popup := range g.fieldDataPopups {
		g.drawFieldDataPopup(screen, popup)
	}
}

func (g *Game) drawFieldDataPopup(screen *ebiten.Image, popup fieldDataPopup) {
	const duration = 1.25
	progress := clampRange(1-popup.timer/duration, 0, 1)
	fade := clampRange(popup.timer/0.48, 0, 1)
	lift := 36 * progress
	cx := popup.x
	cy := popup.y - lift
	ease := 1 - math.Pow(1-progress, 3)

	for i := 0; i < 10; i++ {
		angle := float64(i)/10*math.Pi*2 + g.animationTime*0.35
		inner := 10.0
		outer := 18 + 24*ease
		points := []screenPoint{
			{x: cx + math.Cos(angle-0.08)*inner, y: cy + math.Sin(angle-0.08)*inner},
			{x: cx + math.Cos(angle+0.08)*inner, y: cy + math.Sin(angle+0.08)*inner},
			{x: cx + math.Cos(angle+0.025)*outer, y: cy + math.Sin(angle+0.025)*outer},
			{x: cx + math.Cos(angle-0.025)*outer, y: cy + math.Sin(angle-0.025)*outer},
		}
		drawScreenPolygon(screen, points, color.RGBA{76, 230, 190, uint8(74 * fade)})
	}

	size := 24.0 + 7*math.Sin(progress*math.Pi)
	g.drawFloppyIcon(screen, cx, cy, size, fade)
	label := fmt.Sprintf("+%d data", popup.amount)
	g.drawTintedDebugTextBlock(screen, cx-26, cy+size*0.62, []string{label}, fade, 0.78, 1, 0.86)
}

func (g *Game) drawFloppyIcon(screen *ebiten.Image, cx, cy, size, alpha float64) {
	a := uint8(245 * alpha)
	x := cx - size*0.5
	y := cy - size*0.5
	drawRoundedRect(screen, float32(x+2), float32(y+3), float32(size), float32(size), 4, color.RGBA{0, 0, 0, uint8(90 * alpha)})
	drawRoundedRect(screen, float32(x), float32(y), float32(size), float32(size), 4, color.RGBA{48, 220, 158, a})
	drawRectOutline(screen, float32(x), float32(y), float32(size), float32(size), color.RGBA{212, 255, 230, a})
	drawFilledRect(screen, float32(x+size*0.18), float32(y+size*0.12), float32(size*0.50), float32(size*0.24), color.RGBA{18, 54, 58, a})
	drawFilledRect(screen, float32(x+size*0.56), float32(y+size*0.16), float32(size*0.10), float32(size*0.14), color.RGBA{232, 255, 236, a})
	drawFilledRect(screen, float32(x+size*0.22), float32(y+size*0.58), float32(size*0.56), float32(size*0.22), color.RGBA{220, 246, 236, a})
}

func tacticalResourceSprite(resource core.ResourceType) *ebiten.Image {
	switch resource {
	case core.ResourceIronOre:
		return resourceSprites[1]
	case core.ResourceCopperOre:
		return resourceSprites[2]
	case core.ResourceCoal:
		return resourceSprites[3]
	default:
		return nil
	}
}

func (g *Game) drawResourceIndicator(screen *ebiten.Image, x, y, scale float64, tile *core.TacticalTile) {
	if tile == nil || tile.ResourceRemaining <= 0 {
		return
	}
	sprite := tacticalResourceSprite(tile.Resource)
	if sprite == nil {
		return
	}

	clr := resourceIndicatorColor(tile)
	drawDisc(screen, float32(x+scale*0.01), float32(y+scale*0.02), float32(scale*0.18), color.RGBA{0, 0, 0, 88})
	drawDisc(screen, float32(x), float32(y), float32(scale*0.16), clr)
	drawCenteredSprite(screen, sprite, x, y, scale*0.34, scale*0.01, scale*0.01, 0.12, color.RGBA{})
}

func resourceIndicatorColor(tile *core.TacticalTile) color.RGBA {
	if tile == nil {
		return color.RGBA{92, 210, 118, 255}
	}
	capacity := core.ResourceCapacity(tile.Resource, tile.ResourceRichness)
	if capacity <= 0 {
		return color.RGBA{92, 210, 118, 255}
	}
	if tile.ResourceRemaining <= capacity*0.5 {
		return color.RGBA{234, 200, 82, 255}
	}
	return color.RGBA{92, 210, 118, 255}
}

func tacticalEntitySprite(entity core.TacticalEntity, animationTime float64) *ebiten.Image {
	frame := int(animationTime*7+float64(entity.ID)*0.7) % 4
	if frame < 0 {
		frame = 0
	}
	switch entity.Kind {
	case core.TacticalEntityPurple:
		return purpleCavemanSprites[frame]
	case core.TacticalEntityRed:
		return dangerOrganismSprites[frame]
	case core.TacticalEntityGreen:
		return friendlyOrganismSprites[frame]
	default:
		if entityDangerous(entity) {
			return dangerOrganismSprites[frame]
		}
		return friendlyOrganismSprites[frame]
	}
}

func entityDangerous(entity core.TacticalEntity) bool {
	return entity.Fill.R > entity.Fill.G
}

func tacticalTextureForTile(tile *core.TacticalTile) *ebiten.Image {
	if tile == nil {
		return nil
	}
	switch tacticalTextureKind(tile) {
	case 0:
		return tacticalTextures[0]
	case 1:
		return tacticalTextures[1]
	case 2:
		return tacticalTextures[2]
	case 3:
		return tacticalTextures[3]
	default:
		return tacticalTextures[4]
	}
}

func tacticalTextureKind(tile *core.TacticalTile) int {
	if tile == nil {
		return 4
	}
	// Older saves won't have Ocean persisted; fall back to the blue-water fill.
	if tile.Ocean || int(tile.Fill.B) > int(tile.Fill.G)+20 {
		return 0
	}
	switch {
	case tile.Elevation > 0.82:
		return 1
	case tile.Moisture < 0.28:
		return 2
	case tile.Moisture > 0.66:
		return 3
	default:
		return 4
	}
}

func tacticalTextureAlpha(tile *core.TacticalTile) float32 {
	if tile == nil {
		return 0.24
	}
	if tacticalTextureKind(tile) == 0 {
		return 0.30
	}
	return 0.24
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
	if powerBuffer <= 0.02 {
		return
	}
	sprite := powerIndicatorSprite(powerBuffer, runCost)
	if sprite == nil {
		clr := powerIndicatorColor(powerBuffer, runCost)
		g.drawLightningBolt(screen, x+scale*0.02, y+scale*0.02, scale*0.36, color.RGBA{0, 0, 0, 88})
		g.drawLightningBolt(screen, x, y, scale*0.36, clr)
		return
	}

	drawPowerIndicatorSprite(screen, sprite, x, y, scale)
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

func (g *Game) drawLightningBolt(screen *ebiten.Image, x, y, size float64, clr color.RGBA) {
	bolt := []screenPoint{
		{x: x - size*0.30, y: y - size*0.62},
		{x: x + size*0.04, y: y - size*0.18},
		{x: x - size*0.10, y: y - size*0.18},
		{x: x + size*0.28, y: y + size*0.62},
		{x: x - size*0.02, y: y + size*0.10},
		{x: x + size*0.12, y: y + size*0.10},
	}
	drawScreenPolygon(screen, bolt, clr)
}

func powerIndicatorSprite(powerBuffer, runCost float64) *ebiten.Image {
	switch {
	case powerBuffer <= 0.02:
		return powerIndicatorSprites[0]
	case runCost <= 0 || powerBuffer >= runCost:
		return powerIndicatorSprites[2]
	default:
		return powerIndicatorSprites[1]
	}
}

func drawPowerIndicatorSprite(screen, sprite *ebiten.Image, x, y, scale float64) {
	bounds := sprite.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return
	}

	targetHeight := scale * 0.84
	scaleFactor := targetHeight / float64(bounds.Dy())
	targetWidth := float64(bounds.Dx()) * scaleFactor
	drawX := x - targetWidth/2
	drawY := y - targetHeight/2

	shadow := &ebiten.DrawImageOptions{}
	shadow.GeoM.Scale(scaleFactor, scaleFactor)
	shadow.GeoM.Translate(drawX+scale*0.03, drawY+scale*0.04)
	shadow.ColorScale.Scale(0, 0, 0, 0.35)
	screen.DrawImage(sprite, shadow)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleFactor, scaleFactor)
	op.GeoM.Translate(drawX, drawY)
	screen.DrawImage(sprite, op)
}

func drawCenteredSprite(screen, sprite *ebiten.Image, centerX, centerY, targetHeight, shadowDX, shadowDY, shadowAlpha float64, tint color.RGBA) {
	if sprite == nil {
		return
	}
	bounds := sprite.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 || targetHeight <= 0 {
		return
	}

	scaleFactor := targetHeight / float64(bounds.Dy())
	targetWidth := float64(bounds.Dx()) * scaleFactor
	drawX := centerX - targetWidth/2
	drawY := centerY - targetHeight/2

	shadow := &ebiten.DrawImageOptions{}
	shadow.GeoM.Scale(scaleFactor, scaleFactor)
	shadow.GeoM.Translate(drawX+shadowDX, drawY+shadowDY)
	shadow.ColorScale.Scale(0, 0, 0, float32(shadowAlpha))
	screen.DrawImage(sprite, shadow)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleFactor, scaleFactor)
	op.GeoM.Translate(drawX, drawY)
	if tint.A > 0 {
		op.ColorScale.Scale(float32(tint.R)/255, float32(tint.G)/255, float32(tint.B)/255, float32(tint.A)/255)
	}
	screen.DrawImage(sprite, op)
}

func initPowerIndicatorSprites() {
	img, err := png.Decode(bytes.NewReader(powerIndicatorSpriteSheetPNG))
	if err != nil {
		log.Printf("power indicator sprite decode failed: %v", err)
		return
	}

	cleaned := removeConnectedNeutralBackground(img)
	bounds := cleaned.Bounds()
	cellWidth := bounds.Dx() / 3
	cellHeight := bounds.Dy() / 2
	if cellWidth <= 0 || cellHeight <= 0 {
		log.Printf("power indicator sprite sheet has invalid bounds: %v", bounds)
		return
	}

	for i := 0; i < 3; i++ {
		cellRect := image.Rect(i*cellWidth, 0, (i+1)*cellWidth, cellHeight)
		cell := image.NewNRGBA(image.Rect(0, 0, cellRect.Dx(), cellRect.Dy()))
		draw.Draw(cell, cell.Bounds(), cleaned, cellRect.Min, draw.Src)
		trimmed := trimAlphaImage(cell)
		if trimmed == nil {
			continue
		}
		powerIndicatorSprites[i] = ebiten.NewImageFromImage(trimmed)
	}
}

func initMinerSprites() {
	img, err := png.Decode(bytes.NewReader(minerSpriteSheetPNG))
	if err != nil {
		log.Printf("miner sprite decode failed: %v", err)
		return
	}

	bounds := img.Bounds()
	cellWidth := bounds.Dx() / 4
	cellHeight := bounds.Dy()
	if cellWidth <= 0 || cellHeight <= 0 {
		log.Printf("miner sprite sheet has invalid bounds: %v", bounds)
		return
	}

	for i := 0; i < 4; i++ {
		cellRect := image.Rect(i*cellWidth, 0, (i+1)*cellWidth, cellHeight)
		cell := image.NewNRGBA(image.Rect(0, 0, cellRect.Dx(), cellRect.Dy()))
		draw.Draw(cell, cell.Bounds(), img, cellRect.Min, draw.Src)
		trimmed := trimAlphaImage(cell)
		if trimmed == nil {
			continue
		}
		minerSprites[i] = ebiten.NewImageFromImage(trimmed)
	}
}

func initAutoMinerSprites() {
	img, err := png.Decode(bytes.NewReader(autoMinerSpriteSheetPNG))
	if err != nil {
		log.Printf("auto miner sprite decode failed: %v", err)
		return
	}
	loadSpriteStripInto(img, autoMinerSprites[:], 4)
}

func initResourceSprites() {
	img, err := png.Decode(bytes.NewReader(resourceSpriteSheetPNG))
	if err != nil {
		log.Printf("resource sprite decode failed: %v", err)
		return
	}
	loadSpriteStripInto(img, resourceSprites[:], 4)
}

func initOrganismSprites() {
	friendlyImg, err := png.Decode(bytes.NewReader(friendlyOrganismSheetPNG))
	if err != nil {
		log.Printf("friendly organism sprite decode failed: %v", err)
	} else {
		loadSpriteStripInto(friendlyImg, friendlyOrganismSprites[:], 4)
	}

	dangerImg, err := png.Decode(bytes.NewReader(dangerOrganismSheetPNG))
	if err != nil {
		log.Printf("danger organism sprite decode failed: %v", err)
		return
	}
	loadSpriteStripInto(dangerImg, dangerOrganismSprites[:], 4)

	purpleImg, err := png.Decode(bytes.NewReader(purpleCavemanSheetPNG))
	if err != nil {
		log.Printf("purple caveman sprite decode failed: %v", err)
		return
	}
	loadSpriteStripInto(purpleImg, purpleCavemanSprites[:], 4)
}

func initDeviceSprites() {
	img, err := png.Decode(bytes.NewReader(deviceSpriteSheetPNG))
	if err != nil {
		log.Printf("device sprite sheet decode failed: %v", err)
		return
	}
	loadSpriteStripInto(img, deviceSprites[:], 6)
}

func initGateUplinkSprite() {
	img, err := png.Decode(bytes.NewReader(gateUplinkSpritePNG))
	if err != nil {
		log.Printf("gate uplink sprite decode failed: %v", err)
		return
	}
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, bounds.Min, draw.Src)
	trimmed := trimAlphaImage(nrgba)
	if trimmed == nil {
		return
	}
	gateUplinkSprite = ebiten.NewImageFromImage(trimmed)
}

func initAssemblerSprites() {
	img, err := png.Decode(bytes.NewReader(assemblerSpriteSheetPNG))
	if err != nil {
		log.Printf("assembler sprite decode failed: %v", err)
		return
	}
	loadSpriteStripInto(img, assemblerSprites[:], 4)
}

func initTacticalTextures() {
	img, err := png.Decode(bytes.NewReader(tacticalTextureAtlasPNG))
	if err != nil {
		log.Printf("tactical texture atlas decode failed: %v", err)
		return
	}
	loadSpriteStripInto(img, tacticalTextures[:], 5)
}

func loadSpriteStripInto(img image.Image, target []*ebiten.Image, count int) {
	if img == nil || count <= 0 || len(target) < count {
		return
	}
	bounds := img.Bounds()
	cellWidth := bounds.Dx() / count
	cellHeight := bounds.Dy()
	if cellWidth <= 0 || cellHeight <= 0 {
		log.Printf("sprite strip has invalid bounds: %v", bounds)
		return
	}

	for i := 0; i < count; i++ {
		cellRect := image.Rect(i*cellWidth, 0, (i+1)*cellWidth, cellHeight)
		cell := image.NewNRGBA(image.Rect(0, 0, cellRect.Dx(), cellRect.Dy()))
		draw.Draw(cell, cell.Bounds(), img, cellRect.Min, draw.Src)
		trimmed := trimAlphaImage(cell)
		if trimmed == nil {
			continue
		}
		target[i] = ebiten.NewImageFromImage(trimmed)
	}
}

func removeConnectedNeutralBackground(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, color.NRGBAModel.Convert(src.At(x, y)))
		}
	}

	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return dst
	}

	visited := make([]bool, width*height)
	queue := make([]image.Point, 0, width*2+height*2)
	push := func(x, y int) {
		idx := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
		if visited[idx] || !isNeutralSpriteBackground(dst.NRGBAAt(x, y)) {
			return
		}
		visited[idx] = true
		queue = append(queue, image.Point{X: x, Y: y})
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		push(x, bounds.Min.Y)
		push(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		push(bounds.Min.X, y)
		push(bounds.Max.X-1, y)
	}

	for head := 0; head < len(queue); head++ {
		pt := queue[head]
		pixel := dst.NRGBAAt(pt.X, pt.Y)
		pixel.A = 0
		dst.SetNRGBA(pt.X, pt.Y, pixel)

		for _, step := range [...]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			nx := pt.X + step.X
			ny := pt.Y + step.Y
			if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
				continue
			}
			push(nx, ny)
		}
	}

	return dst
}

func isNeutralSpriteBackground(px color.NRGBA) bool {
	maxCh := px.R
	minCh := px.R
	for _, ch := range [...]uint8{px.G, px.B} {
		if ch > maxCh {
			maxCh = ch
		}
		if ch < minCh {
			minCh = ch
		}
	}
	avg := (int(px.R) + int(px.G) + int(px.B)) / 3
	return maxCh-minCh <= 18 && avg >= 214
}

func trimAlphaImage(src *image.NRGBA) image.Image {
	bounds := src.Bounds()
	minX := bounds.Max.X
	minY := bounds.Max.Y
	maxX := bounds.Min.X
	maxY := bounds.Min.Y
	found := false

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if src.NRGBAAt(x, y).A == 0 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if !found {
		return nil
	}

	trimmedBounds := image.Rect(0, 0, maxX-minX, maxY-minY)
	trimmed := image.NewNRGBA(trimmedBounds)
	draw.Draw(trimmed, trimmedBounds, src, image.Point{X: minX, Y: minY}, draw.Src)
	return trimmed
}

func tacticalDeviceSignalColor(tile *core.TacticalTile) color.RGBA {
	if tile == nil {
		return color.RGBA{118, 186, 210, 255}
	}
	capacity := core.ResourceCapacity(tile.Resource, tile.ResourceRichness)
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

func (g *Game) drawTacticalDepositButton(screen *ebiten.Image) {
	x, y, w, h := g.depositButtonRect()
	available := g.currentRegionHasDepositableSupply()
	fill := color.RGBA{42, 72, 54, 236}
	border := color.RGBA{92, 144, 110, 255}
	text := color.RGBA{156, 186, 166, 255}
	label := "LOCKED"
	if available {
		label = "UPLINK"
		fill = color.RGBA{30, 184, 58, 236}
		border = color.RGBA{118, 230, 154, 255}
		text = color.RGBA{228, 246, 232, 255}
	} else if g.gateUplinkUnlocked {
		label = "UPLINK"
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	g.drawTintedDebugTextBlock(screen, x+14, y+12, []string{label}, 1, float32(text.R)/255, float32(text.G)/255, float32(text.B)/255)
}

func (g *Game) drawArrowBackButton(screen *ebiten.Image) {
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
	ebitenutil.DebugPrintAt(screen, "FAB", int(x)+32, int(y)+12)
}

func (g *Game) drawPendingBuildCard(screen *ebiten.Image) {
	if g.pendingBuildRecipeID == "" {
		return
	}
	title := g.buildRecipeTitle(g.pendingBuildRecipeID)
	lines := []string{
		"PLACE " + strings.ToUpper(title),
		"tap a green empty tile",
	}
	w := 210.0
	h := 56.0
	x := float64(g.screenWidth)*0.5 - w*0.5
	y := float64(g.screenHeight) - 128
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, 228})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{128, 226, 160, 240})
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, lines, 1)
}

func (g *Game) drawTacticalDisassembleButton(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return
	}
	if tile.Device.SpecialStarter && tile.Device.Kind == core.DeviceKindMiner {
		return
	}
	x, y, w, h := g.disassembleButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{112, 56, 52, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{236, 170, 160, 255})
	label := "SALVAGE"
	labelX := int(x) + 22
	if tile.Device.SpecialStarter {
		label = "RECOVER"
		labelX = int(x) + 22
	} else if tile.Device.Kind == core.DeviceKindSmelter || tile.Device.Kind == core.DeviceKindAssembler || (tile.Device.Kind == core.DeviceKindGenerator && g.solarRetrofitUnlocked()) {
		label = "TUNE"
		labelX = int(x) + 34
	}
	ebitenutil.DebugPrintAt(screen, label, labelX, int(y)+12)
}

func (g *Game) drawTacticalPlaceBuildButton(screen *ebiten.Image) {
	if g.pendingBuildRecipeID != "" {
		x, y, w, h := g.disassembleButtonRect()
		drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{92, 70, 42, 236})
		drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{236, 206, 150, 255})
		ebitenutil.DebugPrintAt(screen, "ABORT", int(x)+28, int(y)+12)
	}
}

func (g *Game) finishTacticalPointer(x, y int) {
	if g.mugDragActive {
		g.finishTacticalMUGDrag(x, y)
		return
	}
	if g.dragMoved {
		return
	}
	if g.handleInventoryButtonTap(x, y) {
		return
	}
	buttonX, buttonY, buttonW, buttonH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		if g.pendingBuildRecipeID != "" {
			g.pendingBuildRecipeID = ""
			return
		}
		g.shutdownTacticalPerkSystem()
		g.mode = modeStrategic
		return
	}
	depositX, depositY, depositW, depositH := g.depositButtonRect()
	if g.pointInRect(float64(x), float64(y), depositX, depositY, depositW, depositH) {
		if g.pendingBuildRecipeID != "" {
			g.pendingBuildRecipeID = ""
			return
		}
		g.depositCurrentRegionToGlobal()
		return
	}
	buildX, buildY, buildW, buildH := g.buildButtonRect()
	if g.pointInRect(float64(x), float64(y), buildX, buildY, buildW, buildH) {
		if g.pendingBuildRecipeID != "" {
			g.pendingBuildRecipeID = ""
			return
		}
		g.mode = modeBuild
		return
	}
	disX, disY, disW, disH := g.disassembleButtonRect()
	if g.pointInRect(float64(x), float64(y), disX, disY, disW, disH) {
		if g.pendingBuildRecipeID != "" {
			g.pendingBuildRecipeID = ""
			return
		}
		tile := g.currentTacticalTile()
		if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
			return
		}
		if tile != nil && tile.Device != nil && tile.Device.Kind == core.DeviceKindSmelter {
			g.configTileID = tile.ID
			g.mode = modeSmelterConfig
			return
		}
		if tile != nil && tile.Device != nil && tile.Device.Kind == core.DeviceKindAssembler {
			g.configTileID = tile.ID
			g.mode = modeAssemblerConfig
			return
		}
		if tile != nil && tile.Device != nil && tile.Device.Kind == core.DeviceKindGenerator && g.solarRetrofitUnlocked() {
			g.configTileID = tile.ID
			g.mode = modeGeneratorConfig
			return
		}
		if tile != nil && tile.Device != nil && tile.Device.SpecialStarter && tile.Device.Kind == core.DeviceKindMiner {
			return
		}
		g.disassembleCurrentTacticalDevice()
		return
	}
	if g.tacticalTile >= 0 {
		popX, popY, popW, popH := g.tacticalSelectionPopupRect()
		if g.pointInRect(float64(x), float64(y), popX, popY, popW, popH) {
			return
		}
	}
	if tileID, ok := g.pickTacticalTile(x, y); ok {
		if g.pendingBuildRecipeID != "" {
			g.tacticalTile = tileID
			if g.placeRecipeOnTile(g.pendingBuildRecipeID, tileID) {
				g.pendingBuildRecipeID = ""
				g.saveSoon()
			}
			return
		}
		now := time.Now()
		if tileID == g.tacticalTile && tileID == g.lastTapTacticalTileID && now.Sub(g.lastTapTacticalTime) <= doubleTapWindow {
			g.lastTapTacticalTileID = -1
			g.enterDish(tileID)
			return
		}
		if g.tryCrankTacticalDevice(tileID) {
			g.tacticalTile = tileID
			g.lastTapTacticalTileID = tileID
			g.lastTapTacticalTime = now
			return
		}
		g.tacticalTile = tileID
		g.lastTapTacticalTileID = tileID
		g.lastTapTacticalTime = now
		return
	}
	if g.pendingBuildRecipeID == "" {
		g.tacticalTile = -1
		g.lastTapTacticalTileID = -1
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
	hasCrank := layout.Kind == core.DeviceKindMiner
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

func (g *Game) tryHoldPowerStarterMiner(x, y int) bool {
	if g.dragMoved || g.mugDragActive || !g.hasActivePerkKind(core.PerkHoldPower) {
		return false
	}
	if absInt(x-g.dragStartX) > dragThreshold || absInt(y-g.dragStartY) > dragThreshold {
		return false
	}
	startTileID, ok := g.pickTacticalTile(g.dragStartX, g.dragStartY)
	if !ok {
		return false
	}
	currentTileID, ok := g.pickTacticalTile(x, y)
	if !ok || currentTileID != startTileID {
		return false
	}
	tmap := g.currentTacticalMap()
	if tmap == nil || startTileID < 0 || startTileID >= len(tmap.Tiles) {
		return false
	}
	tile := &tmap.Tiles[startTileID]
	if tile.Device == nil || tile.Device.Kind != core.DeviceKindMiner || !tile.Device.SpecialStarter {
		return false
	}
	tile.PowerBuffer = math.Min(1, tile.PowerBuffer+starterHoldPower*g.crankPowerBoost())
	g.tacticalTile = startTileID
	return true
}

func (g *Game) beginTacticalMUGDrag(x, y int) bool {
	g.mugDragActive = false
	g.mugDragStartTile = -1
	g.mugDragTargetTile = -1
	startID, ok := g.pickTacticalTile(x, y)
	if !ok || !g.tileHasStarterMUG(startID) {
		return false
	}
	g.mugDragStartTile = startID
	return true
}

func (g *Game) maybeArmTacticalMUGDrag(x, y int) bool {
	if g.mugDragActive {
		return true
	}
	if g.dragMoved || g.mugDragStartTile < 0 {
		return false
	}
	if absInt(x-g.dragStartX) <= dragThreshold && absInt(y-g.dragStartY) <= dragThreshold {
		return false
	}
	startID := g.mugDragStartTile
	if !g.tileHasStarterMUG(startID) {
		return false
	}
	targetID, ok := g.pickTacticalTile(x, y)
	if !ok || targetID == startID {
		return true
	}
	g.mugDragActive = true
	g.mugDragStartTile = startID
	g.mugDragTargetTile = targetID
	g.tacticalTile = targetID
	return true
}

func (g *Game) finishTacticalMUGDrag(x, y int) {
	start := g.mugDragStartTile
	target := g.mugDragTargetTile
	if picked, ok := g.pickTacticalTile(x, y); ok {
		target = picked
	}
	g.mugDragActive = false
	g.mugDragStartTile = -1
	g.mugDragTargetTile = -1
	if !g.dragMoved || start < 0 || target < 0 || start == target {
		g.tacticalTile = start
		return
	}
	g.queueMUGMove(start, target)
}

func (g *Game) tileHasStarterMUG(tileID int) bool {
	tmap := g.currentTacticalMap()
	if tmap == nil || tileID < 0 || tileID >= len(tmap.Tiles) {
		return false
	}
	tile := &tmap.Tiles[tileID]
	return tile.Device != nil && tile.Device.Kind == core.DeviceKindMiner && tile.Device.SpecialStarter
}

func (g *Game) queueMUGMove(startID, targetID int) bool {
	tmap := g.currentTacticalMap()
	if tmap == nil || !g.tileHasStarterMUG(startID) || targetID < 0 || targetID >= len(tmap.Tiles) {
		return false
	}
	target := &tmap.Tiles[targetID]
	if target.Device != nil && target.Device.Kind != core.DeviceKindNone {
		return false
	}
	path := tmap.TilePath(startID, targetID)
	if len(path) < 2 {
		return false
	}
	if g.mugMoves == nil {
		g.mugMoves = map[int]*mugMoveState{}
	}
	g.mugMoves[tmap.CellID] = &mugMoveState{path: path}
	g.tacticalTile = targetID
	return true
}

func (g *Game) advanceTacticalDeviceMotion(dt float64) {
	for _, tmap := range g.tacticalMaps {
		for i := range tmap.Tiles {
			tile := &tmap.Tiles[i]
			if tile.Device != nil && tile.Device.DeployTimer > 0 {
				tile.Device.DeployTimer = math.Max(0, tile.Device.DeployTimer-dt)
			}
		}
	}
	if len(g.mugMoves) == 0 {
		return
	}
	for cellID, move := range g.mugMoves {
		tmap := g.tacticalMaps[cellID]
		if tmap == nil || move == nil || len(move.path) < 2 {
			delete(g.mugMoves, cellID)
			continue
		}
		currentID := move.path[0]
		nextID := move.path[1]
		if !tacticalTileInRange(tmap, currentID) || !tacticalTileInRange(tmap, nextID) {
			delete(g.mugMoves, cellID)
			continue
		}
		current := &tmap.Tiles[currentID]
		next := &tmap.Tiles[nextID]
		if current.Device == nil || current.Device.Kind != core.DeviceKindMiner || !current.Device.SpecialStarter {
			delete(g.mugMoves, cellID)
			continue
		}
		if next.Device != nil && next.Device.Kind != core.DeviceKindNone {
			delete(g.mugMoves, cellID)
			continue
		}
		if current.PowerBuffer < mugMoveStepPower {
			continue
		}
		move.progress += dt * mugMoveTilesPerSec
		if move.progress < 1 {
			continue
		}
		move.progress = 0
		device := current.Device
		power := math.Max(0, current.PowerBuffer-mugMoveStepPower)
		current.Device = core.NewDeviceLayout(device.Width, device.Height)
		current.PowerBuffer = 0
		next.Device = device
		next.PowerBuffer = power
		move.path = move.path[1:]
		if g.tacticalID == cellID {
			g.tacticalTile = nextID
		}
		if len(move.path) < 2 {
			delete(g.mugMoves, cellID)
		}
	}
}

func tacticalTileInRange(tmap *core.TacticalMap, tileID int) bool {
	return tmap != nil && tileID >= 0 && tileID < len(tmap.Tiles)
}

func (g *Game) disassembleCurrentTacticalDevice() bool {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind == core.DeviceKindNone {
		return false
	}
	if tile.Device.SpecialStarter {
		switch tile.Device.Kind {
		case core.DeviceKindMiner:
			g.starterMinerCount++
			g.starterMinerRecovered++
		case core.DeviceKindGate:
			g.starterGateCount++
		}
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
	if g.refundRecordedDeviceCost(layout) {
		return
	}
	if g.refundDeviceRecipeCost(layout.Kind) {
		return
	}
	for _, part := range layout.Parts {
		if part == core.DevicePartEmpty {
			continue
		}
		g.partInventory[part]++
	}
}

func (g *Game) refundRecordedDeviceCost(layout *core.DeviceLayout) bool {
	if layout == nil || (len(layout.RefundResources) == 0 && len(layout.RefundParts) == 0) {
		return false
	}
	resources := g.buildResourceInventory()
	for resource, amount := range layout.RefundResources {
		if amount > 0 {
			resources[resource] += amount
		}
	}
	for part, amount := range layout.RefundParts {
		if amount > 0 {
			g.partInventory[part] += amount
		}
	}
	return true
}

func (g *Game) refundDeviceRecipeCost(kind core.DeviceKind) bool {
	if kind == core.DeviceKindNone {
		return false
	}
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	for _, recipe := range g.recipes.Recipes {
		if recipe.Kind != core.RecipeDevice || recipe.Device != kind {
			continue
		}
		for resource, amount := range g.recipes.RawCost(recipe.ID) {
			if amount > 0 {
				g.buildResourceInventory()[resource] += amount
			}
		}
		return true
	}
	return false
}

func copyResourceCounts(in map[core.ResourceType]int) map[core.ResourceType]int {
	out := make(map[core.ResourceType]int, len(in))
	for resource, amount := range in {
		if amount > 0 {
			out[resource] = amount
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyPartCounts(in map[core.DevicePart]int) map[core.DevicePart]int {
	out := make(map[core.DevicePart]int, len(in))
	for part, amount := range in {
		if amount > 0 {
			out[part] = amount
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func (g *Game) pickDishCell(x, y int) (int, bool) {
	cells := g.currentDishCells()
	if len(cells) == 0 {
		return -1, false
	}
	cx, cy := g.dishCenter()
	scale := g.dishCellScale()
	p := screenPoint{x: float64(x), y: float64(y)}
	for _, cell := range cells {
		cellX := cx + cell.Center.X*scale
		cellY := cy + cell.Center.Y*scale
		if math.Hypot(p.x-cellX, p.y-cellY) > scale*0.64 {
			continue
		}
		if pointInPolygon(p, dishHexPoints(cellX, cellY, scale*0.52)) {
			return cell.ID, true
		}
	}
	return -1, false
}

func (g *Game) drawTacticalEntities(screen *ebiten.Image, tmap *core.TacticalMap, cx, cy, tileScale float64) {
	for _, entity := range tmap.Entities {
		if entity.TileID < 0 || entity.TileID >= len(tmap.Tiles) {
			continue
		}
		tile := &tmap.Tiles[entity.TileID]
		if tile.Device != nil && tile.Device.Kind != core.DeviceKindNone {
			continue
		}
		center := tile.Center
		if entity.MoveProgress > 0 && entity.MoveProgress < 1 && entity.MoveFromTileID >= 0 && entity.MoveFromTileID < len(tmap.Tiles) {
			from := tmap.Tiles[entity.MoveFromTileID].Center
			t := entity.MoveProgress
			t = t * t * (3 - 2*t)
			center = from.Mul(1 - t).Add(tile.Center.Mul(t))
		}
		centerX := cx + g.tacticalPanX + center.X*tileScale
		centerY := cy + g.tacticalPanY + center.Y*tileScale
		if sprite := tacticalEntitySprite(entity, g.animationTime); sprite != nil {
			drawCenteredSprite(screen, sprite, centerX, centerY, tileScale*0.44, tileScale*0.03, tileScale*0.05, 0.34, color.RGBA{})
			continue
		}
		drawDisc(screen, float32(centerX+tileScale*0.05), float32(centerY+tileScale*0.07), float32(tileScale*0.13), color.RGBA{0, 0, 0, 70})
		drawDisc(screen, float32(centerX), float32(centerY), float32(tileScale*0.12), entity.Fill)
	}
}

func (g *Game) enterTactical() {
	if g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		return
	}
	g.tacticalID = g.globe.SelectedCell
	g.tacticalMapForCell(g.tacticalID)
	g.tacticalTile = -1
	g.dishTileID = -1
	g.dishCells = nil
	g.lastTapTacticalTileID = -1
	g.tacticalZoom = 1
	g.tacticalPanX = 0
	g.tacticalPanY = 0
	g.resetTacticalPerkRun()
	g.mode = modeTactical
}

func (g *Game) enterDish(tileID int) {
	tmap := g.currentTacticalMap()
	if tmap == nil || tileID < 0 || tileID >= len(tmap.Tiles) {
		return
	}
	g.tacticalTile = tileID
	g.dishTileID = tileID
	g.dishCells = buildDishCells(g.tacticalID, tileID)
	g.mode = modeDish
}

func (g *Game) resetTacticalPerkRun() {
	g.activePerks = nil
	g.stagePowerSpent = map[string]float64{}
	g.perksAwarded = map[string]int{}
	g.perkChoice = nil
	g.perkOfferCooldown = 0
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

func (g *Game) currentDishCells() []dishCell {
	if g.dishTileID < 0 {
		return nil
	}
	if len(g.dishCells) == 0 {
		g.dishCells = buildDishCells(g.tacticalID, g.dishTileID)
	}
	return g.dishCells
}

func buildDishCells(regionID, tileID int) []dishCell {
	const radius = 4
	seed := int64(regionID*73856093 ^ tileID*19349663)
	rng := rand.New(rand.NewSource(seed))
	cells := make([]dishCell, 0, 61)
	id := 0
	for q := -radius; q <= radius; q++ {
		rMin := maxIntLocal(-radius, -q-radius)
		rMax := minIntLocal(radius, -q+radius)
		for r := rMin; r <= rMax; r++ {
			x := math.Sqrt(3) * (float64(q) + float64(r)*0.5)
			y := 1.5 * float64(r)
			dist := math.Hypot(x, y)
			if dist > 7.05 {
				continue
			}
			kind := rng.Intn(4)
			cells = append(cells, dishCell{
				ID:     id,
				Q:      q,
				R:      r,
				Center: core.Vec3{X: x, Y: y},
				Kind:   kind,
				Phase:  rng.Float64() * math.Pi * 2,
			})
			id++
		}
	}
	return cells
}

func (g *Game) dishCenter() (float64, float64) {
	return float64(g.screenWidth) * 0.5, float64(g.screenHeight) * 0.48
}

func (g *Game) dishRadius() float64 {
	return math.Min(float64(g.screenWidth), float64(g.screenHeight)) * 0.42
}

func (g *Game) dishCellScale() float64 {
	return g.dishRadius() / 7.8
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
	if g.mugDragActive {
		if targetID, ok := g.pickTacticalTile(x, y); ok {
			g.mugDragTargetTile = targetID
		}
		if !g.dragMoved && (absInt(x-g.dragStartX) > dragThreshold || absInt(y-g.dragStartY) > dragThreshold) {
			g.dragMoved = true
		}
		g.dragLastX = x
		g.dragLastY = y
		return
	}
	if g.pendingBuildRecipeID == "" && g.maybeArmTacticalMUGDrag(x, y) {
		if g.mugDragActive {
			g.dragMoved = true
		}
		g.dragLastX = x
		g.dragLastY = y
		return
	}
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

func wrapDebugText(text string, maxChars, maxLines int) []string {
	if maxLines <= 0 || maxChars <= 0 {
		return nil
	}
	if len(text) <= maxChars {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		if len(text) > maxChars {
			return []string{text[:maxChars]}
		}
		return []string{text}
	}
	lines := make([]string, 0, maxLines)
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= maxChars {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
			if len(lines) == maxLines {
				break
			}
		}
	}
	if len(lines) < maxLines && current != "" {
		lines = append(lines, current)
	}
	if len(lines) == maxLines {
		consumed := 0
		for _, line := range lines {
			consumed += len(line)
		}
		if consumed < len(strings.Join(words, " ")) {
			last := lines[maxLines-1]
			if len(last) >= maxChars {
				last = last[:maxIntLocal(0, maxChars-1)]
			}
			lines[maxLines-1] = strings.TrimRight(last, " ,") + "..."
		}
	}
	for i, line := range lines {
		if len(line) > maxChars {
			lines[i] = line[:maxIntLocal(0, maxChars-3)] + "..."
		}
	}
	return lines
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

func (g *Game) backButtonRect() (float64, float64, float64, float64) {
	return 16, float64(g.screenHeight - 62), 88, 38
}

func (g *Game) depositButtonRect() (float64, float64, float64, float64) {
	backX, backY, backW, backH := g.backButtonRect()
	return backX + backW + 10, backY, 88, backH
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

func (g *Game) tacticalSelectionPopupRect() (float64, float64, float64, float64) {
	buildX, buildY, buildW, _ := g.buildButtonRect()
	w := 170.0
	h := 104.0
	x := buildX + buildW - w
	y := buildY - h - 12
	return x, y, w, h
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

func dishHexPoints(cx, cy, radius float64) []screenPoint {
	points := make([]screenPoint, 0, 6)
	for i := 0; i < 6; i++ {
		angle := math.Pi/6 + float64(i)*math.Pi/3
		points = append(points, screenPoint{
			x: cx + math.Cos(angle)*radius,
			y: cy + math.Sin(angle)*radius,
		})
	}
	return points
}

func dishCellColor(kind int) color.RGBA {
	switch kind % 4 {
	case 0:
		return color.RGBA{90, 188, 118, 235}
	case 1:
		return color.RGBA{112, 206, 186, 235}
	case 2:
		return color.RGBA{178, 126, 214, 235}
	default:
		return color.RGBA{214, 166, 82, 235}
	}
}

func dishInfluenceColor(influence int) color.RGBA {
	switch influence {
	case 1:
		return color.RGBA{134, 238, 134, 255}
	case 2:
		return color.RGBA{246, 118, 92, 255}
	case 3:
		return color.RGBA{178, 214, 255, 255}
	default:
		return color.RGBA{220, 235, 220, 255}
	}
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

// compactInventoryResources are the fixed resources always shown on the
// compact inventory card. Tactical keeps this to local supply basics; strategic
// also surfaces Field Data because it gates tech progression.
var compactInventoryResources = []core.ResourceType{
	core.ResourceStone,
	core.ResourceIronOre,
	core.ResourceCopperOre,
	core.ResourceCoal,
}

var strategicCompactInventoryResources = []core.ResourceType{
	core.ResourceStone,
	core.ResourceIronOre,
	core.ResourceCopperOre,
	core.ResourceFieldData,
}

func (g *Game) visibleInventoryResources() []core.ResourceType {
	if g.mode == modeStrategic {
		return strategicCompactInventoryResources
	}
	return compactInventoryResources
}

func (g *Game) inventoryCardLines() []string {
	lines := []string{"INVENTORY"}
	for _, resource := range g.visibleInventoryResources() {
		lines = append(lines, fmt.Sprintf("%s %d", resourceShortLabel(resource), g.inventory[resource]))
	}
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
		return "Fe plate"
	case core.ResourceCopperIngot:
		return "Cu plate"
	case core.ResourceGear:
		return "gear"
	case core.ResourceCrystal:
		return "crystal"
	case core.ResourceFieldData:
		return "data"
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
	for _, r := range g.visibleInventoryResources() {
		visible[r] = true
	}
	_, inventory := g.inventoryDisplaySource()
	for r, count := range inventory {
		if !visible[r] && count > 0 {
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
	w, h := g.inventoryCardSize()
	title, inventory := g.inventoryDisplaySource()
	if inventory == nil {
		inventory = map[core.ResourceType]int{}
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, uint8(210 * alpha)})
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, []string{title}, alpha)
	rowY := y + 28
	for _, resource := range g.visibleInventoryResources() {
		textX := int(x) + 30
		if resourceHasMapIcon(resource) {
			g.drawInventoryResourceIcon(screen, x+18, rowY+7, resource)
		} else {
			textX = int(x) + 18
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s %d", resourceShortLabel(resource), inventory[resource]), textX, int(rowY))
		rowY += 16
	}
	if g.inventoryHasOverflow() {
		bx, by, bw, bh := g.inventoryMoreButtonRect(x, y)
		drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 6, color.RGBA{36, 56, 84, uint8(220 * alpha)})
		drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), color.RGBA{146, 196, 230, uint8(220 * alpha)})
		ebitenutil.DebugPrintAt(screen, "...", int(bx)+8, int(by)+5)
	}
}

func (g *Game) drawPowerProgressCard(screen *ebiten.Image, x, y, w, alpha float64) float64 {
	power, runCost, ok := g.selectedPowerBuffer()
	if !ok {
		return 0
	}
	h := 44.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 8, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{92, 200, 238, uint8(220 * alpha)})

	label := fmt.Sprintf("POWER %.2f / 1.00", power)
	g.drawAlphaDebugTextBlock(screen, x+30, y+8, []string{label}, alpha)
	g.drawInventoryPowerIcon(screen, x+18, y+16, power, runCost)

	barX := x + 10
	barY := y + 28
	barW := w - 20
	barH := 8.0
	drawFilledRect(screen, float32(barX), float32(barY), float32(barW), float32(barH), color.RGBA{10, 24, 40, uint8(230 * alpha)})
	progress := clampRange(power, 0, 1)
	if progress > 0 {
		drawFilledRect(screen, float32(barX), float32(barY), float32(barW*progress), float32(barH), powerBarFillColor(progress, alpha))
	}
	if runCost > 0 && runCost < 1 {
		markerX := barX + barW*runCost
		drawFilledRect(screen, float32(markerX), float32(barY-2), 2, float32(barH+4), color.RGBA{236, 246, 255, uint8(220 * alpha)})
	}
	return h
}

func (g *Game) selectedPowerBuffer() (float64, float64, bool) {
	tile := g.currentTacticalTile()
	if tile == nil {
		return 0, 0, false
	}
	runCost := 0.0
	if tile.Device != nil {
		runCost = core.DeviceDefinition(tile.Device.Kind).RunPowerCost
	}
	return tile.PowerBuffer, runCost, true
}

func powerBarFillColor(progress, alpha float64) color.RGBA {
	r := uint8(64 + 52*progress)
	g := uint8(164 + 58*progress)
	b := uint8(230 + 20*progress)
	return color.RGBA{r, g, b, uint8(240 * alpha)}
}

func (g *Game) inventoryDisplaySource() (string, map[core.ResourceType]int) {
	if g.mode == modeTactical {
		if tmap := g.currentTacticalMap(); tmap != nil {
			if tmap.HasDevice(core.DeviceKindGate) {
				return "GATE STORAGE", tmap.Supply
			}
			return "NO GATE STORAGE", nil
		}
	}
	return "GLOBAL SUPPLY", g.inventory
}

func (g *Game) drawInventoryPowerIcon(screen *ebiten.Image, x, y, powerBuffer, runCost float64) {
	sprite := powerIndicatorSprite(powerBuffer, runCost)
	if sprite == nil {
		g.drawLightningBolt(screen, x, y, 11, color.RGBA{232, 210, 92, 255})
		return
	}
	drawPowerIndicatorSprite(screen, sprite, x, y, 15)
}

func (g *Game) drawInventoryResourceIcon(screen *ebiten.Image, cx, cy float64, resource core.ResourceType) {
	if resource != core.ResourceStone {
		if sprite := tacticalResourceSprite(resource); sprite != nil {
			drawCenteredSprite(screen, sprite, cx, cy, 13, 1, 1, 0.24, color.RGBA{})
			return
		}
	}
	base := core.ResourceColor(resource)
	switch resource {
	case core.ResourceStone:
		return
	case core.ResourceIronIngot:
		drawRoundedRect(screen, float32(cx-6), float32(cy-3), 12, 6, 2, base)
	case core.ResourceCopperIngot:
		drawRoundedRect(screen, float32(cx-6), float32(cy-3), 12, 6, 2, base)
		drawFilledRect(screen, float32(cx-4), float32(cy-1), 8, 2, color.RGBA{255, 255, 255, 48})
	case core.ResourceGear:
		drawDisc(screen, float32(cx), float32(cy), 5, base)
		drawDisc(screen, float32(cx), float32(cy), 2, color.RGBA{30, 38, 48, 255})
	case core.ResourceCrystal:
		verts := []ebiten.Vertex{
			{DstX: float32(cx), DstY: float32(cy - 5)},
			{DstX: float32(cx + 4), DstY: float32(cy)},
			{DstX: float32(cx), DstY: float32(cy + 5)},
			{DstX: float32(cx - 4), DstY: float32(cy)},
		}
		drawFilledPolygon(screen, verts, base)
	default:
		drawDisc(screen, float32(cx), float32(cy), 3, base)
	}
}

func resourceHasMapIcon(resource core.ResourceType) bool {
	return resource != core.ResourceStone && tacticalResourceSprite(resource) != nil
}

func (g *Game) drawTechView(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawScreenBackButton(screen)
	g.drawTechHeader(screen)
	g.drawTechList(screen)
	g.drawTechFooter(screen)
}

func (g *Game) drawTechHeader(screen *ebiten.Image) {
	stage := g.currentStage()
	lines := []string{
		"TECH",
		stage.Title,
		"Spend transferred regional supply to unlock devices.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawTechList(screen *ebiten.Image) {
	recipes := g.currentStageRecipes()
	if len(recipes) == 0 {
		g.drawAlphaDebugTextBlock(screen, 22, 112, []string{
			"No device tech in this stage.",
			"Complete goals to open the next stage.",
		}, 1)
		return
	}
	x := 22.0
	ry := 112.0
	w := float64(g.screenWidth) - 44
	for _, recipe := range recipes {
		h := g.techRecipeCardHeight(recipe.ID)
		if ry+h > float64(g.screenHeight)-80 {
			break
		}
		g.drawTechRecipeCard(screen, x, ry, w, h, recipe)
		ry += h + 8
	}
}

func (g *Game) techRecipeCardHeight(recipeID string) float64 {
	if g.expandedTechRecipeID != recipeID {
		return 74
	}
	return 74 + float64(len(g.techIngredientLines(recipeID)))*16 + 10
}

func (g *Game) drawTechRecipeCard(screen *ebiten.Image, x, y, w, h float64, recipe core.RecipeDef) {
	known := g.knownRecipes[recipe.ID]
	affordable := g.canUnlockRecipe(recipe.ID)
	fill := color.RGBA{52, 44, 34, 230}
	border := color.RGBA{188, 150, 94, 255}
	status := "NEED"
	if known {
		fill = color.RGBA{24, 40, 30, 230}
		border = color.RGBA{116, 198, 140, 255}
		status = "KNOWN"
	} else if affordable {
		fill = color.RGBA{30, 72, 60, 230}
		border = color.RGBA{128, 226, 188, 255}
		status = "UNLOCK"
	}
	sw := float64(len(status))*7 + 18
	costLines := wrapDebugText(g.techCostText(recipe.ID), int((w-24-sw-12)/7), 2)
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	lines := append([]string{recipe.Title}, costLines...)
	g.drawAlphaDebugTextBlock(screen, x+12, y+10, lines, 1)
	g.drawTechDeviceIcon(screen, recipe.Device, x+w-sw-58, y+h*0.5, 34)
	drawRoundedRect(screen, float32(x+w-sw-12), float32(y+18), float32(sw), 20, 8, color.RGBA{8, 18, 32, 220})
	ebitenutil.DebugPrintAt(screen, status, int(x+w-sw-5), int(y)+20)
	if !known && !affordable {
		lineY := y + 28
		drawFilledRect(screen, float32(x+w-sw-7), float32(lineY), float32(sw-10), 2, color.RGBA{236, 190, 150, 255})
	}
	if g.expandedTechRecipeID == recipe.ID && !known && !affordable {
		g.drawTintedDebugTextBlock(screen, x+12, y+74, g.techIngredientLines(recipe.ID), 1, 1, 0.86, 0.68)
	}
}

func (g *Game) drawTechFooter(screen *ebiten.Image) {
	lines := []string{
		"Transfer from a local GATE, then unlock tech here.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, float64(g.screenHeight-84), lines, 1)
}

func (g *Game) drawBuildView(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawScreenBackButton(screen)
	g.drawBuildHeader(screen)
	g.drawBuildList(screen)
	g.drawBuildFooter(screen)
}

func (g *Game) drawBuildHeader(screen *ebiten.Image) {
	lines := []string{
		"BUILD",
		"Discovered devices",
		"Pick a device, then place it.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawBuildList(screen *ebiten.Image) {
	ids := g.buildListIDs()
	if len(ids) == 0 {
		g.drawAlphaDebugTextBlock(screen, 22, 112, []string{
			"No buildable devices.",
			"Unlock device tech from strategic view.",
		}, 1)
		return
	}
	x := 22.0
	ry := 112.0
	w := float64(g.screenWidth) - 44
	for _, recipeID := range ids {
		h := g.buildRecipeCardHeight(recipeID)
		if ry+h > float64(g.screenHeight)-80 {
			break
		}
		g.drawBuildRecipeCard(screen, x, ry, w, h, recipeID)
		ry += h + 8
	}
}

func (g *Game) buildRecipeCardHeight(recipeID string) float64 {
	if g.expandedBuildRecipeID != recipeID {
		return 74
	}
	return 74 + float64(len(g.buildIngredientLines(recipeID)))*16 + 10
}

func (g *Game) drawBuildRecipeCard(screen *ebiten.Image, x, y, w, h float64, recipeID string) {
	title := g.buildRecipeTitle(recipeID)
	costText := g.recipeCostText(recipeID)
	affordable := g.canAffordRecipe(recipeID)
	fill := color.RGBA{72, 34, 38, 230}
	border := color.RGBA{216, 108, 118, 255}
	status := "NEED"
	if affordable {
		fill = color.RGBA{28, 72, 46, 230}
		border = color.RGBA{128, 226, 160, 255}
		status = "FAB"
	}
	sw := float64(len(status))*7 + 18
	costLines := wrapDebugText(costText, int((w-24-sw-12)/7), 2)
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	lines := append([]string{title}, costLines...)
	g.drawAlphaDebugTextBlock(screen, x+12, y+10, lines, 1)
	drawRoundedRect(screen, float32(x+w-sw-12), float32(y+18), float32(sw), 20, 8, color.RGBA{8, 18, 32, 220})
	ebitenutil.DebugPrintAt(screen, status, int(x+w-sw-5), int(y)+20)
	if !affordable {
		lineY := y + 28
		drawFilledRect(screen, float32(x+w-sw-7), float32(lineY), float32(sw-10), 2, color.RGBA{236, 170, 170, 255})
	}
	if g.expandedBuildRecipeID == recipeID && !affordable {
		g.drawTintedDebugTextBlock(screen, x+12, y+74, g.buildIngredientLines(recipeID), 1, 1, 0.78, 0.78)
	}
}

func (g *Game) drawBuildFooter(screen *ebiten.Image) {
	lines := []string{
		"Tap a green device, then tap a highlighted tile.",
	}
	g.drawAlphaDebugTextBlock(screen, 18, float64(g.screenHeight-84), lines, 1)
}

func (g *Game) drawTechDeviceIcon(screen *ebiten.Image, kind core.DeviceKind, centerX, centerY, size float64) {
	switch kind {
	case core.DeviceKindMiner:
		drawCenteredSprite(screen, autoMinerSprites[0], centerX, centerY, size, size*0.05, size*0.07, 0.24, color.RGBA{})
	case core.DeviceKindSmelter:
		drawCenteredSprite(screen, deviceSprites[1], centerX, centerY, size, size*0.05, size*0.07, 0.24, color.RGBA{})
	case core.DeviceKindGate:
		drawCenteredSprite(screen, deviceSprites[0], centerX, centerY, size*0.82, size*0.05, size*0.07, 0.24, color.RGBA{})
	case core.DeviceKindGenerator:
		drawCenteredSprite(screen, deviceSprites[5], centerX, centerY, size, size*0.05, size*0.07, 0.24, color.RGBA{})
	case core.DeviceKindAssembler:
		if assemblerSprites[1] != nil {
			drawCenteredSprite(screen, assemblerSprites[1], centerX, centerY, size*1.08, size*0.05, size*0.07, 0.24, color.RGBA{})
			return
		}
		drawRoundedRect(screen, float32(centerX-size*0.26), float32(centerY-size*0.18), float32(size*0.52), float32(size*0.36), 5, color.RGBA{126, 104, 64, 255})
		drawRectOutline(screen, float32(centerX-size*0.26), float32(centerY-size*0.18), float32(size*0.52), float32(size*0.36), color.RGBA{226, 204, 150, 255})
		drawDisc(screen, float32(centerX-size*0.10), float32(centerY), float32(size*0.09), color.RGBA{238, 210, 104, 255})
		drawDisc(screen, float32(centerX+size*0.10), float32(centerY), float32(size*0.09), color.RGBA{238, 210, 104, 255})
	}
}

func (g *Game) currentStageRecipes() []core.RecipeDef {
	stage := g.currentStage()
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	recipes := make([]core.RecipeDef, 0, len(stage.KnownRecipes))
	for _, recipeID := range stage.KnownRecipes {
		if recipe, ok := g.recipes.Recipe(recipeID); ok {
			if (recipe.Kind != core.RecipeDevice && recipe.Kind != core.RecipeUpgrade) || recipe.Device == core.DeviceKindNone {
				continue
			}
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
const starterGateRecipeID = "__starter_gate"

func (g *Game) buildListIDs() []string {
	ids := make([]string, 0, len(g.knownRecipes)+1)
	if g.starterGateCount > 0 {
		ids = append(ids, starterGateRecipeID)
	}
	if g.starterMinerCount > 0 {
		ids = append(ids, starterMinerRecipeID)
	}
	for _, recipe := range g.currentBuildRecipes() {
		ids = append(ids, recipe.ID)
	}
	return ids
}

func (g *Game) pickTechRecipe(x, y int) (string, bool) {
	recipes := g.currentStageRecipes()
	cardX := 22.0
	cardY := 112.0
	cardW := float64(g.screenWidth) - 44
	for _, recipe := range recipes {
		cardH := g.techRecipeCardHeight(recipe.ID)
		ry := cardY
		if ry+cardH > float64(g.screenHeight)-80 {
			break
		}
		if g.pointInRect(float64(x), float64(y), cardX, ry, cardW, cardH) {
			return recipe.ID, true
		}
		cardY += cardH + 8
	}
	return "", false
}

func (g *Game) pickBuildRecipe(x, y int) (string, bool) {
	ids := g.buildListIDs()
	cardX := 22.0
	cardY := 112.0
	cardW := float64(g.screenWidth) - 44
	for _, recipeID := range ids {
		cardH := g.buildRecipeCardHeight(recipeID)
		ry := cardY
		if ry+cardH > float64(g.screenHeight)-80 {
			break
		}
		if g.pointInRect(float64(x), float64(y), cardX, ry, cardW, cardH) {
			return recipeID, true
		}
		cardY += cardH + 8
	}
	return "", false
}

func (g *Game) unlockRecipe(recipeID string) {
	if g.knownRecipes == nil {
		g.knownRecipes = map[string]bool{}
	}
	if recipeID == "" {
		return
	}
	g.knownRecipes[recipeID] = true
}

func (g *Game) canUnlockRecipe(recipeID string) bool {
	if g.knownRecipes != nil && g.knownRecipes[recipeID] {
		return false
	}
	if !g.currentStageHasRecipe(recipeID) {
		return false
	}
	cost := g.techCost(recipeID)
	if len(cost) == 0 {
		return true
	}
	for resource, needed := range cost {
		if needed > 0 && g.inventory[resource] < needed {
			return false
		}
	}
	return true
}

func (g *Game) unlockRecipeWithGlobalSupply(recipeID string) bool {
	if !g.canUnlockRecipe(recipeID) {
		return false
	}
	cost := g.techCost(recipeID)
	for resource, amount := range cost {
		if amount > 0 {
			g.inventory[resource] -= amount
		}
	}
	g.unlockRecipe(recipeID)
	g.saveSoon()
	return true
}

func (g *Game) currentStageHasRecipe(recipeID string) bool {
	stage := g.currentStage()
	for _, id := range stage.KnownRecipes {
		if id == recipeID {
			return true
		}
	}
	return false
}

func (g *Game) techCost(recipeID string) map[core.ResourceType]int {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	cost := g.recipes.RawCost(recipeID)
	if cost == nil {
		cost = map[core.ResourceType]int{}
	}
	delete(cost, core.ResourceCoal)
	if data := g.techFieldDataCost(recipeID); data > 0 {
		cost[core.ResourceFieldData] += data
	}
	return cost
}

func (g *Game) techFieldDataCost(recipeID string) int {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	recipe, ok := g.recipes.Recipe(recipeID)
	if !ok {
		return 0
	}
	switch recipe.StageID {
	case "smelting":
		return 4
	case "coal_power":
		return 8
	case "assembly":
		return 14
	case "mechanics":
		return 20
	default:
		return 6
	}
}

func (g *Game) techCostText(recipeID string) string {
	if g.knownRecipes != nil && g.knownRecipes[recipeID] {
		return "unlocked for tactical builds"
	}
	cost := g.techCost(recipeID)
	if len(cost) == 0 {
		return "free tech"
	}
	resources := make([]core.ResourceType, 0, len(cost))
	for resource, amount := range cost {
		if amount > 0 {
			resources = append(resources, resource)
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		return resourceLabel(resources[i]) < resourceLabel(resources[j])
	})
	parts := make([]string, 0, len(resources))
	for _, resource := range resources {
		needed := cost[resource]
		parts = append(parts, fmt.Sprintf("%s %d/%d", resourceLabel(resource), g.inventory[resource], needed))
	}
	return "global " + strings.Join(parts, ", ")
}

func (g *Game) techIngredientLines(recipeID string) []string {
	return ingredientProgressLines("GLOBAL INGREDIENTS", g.techCost(recipeID), g.inventory)
}

func (g *Game) canAffordRecipe(recipeID string) bool {
	if recipeID == starterMinerRecipeID {
		return g.starterMinerCount > 0 && g.currentRegionHasGate()
	}
	if recipeID == starterGateRecipeID {
		tmap := g.currentTacticalMap()
		return g.starterGateCount > 0 && tmap != nil && !tmap.HasDevice(core.DeviceKindGate)
	}
	if !g.currentRegionHasGate() {
		return false
	}
	_, ok := g.buildPlanForRecipe(recipeID)
	return ok
}

func (g *Game) buildIngredientLines(recipeID string) []string {
	switch recipeID {
	case starterMinerRecipeID:
		if !g.currentRegionHasGate() {
			return []string{"REQUIREMENTS", "local GATE 0/1"}
		}
		return []string{"REQUIREMENTS", "MUG unit ready"}
	case starterGateRecipeID:
		if tmap := g.currentTacticalMap(); tmap != nil && tmap.HasDevice(core.DeviceKindGate) {
			return []string{"REQUIREMENTS", "one GATE per region"}
		}
		return []string{"REQUIREMENTS", "GATE unit ready"}
	}
	return ingredientProgressLines("LOCAL INGREDIENTS", g.recipes.RawCost(recipeID), g.buildResourceInventory())
}

func (g *Game) recipeCostText(recipeID string) string {
	if recipeID == starterMinerRecipeID {
		if !g.currentRegionHasGate() {
			return "requires local GATE"
		}
		return "movable unit gatherer"
	}
	if recipeID == starterGateRecipeID {
		if tmap := g.currentTacticalMap(); tmap != nil && tmap.HasDevice(core.DeviceKindGate) {
			return "one GATE per region"
		}
		return "starter gate"
	}
	if !g.currentRegionHasGate() {
		return "requires local GATE"
	}
	plan, ok := g.buildPlanForRecipe(recipeID)
	if !ok {
		return g.recipeRequirementText(recipeID)
	}
	return buildPlanSummary(plan)
}

func ingredientProgressLines(title string, cost, have map[core.ResourceType]int) []string {
	lines := []string{title}
	if len(cost) == 0 {
		return append(lines, "none")
	}
	resources := make([]core.ResourceType, 0, len(cost))
	for resource, needed := range cost {
		if needed > 0 {
			resources = append(resources, resource)
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		return resourceLabel(resources[i]) < resourceLabel(resources[j])
	})
	for _, resource := range resources {
		needed := cost[resource]
		current := 0
		if have != nil {
			current = have[resource]
		}
		status := "ok"
		if current < needed {
			status = fmt.Sprintf("need %d", needed-current)
		}
		lines = append(lines, fmt.Sprintf("%s %d/%d %s", resourceLabel(resource), current, needed, status))
	}
	return lines
}

func (g *Game) recipeRequirementText(recipeID string) string {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	cost := g.recipes.RawCost(recipeID)
	if len(cost) == 0 {
		return "insufficient materials"
	}
	resources := make([]core.ResourceType, 0, len(cost))
	for resource, needed := range cost {
		if needed > 0 {
			resources = append(resources, resource)
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		return resourceLabel(resources[i]) < resourceLabel(resources[j])
	})
	parts := make([]string, 0, len(resources))
	for _, resource := range resources {
		needed := cost[resource]
		have := g.buildResourceInventory()[resource]
		parts = append(parts, fmt.Sprintf("%s %d/%d", resourceLabel(resource), have, needed))
	}
	return "need " + strings.Join(parts, ", ")
}

func (g *Game) spendBuildPlan(plan buildPlan) {
	resources := g.buildResourceInventory()
	for part, amount := range plan.partSpend {
		g.partInventory[part] -= amount
	}
	for resource, amount := range plan.rawSpend {
		resources[resource] -= amount
	}
}

func (g *Game) placeRecipeOnCurrentTile(recipeID string) bool {
	return g.placeRecipeOnTile(recipeID, g.tacticalTile)
}

func (g *Game) placeRecipeOnTile(recipeID string, tileID int) bool {
	tmap := g.currentTacticalMap()
	if tmap == nil || tileID < 0 || tileID >= len(tmap.Tiles) {
		return false
	}
	g.tacticalTile = tileID
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil || tile.Device.Kind != core.DeviceKindNone {
		return false
	}
	if recipeID == starterMinerRecipeID {
		if g.starterMinerCount <= 0 || !g.currentRegionHasGate() {
			return false
		}
		g.starterMinerCount--
		g.starterMinerPlaced++
		tile.Device = g.buildStarterMinerLayout()
		tile.PowerBuffer = 0
		return true
	}
	if recipeID == starterGateRecipeID {
		tmap := g.currentTacticalMap()
		if g.starterGateCount <= 0 || tmap == nil || tmap.HasDevice(core.DeviceKindGate) {
			return false
		}
		g.starterGateCount--
		tile.Device = g.buildStarterGateLayout()
		tile.PowerBuffer = 0
		tmap.StartCreatureSpawn(6)
		return true
	}
	recipe, ok := g.recipes.Recipe(recipeID)
	if !ok || recipe.Kind != core.RecipeDevice || recipe.Device == core.DeviceKindNone {
		return false
	}
	if !g.currentRegionHasGate() {
		return false
	}
	plan, ok := g.buildPlanForRecipe(recipeID)
	if !ok {
		return false
	}
	g.spendBuildPlan(plan)
	layout := core.NewDeviceLayout(5, 5)
	for _, cell := range recipe.Pattern {
		if cell.Device != core.DeviceKindNone {
			layout.SetDevice(cell.X, cell.Y, cell.Device)
			continue
		}
		layout.SetPart(cell.X, cell.Y, cell.Part)
	}
	layout.Kind = recipe.Device
	if layout.Kind == core.DeviceKindSmelter {
		layout.ConfigInput = core.ResourceIronOre
	}
	if layout.Kind == core.DeviceKindAssembler {
		layout.ConfigInput = core.ResourceGear
	}
	if layout.Kind == core.DeviceKindGenerator {
		layout.ConfigMode = core.DeviceModeCoal
	}
	layout.RefundResources = copyResourceCounts(plan.rawSpend)
	layout.RefundParts = copyPartCounts(plan.partSpend)
	tile.Device = layout
	tile.PowerBuffer = 0
	return true
}

func (g *Game) canPlacePendingBuildOnTile(tileID int) bool {
	if g.pendingBuildRecipeID == "" || !g.canAffordRecipe(g.pendingBuildRecipeID) {
		return false
	}
	tmap := g.currentTacticalMap()
	if tmap == nil || tileID < 0 || tileID >= len(tmap.Tiles) {
		return false
	}
	tile := &tmap.Tiles[tileID]
	return tile.Device != nil && tile.Device.Kind == core.DeviceKindNone
}

func (g *Game) currentRegionHasGate() bool {
	tmap := g.currentTacticalMap()
	return tmap != nil && tmap.HasDevice(core.DeviceKindGate)
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
	layout.DeployTimer = 1
	return layout
}

func (g *Game) buildStarterGateLayout() *core.DeviceLayout {
	layout := core.NewDeviceLayout(5, 5)
	layout.Kind = core.DeviceKindGate
	layout.SpecialStarter = true
	layout.DeployTimer = 1
	return layout
}

func (g *Game) buildPlanForRecipe(recipeID string) (buildPlan, bool) {
	if g.recipes == nil {
		g.recipes = core.DefaultRecipeBook()
	}
	inventory := g.buildResourceInventory()
	rawAvail := make(map[core.ResourceType]int, len(inventory))
	for resource, amount := range inventory {
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

func (g *Game) buildResourceInventory() map[core.ResourceType]int {
	if tmap := g.currentTacticalMap(); tmap != nil && tmap.HasDevice(core.DeviceKindGate) {
		if tmap.Supply == nil {
			tmap.Supply = map[core.ResourceType]int{}
		}
		return tmap.Supply
	}
	if g.inventory == nil {
		g.inventory = map[core.ResourceType]int{}
	}
	return g.inventory
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
		resource := core.ResourceType(key)
		raw = append(raw, fmt.Sprintf("%s %d", resourceLabel(resource), plan.rawSpend[resource]))
	}
	if len(raw) > 0 {
		return "spend " + strings.Join(raw, ", ")
	}
	return "ready to build"
}

func (g *Game) buildRecipeTitle(recipeID string) string {
	if recipeID == starterMinerRecipeID {
		return fmt.Sprintf("MUG x%d", g.starterMinerCount)
	}
	if recipeID == starterGateRecipeID {
		return fmt.Sprintf("GATE x%d", g.starterGateCount)
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
	g.drawArrowBackButton(screen)
}

func (g *Game) drawSettingsPanel(screen *ebiten.Image) {
	x := float64(g.screenWidth)*0.5 - 152
	y := 72.0
	w := 304.0
	h := 620.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, 255})
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, []string{
		"SETTINGS",
		"Project " + projectName,
		"Build " + g.displayVersion(),
		"World controls",
		"Dev stage jump",
	}, 1)

	g.drawStageJumpList(screen, x+18, y+104)

	tx, ty, tw, th := g.creaturesToggleRect()
	state := "OFF"
	fill := color.RGBA{38, 48, 62, 236}
	border := color.RGBA{112, 132, 154, 255}
	if g.creaturesEnabled {
		state = "ON"
		fill = color.RGBA{28, 72, 46, 236}
		border = color.RGBA{128, 226, 160, 255}
	}
	g.drawAlphaDebugTextBlock(screen, x+18, y+340, []string{
		"Tactical creatures",
		"Disabled by default for quieter playtests.",
	}, 1)
	drawRoundedRect(screen, float32(tx), float32(ty), float32(tw), float32(th), 10, fill)
	drawRectOutline(screen, float32(tx), float32(ty), float32(tw), float32(th), border)
	ebitenutil.DebugPrintAt(screen, "CREATURES "+state, int(tx)+18, int(ty)+12)

	rx, ry, rw, rh := g.regenerateButtonRect()
	g.drawAlphaDebugTextBlock(screen, x+18, y+430, []string{
		"Regenerate the world and clear tactical state.",
	}, 1)
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 12, color.RGBA{124, 58, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{240, 190, 170, 255})
	ebitenutil.DebugPrintAt(screen, "REGENERATE MAP", int(rx)+18, int(ry)+14)
}

func (g *Game) drawSmelterConfig(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawArrowBackButton(screen)
	tile := g.configTile()
	x := float64(g.screenWidth)*0.5 - 152
	y := 104.0
	w := 304.0
	h := 244.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, 255})
	lines := []string{"SMELTER", "Input from adjacent GATE"}
	if tile != nil && tile.Device != nil {
		input := smelterInputLabel(tile.Device.ConfigInput)
		lines = append(lines, "current: "+input)
	}
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, lines, 1)
	for i, resource := range smelterConfigInputs() {
		bx, by, bw, bh := g.smelterConfigButtonRect(i)
		fill := color.RGBA{30, 42, 58, 236}
		border := color.RGBA{104, 132, 160, 255}
		if tile != nil && tile.Device != nil && tile.Device.ConfigInput == resource {
			fill = color.RGBA{28, 72, 46, 236}
			border = color.RGBA{128, 226, 160, 255}
		}
		drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 8, fill)
		drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), border)
		label := "OFF"
		if output, ok := core.SmelterOutputForInput(resource); ok {
			label = fmt.Sprintf("%s -> %s", resourceLabel(resource), resourceLabel(output))
		}
		ebitenutil.DebugPrintAt(screen, label, int(bx)+10, int(by)+10)
	}
	rx, ry, rw, rh := g.smelterConfigRemoveRect()
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 8, color.RGBA{96, 48, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{226, 144, 136, 255})
	ebitenutil.DebugPrintAt(screen, "DISASSEMBLE", int(rx)+42, int(ry)+10)
}

func (g *Game) configTile() *core.TacticalTile {
	tmap := g.currentTacticalMap()
	if tmap == nil || g.configTileID < 0 || g.configTileID >= len(tmap.Tiles) {
		return nil
	}
	tile := &tmap.Tiles[g.configTileID]
	if tile.Device == nil || tile.Device.Kind != core.DeviceKindSmelter {
		return nil
	}
	return tile
}

func (g *Game) assemblerConfigTile() *core.TacticalTile {
	tmap := g.currentTacticalMap()
	if tmap == nil || g.configTileID < 0 || g.configTileID >= len(tmap.Tiles) {
		return nil
	}
	tile := &tmap.Tiles[g.configTileID]
	if tile.Device == nil || tile.Device.Kind != core.DeviceKindAssembler {
		return nil
	}
	return tile
}

func (g *Game) generatorConfigTile() *core.TacticalTile {
	tmap := g.currentTacticalMap()
	if tmap == nil || g.configTileID < 0 || g.configTileID >= len(tmap.Tiles) {
		return nil
	}
	tile := &tmap.Tiles[g.configTileID]
	if tile.Device == nil || tile.Device.Kind != core.DeviceKindGenerator {
		return nil
	}
	return tile
}

func smelterConfigInputs() []core.ResourceType {
	return []core.ResourceType{
		core.ResourceNone,
		core.ResourceIronOre,
		core.ResourceCopperOre,
	}
}

func assemblerConfigOutputs() []core.ResourceType {
	return []core.ResourceType{
		core.ResourceNone,
		core.ResourceGear,
	}
}

func generatorConfigModes() []string {
	return []string{
		core.DeviceModeCoal,
		core.DeviceModeSolar,
	}
}

func generatorModeLabel(mode string) string {
	if mode == core.DeviceModeSolar {
		return "solar"
	}
	return "coal"
}

func assemblerOutputLabel(resource core.ResourceType) string {
	if resource == core.ResourceNone {
		return "off"
	}
	return resourceLabel(resource)
}

func smelterInputLabel(resource core.ResourceType) string {
	if resource == core.ResourceNone {
		return "off"
	}
	return resourceLabel(resource)
}

func (g *Game) smelterConfigButtonRect(index int) (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	y := 198.0 + float64(index)*48
	return x, y, 264, 38
}

func (g *Game) smelterConfigRemoveRect() (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	return x, 368, 264, 38
}

func (g *Game) drawAssemblerConfig(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawArrowBackButton(screen)
	tile := g.assemblerConfigTile()
	x := float64(g.screenWidth)*0.5 - 152
	y := 104.0
	w := 304.0
	h := 244.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{206, 190, 126, 255})
	lines := []string{"ASSEMBLER", "Output recipe"}
	if tile != nil && tile.Device != nil {
		lines = append(lines, "current: "+assemblerOutputLabel(tile.Device.ConfigInput))
	}
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, lines, 1)
	for i, output := range assemblerConfigOutputs() {
		bx, by, bw, bh := g.assemblerConfigButtonRect(i)
		fill := color.RGBA{30, 42, 58, 236}
		border := color.RGBA{104, 132, 160, 255}
		if tile != nil && tile.Device != nil && tile.Device.ConfigInput == output {
			fill = color.RGBA{74, 68, 36, 236}
			border = color.RGBA{236, 210, 118, 255}
		}
		drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 8, fill)
		drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), border)
		label := "OFF"
		if output == core.ResourceGear {
			label = "IRON + COPPER -> GEARS"
		}
		ebitenutil.DebugPrintAt(screen, label, int(bx)+10, int(by)+10)
	}
	rx, ry, rw, rh := g.assemblerConfigRemoveRect()
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 8, color.RGBA{96, 48, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{226, 144, 136, 255})
	ebitenutil.DebugPrintAt(screen, "DISASSEMBLE", int(rx)+42, int(ry)+10)
}

func (g *Game) assemblerConfigButtonRect(index int) (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	y := 198.0 + float64(index)*48
	return x, y, 264, 38
}

func (g *Game) assemblerConfigRemoveRect() (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	return x, 368, 264, 38
}

func (g *Game) drawGeneratorConfig(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBackdrop(screen)
	g.drawArrowBackButton(screen)
	tile := g.generatorConfigTile()
	x := float64(g.screenWidth)*0.5 - 152
	y := 104.0
	w := 304.0
	h := 244.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{132, 220, 164, 255})
	lines := []string{"GENERATOR", "Power mode"}
	if tile != nil && tile.Device != nil {
		lines = append(lines, "current: "+generatorModeLabel(tile.Device.ConfigMode))
	}
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, lines, 1)
	for i, mode := range generatorConfigModes() {
		bx, by, bw, bh := g.generatorConfigButtonRect(i)
		fill := color.RGBA{30, 42, 58, 236}
		border := color.RGBA{104, 132, 160, 255}
		current := tile != nil && tile.Device != nil && generatorModeLabel(tile.Device.ConfigMode) == generatorModeLabel(mode)
		if current {
			fill = color.RGBA{28, 72, 46, 236}
			border = color.RGBA{128, 226, 160, 255}
		}
		drawRoundedRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 8, fill)
		drawRectOutline(screen, float32(bx), float32(by), float32(bw), float32(bh), border)
		label := "COAL: strong, consumes local coal"
		if mode == core.DeviceModeSolar {
			label = "SOLAR: weak, no coal"
		}
		ebitenutil.DebugPrintAt(screen, label, int(bx)+10, int(by)+10)
	}
	rx, ry, rw, rh := g.generatorConfigRemoveRect()
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 8, color.RGBA{96, 48, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{226, 144, 136, 255})
	ebitenutil.DebugPrintAt(screen, "DISASSEMBLE", int(rx)+42, int(ry)+10)
}

func (g *Game) generatorConfigButtonRect(index int) (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	y := 198.0 + float64(index)*48
	return x, y, 264, 38
}

func (g *Game) generatorConfigRemoveRect() (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 132
	return x, 368, 264, 38
}

func (g *Game) handleSmelterConfigInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleSmelterConfigTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleSmelterConfigTap(x, y)
	}
}

func (g *Game) handleAssemblerConfigInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleAssemblerConfigTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleAssemblerConfigTap(x, y)
	}
}

func (g *Game) handleGeneratorConfigInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleGeneratorConfigTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleGeneratorConfigTap(x, y)
	}
}

func (g *Game) handleGeneratorConfigTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeTactical
		return
	}
	tile := g.generatorConfigTile()
	if tile == nil || tile.Device == nil {
		g.mode = modeTactical
		return
	}
	rx, ry, rw, rh := g.generatorConfigRemoveRect()
	if g.pointInRect(float64(x), float64(y), rx, ry, rw, rh) {
		g.disassembleCurrentTacticalDevice()
		g.configTileID = -1
		g.mode = modeTactical
		return
	}
	for i, mode := range generatorConfigModes() {
		bx, by, bw, bh := g.generatorConfigButtonRect(i)
		if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
			continue
		}
		if mode == core.DeviceModeSolar && !g.solarRetrofitUnlocked() {
			return
		}
		tile.Device.ConfigMode = mode
		g.saveSoon()
		return
	}
}

func (g *Game) handleAssemblerConfigTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeTactical
		return
	}
	tile := g.assemblerConfigTile()
	if tile == nil || tile.Device == nil {
		g.mode = modeTactical
		return
	}
	rx, ry, rw, rh := g.assemblerConfigRemoveRect()
	if g.pointInRect(float64(x), float64(y), rx, ry, rw, rh) {
		g.disassembleCurrentTacticalDevice()
		g.configTileID = -1
		g.mode = modeTactical
		return
	}
	for i, output := range assemblerConfigOutputs() {
		bx, by, bw, bh := g.assemblerConfigButtonRect(i)
		if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
			continue
		}
		tile.Device.ConfigInput = output
		g.saveSoon()
		return
	}
}

func (g *Game) handleSmelterConfigTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeTactical
		return
	}
	tile := g.configTile()
	if tile == nil || tile.Device == nil {
		g.mode = modeTactical
		return
	}
	rx, ry, rw, rh := g.smelterConfigRemoveRect()
	if g.pointInRect(float64(x), float64(y), rx, ry, rw, rh) {
		g.disassembleCurrentTacticalDevice()
		g.configTileID = -1
		g.mode = modeTactical
		return
	}
	for i, resource := range smelterConfigInputs() {
		bx, by, bw, bh := g.smelterConfigButtonRect(i)
		if !g.pointInRect(float64(x), float64(y), bx, by, bw, bh) {
			continue
		}
		tile.Device.ConfigInput = resource
		g.saveSoon()
		return
	}
}

func (g *Game) drawStageJumpList(screen *ebiten.Image, x, y float64) {
	stages := g.orderedStages()
	g.drawAlphaDebugTextBlock(screen, x, y, []string{"Jump to stage"}, 1)
	for i, stage := range stages {
		rx, ry, rw, rh := g.stageJumpButtonRect(i)
		fill := color.RGBA{30, 42, 58, 236}
		border := color.RGBA{104, 132, 160, 255}
		if stage.ID == g.currentStageID {
			fill = color.RGBA{28, 72, 46, 236}
			border = color.RGBA{128, 226, 160, 255}
		}
		drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 8, fill)
		drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), border)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d %s", i+1, stage.Title), int(rx)+10, int(ry)+9)
	}
}

func (g *Game) stageJumpButtonRect(index int) (float64, float64, float64, float64) {
	x := float64(g.screenWidth)*0.5 - 134
	y := 204.0 + float64(index)*42
	return x, y, 268, 34
}

func (g *Game) pickStageJump(x, y int) (string, bool) {
	for i, stage := range g.orderedStages() {
		rx, ry, rw, rh := g.stageJumpButtonRect(i)
		if g.pointInRect(float64(x), float64(y), rx, ry, rw, rh) {
			return stage.ID, true
		}
	}
	return "", false
}

func (g *Game) handleStageJumpKeys() {
	stages := g.orderedStages()
	keys := []ebiten.Key{
		ebiten.Key1,
		ebiten.Key2,
		ebiten.Key3,
		ebiten.Key4,
		ebiten.Key5,
		ebiten.Key6,
		ebiten.Key7,
		ebiten.Key8,
		ebiten.Key9,
	}
	for i, key := range keys {
		if i >= len(stages) {
			return
		}
		if inpututil.IsKeyJustPressed(key) {
			g.jumpToStage(stages[i].ID)
			return
		}
	}
}

func (g *Game) orderedStages() []core.ProgressStage {
	if g.progression == nil {
		g.progression = core.DefaultProgressionBook()
	}
	stages := make([]core.ProgressStage, 0, len(g.progression.Stages))
	seen := map[string]bool{}
	for stageID := g.progression.StartStageID; stageID != ""; {
		stage, ok := g.progression.Stage(stageID)
		if !ok || seen[stageID] {
			break
		}
		stages = append(stages, stage)
		seen[stageID] = true
		stageID = stage.NextStageID
	}
	remaining := make([]core.ProgressStage, 0, len(g.progression.Stages)-len(seen))
	for stageID, stage := range g.progression.Stages {
		if !seen[stageID] {
			remaining = append(remaining, stage)
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].ID < remaining[j].ID
	})
	return append(stages, remaining...)
}

func (g *Game) jumpToStage(stageID string) bool {
	if g.progression == nil {
		g.progression = core.DefaultProgressionBook()
	}
	stage, ok := g.progression.Stage(stageID)
	if !ok {
		return false
	}
	g.currentStageID = stage.ID
	g.backfillDevStageJump(stage.ID)
	g.pendingBuildRecipeID = ""
	g.tutorialLines = nil
	g.perkChoice = nil
	g.saveSoon()
	return true
}

func (g *Game) backfillDevStageJump(stageID string) {
	switch stageID {
	case "smelting":
		g.grantDevResource(core.ResourceStone, 12)
		g.grantDevResource(core.ResourceIronOre, 6)
		g.grantDevResource(core.ResourceCopperOre, 3)
		g.grantDevResource(core.ResourceCoal, 8)
		g.grantDevResource(core.ResourceFieldData, 8)
		g.gateUplinkUnlocked = true
		g.starterGateCount = maxIntLocal(g.starterGateCount, 1)
		if g.minedTotals == nil {
			g.minedTotals = map[core.ResourceType]int{}
		}
		g.minedTotals[core.ResourceStone] = maxIntLocal(g.minedTotals[core.ResourceStone], 10)
		g.minedTotals[core.ResourceIronOre] = maxIntLocal(g.minedTotals[core.ResourceIronOre], 1)
		g.minedTotals[core.ResourceCopperOre] = maxIntLocal(g.minedTotals[core.ResourceCopperOre], 3)
		g.minedTotals[core.ResourceCoal] = maxIntLocal(g.minedTotals[core.ResourceCoal], 8)
	case "coal_power":
		g.backfillDevStageJump("smelting")
		g.grantDevResource(core.ResourceStone, 16)
		g.grantDevResource(core.ResourceIronIngot, 4)
		g.grantDevResource(core.ResourceCopperIngot, 3)
		g.grantDevResource(core.ResourceCoal, 16)
		g.grantDevResource(core.ResourceFieldData, 18)
		if g.knownRecipes == nil {
			g.knownRecipes = map[string]bool{}
		}
		g.knownRecipes["smelter"] = true
		if g.minedTotals == nil {
			g.minedTotals = map[core.ResourceType]int{}
		}
		g.minedTotals[core.ResourceCoal] = maxIntLocal(g.minedTotals[core.ResourceCoal], 16)
		g.minedTotals[core.ResourceIronIngot] = maxIntLocal(g.minedTotals[core.ResourceIronIngot], 1)
	case "assembly":
		g.backfillDevStageJump("coal_power")
		g.grantDevResource(core.ResourceStone, 24)
		g.grantDevResource(core.ResourceIronIngot, 12)
		g.grantDevResource(core.ResourceCopperIngot, 6)
		g.grantDevResource(core.ResourceCoal, 20)
		g.grantDevResource(core.ResourceFieldData, 32)
		if g.knownRecipes == nil {
			g.knownRecipes = map[string]bool{}
		}
		g.knownRecipes["generator"] = true
		g.knownRecipes["miner"] = true
		if g.minedTotals == nil {
			g.minedTotals = map[core.ResourceType]int{}
		}
		g.minedTotals[core.ResourceIronIngot] = maxIntLocal(g.minedTotals[core.ResourceIronIngot], 10)
		g.minedTotals[core.ResourceCopperIngot] = maxIntLocal(g.minedTotals[core.ResourceCopperIngot], 3)
	}
}

func (g *Game) grantDevResource(resource core.ResourceType, minimum int) {
	if resource == core.ResourceNone || minimum <= 0 {
		return
	}
	if g.inventory == nil {
		g.inventory = map[core.ResourceType]int{}
	}
	if g.inventory[resource] < minimum {
		g.inventory[resource] = minimum
	}
}

func maxIntLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minIntLocal(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (g *Game) handleSettingsInput() {
	g.handleStageJumpKeys()
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
		return
	}
	toggleX, toggleY, toggleW, toggleH := g.creaturesToggleRect()
	if g.pointInRect(float64(x), float64(y), toggleX, toggleY, toggleW, toggleH) {
		g.creaturesEnabled = !g.creaturesEnabled
		g.saveSoon()
		return
	}
	if stageID, ok := g.pickStageJump(x, y); ok {
		g.jumpToStage(stageID)
	}
}

func (g *Game) finishSettingsGesture(x, y int) {
	return
}

func (g *Game) drawScreenBackButton(screen *ebiten.Image) {
	g.drawArrowBackButton(screen)
}

func (g *Game) handleTechInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleTechTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleTechTap(x, y)
	}
}

func (g *Game) handleTechTap(x, y int) {
	backX, backY, backW, backH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), backX, backY, backW, backH) {
		g.mode = modeStrategic
		return
	}
	recipeID, ok := g.pickTechRecipe(x, y)
	if !ok {
		return
	}
	if !g.canUnlockRecipe(recipeID) {
		if g.expandedTechRecipeID == recipeID {
			g.expandedTechRecipeID = ""
			return
		}
		g.expandedTechRecipeID = recipeID
		return
	}
	g.expandedTechRecipeID = ""
	g.unlockRecipeWithGlobalSupply(recipeID)
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
	if !ok {
		return
	}
	if !g.canAffordRecipe(recipeID) {
		if g.expandedBuildRecipeID == recipeID {
			g.expandedBuildRecipeID = ""
			return
		}
		g.expandedBuildRecipeID = recipeID
		return
	}
	g.expandedBuildRecipeID = ""
	g.pendingBuildRecipeID = recipeID
	g.mode = modeTactical
}

func (g *Game) disassembleButtonRect() (float64, float64, float64, float64) {
	buildX, buildY, _, buildH := g.buildButtonRect()
	return buildX - 104, buildY, 94, buildH
}

func (g *Game) settingsButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth)*0.5 - 34, float64(g.screenHeight - 62), 68, 38
}

func (g *Game) techButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 104), float64(g.screenHeight - 62), 88, 38
}

func (g *Game) creaturesToggleRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth)*0.5 - 92, 456, 184, 42
}

func (g *Game) regenerateButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth)*0.5 - 92, 540, 184, 46
}

func resourceLabel(resource core.ResourceType) string {
	if resource == core.ResourceNone {
		return "none"
	}
	switch resource {
	case core.ResourceIronIngot:
		return "iron plate"
	case core.ResourceCopperIngot:
		return "copper plate"
	case core.ResourceGear:
		return "gear"
	case core.ResourceFieldData:
		return "field data"
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

func (g *Game) hasPlacedStarterGate() bool {
	for _, tmap := range g.tacticalMaps {
		for _, tile := range tmap.Tiles {
			if tile.Device != nil && tile.Device.Kind == core.DeviceKindGate && tile.Device.SpecialStarter {
				return true
			}
		}
	}
	return false
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
		} {
			resource, amount, ok := buildPartCost(part)
			if ok {
				costs[resource] += amount
			}
		}
	case core.DeviceKindSmelter:
		for _, part := range []core.DevicePart{
			core.DevicePartMotor,
			core.DevicePartOutput,
			core.DevicePartHandCrank,
		} {
			resource, amount, ok := buildPartCost(part)
			if ok {
				costs[resource] += amount
			}
		}
		costs[core.ResourceStone] += 3
	case core.DeviceKindGenerator:
		for _, part := range []core.DevicePart{
			core.DevicePartMotor,
			core.DevicePartOutput,
			core.DevicePartHandCrank,
		} {
			resource, amount, ok := buildPartCost(part)
			if ok {
				costs[resource] += amount
			}
		}
		costs[core.ResourceStone] += 4
		costs[core.ResourceIronIngot] += 2
	case core.DeviceKindAssembler:
		for _, part := range []core.DevicePart{
			core.DevicePartFrame,
			core.DevicePartFrame,
			core.DevicePartMotor,
			core.DevicePartOutput,
			core.DevicePartHandCrank,
		} {
			resource, amount, ok := buildPartCost(part)
			if ok {
				costs[resource] += amount
			}
		}
		costs[core.ResourceStone] += 4
		costs[core.ResourceIronIngot] += 4
		costs[core.ResourceCopperIngot] += 2
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
	case core.DeviceKindSmelter:
		return color.RGBA{218, 104, 62, 255}
	case core.DeviceKindGate:
		return color.RGBA{88, 188, 214, 255}
	case core.DeviceKindGenerator:
		return color.RGBA{118, 204, 116, 255}
	case core.DeviceKindAssembler:
		return color.RGBA{210, 182, 92, 255}
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

func drawAssemblerGearIcon(screen *ebiten.Image, cx, cy, radius, angle float64, clr color.RGBA) {
	drawDisc(screen, float32(cx), float32(cy), float32(radius), clr)
	for i := 0; i < 8; i++ {
		a := angle + float64(i)*math.Pi/4
		tx := cx + math.Cos(a)*radius*0.92
		ty := cy + math.Sin(a)*radius*0.92
		drawDisc(screen, float32(tx), float32(ty), float32(radius*0.22), clr)
	}
	drawDisc(screen, float32(cx), float32(cy), float32(radius*0.38), color.RGBA{34, 42, 46, 245})
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
