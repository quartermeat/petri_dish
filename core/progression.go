package core

type GoalKind string

const (
	GoalMineResource     GoalKind = "mine_resource"
	GoalDiscoverResource GoalKind = "discover_resource"
	GoalBuildDevice      GoalKind = "build_device"
)

type ProgressGoal struct {
	Kind     GoalKind     `json:"kind"`
	Resource ResourceType `json:"resource,omitempty"`
	Device   DeviceKind   `json:"device,omitempty"`
	Amount   int          `json:"amount"`
	Label    string       `json:"label"`
}

type ProgressStage struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	NextStageID      string         `json:"nextStageID"`
	VisibleResources []ResourceType `json:"visibleResources,omitempty"`
	KnownRecipes     []string       `json:"knownRecipes,omitempty"`
	Goals            []ProgressGoal `json:"goals"`
}

type ProgressionBook struct {
	StartStageID string                   `json:"startStageID"`
	Stages       map[string]ProgressStage `json:"stages"`
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
				KnownRecipes: []string{
					"miner",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalMineResource,
						Label:    "mine stone",
						Resource: ResourceStone,
						Amount:   10,
					},
					{
						Kind:     GoalDiscoverResource,
						Label:    "find iron ore",
						Resource: ResourceIronOre,
						Amount:   1,
					},
					{
						Kind:   GoalBuildDevice,
						Label:  "build miner",
						Device: DeviceKindMiner,
						Amount: 1,
					},
				},
			},
			"smelting": {
				ID:          "smelting",
				Title:       "Smelting Basics",
				NextStageID: "mechanics",
				VisibleResources: []ResourceType{
					ResourceStone,
					ResourceIronOre,
					ResourceCopperOre,
					ResourceCoal,
				},
				KnownRecipes: []string{
					"smelter",
					"iron ingot",
					"copper ingot",
				},
				Goals: []ProgressGoal{
					{
						Kind:     GoalMineResource,
						Label:    "mine copper ore",
						Resource: ResourceCopperOre,
						Amount:   20,
					},
					{
						Kind:     GoalMineResource,
						Label:    "mine coal",
						Resource: ResourceCoal,
						Amount:   8,
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
