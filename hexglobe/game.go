package hexglobe

import (
	"image/color"
	"math"
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
)

type viewMode int

const (
	modeStrategic viewMode = iota
	modeTactical
)

type Game struct {
	globe        *core.Globe
	ruleset      core.Ruleset
	screenWidth  int
	screenHeight int
	mode         viewMode
	dragging     bool
	dragTouchID  ebiten.TouchID
	dragStartX   int
	dragStartY   int
	dragLastX    int
	dragLastY    int
	dragMoved    bool
	zoom         float64
	touchIDs     []ebiten.TouchID
	pinching     bool
	pinchTouchA  ebiten.TouchID
	pinchTouchB  ebiten.TouchID
	pinchPrevGap float64
	tacticalMaps map[int]*core.TacticalMap
	tacticalID   int
	tacticalTile int
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

var solidPixel = ebiten.NewImage(1, 1)

func init() {
	solidPixel.Fill(color.White)
}

func NewGame() *Game {
	globe := core.NewGlobe(1, 3)
	rules := core.NewDemoRuleset()
	rules.Init(globe)
	return &Game{
		globe:        globe,
		ruleset:      rules,
		screenWidth:  defaultScreenWidth,
		screenHeight: defaultScreenHeight,
		zoom:         1,
		dragTouchID:  -1,
		pinchTouchA:  -1,
		pinchTouchB:  -1,
		tacticalMaps: map[int]*core.TacticalMap{},
		tacticalID:   -1,
		tacticalTile: -1,
	}
}

func (g *Game) Update() error {
	if g.mode == modeTactical {
		g.handleTacticalInput()
		dt := 1.0 / 60.0
		g.ruleset.Update(g.globe, dt)
		return nil
	}
	g.handlePointerInput()
	dt := 1.0 / 60.0
	g.ruleset.Update(g.globe, dt)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.mode == modeTactical {
		g.drawTactical(screen)
		return
	}
	screen.Fill(color.RGBA{8, 14, 30, 255})
	g.drawBackdrop(screen)
	g.drawGlobe(screen)
	g.drawMinimap(screen)
	g.drawStrategicEnterButton(screen)
	ebitenutil.DebugPrintAt(screen, "HEX GLOBE", 16, 18)
	ebitenutil.DebugPrintAt(screen, "tap to select, drag to pan map, pinch to zoom", 16, 38)
	ebitenutil.DebugPrintAt(screen, g.ruleset.Name(), 16, 58)
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
}

func (g *Game) drawMinimap(screen *ebiten.Image) {
	x0 := float64(g.screenWidth - minimapWidth - 16)
	y0 := 16.0
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
	ebitenutil.DebugPrintAt(screen, "TACTICAL HEX", 16, 18)
	ebitenutil.DebugPrintAt(screen, "tap tiles to inspect, back to return", 16, 38)
	if g.tacticalID >= 0 {
		ebitenutil.DebugPrintAt(screen, "world cell "+itoa(g.tacticalID), 16, 58)
	}
}

func (g *Game) drawTacticalMap(screen *ebiten.Image) {
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return
	}
	cx := float64(g.screenWidth) * 0.5
	cy := float64(g.screenHeight) * 0.54
	scale := math.Min(float64(g.screenWidth), float64(g.screenHeight)) * 0.07
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
				DstX: float32(cx + p.x),
				DstY: float32(cy + p.y),
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
	}
}

func (g *Game) drawTacticalBackButton(screen *ebiten.Image) {
	x, y, w, h := g.backButtonRect()
	drawRoundedRect(screen, float32(x), float32(y), float32(w), float32(h), 10, color.RGBA{64, 80, 98, 236})
	drawRectOutline(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{188, 214, 238, 255})
	ebitenutil.DebugPrintAt(screen, "BACK", int(x)+18, int(y)+12)
}

func (g *Game) handleTacticalInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.handleTacticalTap(x, y)
	}
	justTouched := inpututil.AppendJustPressedTouchIDs(nil)
	if len(justTouched) > 0 {
		x, y := ebiten.TouchPosition(justTouched[0])
		g.handleTacticalTap(x, y)
	}
}

func (g *Game) handleTacticalTap(x, y int) {
	buttonX, buttonY, buttonW, buttonH := g.backButtonRect()
	if g.pointInRect(float64(x), float64(y), buttonX, buttonY, buttonW, buttonH) {
		g.mode = modeStrategic
		return
	}
	if tileID, ok := g.pickTacticalTile(x, y); ok {
		g.tacticalTile = tileID
	}
}

func (g *Game) pickTacticalTile(x, y int) (int, bool) {
	tmap := g.currentTacticalMap()
	if tmap == nil {
		return -1, false
	}
	cx := float64(g.screenWidth) * 0.5
	cy := float64(g.screenHeight) * 0.54
	scale := math.Min(float64(g.screenWidth), float64(g.screenHeight)) * 0.07
	p := screenPoint{x: float64(x), y: float64(y)}
	for _, tile := range tmap.Tiles {
		points := tacticalHexPoints(tile.Center, scale)
		poly := make([]screenPoint, 0, len(points))
		for _, point := range points {
			poly = append(poly, screenPoint{x: cx + point.x, y: cy + point.y})
		}
		if pointInPolygon(p, poly) {
			return tile.ID, true
		}
	}
	return -1, false
}

func (g *Game) enterTactical() {
	if g.globe.SelectedCell < 0 || g.globe.SelectedCell >= len(g.globe.Cells) {
		return
	}
	cell := &g.globe.Cells[g.globe.SelectedCell]
	if _, ok := g.tacticalMaps[cell.ID]; !ok {
		g.tacticalMaps[cell.ID] = core.NewTacticalMap(cell, tacticalRadius)
	}
	g.tacticalID = cell.ID
	g.tacticalTile = -1
	g.mode = modeTactical
}

func (g *Game) currentTacticalMap() *core.TacticalMap {
	if g.tacticalID < 0 {
		return nil
	}
	return g.tacticalMaps[g.tacticalID]
}

func (g *Game) clampCamera() {
	limitLat := math.Pi/2 - g.viewAngularRadius()
	limitLon := math.Pi - g.viewAngularRadius()
	g.globe.CameraLat = clampRange(g.globe.CameraLat, -limitLat, limitLat)
	g.globe.CameraLon = clampRange(g.globe.CameraLon, -limitLon, limitLon)
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

func lonLatToVec3(lon, lat float64) core.Vec3 {
	cosLat := math.Cos(lat)
	return core.Vec3{
		X: math.Sin(lon) * cosLat,
		Y: math.Sin(lat),
		Z: math.Cos(lon) * cosLat,
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

func (g *Game) enterButtonRect() (float64, float64, float64, float64) {
	return float64(g.screenWidth - 148), float64(g.screenHeight - 62), 128, 38
}

func (g *Game) backButtonRect() (float64, float64, float64, float64) {
	return 16, float64(g.screenHeight - 62), 88, 38
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
