package hexglobe

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hex_globe/core"
)

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
	modeSettings
)

type Game struct {
	globe            *core.Globe
	ruleset          core.Ruleset
	screenWidth      int
	screenHeight     int
	mode             viewMode
	dragging         bool
	dragTouchID      ebiten.TouchID
	dragStartX       int
	dragStartY       int
	dragLastX        int
	dragLastY        int
	dragMoved        bool
	zoom             float64
	touchIDs         []ebiten.TouchID
	pinching         bool
	pinchTouchA      ebiten.TouchID
	pinchTouchB      ebiten.TouchID
	pinchPrevGap     float64
	tacticalMaps     map[int]*core.TacticalMap
	tacticalID       int
	tacticalTile     int
	tacticalZoom     float64
	tacticalPanX     float64
	tacticalPanY     float64
	buildPart        core.DevicePart
	inventory        map[core.ResourceType]int
	settingsCard     int
	settingsDown     bool
	settingsX        int
	settingsY        int
	settingsTouch    ebiten.TouchID
	screenshotPath   string
	screenshotFrames int
	screenshotDone   bool
	screenshotErr    error
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

type recipeCard struct {
	title    string
	subtitle string
	lines    []string
}

var solidPixel = ebiten.NewImage(1, 1)

func init() {
	solidPixel.Fill(color.White)
}

func NewGame() *Game {
	globe := core.NewGlobe(1, 3)
	rules := core.NewDemoRuleset()
	rules.Init(globe)
	return &Game{
		globe:         globe,
		ruleset:       rules,
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
		inventory: map[core.ResourceType]int{
			core.ResourceStone:     6,
			core.ResourceIronOre:   1,
			core.ResourceCopperOre: 1,
		},
	}
}

func (g *Game) OpenSettingsForTesting() {
	g.mode = modeSettings
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
	for _, tmap := range g.tacticalMaps {
		tmap.Produce(dt, g.inventory)
	}
	if g.screenshotPath != "" && g.screenshotFrames > 0 {
		g.screenshotFrames--
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
	if g.mode == modeBuild {
		g.drawBuild(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeSettings {
		g.drawSettings(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	if g.mode == modeTactical {
		g.drawTactical(screen)
		g.captureScreenshotIfReady(screen)
		return
	}
	screen.Fill(color.RGBA{8, 14, 30, 255})
	g.drawBackdrop(screen)
	g.drawGlobe(screen)
	g.drawMinimap(screen)
	g.drawStrategicSettingsButton(screen)
	g.drawStrategicEnterButton(screen)
	g.drawStrategicStats(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
	if g.strategicDeviceCount() > 0 {
		enterX, enterY, enterW, _ := g.enterButtonRect()
		deviceH := g.strategicDevicesCardHeight()
		deviceX := enterX + enterW - 170
		deviceY := enterY - 12 - deviceH
		g.drawStrategicDevicesCard(screen, deviceX, deviceY, 1)
	}
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

func (g *Game) finishSelection(x, y int) {
	if g.dragMoved {
		return
	}
	settingsX, settingsY, settingsW, settingsH := g.settingsButtonRect()
	if g.pointInRect(float64(x), float64(y), settingsX, settingsY, settingsW, settingsH) {
		g.mode = modeSettings
		return
	}
	buttonX, buttonY, buttonW, buttonH := g.enterButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		g.enterTactical()
		return
	}
	if cellID, ok := g.pickCellAt(x, y); ok {
		g.globe.SelectedCell = cellID
	}
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
		kinds := g.strategicDeviceKinds(cell.index)
		if len(kinds) == 0 {
			continue
		}
		centerX, centerY, ok := g.projectPoint(cell.center)
		if !ok {
			continue
		}
		for i, kind := range kinds {
			offsetX := (float64(i) - float64(len(kinds)-1)*0.5) * 18
			g.drawStrategicDeviceBadge(screen, centerX+offsetX, centerY-8, kind)
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

func (g *Game) drawStrategicEnterButton(screen *ebiten.Image) {
	x, y, w, h := g.enterButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{21, 86, 112, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{143, 219, 246, 255})
	ebitenutil.DebugPrintAt(screen, "ENTER HEX", int(x)+12, int(y)+12)
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

func (g *Game) strategicDeviceKinds(cellID int) []core.DeviceKind {
	tmap := g.tacticalMapForCell(cellID)
	if tmap == nil {
		return nil
	}
	seen := map[core.DeviceKind]bool{}
	kinds := make([]core.DeviceKind, 0, 3)
	for _, tile := range tmap.Tiles {
		if tile.Device == nil || tile.Device.Kind == core.DeviceKindNone || seen[tile.Device.Kind] {
			continue
		}
		seen[tile.Device.Kind] = true
		kinds = append(kinds, tile.Device.Kind)
	}
	return kinds
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
		y: y0 + (math.Pi/2-lat)/math.Pi*h,
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
	g.drawTacticalBackButton(screen)
	g.drawTacticalBuildButton(screen)
	g.drawTacticalStats(screen)
	g.drawInventoryCard(screen, 16, 16, 1)
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
		shadow := color.RGBA{0, 0, 0, 84}
		body := tacticalDeviceSignalColor(tile)
		drill := color.RGBA{220, 178, 110, 255}
		drawDisc(screen, float32(centerX+2), float32(centerY+4), float32(scale*0.26), shadow)
		drawDisc(screen, float32(centerX), float32(centerY-1), float32(scale*0.22), body)
		drawFilledRect(screen, float32(centerX-scale*0.05), float32(centerY-scale*0.02), float32(scale*0.10), float32(scale*0.27), drill)
		drawFilledRect(screen, float32(centerX-scale*0.16), float32(centerY-scale*0.15), float32(scale*0.32), float32(scale*0.08), body)
		if tile.PowerBuffer > 0.08 {
			drawDisc(screen, float32(centerX+scale*0.2), float32(centerY-scale*0.16), float32(scale*0.07), color.RGBA{250, 238, 170, 220})
		}
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
	ebitenutil.DebugPrintAt(screen, "BACK", int(x)+18, int(y)+12)
}

func (g *Game) drawTacticalBuildButton(screen *ebiten.Image) {
	if g.tacticalTile < 0 {
		return
	}
	x, y, w, h := g.buildButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "BUILD", int(x)+14, int(y)+12)
}

func (g *Game) finishTacticalPointer(x, y int) {
	if g.dragMoved {
		return
	}
	buttonX, buttonY, buttonW, buttonH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		g.mode = modeStrategic
		return
	}
	buildX, buildY, buildW, buildH := g.buildButtonRect()
	if g.tacticalTile >= 0 && g.pointInRect(float64(x), float64(y), buildX, buildY, buildW, buildH) {
		g.mode = modeBuild
		g.buildPart = core.DevicePartFrame
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
	tile.PowerBuffer = math.Min(1, tile.PowerBuffer+0.45)
	return true
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
	op.ColorScale.Scale(1, 1, 1, float32(alpha))
	screen.DrawImage(textImage, op)
}

func (g *Game) enterButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 148), float64(g.screenHeight - 62), 128, 38
}

func (g *Game) backButtonRect() (float64, float64, float64, float64) {
	return 16, float64(g.screenHeight - 62), 88, 38
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

func (g *Game) drawInventoryCard(screen *ebiten.Image, x, y, alpha float64) {
	w := 170.0
	h := 128.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{8, 18, 32, uint8(170 * alpha)})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, uint8(210 * alpha)})
	lines := []string{
		"INVENTORY",
		fmt.Sprintf("stone   %d", g.inventory[core.ResourceStone]),
		fmt.Sprintf("iron ore %d", g.inventory[core.ResourceIronOre]),
		fmt.Sprintf("copper ore %d", g.inventory[core.ResourceCopperOre]),
		fmt.Sprintf("iron ingot %d", g.inventory[core.ResourceIronIngot]),
		fmt.Sprintf("copper ingot %d", g.inventory[core.ResourceCopperIngot]),
		fmt.Sprintf("coal    %d", g.inventory[core.ResourceCoal]),
		fmt.Sprintf("crystal %d", g.inventory[core.ResourceCrystal]),
	}
	g.drawAlphaDebugTextBlock(screen, x+12, y+12, lines, alpha)
}

func (g *Game) drawBuild(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	g.drawBuildBackButton(screen)
	g.drawBuildPalette(screen)
	g.drawBuildGrid(screen)
	g.drawBuildContext(screen)
	g.drawBuildCreateButton(screen)
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
	x := float64(g.screenWidth)*0.5 - 156
	y := 88.0
	w := 312.0
	h := 300.0
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 14, color.RGBA{12, 20, 32, 232})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{126, 176, 210, 255})
	g.drawAlphaDebugTextBlock(screen, x+18, y+18, []string{
		"SETTINGS",
		"Recipe Book",
		"Swipe left or right.",
	}, 1)
	g.drawRecipeBookCard(screen, x+18, y+76, w-36, 182)

	rx, ry, rw, rh := g.regenerateButtonRect()
	g.drawAlphaDebugTextBlock(screen, rx-2, ry-38, []string{
		"New world, clears old tactical state.",
	}, 1)
	drawRoundedRect(screen, float32(rx), float32(ry), float32(rw), float32(rh), 12, color.RGBA{124, 58, 48, 236})
	drawRectOutline(screen, float32(rx), float32(ry), float32(rw), float32(rh), color.RGBA{240, 190, 170, 255})
	ebitenutil.DebugPrintAt(screen, "REGENERATE MAP", int(rx)+18, int(ry)+14)
}

func (g *Game) drawRecipeBookCard(screen *ebiten.Image, x, y, w, h float64) {
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 12, color.RGBA{18, 26, 40, 228})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{92, 112, 138, 255})
	cards := g.settingsRecipeCards()
	if len(cards) == 0 {
		return
	}
	card := cards[g.settingsCard%len(cards)]
	g.drawAlphaDebugTextBlock(screen, x+14, y+14, []string{
		card.title,
		card.subtitle,
	}, 1)
	g.drawRecipeBookMiner(screen, x+18, y+48)
	g.drawRecipeBookCardNotes(screen, x+170, y+52, card.lines)
	if len(cards) > 1 {
		g.drawSettingsPager(screen, x+w*0.5, y+h-14, len(cards), g.settingsCard)
	}
}

func (g *Game) drawRecipeBookMiner(screen *ebiten.Image, x, y float64) {
	cell := 24.0
	gridW := cell * 5
	gridH := cell * 5
	drawRoundedRect(screen, float32(x-10), float32(y-10), float32(gridW+20), float32(gridH+20), 12, color.RGBA{18, 26, 40, 228})
	for gy := 0; gy < 5; gy++ {
		for gx := 0; gx < 5; gx++ {
			px := x + float64(gx)*cell
			py := y + float64(gy)*cell
			drawFilledRect(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{30, 35, 44, 255})
			drawRectOutline(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{74, 86, 102, 255})
		}
	}

	parts := []struct {
		x    int
		y    int
		part core.DevicePart
	}{
		{2, 1, core.DevicePartMotor},
		{1, 2, core.DevicePartFrame},
		{2, 2, core.DevicePartDrill},
		{3, 2, core.DevicePartFrame},
		{2, 3, core.DevicePartOutput},
		{2, 4, core.DevicePartHandCrank},
	}
	for _, entry := range parts {
		px := x + float64(entry.x)*cell + 4
		py := y + float64(entry.y)*cell + 4
		drawFilledRect(screen, float32(px), float32(py), float32(cell-10), float32(cell-10), core.DevicePartColor(entry.part))
	}

	g.drawAlphaDebugTextBlock(screen, x+gridW+18, y+8, []string{
		"M motor",
		"D drill",
		"F frame",
		"O output",
		"H crank",
	}, 1)
	ebitenutil.DebugPrintAt(screen, "M", int(x+2*cell+8), int(y+1*cell+6))
	ebitenutil.DebugPrintAt(screen, "F", int(x+1*cell+8), int(y+2*cell+6))
	ebitenutil.DebugPrintAt(screen, "D", int(x+2*cell+8), int(y+2*cell+6))
	ebitenutil.DebugPrintAt(screen, "F", int(x+3*cell+8), int(y+2*cell+6))
	ebitenutil.DebugPrintAt(screen, "O", int(x+2*cell+8), int(y+3*cell+6))
	ebitenutil.DebugPrintAt(screen, "H", int(x+2*cell+8), int(y+4*cell+6))
}

func (g *Game) drawSettingsPager(screen *ebiten.Image, cx, cy float64, total, current int) {
	for i := 0; i < total; i++ {
		clr := color.RGBA{92, 112, 138, 255}
		if i == current {
			clr = color.RGBA{220, 236, 248, 255}
		}
		drawDisc(screen, float32(cx+float64(i-total/2)*14), float32(cy), 3, clr)
	}
}

func (g *Game) drawRecipeBookCardNotes(screen *ebiten.Image, x, y float64, lines []string) {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) == 0 {
		return
	}
	g.drawAlphaDebugTextBlock(screen, x, y, filtered, 1)
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
		g.regenerateWorld()
	}
}

func (g *Game) finishSettingsGesture(x, y int) {
	dx := x - g.settingsX
	dy := y - g.settingsY
	if absInt(dx) < 40 || absInt(dx) <= absInt(dy) {
		return
	}
	total := len(g.settingsRecipeCards())
	if total <= 1 {
		return
	}
	if dx < 0 {
		g.settingsCard = (g.settingsCard + 1) % total
		return
	}
	g.settingsCard = (g.settingsCard + total - 1) % total
}

func (g *Game) drawBuildBackButton(screen *ebiten.Image) {
	x, y, w, h := g.backButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "BACK", int(x)+18, int(y)+12)
}

func (g *Game) drawBuildPalette(screen *ebiten.Image) {
	parts := []core.DevicePart{core.DevicePartFrame, core.DevicePartDrill, core.DevicePartMotor, core.DevicePartOutput, core.DevicePartHandCrank, core.DevicePartEmpty}
	for i, part := range parts {
		x := 12.0 + float64(i)*68
		y := float64(g.screenHeight - 120)
		drawRoundedRect(screen, float32(x), float32(y), 58, 54, 10, color.RGBA{24, 30, 40, 236})
		border := color.RGBA{96, 112, 130, 255}
		if part == g.buildPart {
			border = color.RGBA{184, 228, 250, 255}
		}
		drawRectOutline(screen, float32(x), float32(y), 58, 54, border)
		drawFilledRect(screen, float32(x+18), float32(y+8), 22, 18, core.DevicePartColor(part))
		if count, ok := g.buildPartOverlayCount(part); ok {
			drawRoundedRect(screen, float32(x+35), float32(y+6), 16, 14, 6, color.RGBA{8, 18, 32, 220})
			ebitenutil.DebugPrintAt(screen, itoa(count), int(x)+39, int(y)+7)
		}
		ebitenutil.DebugPrintAt(screen, core.DevicePartLabel(part), int(x)+6, int(y)+32)
	}
}

func (g *Game) drawBuildGrid(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil {
		return
	}
	x0, y0, cell := g.buildGridMetrics()
	drawRoundedRect(screen, float32(x0-12), float32(y0-12), float32(float64(tile.Device.Width)*cell+24), float32(float64(tile.Device.Height)*cell+24), 12, color.RGBA{16, 20, 26, 236})
	for y := 0; y < tile.Device.Height; y++ {
		for x := 0; x < tile.Device.Width; x++ {
			px := x0 + float64(x)*cell
			py := y0 + float64(y)*cell
			part := tile.Device.PartAt(x, y)
			drawFilledRect(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{30, 35, 44, 255})
			drawRectOutline(screen, float32(px), float32(py), float32(cell-2), float32(cell-2), color.RGBA{74, 86, 102, 255})
			if part != core.DevicePartEmpty {
				drawFilledRect(screen, float32(px+6), float32(py+6), float32(cell-14), float32(cell-14), core.DevicePartColor(part))
			}
		}
	}
}

func (g *Game) drawBuildContext(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil {
		return
	}
	createLabel := "need materials"
	if g.canCreateCurrentBlueprint() {
		createLabel = "ready to create"
	}
	lines := []string{
		fmt.Sprintf("BUILD TILE %d", tile.ID),
		fmt.Sprintf("ore %s", resourceLabel(tile.Resource)),
		fmt.Sprintf("rich %.0f%%", tile.ResourceRichness*100),
		fmt.Sprintf("left %.0f", tile.ResourceRemaining),
		fmt.Sprintf("power %.0f%%", clampRange(tile.PowerBuffer, 0, 1)*100),
		deviceStatusLabel(tile.Device),
		createLabel,
	}
	g.drawAlphaDebugTextBlock(screen, 18, 18, lines, 1)
}

func (g *Game) drawBuildCreateButton(screen *ebiten.Image) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil {
		return
	}
	blueprint := tile.Device.FindBlueprint()
	if blueprint == core.DeviceKindNone || tile.Device.Kind == blueprint {
		return
	}
	x, y, w, h := g.createButtonRect()
	fill := color.RGBA{21, 86, 112, 236}
	border := color.RGBA{143, 219, 246, 255}
	label := "CREATE"
	if !g.canCreateCurrentBlueprint() {
		fill = color.RGBA{54, 62, 76, 228}
		border = color.RGBA{120, 136, 160, 255}
		label = "NEED MAT"
	}
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, fill)
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), border)
	ebitenutil.DebugPrintAt(screen, label, int(x)+12, int(y)+12)
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
	createX, createY, createW, createH := g.createButtonRect()
	if g.pointInRect(float64(x), float64(y), createX, createY, createW, createH) {
		if tile := g.currentTacticalTile(); tile != nil && tile.Device != nil {
			blueprint := tile.Device.FindBlueprint()
			if blueprint != core.DeviceKindNone && tile.Device.Kind != blueprint && g.spendBlueprintCost(blueprint) {
				tile.Device.Kind = blueprint
				tile.PowerBuffer = 0
			}
		}
		return
	}
	if part, ok := g.pickBuildPalettePart(x, y); ok {
		g.buildPart = part
		return
	}
	if gx, gy, ok := g.pickBuildGridCell(x, y); ok {
		if tile := g.currentTacticalTile(); tile != nil && tile.Device != nil {
			tile.Device.SetPart(gx, gy, g.buildPart)
		}
	}
}

func (g *Game) pickBuildPalettePart(x, y int) (core.DevicePart, bool) {
	parts := []core.DevicePart{core.DevicePartFrame, core.DevicePartDrill, core.DevicePartMotor, core.DevicePartOutput, core.DevicePartHandCrank, core.DevicePartEmpty}
	for i, part := range parts {
		px := 12.0 + float64(i)*68
		py := float64(g.screenHeight - 120)
		if g.pointInRect(float64(x), float64(y), px, py, 58, 54) {
			return part, true
		}
	}
	return core.DevicePartEmpty, false
}

func (g *Game) pickBuildGridCell(x, y int) (int, int, bool) {
	tile := g.currentTacticalTile()
	if tile == nil || tile.Device == nil {
		return 0, 0, false
	}
	x0, y0, cell := g.buildGridMetrics()
	if !g.pointInRect(float64(x), float64(y), x0, y0, float64(tile.Device.Width)*cell, float64(tile.Device.Height)*cell) {
		return 0, 0, false
	}
	gx := int((float64(x) - x0) / cell)
	gy := int((float64(y) - y0) / cell)
	if gx < 0 || gy < 0 || gx >= tile.Device.Width || gy >= tile.Device.Height {
		return 0, 0, false
	}
	return gx, gy, true
}

func (g *Game) buildGridMetrics() (float64, float64, float64) {
	cell := 42.0
	x0 := float64(g.screenWidth)*0.5 - cell*2.5
	y0 := 138.0
	return x0, y0, cell
}

func (g *Game) createButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 110), float64(g.screenHeight - 62), 94, 38
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

func (g *Game) settingsRecipeCards() []recipeCard {
	return []recipeCard{
		{
			title:    "Miner",
			subtitle: "Bootstrap Extraction",
			lines: []string{
				"M motor",
				"D drill",
				"F frame x2",
				"O output",
				"H crank",
				"Tap miner to crank.",
			},
		},
	}
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

func (g *Game) regenerateWorld() {
	g.globe = core.NewGlobe(1, 3)
	g.ruleset = core.NewDemoRuleset()
	g.ruleset.Init(g.globe)
	g.tacticalMaps = map[int]*core.TacticalMap{}
	g.tacticalID = -1
	g.tacticalTile = -1
	g.mode = modeStrategic
	g.dragging = false
	g.dragTouchID = -1
	g.pinching = false
	g.pinchTouchA = -1
	g.pinchTouchB = -1
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
