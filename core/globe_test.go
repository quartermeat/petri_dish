package core

import "testing"

func TestNewGlobeProducesHexSphereTopology(t *testing.T) {
	globe := NewGlobe(1, 2)
	if len(globe.Cells) == 0 {
		t.Fatal("expected cells")
	}

	pentagons := 0
	for _, cell := range globe.Cells {
		if len(cell.Corners) < 5 || len(cell.Corners) > 6 {
			t.Fatalf("cell %d has unexpected side count %d", cell.ID, len(cell.Corners))
		}
		if len(cell.Neighbors) < 5 {
			t.Fatalf("cell %d has too few neighbors: %d", cell.ID, len(cell.Neighbors))
		}
		if cell.Pentagon {
			pentagons++
		}
	}

	if pentagons != 12 {
		t.Fatalf("expected 12 pentagons, got %d", pentagons)
	}
}
