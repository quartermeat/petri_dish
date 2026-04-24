package core

import (
	"image/color"
	"math"
)

type CellStyle struct {
	Fill      color.RGBA
	Edge      color.RGBA
	Height    float64
	Highlight float64
}

type Ruleset interface {
	Name() string
	Init(*Globe)
	Update(*Globe, float64)
	StyleCell(*Globe, *Cell) CellStyle
}

type DemoRuleset struct {
	time float64
}

func NewDemoRuleset() *DemoRuleset {
	return &DemoRuleset{}
}

func (r *DemoRuleset) Name() string {
	return "Biome Demo"
}

func (r *DemoRuleset) Init(globe *Globe) {
	firstLand := -1
	for i := range globe.Cells {
		cell := &globe.Cells[i]
		elevation := terrainValue(cell.Center.Normalize())
		moisture := moistureValue(cell.Center.Normalize())
		cell.Elevation = elevation
		cell.Moisture = moisture
		cell.Data["population"] = 0
		cell.Ocean = elevation < 0.5
		cell.Tags["coast"] = math.Abs(elevation-0.5) < 0.045
		cell.BaseColor = biomeColor(cell)
		if firstLand == -1 && !cell.Ocean {
			firstLand = cell.ID
		}
	}
	if firstLand >= 0 {
		globe.SelectedCell = firstLand
	}
}

func (r *DemoRuleset) Update(globe *Globe, dt float64) {
	r.time += dt
}

func (r *DemoRuleset) StyleCell(globe *Globe, cell *Cell) CellStyle {
	fill := cell.BaseColor
	edge := ScaleColor(fill, 0.78)
	height := 0.03
	highlight := 0.0
	if !cell.Ocean {
		height += (cell.Elevation - 0.5) * 0.22
	}
	if cell.Tags["coast"] {
		fill = BlendColor(fill, color.RGBA{220, 212, 164, 255}, 0.35)
		edge = ScaleColor(fill, 0.72)
		height += 0.01
	}
	if cell.ID == globe.SelectedCell {
		pulse := 0.5 + 0.5*math.Sin(r.time*2.2)
		fill = BlendColor(fill, color.RGBA{230, 245, 255, 255}, 0.18+0.18*pulse)
		edge = BlendColor(edge, color.RGBA{170, 240, 255, 255}, 0.4)
		highlight = 0.25 + 0.25*pulse
		height += 0.03
	}
	return CellStyle{
		Fill:      fill,
		Edge:      edge,
		Height:    height,
		Highlight: highlight,
	}
}

func terrainValue(v Vec3) float64 {
	n := 0.0
	n += 0.55 * math.Sin(v.X*3.1+v.Z*1.7)
	n += 0.25 * math.Sin(v.Y*5.3-v.X*2.2)
	n += 0.20 * math.Sin((v.X+v.Y-v.Z)*8.1)
	n += 0.12 * math.Cos(v.Z*11.7+v.Y*4.1)
	return Clamp01(0.5 + n*0.35)
}

func moistureValue(v Vec3) float64 {
	n := 0.0
	n += 0.5 * math.Sin(v.Z*4.4-v.X*2.9)
	n += 0.3 * math.Cos(v.Y*6.2+v.Z*3.3)
	n += 0.2 * math.Sin((v.X-v.Y)*9.1)
	return Clamp01(0.5 + n*0.35)
}

func biomeColor(cell *Cell) color.RGBA {
	lat := math.Abs(cell.Center.Normalize().Y)
	if cell.Ocean {
		deep := color.RGBA{27, 76, 140, 255}
		shallow := color.RGBA{58, 132, 189, 255}
		t := SmootherStep(0.18, 0.56, cell.Elevation)
		if lat > 0.82 {
			return BlendColor(shallow, color.RGBA{214, 228, 236, 255}, SmootherStep(0.82, 0.96, lat))
		}
		return BlendColor(deep, shallow, t)
	}

	switch {
	case lat > 0.78:
		return color.RGBA{228, 234, 232, 255}
	case cell.Elevation > 0.78:
		return color.RGBA{132, 118, 92, 255}
	case cell.Moisture < 0.32:
		return color.RGBA{182, 166, 102, 255}
	case cell.Moisture > 0.66:
		return color.RGBA{82, 148, 88, 255}
	default:
		return color.RGBA{122, 170, 106, 255}
	}
}

func ScaleColor(base color.RGBA, scale float64) color.RGBA {
	return color.RGBA{
		R: uint8(Clamp01(float64(base.R)*scale/255) * 255),
		G: uint8(Clamp01(float64(base.G)*scale/255) * 255),
		B: uint8(Clamp01(float64(base.B)*scale/255) * 255),
		A: base.A,
	}
}

func BlendColor(a, b color.RGBA, t float64) color.RGBA {
	t = Clamp01(t)
	return color.RGBA{
		R: uint8(Lerp(float64(a.R), float64(b.R), t)),
		G: uint8(Lerp(float64(a.G), float64(b.G), t)),
		B: uint8(Lerp(float64(a.B), float64(b.B), t)),
		A: uint8(Lerp(float64(a.A), float64(b.A), t)),
	}
}
