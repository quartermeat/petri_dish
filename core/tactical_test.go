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

func TestGeneratedLandMapHasBeefierIronVeins(t *testing.T) {
	cell := &Cell{ID: 7, Center: Vec3{X: 1, Y: 0.2, Z: -0.4}}
	tmap := NewTacticalMap(cell, 5)
	ironVeins := 0
	totalIron := 0.0
	for _, tile := range tmap.Tiles {
		if tile.Resource != ResourceIronOre {
			continue
		}
		ironVeins++
		totalIron += tile.ResourceRemaining
		if tile.ResourceRemaining != ResourceCapacity(tile.Resource, tile.ResourceRichness) {
			t.Fatalf("expected iron capacity %.2f, got %.2f", ResourceCapacity(tile.Resource, tile.ResourceRichness), tile.ResourceRemaining)
		}
	}
	if ironVeins < 2 {
		t.Fatalf("expected at least two iron veins, got %d", ironVeins)
	}
	if totalIron < 300 {
		t.Fatalf("expected enough early iron to support the solar-prep ramp, got %.2f", totalIron)
	}
}

func TestGeneratorConsumesRegionalCoalWithoutGateAdjacency(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{ResourceCoal: 2},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{3, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: &DeviceLayout{Kind: DeviceKindGate}},
			{ID: 1, Q: 3, R: 0, Device: &DeviceLayout{Kind: DeviceKindGenerator}, ResourceCarry: 0.99},
		},
	}
	tmap.Produce(1, nil, nil, &ProductionMods{GeneratorPowerMul: 1, DecayMul: 1})

	if tmap.Supply[ResourceCoal] >= 2 {
		t.Fatalf("expected remote generator to consume regional coal, got %d", tmap.Supply[ResourceCoal])
	}
}

func TestSolarGeneratorProducesPowerWithoutCoal(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{1, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: &DeviceLayout{Kind: DeviceKindGenerator, ConfigMode: DeviceModeSolar}},
			{ID: 1, Q: 1, R: 0, Device: &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceIronOre}},
		},
	}
	mods := &ProductionMods{GeneratorPowerMul: 1, DecayMul: 1}
	for i := 0; i < 11; i++ {
		tmap.Produce(1, nil, nil, mods)
	}

	if tmap.Supply[ResourceCoal] != 0 {
		t.Fatalf("expected solar generator not to consume coal, got %d", tmap.Supply[ResourceCoal])
	}
	if tmap.Tiles[1].PowerBuffer <= 0 {
		t.Fatal("expected solar generator to provide adjacent power without coal")
	}
}

func TestMinersOutputLocalGateStorage(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{3, 0}: 1,
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
				ID:                1,
				Q:                 3,
				R:                 0,
				Resource:          ResourceIronOre,
				ResourceRichness:  1,
				ResourceRemaining: 120,
				PowerBuffer:       1,
				Device:            &DeviceLayout{Kind: DeviceKindMiner, SpecialStarter: true},
			},
			{
				ID:                2,
				Q:                 0,
				R:                 1,
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

func TestMinerRequiresAdjacentGateStorage(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{},
		tileIndex: map[[2]int]int{
			{0, 0}: 0,
			{2, 0}: 1,
		},
		Tiles: []TacticalTile{
			{ID: 0, Q: 0, R: 0, Device: &DeviceLayout{Kind: DeviceKindGate}},
			{
				ID:                1,
				Q:                 2,
				R:                 0,
				Resource:          ResourceIronOre,
				ResourceRichness:  1,
				ResourceRemaining: 120,
				PowerBuffer:       1,
				Device:            &DeviceLayout{Kind: DeviceKindMiner},
			},
		},
	}
	mined := map[ResourceType]int{}
	tmap.Produce(3, nil, mined, &ProductionMods{OutputMul: 1, PowerCostMul: 1, DecayMul: 1})

	if tmap.Supply[ResourceIronOre] != 0 {
		t.Fatalf("expected non-adjacent miner not to output into GATE storage, got %d", tmap.Supply[ResourceIronOre])
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

func TestSmelterCanBeSwitchedOff(t *testing.T) {
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
				ID:            1,
				Q:             1,
				R:             0,
				PowerBuffer:   1,
				ResourceCarry: 0.99,
				Device:        &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceNone},
			},
		},
	}
	mined := map[ResourceType]int{}
	tmap.Produce(1, nil, mined, &ProductionMods{SmelterOutputMul: 1, SmelterPowerMul: 1, DecayMul: 1})

	if tmap.Supply[ResourceIronOre] != 2 || tmap.Supply[ResourceCoal] != 2 {
		t.Fatalf("expected off smelter not to consume inventory, got iron=%d coal=%d", tmap.Supply[ResourceIronOre], tmap.Supply[ResourceCoal])
	}
	if tmap.Supply[ResourceIronIngot] != 0 {
		t.Fatalf("expected off smelter not to produce plates, got %d", tmap.Supply[ResourceIronIngot])
	}
}

func TestCoalPowerTechIncludesAutomatedMiner(t *testing.T) {
	progression := DefaultProgressionBook()
	stage, ok := progression.Stage("coal_power")
	if !ok {
		t.Fatal("expected coal_power stage")
	}
	if !containsString(stage.KnownRecipes, "generator") {
		t.Fatal("expected generator tech in coal_power")
	}
	if !containsString(stage.KnownRecipes, "miner") {
		t.Fatal("expected automated miner tech in coal_power")
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
		if ingredient.RecipeID != "" {
			t.Fatalf("automated miner should use direct material costs, got nested recipe %q", ingredient.RecipeID)
		}
	}
}

func TestDeviceRecipesUseDirectMaterialCosts(t *testing.T) {
	recipes := DefaultRecipeBook()
	for _, recipeID := range []string{"smelter", "generator", "miner", "assembler"} {
		recipe, ok := recipes.Recipe(recipeID)
		if !ok {
			t.Fatalf("expected %s recipe", recipeID)
		}
		if recipe.Kind != RecipeDevice {
			t.Fatalf("expected %s to be a device recipe", recipeID)
		}
		for _, ingredient := range recipe.Ingredients {
			if ingredient.RecipeID != "" {
				t.Fatalf("%s should use direct material costs, got nested recipe %q", recipeID, ingredient.RecipeID)
			}
			if ingredient.Resource == ResourceNone {
				t.Fatalf("%s has an empty material cost entry", recipeID)
			}
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
		t.Fatal("expected assembler tech in assembly")
	}
	next, ok := progression.Stage(stage.NextStageID)
	if !ok {
		t.Fatalf("expected next stage %q", stage.NextStageID)
	}
	if next.Title != "Solar Preparation" {
		t.Fatalf("expected assembly to lead toward solar preparation, got %q", next.Title)
	}
	if !containsString(next.KnownRecipes, "solar-retrofit") {
		t.Fatal("expected solar preparation to unlock solar retrofit tech")
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
			{1, 0}:  0,
			{1, -1}: 1,
			{0, 1}:  2,
		},
		Tiles: []TacticalTile{
			{ID: 1, Q: 1, R: 0, PowerBuffer: 1, ResourceCarry: 0.99, Device: &DeviceLayout{Kind: DeviceKindAssembler, ConfigInput: ResourceGear}},
			{ID: 2, Q: 1, R: -1, Device: &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceIronOre}},
			{ID: 3, Q: 0, R: 1, Device: &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceCopperOre}},
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

func TestAssemblerCanBeSwitchedOff(t *testing.T) {
	tmap := &TacticalMap{
		Supply: map[ResourceType]int{
			ResourceIronIngot:   2,
			ResourceCopperIngot: 1,
		},
		tileIndex: map[[2]int]int{
			{1, 0}:  0,
			{1, -1}: 1,
			{0, 1}:  2,
		},
		Tiles: []TacticalTile{
			{ID: 1, Q: 1, R: 0, PowerBuffer: 1, ResourceCarry: 0.99, Device: &DeviceLayout{Kind: DeviceKindAssembler, ConfigInput: ResourceNone}},
			{ID: 2, Q: 1, R: -1, Device: &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceIronOre}},
			{ID: 3, Q: 0, R: 1, Device: &DeviceLayout{Kind: DeviceKindSmelter, ConfigInput: ResourceCopperOre}},
		},
	}
	mined := map[ResourceType]int{}
	tmap.Produce(1, nil, mined, &ProductionMods{DecayMul: 1})

	if tmap.Supply[ResourceIronIngot] != 2 || tmap.Supply[ResourceCopperIngot] != 1 {
		t.Fatalf("expected off assembler not to consume plates, got iron=%d copper=%d", tmap.Supply[ResourceIronIngot], tmap.Supply[ResourceCopperIngot])
	}
	if tmap.Supply[ResourceGear] != 0 {
		t.Fatalf("expected off assembler not to produce gears, got %d", tmap.Supply[ResourceGear])
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
