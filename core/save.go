package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const SaveFileName = "save.json"

type SavedCamera struct {
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
	Zoom float64 `json:"zoom"`
}

type SavedTacticalEntry struct {
	CellID int          `json:"cellID"`
	Map    *TacticalMap `json:"map"`
}

type SaveData struct {
	Version           string               `json:"version"`
	WorldSeed         int64                `json:"worldSeed"`
	Inventory         map[ResourceType]int `json:"inventory"`
	PartInventory     map[DevicePart]int   `json:"partInventory,omitempty"`
	StarterMinerCount *int                 `json:"starterMinerCount,omitempty"`
	StarterGateCount  *int                 `json:"starterGateCount,omitempty"`
	TutorialSeen      []string             `json:"tutorialSeen,omitempty"`
	Camera            SavedCamera          `json:"camera"`
	Selected          int                  `json:"selectedCell"`
	CurrentStage      string               `json:"currentStage,omitempty"`
	KnownRecipes      []string             `json:"knownRecipes,omitempty"`
	MinedTotals       map[ResourceType]int `json:"minedTotals,omitempty"`
	ActivePerks       []string             `json:"activePerks,omitempty"`
	StagePowerSpent   map[string]float64   `json:"stagePowerSpent,omitempty"`
	PerksAwarded      map[string]int       `json:"perksAwarded,omitempty"`
	CreaturesEnabled  bool                 `json:"creaturesEnabled,omitempty"`
	Tactical          []SavedTacticalEntry `json:"tactical"`
}

// LoadSave reads and parses save.json from dir. Returns (nil, nil) if no
// save exists. Returns a parse error if the file is corrupt.
func LoadSave(dir string) (*SaveData, error) {
	if dir == "" {
		return nil, nil
	}
	path := filepath.Join(dir, SaveFileName)
	bytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var data SaveData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, fmt.Errorf("parse save: %w", err)
	}
	return &data, nil
}

// Save writes the data atomically (write to .tmp, then rename) so a crash
// mid-write doesn't leave a corrupt file in place.
func (s *SaveData) Save(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		return err
	}
	finalPath := filepath.Join(dir, SaveFileName)
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, finalPath)
}

// VersionMatches reports whether s was written by the given build version.
// Plain string equality — both empty matches (unsigned dev builds reload
// cleanly), one empty + one set mismatches (signed binary refuses to load
// an unsigned save and vice versa).
func (s *SaveData) VersionMatches(buildVersion string) bool {
	return s.Version == buildVersion
}
