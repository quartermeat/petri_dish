package core

import (
	"image/color"
	"math"
	"sort"
)

type Cell struct {
	ID          int
	Center      Vec3
	Corners     []Vec3
	Neighbors   []int
	Pentagon    bool
	Elevation   float64
	Moisture    float64
	Temperature float64
	Ocean       bool
	BaseColor   color.RGBA
	Data        map[string]float64
	Tags        map[string]bool
}

type Globe struct {
	Radius       float64
	Seed         int64
	Cells        []Cell
	CameraLon    float64
	CameraLat    float64
	SelectedCell int
}

type triangle struct {
	A int
	B int
	C int
}

type midpointKey struct {
	A int
	B int
}

func NewGlobe(radius float64, subdivisions int) *Globe {
	return NewGlobeWithSeed(radius, subdivisions, 0)
}

func NewGlobeWithSeed(radius float64, subdivisions int, seed int64) *Globe {
	vertices, faces := buildIcosphere(subdivisions)
	cells := buildDualCells(vertices, faces, radius)
	return &Globe{
		Radius:       radius,
		Seed:         seed,
		Cells:        cells,
		CameraLon:    0,
		CameraLat:    -0.32,
		SelectedCell: 0,
	}
}

func buildIcosphere(subdivisions int) ([]Vec3, []triangle) {
	t := (1.0 + math.Sqrt(5)) / 2.0
	vertices := []Vec3{
		{-1, t, 0}, {1, t, 0}, {-1, -t, 0}, {1, -t, 0},
		{0, -1, t}, {0, 1, t}, {0, -1, -t}, {0, 1, -t},
		{t, 0, -1}, {t, 0, 1}, {-t, 0, -1}, {-t, 0, 1},
	}
	for i := range vertices {
		vertices[i] = vertices[i].Normalize()
	}

	faces := []triangle{
		{0, 11, 5}, {0, 5, 1}, {0, 1, 7}, {0, 7, 10}, {0, 10, 11},
		{1, 5, 9}, {5, 11, 4}, {11, 10, 2}, {10, 7, 6}, {7, 1, 8},
		{3, 9, 4}, {3, 4, 2}, {3, 2, 6}, {3, 6, 8}, {3, 8, 9},
		{4, 9, 5}, {2, 4, 11}, {6, 2, 10}, {8, 6, 7}, {9, 8, 1},
	}

	for i := 0; i < subdivisions; i++ {
		cache := map[midpointKey]int{}
		next := make([]triangle, 0, len(faces)*4)
		for _, face := range faces {
			ab := midpointIndex(&vertices, cache, face.A, face.B)
			bc := midpointIndex(&vertices, cache, face.B, face.C)
			ca := midpointIndex(&vertices, cache, face.C, face.A)
			next = append(next,
				triangle{A: face.A, B: ab, C: ca},
				triangle{A: face.B, B: bc, C: ab},
				triangle{A: face.C, B: ca, C: bc},
				triangle{A: ab, B: bc, C: ca},
			)
		}
		faces = next
	}

	return vertices, faces
}

func midpointIndex(vertices *[]Vec3, cache map[midpointKey]int, a, b int) int {
	if a > b {
		a, b = b, a
	}
	key := midpointKey{A: a, B: b}
	if idx, ok := cache[key]; ok {
		return idx
	}
	mid := (*vertices)[a].Add((*vertices)[b]).Normalize()
	*vertices = append(*vertices, mid)
	idx := len(*vertices) - 1
	cache[key] = idx
	return idx
}

func buildDualCells(vertices []Vec3, faces []triangle, radius float64) []Cell {
	faceCenters := make([]Vec3, len(faces))
	facesByVertex := make([][]int, len(vertices))
	neighbors := make([]map[int]struct{}, len(vertices))
	for i := range neighbors {
		neighbors[i] = map[int]struct{}{}
	}

	for i, face := range faces {
		center := vertices[face.A].Add(vertices[face.B]).Add(vertices[face.C]).Normalize()
		faceCenters[i] = center.Mul(radius)
		facesByVertex[face.A] = append(facesByVertex[face.A], i)
		facesByVertex[face.B] = append(facesByVertex[face.B], i)
		facesByVertex[face.C] = append(facesByVertex[face.C], i)

		neighbors[face.A][face.B] = struct{}{}
		neighbors[face.A][face.C] = struct{}{}
		neighbors[face.B][face.A] = struct{}{}
		neighbors[face.B][face.C] = struct{}{}
		neighbors[face.C][face.A] = struct{}{}
		neighbors[face.C][face.B] = struct{}{}
	}

	cells := make([]Cell, len(vertices))
	for i, center := range vertices {
		localUp := center.Normalize()
		reference := Vec3{Y: 1}
		if math.Abs(localUp.Dot(reference)) > 0.95 {
			reference = Vec3{X: 1}
		}
		tangent := reference.Cross(localUp).Normalize()
		bitangent := localUp.Cross(tangent).Normalize()

		type orderedCorner struct {
			angle  float64
			corner Vec3
		}
		ordered := make([]orderedCorner, 0, len(facesByVertex[i]))
		for _, fi := range facesByVertex[i] {
			corner := faceCenters[fi]
			dir := corner.Normalize().Sub(localUp.Mul(localUp.Dot(corner.Normalize()))).Normalize()
			angle := math.Atan2(dir.Dot(bitangent), dir.Dot(tangent))
			ordered = append(ordered, orderedCorner{angle: angle, corner: corner})
		}
		sort.Slice(ordered, func(a, b int) bool {
			return ordered[a].angle < ordered[b].angle
		})

		corners := make([]Vec3, 0, len(ordered))
		for _, entry := range ordered {
			corners = append(corners, entry.corner)
		}

		neighborIDs := make([]int, 0, len(neighbors[i]))
		for id := range neighbors[i] {
			neighborIDs = append(neighborIDs, id)
		}
		sort.Ints(neighborIDs)

		cells[i] = Cell{
			ID:          i,
			Center:      center.Mul(radius),
			Corners:     corners,
			Neighbors:   neighborIDs,
			Pentagon:    len(corners) == 5,
			Data:        map[string]float64{},
			Tags:        map[string]bool{},
			Temperature: 1 - math.Abs(center.Y),
		}
	}

	return cells
}
