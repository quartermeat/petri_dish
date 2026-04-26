package core

import "testing"

func TestCoalGeneratorMinerLoopIsNetPositive(t *testing.T) {
	tmap := &TacticalMap{
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{1, 0}: 1,
			{0, 1}: 2,
		},
		Tiles: []TacticalTile{
			{
				ID:     0,
				Q:      0,
				R:      0,
				Device: &DeviceLayout{Kind: DeviceKindGate},
			},
			{
				ID:            1,
				Q:             1,
				R:             0,
				Device:        &DeviceLayout{Kind: DeviceKindGenerator},
				PowerBuffer:   1,
				ResourceCarry: 0.99,
			},
			{
				ID:                2,
				Q:                 0,
				R:                 1,
				Resource:          ResourceCoal,
				ResourceRichness:  0.44,
				ResourceRemaining: 120,
				Device:            &DeviceLayout{Kind: DeviceKindMiner, SpecialStarter: true},
			},
		},
	}
	inventory := map[ResourceType]int{ResourceCoal: 1}
	mined := map[ResourceType]int{}
	mods := &ProductionMods{
		OutputMul:         1,
		PowerCostMul:      1,
		GeneratorPowerMul: 1,
		DecayMul:          1,
	}

	for i := 0; i < 60*30; i++ {
		tmap.Produce(1.0/60.0, inventory, mined, mods)
	}

	if inventory[ResourceCoal] <= 1 {
		t.Fatalf("expected coal loop to be net-positive, got coal=%d mined=%d", inventory[ResourceCoal], mined[ResourceCoal])
	}
}
