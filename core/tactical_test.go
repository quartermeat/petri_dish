package core

import "testing"

func TestCoalGeneratorMinerLoopIsNetPositive(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{ResourceCoal: 1},
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
				Device:            &DeviceLayout{Kind: DeviceKindMiner},
			},
		},
	}
	inventory := map[ResourceType]int{}
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

	if tmap.Supply[ResourceCoal] <= 1 {
		t.Fatalf("expected coal loop to be net-positive in local supply, got local coal=%d global coal=%d mined=%d", tmap.Supply[ResourceCoal], inventory[ResourceCoal], mined[ResourceCoal])
	}
}

func TestMinersOutputLocalGateStorage(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{},
		Tiles: []TacticalTile{
			{
				ID:     0,
				Device: &DeviceLayout{Kind: DeviceKindGate},
			},
			{
				ID:                1,
				Resource:          ResourceIronOre,
				ResourceRichness:  1,
				ResourceRemaining: 120,
				PowerBuffer:       1,
				Device:            &DeviceLayout{Kind: DeviceKindMiner, SpecialStarter: true},
			},
			{
				ID:                2,
				Resource:          ResourceCopperOre,
				ResourceRichness:  1,
				ResourceRemaining: 120,
				PowerBuffer:       1,
				Device:            &DeviceLayout{Kind: DeviceKindMiner},
			},
		},
	}
	global := map[ResourceType]int{}
	mined := map[ResourceType]int{}
	mods := &ProductionMods{OutputMul: 1, PowerCostMul: 1, DecayMul: 1}

	tmap.Produce(3, global, mined, mods)

	if tmap.Supply[ResourceIronOre] == 0 {
		t.Fatal("expected starter MUG miner to output to local GATE storage")
	}
	if tmap.Supply[ResourceCopperOre] == 0 {
		t.Fatal("expected automated miner to output to local GATE storage")
	}
	if global[ResourceIronOre] != 0 {
		t.Fatalf("expected starter MUG not to dump iron into global inventory, got %d", global[ResourceIronOre])
	}
	if global[ResourceCopperOre] != 0 {
		t.Fatalf("expected automated miner not to dump copper into global inventory, got %d", global[ResourceCopperOre])
	}
}

func TestSmelterConsumesAndOutputsLocalSupply(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{
			ResourceIronOre: 2,
			ResourceCoal:    2,
		},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{1, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: &DeviceLayout{Kind: DeviceKindGate}},
			{
				ID:          1,
				Q:           1,
				R:           0,
				PowerBuffer: 1,
				Device: &DeviceLayout{
					Kind:        DeviceKindSmelter,
					ConfigInput: ResourceIronOre,
				},
			},
		},
	}
	global := map[ResourceType]int{}
	mined := map[ResourceType]int{}
	mods := &ProductionMods{SmelterOutputMul: 1, SmelterPowerMul: 1, DecayMul: 1}

	tmap.Produce(10, global, mined, mods)

	if tmap.Supply[ResourceIronIngot] == 0 {
		t.Fatal("expected smelter output to stay in local region supply")
	}
	if global[ResourceIronIngot] != 0 {
		t.Fatalf("expected smelter not to dump output into global inventory, got %d", global[ResourceIronIngot])
	}
}

func TestTacticalEntitiesUseTileGridAndAvoidDevices(t *testing.T) {
	tmap := &TacticalMap{
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{1, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: NewDeviceLayout(5, 5)},
			{ID: 1, Q: 1, R: 0, Device: &DeviceLayout{Kind: DeviceKindGate}},
		},
		Entities: []TacticalEntity{
			{ID: 0, MicroCellID: -1, TileID: 0, StepTicks: 1},
		},
	}

	for i := 0; i < 20; i++ {
		tmap.Update()
	}

	if got := tmap.Entities[0].TileID; got != 0 {
		t.Fatalf("expected entity to avoid device tile and stay on tile 0, got %d", got)
	}
}

func TestCoalPowerResearchIncludesAutomatedMiner(t *testing.T) {
	progression := DefaultProgressionBook()
	stage, ok := progression.Stage("coal_power")
	if !ok {
		t.Fatal("expected coal_power stage")
	}
	if !containsString(stage.KnownRecipes, "generator") {
		t.Fatal("expected generator research in coal_power")
	}
	if !containsString(stage.KnownRecipes, "miner") {
		t.Fatal("expected automated miner research in coal_power")
	}

	recipes := DefaultRecipeBook()
	miner, ok := recipes.Recipe("miner")
	if !ok {
		t.Fatal("expected miner recipe")
	}
	if miner.StageID != "coal_power" {
		t.Fatalf("expected miner to unlock in coal_power, got %q", miner.StageID)
	}
	for _, cell := range miner.Pattern {
		if cell.Part == DevicePartHandCrank {
			t.Fatal("automated miner recipe should not include a hand crank")
		}
	}
	for _, ingredient := range miner.Ingredients {
		if ingredient.RecipeID == "crank" {
			t.Fatal("automated miner should not require a crank ingredient")
		}
	}
}

func TestAssemblyStageIncludesAssemblerAndGearProduction(t *testing.T) {
	progression := DefaultProgressionBook()
	coal, ok := progression.Stage("coal_power")
	if !ok {
		t.Fatal("expected coal_power stage")
	}
	if coal.NextStageID != "assembly" {
		t.Fatalf("expected coal_power to advance to assembly, got %q", coal.NextStageID)
	}
	stage, ok := progression.Stage("assembly")
	if !ok {
		t.Fatal("expected assembly stage")
	}
	if !containsString(stage.KnownRecipes, "assembler") {
		t.Fatal("expected assembler research in assembly")
	}

	recipes := DefaultRecipeBook()
	assembler, ok := recipes.Recipe("assembler")
	if !ok {
		t.Fatal("expected assembler recipe")
	}
	if assembler.Device != DeviceKindAssembler {
		t.Fatalf("expected assembler recipe to build assembler, got %v", assembler.Device)
	}

	tmap := &TacticalMap{
		Supply: map[ResourceType]int{
			ResourceIronIngot:   2,
			ResourceCopperIngot: 1,
		},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{1, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: &DeviceLayout{Kind: DeviceKindGate}},
			{ID: 1, Q: 1, R: 0, PowerBuffer: 1, ResourceCarry: 0.99, Device: &DeviceLayout{Kind: DeviceKindAssembler}},
		},
	}
	mined := map[ResourceType]int{}
	tmap.Produce(1, nil, mined, &ProductionMods{})

	if tmap.Supply[ResourceGear] == 0 {
		t.Fatal("expected assembler to produce gears into local supply")
	}
	if mined[ResourceGear] == 0 {
		t.Fatal("expected gear production to count toward progression totals")
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
