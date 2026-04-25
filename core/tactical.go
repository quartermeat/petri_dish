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

type ResourceType string

const (
	ResourceNone        ResourceType = ""
	ResourceStone       ResourceType = "stone"
	ResourceIronOre     ResourceType = "iron ore"
	ResourceCopperOre   ResourceType = "copper ore"
	ResourceIronIngot   ResourceType = "iron ingot"
	ResourceCopperIngot ResourceType = "copper ingot"
	ResourceCoal        ResourceType = "coal"
	ResourceCrystal     ResourceType = "crystal"
)

type DevicePart int

const (
	DevicePartEmpty DevicePart = iota
	DevicePartFrame
	DevicePartDrill
	DevicePartMotor
	DevicePartOutput
	DevicePartHandCrank
)

type PortChannel string

const (
	ChannelPower PortChannel = "power"
	ChannelItem  PortChannel = "item"
)

type PortKind string

const (
	PortInput  PortKind = "input"
	PortOutput PortKind = "output"
)

type PortDef struct {
	Kind     PortKind
	Channel  PortChannel
	Side     int
	Resource ResourceType
}

type PartDef struct {
	Part          DevicePart
	Label         string
	Cost          map[ResourceType]int
	PowerGenerate float64
	PowerConsume  float64
	Ports         []PortDef
}

type DeviceDef struct {
	Kind              DeviceKind
	Label             string
	Ports             []PortDef
	RunPowerCost      float64
	OutputPerSecond   float64
	RequiresPoolRoute bool
}

type DeviceLayout struct {
	Width  int
	Height int
	Parts  []DevicePart
	Kind   DeviceKind
}

type DeviceKind int

const (
	DeviceKindNone DeviceKind = iota
	DeviceKindMiner
)

type TacticalTile struct {
	ID                int
	Q                 int
	R                 int
	S                 int
	Center            Vec3
	Elevation         float64
	Moisture          float64
	Fill              color.RGBA
	Resource          ResourceType
	ResourceRichness  float64
	ResourceRemaining float64
	ResourceCarry     float64
	PowerBuffer       float64
	Device            *DeviceLayout
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
			res, richness := tacticalResource(cell, q, r, elevation, moisture)
			tmap.tileIndex[[2]int{q, r}] = id
			tmap.Tiles = append(tmap.Tiles, TacticalTile{
				ID:                id,
				Q:                 q,
				R:                 r,
				S:                 s,
				Center:            axialToWorld(q, r),
				Elevation:         elevation,
				Moisture:          moisture,
				Fill:              fill,
				Resource:          res,
				ResourceRichness:  richness,
				ResourceRemaining: richness * 120,
				Device:            NewDeviceLayout(5, 5),
			})
			id++
		}
	}

	if !cell.Ocean {
		tmap.ensureResourcePresence(ResourceIronOre, func(tile TacticalTile) float64 {
			return tile.Elevation*1.3 + (1-tile.Moisture)*0.6
		}, 0.52)
		tmap.ensureResourcePresence(ResourceCoal, func(tile TacticalTile) float64 {
			return (1-tile.Moisture)*1.2 + (1-math.Abs(tile.Elevation-0.45))*0.4
		}, 0.44)
		tmap.ensureResourcePresence(ResourceCrystal, func(tile TacticalTile) float64 {
			return tile.Moisture*1.2 + tile.Elevation*0.5
		}, 0.34)
	}

	tmap.buildMicroCells()
	tmap.spawnEntities(cell)
	return tmap
}

func NewDeviceLayout(width, height int) *DeviceLayout {
	return &DeviceLayout{
		Width:  width,
		Height: height,
		Parts:  make([]DevicePart, width*height),
	}
}

func (d *DeviceLayout) PartAt(x, y int) DevicePart {
	if x < 0 || y < 0 || x >= d.Width || y >= d.Height {
		return DevicePartEmpty
	}
	return d.Parts[y*d.Width+x]
}

func (d *DeviceLayout) SetPart(x, y int, part DevicePart) {
	if x < 0 || y < 0 || x >= d.Width || y >= d.Height {
		return
	}
	d.Parts[y*d.Width+x] = part
	d.Kind = DeviceKindNone
}

func (d *DeviceLayout) IsMiner() bool {
	return d.FindBlueprint() == DeviceKindMiner
}

func (d *DeviceLayout) FindBlueprint() DeviceKind {
	for y := 1; y < d.Height-2; y++ {
		for x := 1; x < d.Width-1; x++ {
			if d.PartAt(x, y) == DevicePartDrill &&
				d.PartAt(x, y-1) == DevicePartMotor &&
				d.PartAt(x-1, y) == DevicePartFrame &&
				d.PartAt(x+1, y) == DevicePartFrame &&
				d.PartAt(x, y+1) == DevicePartOutput &&
				d.PartAt(x, y+2) == DevicePartHandCrank {
				return DeviceKindMiner
			}
		}
	}
	return DeviceKindNone
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

func (m *TacticalMap) Produce(dt float64, inventory map[ResourceType]int) {
	for i := range m.Tiles {
		tile := &m.Tiles[i]
		if tile.Device == nil || tile.Resource == ResourceNone || tile.ResourceRemaining <= 0 {
			continue
		}
		if tile.Device.Kind != DeviceKindMiner {
			continue
		}
		def := DeviceDefinition(tile.Device.Kind)
		if def.Kind == DeviceKindNone {
			continue
		}
		tile.PowerBuffer = math.Max(0, tile.PowerBuffer-dt*0.04)
		if tile.PowerBuffer < def.RunPowerCost || !def.RequiresPoolRoute {
			continue
		}
		tile.PowerBuffer -= def.RunPowerCost
		rate := def.OutputPerSecond + tile.ResourceRichness*0.42
		produced := math.Min(tile.ResourceRemaining, rate*dt)
		if produced <= 0 {
			continue
		}
		tile.ResourceRemaining -= produced
		tile.ResourceCarry += produced
		whole := int(tile.ResourceCarry)
		if whole > 0 {
			inventory[tile.Resource] += whole
			tile.ResourceCarry -= float64(whole)
		}
	}
}

func (m *TacticalMap) ensureResourcePresence(resource ResourceType, score func(TacticalTile) float64, minRichness float64) {
	for _, tile := range m.Tiles {
		if tile.Resource == resource {
			return
		}
	}

	bestIndex := -1
	bestScore := -1.0
	for i, tile := range m.Tiles {
		if tile.Resource == ResourceCrystal || tile.Resource == resource {
			continue
		}
		value := score(tile)
		if value > bestScore {
			bestScore = value
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return
	}

	tile := &m.Tiles[bestIndex]
	tile.Resource = resource
	tile.ResourceRichness = math.Max(tile.ResourceRichness, minRichness)
	tile.ResourceRemaining = tile.ResourceRichness * 120
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

func tacticalResource(cell *Cell, q, r int, elevation, moisture float64) (ResourceType, float64) {
	base := cell.Center.Normalize()
	n := Clamp01(0.5 +
		0.30*math.Sin(float64(q)*0.73+base.X*5.4) +
		0.22*math.Cos(float64(r)*0.61-base.Z*4.2) +
		0.18*math.Sin(float64(q-r)*0.49+base.Y*6.1))
	if cell.Ocean {
		return ResourceStone, 0.2 + n*0.2
	}

	if elevation > 0.70 && moisture < 0.60 && n > 0.38 {
		return ResourceIronOre, 0.40 + n*0.36
	}
	if moisture < 0.34 && elevation < 0.72 && n > 0.34 {
		return ResourceCoal, 0.34 + n*0.34
	}
	if moisture > 0.68 && elevation > 0.42 && n > 0.46 {
		return ResourceCrystal, 0.22 + n*0.26
	}
	if moisture > 0.46 && elevation < 0.76 && n > 0.32 {
		return ResourceCopperOre, 0.28 + n*0.28
	}

	switch {
	case elevation > 0.72:
		return ResourceStone, 0.18 + n*0.16
	case moisture < 0.32:
		return ResourceStone, 0.18 + n*0.16
	case moisture > 0.62:
		return ResourceCopperOre, 0.20 + n*0.16
	default:
		return ResourceStone, 0.15 + n*0.15
	}
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

func ResourceColor(resource ResourceType) color.RGBA {
	switch resource {
	case ResourceIronOre:
		return color.RGBA{132, 166, 198, 255}
	case ResourceIronIngot:
		return color.RGBA{176, 196, 216, 255}
	case ResourceCopperOre:
		return color.RGBA{216, 150, 92, 255}
	case ResourceCopperIngot:
		return color.RGBA{230, 182, 128, 255}
	case ResourceCoal:
		return color.RGBA{82, 84, 90, 255}
	case ResourceCrystal:
		return color.RGBA{147, 228, 246, 255}
	default:
		return color.RGBA{164, 164, 154, 255}
	}
}

func DevicePartColor(part DevicePart) color.RGBA {
	switch part {
	case DevicePartFrame:
		return color.RGBA{122, 132, 148, 255}
	case DevicePartDrill:
		return color.RGBA{212, 168, 110, 255}
	case DevicePartMotor:
		return color.RGBA{136, 188, 218, 255}
	case DevicePartOutput:
		return color.RGBA{152, 214, 164, 255}
	case DevicePartHandCrank:
		return color.RGBA{232, 202, 120, 255}
	default:
		return color.RGBA{28, 34, 44, 255}
	}
}

func DevicePartLabel(part DevicePart) string {
	switch part {
	case DevicePartFrame:
		return "FRAME"
	case DevicePartDrill:
		return "DRILL"
	case DevicePartMotor:
		return "MOTOR"
	case DevicePartOutput:
		return "OUTPUT"
	case DevicePartHandCrank:
		return "CRANK"
	default:
		return "ERASE"
	}
}

func DeviceKindLabel(kind DeviceKind) string {
	switch kind {
	case DeviceKindMiner:
		return "miner"
	default:
		return "idle"
	}
}

func PartDefinition(part DevicePart) PartDef {
	switch part {
	case DevicePartFrame:
		return PartDef{
			Part:  part,
			Label: "FRAME",
			Cost:  map[ResourceType]int{ResourceStone: 1},
		}
	case DevicePartDrill:
		return PartDef{
			Part:  part,
			Label: "DRILL",
			Cost:  map[ResourceType]int{ResourceIronOre: 1},
		}
	case DevicePartMotor:
		return PartDef{
			Part:  part,
			Label: "MOTOR",
			Cost:  map[ResourceType]int{ResourceCopperOre: 1},
			Ports: []PortDef{{Kind: PortInput, Channel: ChannelPower, Side: 0}},
		}
	case DevicePartOutput:
		return PartDef{
			Part:  part,
			Label: "OUTPUT",
			Cost:  map[ResourceType]int{ResourceStone: 1},
			Ports: []PortDef{{Kind: PortOutput, Channel: ChannelItem, Side: 3}},
		}
	case DevicePartHandCrank:
		return PartDef{
			Part:          part,
			Label:         "CRANK",
			Cost:          map[ResourceType]int{ResourceStone: 1},
			PowerGenerate: 1,
			Ports:         []PortDef{{Kind: PortOutput, Channel: ChannelPower, Side: 0}},
		}
	default:
		return PartDef{Part: part, Label: "ERASE"}
	}
}

func DeviceDefinition(kind DeviceKind) DeviceDef {
	switch kind {
	case DeviceKindMiner:
		return DeviceDef{
			Kind:  kind,
			Label: "miner",
			Ports: []PortDef{
				{Kind: PortInput, Channel: ChannelPower, Side: 0},
				{Kind: PortOutput, Channel: ChannelItem, Side: 3},
			},
			RunPowerCost:      0.16,
			OutputPerSecond:   0.18,
			RequiresPoolRoute: true,
		}
	default:
		return DeviceDef{}
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
