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
)

type Game struct {
	globe        *core.Globe
	ruleset      core.Ruleset
	screenWidth  int
	screenHeight int
	dragging     bool
	dragTouchID  ebiten.TouchID
	dragLastX    int
	dragLastY    int
	autoRotate   float64
}

type drawCell struct {
	index   int
	center  core.Vec3
	corners []core.Vec3
	style   core.CellStyle
	depth   float64
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
		autoRotate:   0.32,
		dragTouchID:  -1,
	}
}

func (g *Game) Update() error {
	g.handlePointerInput()
	dt := 1.0 / 60.0
	if !g.dragging {
		g.globe.RotationY += g.autoRotate * dt
	}
	g.ruleset.Update(g.globe, dt)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{8, 14, 30, 255})
	g.drawBackdrop(screen)
	g.drawGlobe(screen)
	ebitenutil.DebugPrintAt(screen, "HEX GLOBE", 16, 18)
	ebitenutil.DebugPrintAt(screen, "drag to rotate", 16, 38)
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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.dragging = true
		g.dragTouchID = -1
		g.dragLastX, g.dragLastY = ebiten.CursorPosition()
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.dragTouchID == -1 {
		g.dragging = false
	}

	if g.dragTouchID == -1 {
		justTouched := inpututil.AppendJustPressedTouchIDs(nil)
		if len(justTouched) > 0 {
			g.dragging = true
			g.dragTouchID = justTouched[0]
			g.dragLastX, g.dragLastY = ebiten.TouchPosition(g.dragTouchID)
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

func (g *Game) applyDrag(x, y int) {
	dx := x - g.dragLastX
	dy := y - g.dragLastY
	g.dragLastX = x
	g.dragLastY = y
	g.globe.RotationY += float64(dx) * 0.012
	g.globe.TiltX += float64(dy) * 0.006
	g.globe.TiltX = math.Max(-0.9, math.Min(0.25, g.globe.TiltX))
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
	cx := float64(g.screenWidth) * 0.5
	cy := float64(g.screenHeight) * 0.46
	scale := math.Min(float64(g.screenWidth), float64(g.screenHeight)*0.72) * 0.27
	cameraZ := 3.1

	cells := make([]drawCell, 0, len(g.globe.Cells))
	for i := range g.globe.Cells {
		cell := &g.globe.Cells[i]
		style := g.ruleset.StyleCell(g.globe, cell)
		height := 1 + style.Height
		center := core.RotateX(core.RotateY(cell.Center.Mul(height), g.globe.RotationY), g.globe.TiltX)
		viewDir := core.Vec3{Z: cameraZ}.Sub(center).Normalize()
		if center.Normalize().Dot(viewDir) <= 0 {
			continue
		}

		projectedCorners := make([]core.Vec3, 0, len(cell.Corners))
		for _, corner := range cell.Corners {
			projectedCorners = append(projectedCorners, core.RotateX(core.RotateY(corner.Mul(height), g.globe.RotationY), g.globe.TiltX))
		}
		cells = append(cells, drawCell{
			index:   i,
			center:  center,
			corners: projectedCorners,
			style:   style,
			depth:   center.Z,
		})
	}

	sort.Slice(cells, func(i, j int) bool {
		return cells[i].depth < cells[j].depth
	})

	light := core.Vec3{X: -0.5, Y: 0.6, Z: 1}.Normalize()
	for _, cell := range cells {
		points := make([]ebiten.Vertex, 0, len(cell.corners))
		valid := true
		for _, corner := range cell.corners {
			screenX, screenY, ok := projectPoint(corner, cx, cy, scale, cameraZ)
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

func projectPoint(v core.Vec3, cx, cy, scale, cameraZ float64) (float64, float64, bool) {
	denom := cameraZ - v.Z
	if denom <= 0.01 {
		return 0, 0, false
	}
	perspective := scale / denom
	return cx + v.X*perspective, cy + v.Y*perspective, true
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
