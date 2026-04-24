package core

import (
	"image/color"
	"math"
)

type TacticalTile struct {
	ID        int
	Q         int
	R         int
	S         int
	Center    Vec3
	Elevation float64
	Moisture  float64
	Fill      color.RGBA
}

type TacticalMap struct {
	CellID int
	Radius int
	Tiles  []TacticalTile
}

func NewTacticalMap(cell *Cell, radius int) *TacticalMap {
	tiles := make([]TacticalTile, 0, 1+3*radius*(radius+1))
	id := 0
	for q := -radius; q <= radius; q++ {
		rMin := maxInt(-radius, -q-radius)
		rMax := minInt(radius, -q+radius)
		for r := rMin; r <= rMax; r++ {
			s := -q - r
			elevation := tacticalValue(cell, q, r, 0.19, 0.11)
			moisture := tacticalValue(cell, q, r, 0.07, -0.17)
			fill := tacticalTileColor(cell, elevation, moisture)
			tiles = append(tiles, TacticalTile{
				ID:        id,
				Q:         q,
				R:         r,
				S:         s,
				Center:    axialToWorld(q, r),
				Elevation: elevation,
				Moisture:  moisture,
				Fill:      fill,
			})
			id++
		}
	}
	return &TacticalMap{
		CellID: cell.ID,
		Radius: radius,
		Tiles:  tiles,
	}
}

func axialToWorld(q, r int) Vec3 {
	x := math.Sqrt(3) * (float64(q) + float64(r)*0.5)
	y := 1.5 * float64(r)
	return Vec3{X: x, Y: y}
}

func tacticalValue(cell *Cell, q, r int, a, b float64) float64 {
	x := float64(q)
	y := float64(r)
	base := cell.Center.Normalize()
	n := 0.0
	n += 0.55 * math.Sin((x+base.X*5.1)*0.9+(y+base.Z*3.7)*0.7)
	n += 0.30 * math.Cos((x-y)*0.8+base.Y*4.6+a*13)
	n += 0.15 * math.Sin((x+y)*1.3+b*19)
	return Clamp01(0.5 + n*0.35)
}

func tacticalTileColor(cell *Cell, elevation, moisture float64) color.RGBA {
	if cell.Ocean {
		deep := color.RGBA{16, 70, 128, 255}
		shallow := color.RGBA{53, 128, 186, 255}
		return BlendColor(deep, shallow, elevation)
	}
	switch {
	case elevation > 0.82:
		return color.RGBA{142, 129, 103, 255}
	case elevation > 0.68:
		return color.RGBA{112, 132, 98, 255}
	case moisture < 0.28:
		return color.RGBA{186, 166, 104, 255}
	case moisture > 0.66:
		return color.RGBA{76, 141, 84, 255}
	default:
		return color.RGBA{116, 164, 101, 255}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
