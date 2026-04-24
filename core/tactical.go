package core

import (
	"image/color"
	"math"
)

const tacticalMicroScale = 3

var tacticalDirections = [][2]int{
	{1, 0},
	{1, -1},
	{0, -1},
	{-1, 0},
	{-1, 1},
	{0, 1},
}

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

type TacticalMicroCell struct {
	ID           int
	Q            int
	R            int
	S            int
	ParentTileID int
	Center       Vec3
}

type TacticalEntity struct {
	ID          int
	MicroCellID int
	Fill        color.RGBA
	StepTicks   int
}

type TacticalMap struct {
	CellID        int
	Radius        int
	Tiles         []TacticalTile
	MicroCells    []TacticalMicroCell
	Entities      []TacticalEntity
	microIndex    map[[2]int]int
	tileIndex     map[[2]int]int
	microOccupied map[int]int
	tick          int
}

func NewTacticalMap(cell *Cell, radius int) *TacticalMap {
	tmap := &TacticalMap{
		CellID:        cell.ID,
		Radius:        radius,
		microIndex:    map[[2]int]int{},
		tileIndex:     map[[2]int]int{},
		microOccupied: map[int]int{},
	}

	tmap.Tiles = make([]TacticalTile, 0, 1+3*radius*(radius+1))
	id := 0
	for q := -radius; q <= radius; q++ {
		rMin := maxInt(-radius, -q-radius)
		rMax := minInt(radius, -q+radius)
		for r := rMin; r <= rMax; r++ {
			s := -q - r
			elevation := tacticalValue(cell, q, r, 0.19, 0.11)
			moisture := tacticalValue(cell, q, r, 0.07, -0.17)
			fill := tacticalTileColor(cell, elevation, moisture)
			tmap.tileIndex[[2]int{q, r}] = id
			tmap.Tiles = append(tmap.Tiles, TacticalTile{
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

	tmap.buildMicroCells()
	tmap.spawnEntities(cell)
	return tmap
}

func (m *TacticalMap) buildMicroCells() {
	radius := m.Radius * tacticalMicroScale
	id := 0
	m.MicroCells = make([]TacticalMicroCell, 0, 1+3*radius*(radius+1))
	for q := -radius; q <= radius; q++ {
		rMin := maxInt(-radius, -q-radius)
		rMax := minInt(radius, -q+radius)
		for r := rMin; r <= rMax; r++ {
			parentQ := roundDiv(q, tacticalMicroScale)
			parentR := roundDiv(r, tacticalMicroScale)
			parentID, ok := m.tileIndex[[2]int{parentQ, parentR}]
			if !ok {
				continue
			}
			s := -q - r
			m.microIndex[[2]int{q, r}] = id
			m.MicroCells = append(m.MicroCells, TacticalMicroCell{
				ID:           id,
				Q:            q,
				R:            r,
				S:            s,
				ParentTileID: parentID,
				Center:       axialToWorldFractional(float64(q)/tacticalMicroScale, float64(r)/tacticalMicroScale),
			})
			id++
		}
	}
}

func (m *TacticalMap) spawnEntities(cell *Cell) {
	count := 6
	if cell.Ocean {
		count = 4
	}
	m.Entities = make([]TacticalEntity, 0, count)
	for i := 0; i < count && len(m.MicroCells) > 0; i++ {
		idx := deterministicIndex(cell.ID, i, len(m.MicroCells))
		for tries := 0; tries < len(m.MicroCells); tries++ {
			microID := m.MicroCells[(idx+tries)%len(m.MicroCells)].ID
			if _, occupied := m.microOccupied[microID]; occupied {
				continue
			}
			entityID := len(m.Entities)
			m.Entities = append(m.Entities, TacticalEntity{
				ID:          entityID,
				MicroCellID: microID,
				Fill:        tacticalEntityColor(cell, i),
				StepTicks:   12 + deterministicIndex(cell.ID+17, i+3, 36),
			})
			m.microOccupied[microID] = entityID
			break
		}
	}
}

func (m *TacticalMap) Update() {
	m.tick++
	for i := range m.Entities {
		entity := &m.Entities[i]
		if entity.StepTicks <= 0 || m.tick%entity.StepTicks != 0 {
			continue
		}
		current := m.MicroCells[entity.MicroCellID]
		dirOffset := deterministicIndex(m.CellID+m.tick, entity.ID, len(tacticalDirections))
		for step := 0; step < len(tacticalDirections); step++ {
			dir := tacticalDirections[(dirOffset+step)%len(tacticalDirections)]
			nextID, ok := m.microIndex[[2]int{current.Q + dir[0], current.R + dir[1]}]
			if !ok {
				continue
			}
			if _, occupied := m.microOccupied[nextID]; occupied {
				continue
			}
			delete(m.microOccupied, entity.MicroCellID)
			entity.MicroCellID = nextID
			m.microOccupied[nextID] = entity.ID
			break
		}
	}
}

func axialToWorld(q, r int) Vec3 {
	return axialToWorldFractional(float64(q), float64(r))
}

func axialToWorldFractional(q, r float64) Vec3 {
	x := math.Sqrt(3) * (q + r*0.5)
	y := 1.5 * r
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

func tacticalEntityColor(cell *Cell, index int) color.RGBA {
	if cell.Ocean {
		palette := []color.RGBA{
			{247, 238, 164, 255},
			{255, 206, 117, 255},
			{255, 245, 198, 255},
		}
		return palette[index%len(palette)]
	}
	palette := []color.RGBA{
		{255, 213, 128, 255},
		{241, 245, 252, 255},
		{255, 159, 122, 255},
		{171, 239, 191, 255},
	}
	return palette[index%len(palette)]
}

func deterministicIndex(seed, salt, mod int) int {
	if mod <= 0 {
		return 0
	}
	v := seed*97 + salt*57 + 31
	if v < 0 {
		v = -v
	}
	return v % mod
}

func roundDiv(v, scale int) int {
	if v >= 0 {
		return (v + scale/2) / scale
	}
	return -((-v + scale/2) / scale)
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
