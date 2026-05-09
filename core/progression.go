package core

type GoalKind string

const (
	GoalMineResource       GoalKind = "mine_resource"
	GoalProduceResource    GoalKind = "produce_resource"
	GoalExportResource     GoalKind = "export_resource"
	GoalDiscoverResource   GoalKind = "discover_resource"
	GoalDiscoverRecipe     GoalKind = "discover_recipe"
	GoalBuildDevice        GoalKind = "build_device"
	GoalPlaceStarterUnit   GoalKind = "place_starter_unit"
	GoalRecoverStarterUnit GoalKind = "recover_starter_unit"
)

type ProgressGoal struct {
	Kind     GoalKind     `json:"kind"`
	Resource ResourceType `json:"resource,omitempty"`
	Device   DeviceKind   `json:"device,omitempty"`
	RecipeID string       `json:"recipeID,omitempty"`
	Amount   int          `json:"amount"`
	Label    string       `json:"label"`
}

type ProgressStage struct {
	ID                  string         `json:"id"`
	Title               string         `json:"title"`
	NextStageID         string         `json:"nextStageID"`
	VisibleResources    []ResourceType `json:"visibleResources,omitempty"`
	KnownRecipes        []string       `json:"knownRecipes,omitempty"`
	Goals               []ProgressGoal `json:"goals"`
	PerkPowerThresholds []float64      `json:"perkPowerThresholds,omitempty"`
	PerkPool            []string       `json:"perkPool,omitempty"`
}

type ProgressionBook struct {
	StartStageID string                   `json:"startStageID"`
	Stages       map[string]ProgressStage `json:"stages"`
}

type RecipeKind string

const (
	RecipePart    RecipeKind = "part"
	RecipeDevice  RecipeKind = "device"
	RecipeUpgrade RecipeKind = "upgrade"
)

type RecipeIngredient struct {
	Resource ResourceType `json:"resource,omitempty"`
	RecipeID string       `json:"recipeID,omitempty"`
	Amount   int          `json:"amount"`
}

type RecipeCell struct {
	X      int        `json:"x"`
	Y      int        `json:"y"`
	Part   DevicePart `json:"part,omitempty"`
	Device DeviceKind `json:"device,omitempty"`
}

type RecipeDef struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Kind        RecipeKind         `json:"kind"`
	Part        DevicePart         `json:"part,omitempty"`
	Device      DeviceKind         `json:"device,omitempty"`
	StageID     string             `json:"stageID,omitempty"`
	Pattern     []RecipeCell       `json:"pattern,omitempty"`
	Ingredients []RecipeIngredient `json:"ingredients"`
}

type RecipeBook struct {
	Recipes map[string]RecipeDef `json:"recipes"`
}

type PerkKind string

const (
	PerkCrankPower      PerkKind = "crank_power"      // +Magnitude (fraction) added to crank input per tap
	PerkMinerOutput     PerkKind = "miner_output"     // +Magnitude (fraction) added to miner output rate
	PerkPowerEfficiency PerkKind = "power_efficiency" // -Magnitude (fraction) of miner power consumption
	PerkSmelterOutput   PerkKind = "smelter_output"   // +Magnitude (fraction) added to smelter output rate
	PerkSmelterPower    PerkKind = "smelter_power"    // -Magnitude (fraction) of smelter power consumption
	PerkGeneratorOutput PerkKind = "generator_output" // +Magnitude (fraction) added to generator output
	PerkBufferDecay     PerkKind = "buffer_decay"     // -Magnitude (fraction) of buffer decay rate
	PerkHoldPower       PerkKind = "hold_power"       // hold on starter miner to transfer power continuously
	PerkResourceGift    PerkKind = "resource_gift"    // one-shot: add Magnitude units of Resource
	PerkGateUplink      PerkKind = "gate_uplink"      // one-shot: unlock GATE uplink/deposit
)

type PerkDef struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Kind        PerkKind     `json:"kind"`
	StageID     string       `json:"stageID,omitempty"`
	Magnitude   float64      `json:"magnitude,omitempty"`
	Resource    ResourceType `json:"resource,omitempty"`
	OneShot     bool         `json:"oneShot,omitempty"`
}

type PerkBook struct {
	Perks map[string]PerkDef `json:"perks"`
}

func DefaultProgressionBook() *ProgressionBook {
	return &ProgressionBook{
		StartStageID: "bootstrap",
		Stages: map[string]ProgressStage{
			"bootstrap": {
				ID:          "bootstrap",
				Title:       "Bootstrap Extraction",
				NextStageID: "smelting",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
				},
				KnownRecipes: nil,
				PerkPowerThresholds: []float64{
					2,
					4,
					8,
				},
				PerkPool: []string{
					"gate-uplink",
					"sturdy-crank",
					"heavy-crank",
					"patient-drill",
					"eager-drill",
					"youve-got-the-power",
					"steady-buffer",
					"stone-cache",
					"iron-cache",
				},
				Goals: []ProgressGoal{
					{
						Kind:   GoalPlaceStarterUnit,
						Label:  "place GATE",
						Device: DeviceKindGate,
						Amount: 1,
					},
					{
						Kind:   GoalPlaceStarterUnit,
						Label:  "place MUG",
						Device: DeviceKindMiner,
						Amount: 1,
					},
					{
						Kind:     GoalMineResource,
						Label:    "mine stone",
						Resource: ResourceStone,
						Amount:   12,
					},
					{
						Kind:     GoalDiscoverResource,
						Label:    "find iron ore",
						Resource: ResourceIronOre,
						Amount:   1,
					},
					{
						Kind:     GoalMineResource,
						Label:    "collect field data",
						Resource: ResourceFieldData,
						Amount:   4,
					},
					{
						Kind:     GoalExportResource,
						Label:    "transfer field data",
						Resource: ResourceFieldData,
						Amount:   4,
					},
				},
			},
			"smelting": {
				ID:          "smelting",
				Title:       "Smelting Basics",
				NextStageID: "coal_power",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
					ResourceCoal,
				},
				KnownRecipes: []string{
					"smelter",
				},
				PerkPowerThresholds: []float64{
					80,
					220,
					420,
				},
				PerkPool: []string{
					"hot-furnace",
					"blast-draft",
					"clean-burn",
					"insulated-bricks",
					"coal-cache",
					"iron-plate-cache",
					"copper-plate-cache",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalDiscoverResource,
						Label:    "find coal",
						Resource: ResourceCoal,
						Amount:   1,
					},
					{
						Kind:     GoalMineResource,
						Label:    "mine coal",
						Resource: ResourceCoal,
						Amount:   8,
					},
					{
						Kind:     GoalMineResource,
						Label:    "mine copper ore",
						Resource: ResourceCopperOre,
						Amount:   3,
					},
					{
						Kind:     GoalDiscoverRecipe,
						Label:    "discover smelter",
						RecipeID: "smelter",
						Amount:   1,
					},
					{
						Kind:   GoalBuildDevice,
						Label:  "build smelter",
						Device: DeviceKindSmelter,
						Amount: 1,
					},
					{
						Kind:     GoalProduceResource,
						Label:    "make iron plate",
						Resource: ResourceIronIngot,
						Amount:   1,
					},
				},
			},
			"coal_power": {
				ID:          "coal_power",
				Title:       "Coal Power",
				NextStageID: "assembly",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
					ResourceCoal,
					ResourceIronIngot,
					ResourceCopperIngot,
				},
				KnownRecipes: []string{
					"generator",
					"miner",
				},
				PerkPowerThresholds: []float64{
					120,
					320,
					620,
				},
				PerkPool: []string{
					"high-pressure-boiler",
					"coal-saver",
					"clean-burn",
					"insulated-bricks",
					"coal-cache",
					"iron-plate-cache",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalDiscoverRecipe,
						Label:    "discover generator",
						RecipeID: "generator",
						Amount:   1,
					},
					{
						Kind:   GoalBuildDevice,
						Label:  "build generator",
						Device: DeviceKindGenerator,
						Amount: 1,
					},
					{
						Kind:     GoalDiscoverRecipe,
						Label:    "discover miner",
						RecipeID: "miner",
						Amount:   1,
					},
					{
						Kind:   GoalBuildDevice,
						Label:  "build miner",
						Device: DeviceKindMiner,
						Amount: 1,
					},
					{
						Kind:     GoalMineResource,
						Label:    "mine coal",
						Resource: ResourceCoal,
						Amount:   16,
					},
					{
						Kind:     GoalProduceResource,
						Label:    "make iron plates",
						Resource: ResourceIronIngot,
						Amount:   10,
					},
				},
			},
			"assembly": {
				ID:          "assembly",
				Title:       "Assembly Ecology",
				NextStageID: "mechanics",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
					ResourceCoal,
					ResourceIronIngot,
					ResourceCopperIngot,
					ResourceGear,
				},
				KnownRecipes: []string{
					"assembler",
				},
				PerkPowerThresholds: []float64{
					180,
					420,
					780,
				},
				PerkPool: []string{
					"sturdy-crank",
					"heavy-crank",
					"hot-furnace",
					"blast-draft",
					"high-pressure-boiler",
					"coal-saver",
					"gear-cache",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalDiscoverRecipe,
						Label:    "discover assembler",
						RecipeID: "assembler",
						Amount:   1,
					},
					{
						Kind:   GoalBuildDevice,
						Label:  "build assembler",
						Device: DeviceKindAssembler,
						Amount: 1,
					},
					{
						Kind:     GoalProduceResource,
						Label:    "make gears",
						Resource: ResourceGear,
						Amount:   3,
					},
				},
			},
			"mechanics": {
				ID:          "mechanics",
				Title:       "Solar Preparation",
				NextStageID: "",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
					ResourceCoal,
					ResourceIronIngot,
					ResourceCopperIngot,
					ResourceGear,
				},
				KnownRecipes: []string{
					"solar-retrofit",
				},
				PerkPowerThresholds: []float64{
					180,
					420,
					780,
				},
				PerkPool: []string{
					"sturdy-crank",
					"heavy-crank",
					"hot-furnace",
					"blast-draft",
					"high-pressure-boiler",
					"coal-saver",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalProduceResource,
						Label:    "stockpile solar gears",
						Resource: ResourceGear,
						Amount:   8,
					},
					{
						Kind:     GoalDiscoverRecipe,
						Label:    "unlock solar retrofit",
						RecipeID: "solar-retrofit",
						Amount:   1,
					},
				},
			},
		},
	}
}

func (b *ProgressionBook) Stage(id string) (ProgressStage, bool) {
	if b == nil {
		return ProgressStage{}, false
	}
	stage, ok := b.Stages[id]
	return stage, ok
}

func DefaultRecipeBook() *RecipeBook {
	return &RecipeBook{
		Recipes: map[string]RecipeDef{
			"frame": {
				ID:      "frame",
				Title:   "Frame",
				Kind:    RecipePart,
				Part:    DevicePartFrame,
				StageID: "smelting",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceIronIngot, Amount: 2},
					{Resource: ResourceStone, Amount: 1},
				},
			},
			"drill": {
				ID:      "drill",
				Title:   "Drill",
				Kind:    RecipePart,
				Part:    DevicePartDrill,
				StageID: "bootstrap",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 1},
					{Resource: ResourceIronOre, Amount: 2},
				},
			},
			"motor": {
				ID:      "motor",
				Title:   "Motor",
				Kind:    RecipePart,
				Part:    DevicePartMotor,
				StageID: "bootstrap",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 1},
					{Resource: ResourceIronOre, Amount: 1},
					{Resource: ResourceCopperOre, Amount: 2},
				},
			},
			"output": {
				ID:      "output",
				Title:   "Output",
				Kind:    RecipePart,
				Part:    DevicePartOutput,
				StageID: "bootstrap",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 2},
					{Resource: ResourceCopperOre, Amount: 1},
				},
			},
			"crank": {
				ID:      "crank",
				Title:   "Hand Crank",
				Kind:    RecipePart,
				Part:    DevicePartHandCrank,
				StageID: "bootstrap",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 2},
					{Resource: ResourceCopperOre, Amount: 1},
				},
			},
			"miner": {
				ID:      "miner",
				Title:   "Miner",
				Kind:    RecipeDevice,
				Device:  DeviceKindMiner,
				StageID: "coal_power",
				Pattern: []RecipeCell{
					{X: 2, Y: 1, Part: DevicePartMotor},
					{X: 1, Y: 2, Part: DevicePartFrame},
					{X: 2, Y: 2, Part: DevicePartDrill},
					{X: 3, Y: 2, Part: DevicePartFrame},
					{X: 2, Y: 3, Part: DevicePartOutput},
				},
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 6},
					{Resource: ResourceIronOre, Amount: 5},
					{Resource: ResourceCopperOre, Amount: 3},
				},
			},
			"smelter": {
				ID:      "smelter",
				Title:   "Smelter",
				Kind:    RecipeDevice,
				Device:  DeviceKindSmelter,
				StageID: "smelting",
				Pattern: []RecipeCell{
					{X: 2, Y: 2, Part: DevicePartMotor},
					{X: 2, Y: 3, Part: DevicePartOutput},
					{X: 2, Y: 4, Part: DevicePartHandCrank},
				},
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 6},
					{Resource: ResourceIronOre, Amount: 1},
					{Resource: ResourceCopperOre, Amount: 3},
				},
			},
			"generator": {
				ID:      "generator",
				Title:   "Generator",
				Kind:    RecipeDevice,
				Device:  DeviceKindGenerator,
				StageID: "coal_power",
				Pattern: []RecipeCell{
					{X: 2, Y: 1, Part: DevicePartHandCrank},
					{X: 2, Y: 2, Part: DevicePartMotor},
					{X: 2, Y: 3, Part: DevicePartOutput},
				},
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 8},
					{Resource: ResourceIronIngot, Amount: 2},
					{Resource: ResourceIronOre, Amount: 1},
					{Resource: ResourceCopperOre, Amount: 4},
				},
			},
			"assembler": {
				ID:      "assembler",
				Title:   "Assembler",
				Kind:    RecipeDevice,
				Device:  DeviceKindAssembler,
				StageID: "assembly",
				Pattern: []RecipeCell{
					{X: 2, Y: 1, Part: DevicePartFrame},
					{X: 1, Y: 2, Part: DevicePartFrame},
					{X: 2, Y: 2, Part: DevicePartMotor},
					{X: 3, Y: 2, Part: DevicePartOutput},
					{X: 2, Y: 3, Part: DevicePartHandCrank},
				},
				Ingredients: []RecipeIngredient{
					{Resource: ResourceStone, Amount: 10},
					{Resource: ResourceIronIngot, Amount: 8},
					{Resource: ResourceCopperIngot, Amount: 4},
					{Resource: ResourceCopperOre, Amount: 3},
				},
			},
			"iron-ingot": {
				ID:      "iron-ingot",
				Title:   "Iron Plate",
				Kind:    RecipePart,
				StageID: "smelting",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceIronOre, Amount: 1},
					{Resource: ResourceCoal, Amount: 1},
				},
			},
			"copper-ingot": {
				ID:      "copper-ingot",
				Title:   "Copper Plate",
				Kind:    RecipePart,
				StageID: "smelting",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceCopperOre, Amount: 1},
					{Resource: ResourceCoal, Amount: 1},
				},
			},
			"solar-retrofit": {
				ID:      "solar-retrofit",
				Title:   "Solar Retrofit",
				Kind:    RecipeUpgrade,
				Device:  DeviceKindGenerator,
				StageID: "mechanics",
				Ingredients: []RecipeIngredient{
					{Resource: ResourceGear, Amount: 8},
					{Resource: ResourceCopperIngot, Amount: 6},
				},
			},
		},
	}
}

func (b *RecipeBook) Recipe(id string) (RecipeDef, bool) {
	if b == nil {
		return RecipeDef{}, false
	}
	def, ok := b.Recipes[id]
	return def, ok
}

func (b *RecipeBook) StageRecipes(stageID string) []RecipeDef {
	if b == nil {
		return nil
	}
	out := make([]RecipeDef, 0, 8)
	for _, recipe := range b.Recipes {
		if recipe.StageID == stageID {
			out = append(out, recipe)
		}
	}
	return out
}

func (b *RecipeBook) RawCost(id string) map[ResourceType]int {
	if b == nil {
		return nil
	}
	return b.rawCost(id, map[string]bool{})
}

func DefaultPerkBook() *PerkBook {
	return &PerkBook{
		Perks: map[string]PerkDef{
			"sturdy-crank": {
				ID:          "sturdy-crank",
				Title:       "Sturdy Crank",
				Description: "+25% crank power per tap.",
				Kind:        PerkCrankPower,
				StageID:     "bootstrap",
				Magnitude:   0.25,
			},
			"gate-uplink": {
				ID:          "gate-uplink",
				Title:       "Gate Uplink",
				Description: "Unlock GATE uplink deposits.",
				Kind:        PerkGateUplink,
				StageID:     "bootstrap",
				OneShot:     true,
			},
			"heavy-crank": {
				ID:          "heavy-crank",
				Title:       "Heavy Crank",
				Description: "+50% crank power per tap.",
				Kind:        PerkCrankPower,
				StageID:     "bootstrap",
				Magnitude:   0.50,
			},
			"patient-drill": {
				ID:          "patient-drill",
				Title:       "Patient Drill",
				Description: "Miners use 20% less power.",
				Kind:        PerkPowerEfficiency,
				StageID:     "bootstrap",
				Magnitude:   0.20,
			},
			"eager-drill": {
				ID:          "eager-drill",
				Title:       "Eager Drill",
				Description: "Miners produce 25% faster.",
				Kind:        PerkMinerOutput,
				StageID:     "bootstrap",
				Magnitude:   0.25,
			},
			"steady-buffer": {
				ID:          "steady-buffer",
				Title:       "Steady Buffer",
				Description: "Power decays half as fast.",
				Kind:        PerkBufferDecay,
				StageID:     "bootstrap",
				Magnitude:   0.50,
			},
			"youve-got-the-power": {
				ID:          "youve-got-the-power",
				Title:       "You've Got the Power",
				Description: "Hold on the MUG to transfer power.",
				Kind:        PerkHoldPower,
				StageID:     "bootstrap",
				Magnitude:   1,
			},
			"stone-cache": {
				ID:          "stone-cache",
				Title:       "Stone Cache",
				Description: "+5 stone now.",
				Kind:        PerkResourceGift,
				StageID:     "bootstrap",
				Resource:    ResourceStone,
				Magnitude:   5,
				OneShot:     true,
			},
			"iron-cache": {
				ID:          "iron-cache",
				Title:       "Iron Cache",
				Description: "+2 iron ore now.",
				Kind:        PerkResourceGift,
				StageID:     "bootstrap",
				Resource:    ResourceIronOre,
				Magnitude:   2,
				OneShot:     true,
			},
			"hot-furnace": {
				ID:          "hot-furnace",
				Title:       "Hot Furnace",
				Description: "Smelters produce 25% faster.",
				Kind:        PerkSmelterOutput,
				StageID:     "smelting",
				Magnitude:   0.25,
			},
			"blast-draft": {
				ID:          "blast-draft",
				Title:       "Blast Draft",
				Description: "Smelters produce 50% faster.",
				Kind:        PerkSmelterOutput,
				StageID:     "smelting",
				Magnitude:   0.50,
			},
			"clean-burn": {
				ID:          "clean-burn",
				Title:       "Clean Burn",
				Description: "Smelters use 20% less power.",
				Kind:        PerkSmelterPower,
				StageID:     "smelting",
				Magnitude:   0.20,
			},
			"insulated-bricks": {
				ID:          "insulated-bricks",
				Title:       "Insulated Bricks",
				Description: "Power decays 35% slower.",
				Kind:        PerkBufferDecay,
				StageID:     "smelting",
				Magnitude:   0.35,
			},
			"coal-cache": {
				ID:          "coal-cache",
				Title:       "Coal Cache",
				Description: "+5 coal now.",
				Kind:        PerkResourceGift,
				StageID:     "smelting",
				Resource:    ResourceCoal,
				Magnitude:   5,
				OneShot:     true,
			},
			"iron-plate-cache": {
				ID:          "iron-plate-cache",
				Title:       "Iron Plates",
				Description: "+3 iron plates now.",
				Kind:        PerkResourceGift,
				StageID:     "smelting",
				Resource:    ResourceIronIngot,
				Magnitude:   3,
				OneShot:     true,
			},
			"copper-plate-cache": {
				ID:          "copper-plate-cache",
				Title:       "Copper Plates",
				Description: "+3 copper plates now.",
				Kind:        PerkResourceGift,
				StageID:     "smelting",
				Resource:    ResourceCopperIngot,
				Magnitude:   3,
				OneShot:     true,
			},
			"high-pressure-boiler": {
				ID:          "high-pressure-boiler",
				Title:       "High Pressure",
				Description: "Generators output 35% more power.",
				Kind:        PerkGeneratorOutput,
				StageID:     "coal_power",
				Magnitude:   0.35,
			},
			"coal-saver": {
				ID:          "coal-saver",
				Title:       "Coal Saver",
				Description: "Generators output 20% more power.",
				Kind:        PerkGeneratorOutput,
				StageID:     "coal_power",
				Magnitude:   0.20,
			},
			"gear-cache": {
				ID:          "gear-cache",
				Title:       "Gear Cache",
				Description: "+2 gears now.",
				Kind:        PerkResourceGift,
				StageID:     "assembly",
				Resource:    ResourceGear,
				Magnitude:   2,
				OneShot:     true,
			},
		},
	}
}

func (b *PerkBook) Perk(id string) (PerkDef, bool) {
	if b == nil {
		return PerkDef{}, false
	}
	def, ok := b.Perks[id]
	return def, ok
}

func (b *RecipeBook) rawCost(id string, seen map[string]bool) map[ResourceType]int {
	def, ok := b.Recipes[id]
	if !ok || seen[id] {
		return map[ResourceType]int{}
	}
	seen[id] = true
	cost := map[ResourceType]int{}
	for _, ing := range def.Ingredients {
		if ing.Amount <= 0 {
			continue
		}
		if ing.RecipeID != "" {
			sub := b.rawCost(ing.RecipeID, seen)
			for resource, amount := range sub {
				cost[resource] += amount * ing.Amount
			}
			continue
		}
		if ing.Resource != ResourceNone {
			cost[ing.Resource] += ing.Amount
		}
	}
	delete(seen, id)
	return cost
}
