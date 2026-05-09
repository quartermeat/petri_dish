package petridish

import "testing"

func newRunSkipTestGame() *Game {
	return &Game{
		screenWidth:       defaultScreenWidth,
		screenHeight:      defaultScreenHeight,
		roster:            defaultRosterUnits(),
		runActiveTeam:     runTeamBlue,
		runBroth:          [2]int{runStartingBroth, runStartingBroth},
		runBaseHP:         [2]int{runBaseMaxHP, runBaseMaxHP},
		runWells:          defaultRunWells(),
		runNextUnitID:     1,
		runBuyPending:     true,
		runSpawnMode:      true,
		runBoughtThisTurn: [2]bool{},
	}
}

func TestSkipRunBuyBanksOozeAndAdvancesAutoBattle(t *testing.T) {
	g := newRunSkipTestGame()

	g.skipRunBuy()

	if g.runBuyPending {
		t.Fatal("skip should close the buy prompt")
	}
	if g.runSpawnMode {
		t.Fatal("skip should leave spawn mode")
	}
	if len(g.runUnits) != 0 {
		t.Fatalf("skip should not spawn a unit, got %d units", len(g.runUnits))
	}
	if g.runBroth[runTeamBlue] != runStartingBroth {
		t.Fatalf("skip should bank current ooze before combat, got %d", g.runBroth[runTeamBlue])
	}
	if len(g.runWalls) != 1 {
		t.Fatalf("skip should drop one wall when it misses units, got %d", len(g.runWalls))
	}

	g.advanceRunAutoBattle(runAITurnDelay)
	g.advanceRunAutoBattle(runAITurnDelay)

	if g.runActiveTeam != runTeamRed {
		t.Fatalf("skip with no acting units should pass to red, got team %d", g.runActiveTeam)
	}
	wantOoze := runStartingBroth + runBaseIncome
	if g.runBroth[runTeamBlue] != wantOoze {
		t.Fatalf("blue should keep skipped ooze and gain base income, got %d want %d", g.runBroth[runTeamBlue], wantOoze)
	}
}

func TestBuyRunRosterUnitKeepsBuildChoiceOpen(t *testing.T) {
	g := newRunSkipTestGame()

	if !g.buyRunRosterUnit(0) {
		t.Fatal("expected first buy to succeed")
	}
	if !g.runBuyPending {
		t.Fatal("buying should keep the build choice open")
	}
	if g.runBoughtThisTurn[runTeamBlue] {
		t.Fatal("buying should not release the auto battle phase")
	}
	if len(g.runUnits) != 1 {
		t.Fatalf("expected one bought unit, got %d", len(g.runUnits))
	}
	if g.runBroth[runTeamBlue] != runStartingBroth-runWhiteCellCost {
		t.Fatalf("expected remaining ooze after first buy, got %d", g.runBroth[runTeamBlue])
	}
	firstQ, firstR := g.runUnits[0].Q, g.runUnits[0].R
	if !runTileExists(firstQ, firstR) || g.isRunSpawnTile(firstQ, firstR, runTeamBlue) {
		t.Fatalf("bought unit should take an entry move off the spawn ring, got %d,%d", firstQ, firstR)
	}

	if !g.buyRunRosterUnit(0) {
		t.Fatal("expected second buy to succeed")
	}
	if g.runBuyPending {
		t.Fatal("spending the last ooze should release the build choice")
	}
	if len(g.runUnits) != 2 {
		t.Fatalf("expected two bought units, got %d", len(g.runUnits))
	}
	if g.runBroth[runTeamBlue] != 0 {
		t.Fatalf("expected all ooze spent after second buy, got %d", g.runBroth[runTeamBlue])
	}
	if g.runSpawnMode {
		t.Fatal("spawn highlight should turn off when no more buys are affordable")
	}
	if !g.runBoughtThisTurn[runTeamBlue] {
		t.Fatal("spending the last ooze should mark the build choice complete")
	}
}

func TestBuyPromptClosesWhenOozeIsGone(t *testing.T) {
	g := newRunSkipTestGame()
	g.runBroth[runTeamBlue] = 0

	g.skipRunBuy()

	if len(g.runWalls) != 0 {
		t.Fatalf("empty ooze skip should not drop a wall, got %d", len(g.runWalls))
	}
	g.advanceRunAutoBattle(runAITurnDelay)

	if g.runBuyPending {
		t.Fatal("build prompt should close when no ooze remains")
	}
	if !g.runBoughtThisTurn[runTeamBlue] {
		t.Fatal("empty ooze pool should complete the build choice")
	}
	if g.runSpawnMode {
		t.Fatal("spawn mode should be off when no ooze remains")
	}

	g.advanceRunAutoBattle(runAITurnDelay)
	g.advanceRunAutoBattle(runAITurnDelay)

	if g.runActiveTeam != runTeamRed {
		t.Fatalf("empty ooze pool should let the turn proceed, got active team %d", g.runActiveTeam)
	}
}

func TestBuyPromptClosesWhenUnitLimitBlocksPlacement(t *testing.T) {
	g := newRunSkipTestGame()
	g.runBroth[runTeamBlue] = runStartingBroth + 4
	g.runUnits = nil
	for i := 0; i < runUnitLimit; i++ {
		q := -2 + i%3
		r := i / 3
		g.runUnits = append(g.runUnits, runUnit{
			ID:      i + 1,
			Team:    runTeamBlue,
			Kind:    dishLife,
			Q:       q,
			R:       r,
			HP:      runWhiteCellHP,
			MaxHP:   runWhiteCellHP,
			Attack:  runWhiteCellAttack,
			Actions: runWhiteCellAP,
		})
	}
	g.runNextUnitID = runUnitLimit + 1

	if g.runSkipAvailable() {
		t.Fatal("skip should not be available when unit limit blocks placement")
	}
	g.skipRunBuy()

	if len(g.runWalls) != 0 {
		t.Fatalf("unit-limit skip should not drop a wall, got %d", len(g.runWalls))
	}
	if g.runBuyPending {
		t.Fatal("unit limit should close the build prompt")
	}
	if !g.runBoughtThisTurn[runTeamBlue] {
		t.Fatal("unit limit should complete the build choice")
	}
	if g.runBroth[runTeamBlue] != runStartingBroth+4 {
		t.Fatalf("unit limit should bank existing ooze unchanged, got %d", g.runBroth[runTeamBlue])
	}
}

func TestSkipWallDamagesEnemyAndOnlyPersistsOnKill(t *testing.T) {
	g := newRunSkipTestGame()
	g.runUnits = []runUnit{{
		ID:      1,
		Team:    runTeamRed,
		Kind:    dishLife,
		Q:       1,
		R:       0,
		HP:      runWhiteCellHP,
		MaxHP:   runWhiteCellHP,
		Attack:  runWhiteCellAttack,
		Actions: runWhiteCellAP,
	}}
	g.runNextUnitID = 2

	g.skipRunBuy()

	if len(g.runWalls) != 0 {
		t.Fatalf("wall should disappear when the hit unit survives, got %d walls", len(g.runWalls))
	}
	if len(g.runWallDropFX) != 1 {
		t.Fatalf("surviving wall hit should leave an impact effect, got %d", len(g.runWallDropFX))
	}
	if fx := g.runWallDropFX[0]; !fx.Hit || fx.Lethal || fx.Team != runTeamRed || fx.Q != 1 || fx.R != 0 {
		t.Fatalf("surviving wall hit effect should mark the red hit tile, got %#v", fx)
	}
	if len(g.runUnits) != 1 {
		t.Fatalf("surviving unit should remain, got %d units", len(g.runUnits))
	}
	if got, want := g.runUnits[0].HP, runWhiteCellHP-runWallDropDamage; got != want {
		t.Fatalf("wall hit should damage unit, got hp %d want %d", got, want)
	}

	g = newRunSkipTestGame()
	g.runUnits = []runUnit{{
		ID:      1,
		Team:    runTeamRed,
		Kind:    dishLife,
		Q:       1,
		R:       0,
		HP:      runWallDropDamage,
		MaxHP:   runWhiteCellHP,
		Attack:  runWhiteCellAttack,
		Actions: runWhiteCellAP,
	}}
	g.runNextUnitID = 2

	g.skipRunBuy()

	if len(g.runUnits) != 0 {
		t.Fatalf("killed unit should be removed, got %d units", len(g.runUnits))
	}
	if len(g.runWalls) != 1 || g.runWalls[0].Q != 1 || g.runWalls[0].R != 0 {
		t.Fatalf("fatal wall hit should leave wall on killed unit tile, got %#v", g.runWalls)
	}
	if len(g.runWallDropFX) != 1 || !g.runWallDropFX[0].Hit || !g.runWallDropFX[0].Lethal {
		t.Fatalf("fatal wall hit should leave a lethal impact effect, got %#v", g.runWallDropFX)
	}
	g.advanceRunWallDropFX(runWallDropFXSeconds)
	if len(g.runWallDropFX) != 0 {
		t.Fatalf("impact effect should expire, got %#v", g.runWallDropFX)
	}
}

func TestRunWallsBlockMovementSpawnsAndWellIncome(t *testing.T) {
	g := newRunSkipTestGame()
	g.runBuyPending = false
	g.runSpawnMode = false
	g.runSelectedUnitID = 1
	g.runUnits = []runUnit{{
		ID:      1,
		Team:    runTeamBlue,
		Kind:    dishLife,
		Q:       -1,
		R:       0,
		HP:      runWhiteCellHP,
		MaxHP:   runWhiteCellHP,
		Attack:  runWhiteCellAttack,
		Actions: 1,
	}}
	g.runWalls = []runWall{{Q: 0, R: 0}}

	if g.tryRunMove(0, 0) {
		t.Fatal("unit should not move onto a wall")
	}
	if g.runUnits[0].Q != -1 || g.runUnits[0].R != 0 {
		t.Fatalf("blocked unit moved to %d,%d", g.runUnits[0].Q, g.runUnits[0].R)
	}

	g.runSpawnMode = true
	g.runBroth[runTeamBlue] = runWhiteCellCost
	g.runWalls = append(g.runWalls, runWall{Q: -2, R: 0})
	if g.spawnRunWhiteCell(-2, 0) {
		t.Fatal("spawn should fail on a wall tile")
	}

	g.runWells = []runWell{{Q: 0, R: 0, Owner: runTeamBlue}}
	if count := g.runControlledWellCount(runTeamBlue); count != 0 {
		t.Fatalf("walled well should not produce income, got %d controlled wells", count)
	}
}
