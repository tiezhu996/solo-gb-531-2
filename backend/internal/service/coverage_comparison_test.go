package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"hazop-safeguard-coverage/backend/internal/algorithm"
	"hazop-safeguard-coverage/backend/internal/constants"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/util"

	"gorm.io/gorm"
)

type comparisonFixture struct {
	db        *gorm.DB
	service   CoverageEvaluationService
	node      model.ProcessNode
	scenario  model.DeviationScenario
	otherNode model.ProcessNode
	other     model.DeviationScenario
	reference time.Time
}

func newComparisonFixture(t *testing.T) comparisonFixture {
	t.Helper()
	db := testDB(t)
	reference := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	nodeRepo := repository.NewProcessNodeRepository(db)
	scenarioRepo := repository.NewDeviationScenarioRepository(db)
	evaluationRepo := repository.NewCoverageEvaluationRepository(db)
	safeguardRepo := repository.NewSafeguardRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	node := model.ProcessNode{
		NodeCode: "C-101", Name: "Comparison Reactor", UnitName: "Unit A", Medium: "propylene",
		DesignPressure: 25, DesignTemperature: 180, OwnerTeam: "psm", Status: "active",
		CreatedAt: reference, UpdatedAt: reference,
	}
	if err := nodeRepo.Create(context.Background(), &node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	scenario := model.DeviationScenario{
		ProcessNodeID: node.ID, Guideword: "more", Parameter: "pressure",
		Cause:       "cooling water loss",
		Consequence: "reactor overpressure",
		Likelihood:  4, Severity: 5, ScenarioState: "analyzed", Version: 1,
		CreatedBy: 1, CreatedByName: "engineer", CreatedAt: reference, UpdatedAt: reference,
	}
	if err := scenarioRepo.Create(context.Background(), &scenario); err != nil {
		t.Fatalf("create scenario: %v", err)
	}
	otherNode := model.ProcessNode{
		NodeCode: "C-202", Name: "Other Vessel", UnitName: "Unit B", Medium: "steam",
		DesignPressure: 10, DesignTemperature: 120, OwnerTeam: "psm", Status: "active",
		CreatedAt: reference, UpdatedAt: reference,
	}
	if err := nodeRepo.Create(context.Background(), &otherNode); err != nil {
		t.Fatalf("create other node: %v", err)
	}
	otherScenario := model.DeviationScenario{
		ProcessNodeID: otherNode.ID, Guideword: "less", Parameter: "flow",
		Cause: "pump trip", Consequence: "dry run",
		Likelihood: 2, Severity: 3, ScenarioState: "draft", Version: 1,
		CreatedBy: 1, CreatedByName: "engineer", CreatedAt: reference, UpdatedAt: reference,
	}
	if err := scenarioRepo.Create(context.Background(), &otherScenario); err != nil {
		t.Fatalf("create other scenario: %v", err)
	}
	svc := NewCoverageEvaluationService(
		evaluationRepo, scenarioRepo, nodeRepo, safeguardRepo, auditRepo, algorithm.NewEvaluator(),
	)
	return comparisonFixture{
		db: db, service: svc, node: node, scenario: scenario,
		otherNode: otherNode, other: otherScenario, reference: reference,
	}
}

func activeSafeguard(id uint, name, key string, effectiveness float64, verifiedAt time.Time) model.Safeguard {
	return model.Safeguard{
		ID: id, Name: name, SafeguardType: "interlock",
		TargetScenarioID: 0, IndependenceKey: key, Effectiveness: effectiveness,
		TestIntervalDays: 365, LastVerifiedAt: &verifiedAt,
		LifecycleState: "active", EvidenceNote: "test certificate",
		CreatedAt: verifiedAt, UpdatedAt: verifiedAt,
	}
}

func persistCompletedEvaluation(
	t *testing.T,
	db *gorm.DB,
	id uint,
	scenario model.DeviationScenario,
	node model.ProcessNode,
	safeguards []model.Safeguard,
	state string,
	reference time.Time,
) model.CoverageEvaluation {
	t.Helper()
	snapshot := algorithm.NewSnapshot(node, scenario, safeguards, reference)
	result, err := algorithm.NewEvaluator().Evaluate(snapshot)
	if err != nil {
		t.Fatalf("evaluate snapshot: %v", err)
	}
	evaluation := model.CoverageEvaluation{
		ID: id, ScenarioID: scenario.ID, AlgorithmVersion: algorithm.Version,
		InputSnapshot: result.SnapshotJSON, InputHash: result.InputHash,
		CoverageScore: result.CoverageScore, UncoveredPaths: result.UncoveredJSON,
		DeduplicatedSafeguards: result.DeduplicatedJSON, Explanation: result.ExplanationJSON,
		RiskRankBefore: result.RiskBefore, RiskRankAfter: result.RiskAfter,
		EvaluationState: state, EvaluatedBy: 7, EvaluatedByName: "engineer",
		EvaluatedAt: reference, IdempotencyKey: "compare-key-" + string(rune('a'+id-1)),
		CreatedAt: reference, UpdatedAt: reference,
	}
	if err := db.Create(&evaluation).Error; err != nil {
		t.Fatalf("persist evaluation: %v", err)
	}
	return evaluation
}

func TestCompareDetailsSafeguardAndPathChanges(t *testing.T) {
	fixture := newComparisonFixture(t)
	verified := fixture.reference.AddDate(0, -1, 0)
	weak := activeSafeguard(101, "PSV-101", "PSV", 0.2, verified)
	strong := weak
	strong.Effectiveness = 0.8
	added := activeSafeguard(102, "SIS-201", "SIS", 0.7, verified)

	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		[]model.Safeguard{weak}, string(constants.CoverageCompleted), fixture.reference)
	otherEval := persistCompletedEvaluation(t, fixture.db, 2, fixture.scenario, fixture.node,
		[]model.Safeguard{strong, added}, string(constants.CoverageConfirmed), fixture.reference.Add(time.Hour))

	comparison, err := fixture.service.Compare(context.Background(), baseEval.ID, otherEval.ID)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}

	if comparison.Base.ID != baseEval.ID || comparison.Compared.ID != otherEval.ID {
		t.Fatalf("version summaries mismatch: %+v", comparison)
	}
	if comparison.Base.EvaluationState != "completed" || comparison.Compared.EvaluationState != "confirmed" {
		t.Fatalf("states should be shown side by side: %s -> %s", comparison.Base.EvaluationState, comparison.Compared.EvaluationState)
	}
	if comparison.Base.InputHash == comparison.Compared.InputHash || !comparison.InputChanged {
		t.Fatalf("input hashes should differ and be reported")
	}
	if comparison.Base.AlgorithmVersion != algorithm.Version || comparison.Compared.AlgorithmVersion != algorithm.Version {
		t.Fatalf("algorithm versions missing: %+v", comparison)
	}
	if comparison.Base.EvaluatedAt == "" || comparison.Compared.EvaluatedAt == "" {
		t.Fatalf("evaluated times must be present: %+v", comparison)
	}
	if comparison.ScoreDelta <= 0 {
		t.Fatalf("score should increase after stronger protection, delta=%f", comparison.ScoreDelta)
	}

	changeByID := map[uint]dto.SafeguardComparisonChange{}
	for _, change := range comparison.SafeguardChanges {
		changeByID[change.SafeguardID] = change
	}
	changed, ok := changeByID[101]
	if !ok || changed.ChangeType != "changed" {
		t.Fatalf("safeguard 101 should be marked changed: %+v", changeByID)
	}
	var effectivenessChange *dto.ComparisonFieldChange
	for index := range changed.FieldChanges {
		if changed.FieldChanges[index].Field == "effectiveness" {
			effectivenessChange = &changed.FieldChanges[index]
		}
	}
	if effectivenessChange == nil || effectivenessChange.Before == effectivenessChange.After {
		t.Fatalf("effectiveness field change missing: %+v", changed.FieldChanges)
	}
	if addedChange, ok := changeByID[102]; !ok || addedChange.ChangeType != "added" {
		t.Fatalf("safeguard 102 should be marked added: %+v", changeByID)
	}

	if len(comparison.EliminatedUncovered) == 0 {
		t.Fatalf("raising protection above threshold must eliminate uncovered paths: %+v", comparison)
	}
	for _, path := range comparison.EliminatedUncovered {
		if path.ChangeType != "eliminated" || path.Reason == "" || path.ReasonCode == "" {
			t.Fatalf("eliminated path requires reason: %+v", path)
		}
		if path.AfterCombinedProtection == nil || *path.AfterCombinedProtection < 0.5 {
			t.Fatalf("eliminated path should carry new protection above threshold: %+v", path)
		}
	}
	if comparison.UncoveredPathDelta >= 0 {
		t.Fatalf("uncovered path delta should be negative, got %d", comparison.UncoveredPathDelta)
	}
}

func TestCompareDetailsRemovedSafeguardAddsUncoveredPath(t *testing.T) {
	fixture := newComparisonFixture(t)
	verified := fixture.reference.AddDate(0, -1, 0)
	safeguard := activeSafeguard(201, "PSV-201", "PSV", 0.8, verified)

	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		[]model.Safeguard{safeguard}, string(constants.CoverageCompleted), fixture.reference)
	otherEval := persistCompletedEvaluation(t, fixture.db, 2, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), fixture.reference.Add(time.Hour))

	comparison, err := fixture.service.Compare(context.Background(), baseEval.ID, otherEval.ID)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if len(comparison.AddedUncovered) == 0 {
		t.Fatalf("removing protection must add uncovered paths: %+v", comparison)
	}
	for _, path := range comparison.AddedUncovered {
		if path.ChangeType != "added" || path.ReasonCode != "protection_lost" {
			t.Fatalf("newly uncovered path should cite lost protection: %+v", path)
		}
	}
	removedFound := false
	for _, change := range comparison.SafeguardChanges {
		if change.SafeguardID == 201 && change.ChangeType == "removed" {
			removedFound = true
		}
	}
	if !removedFound {
		t.Fatalf("safeguard 201 should be reported as removed: %+v", comparison.SafeguardChanges)
	}
}

func TestCompareRejectsDifferentScenarios(t *testing.T) {
	fixture := newComparisonFixture(t)
	verified := fixture.reference.AddDate(0, -1, 0)
	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), fixture.reference)
	otherEval := persistCompletedEvaluation(t, fixture.db, 2, fixture.other, fixture.otherNode,
		[]model.Safeguard{activeSafeguard(301, "X", "X", 0.9, verified)},
		string(constants.CoverageCompleted), fixture.reference.Add(time.Hour))

	_, err := fixture.service.Compare(context.Background(), baseEval.ID, otherEval.ID)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Status != 422 {
		t.Fatalf("different scenarios should return 422, got %v", err)
	}
}

func TestCompareRejectsUnfinishedEvaluation(t *testing.T) {
	fixture := newComparisonFixture(t)
	reference := fixture.reference
	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), reference)
	running := model.CoverageEvaluation{
		ID: 2, ScenarioID: fixture.scenario.ID, AlgorithmVersion: algorithm.Version,
		InputSnapshot: baseEval.InputSnapshot, InputHash: baseEval.InputHash,
		UncoveredPaths: "[]", DeduplicatedSafeguards: "[]", Explanation: "{}",
		RiskRankBefore: "high", RiskRankAfter: "high", EvaluationState: string(constants.CoverageRunning),
		EvaluatedBy: 7, EvaluatedByName: "engineer", EvaluatedAt: reference,
		IdempotencyKey: "compare-running-2", CreatedAt: reference, UpdatedAt: reference,
	}
	if err := fixture.db.Create(&running).Error; err != nil {
		t.Fatalf("create running evaluation: %v", err)
	}
	_, err := fixture.service.Compare(context.Background(), baseEval.ID, running.ID)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Status != 409 {
		t.Fatalf("unfinished evaluation should return 409, got %v", err)
	}

	failed := running
	failed.ID = 3
	failed.EvaluationState = string(constants.CoverageFailed)
	failed.IdempotencyKey = "compare-failed-3"
	if err := fixture.db.Create(&failed).Error; err != nil {
		t.Fatalf("create failed evaluation: %v", err)
	}
	_, err = fixture.service.Compare(context.Background(), baseEval.ID, failed.ID)
	if !errors.As(err, &appErr) || appErr.Status != 409 {
		t.Fatalf("failed evaluation should return 409, got %v", err)
	}
}

func TestCompareRejectsCorruptedSnapshot(t *testing.T) {
	fixture := newComparisonFixture(t)
	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), fixture.reference)
	corrupted := model.CoverageEvaluation{
		ID: 2, ScenarioID: fixture.scenario.ID, AlgorithmVersion: algorithm.Version,
		InputSnapshot: "{not-json", InputHash: util.HashString("{not-json"),
		UncoveredPaths: "[]", DeduplicatedSafeguards: "[]", Explanation: "{}",
		RiskRankBefore: "high", RiskRankAfter: "high",
		EvaluationState: string(constants.CoverageCompleted),
		EvaluatedBy:     7, EvaluatedByName: "engineer", EvaluatedAt: fixture.reference,
		IdempotencyKey: "compare-corrupt-2", CreatedAt: fixture.reference, UpdatedAt: fixture.reference,
	}
	if err := fixture.db.Create(&corrupted).Error; err != nil {
		t.Fatalf("create corrupted evaluation: %v", err)
	}
	_, err := fixture.service.Compare(context.Background(), baseEval.ID, corrupted.ID)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Status != 422 {
		t.Fatalf("corrupted snapshot should return 422, got %v", err)
	}

	tampered := baseEval
	tampered.ID = 3
	tampered.InputHash = "deadbeef"
	tampered.EvaluationState = string(constants.CoverageCompleted)
	tampered.IdempotencyKey = "compare-tampered-3"
	if err := fixture.db.Create(&tampered).Error; err != nil {
		t.Fatalf("create tampered evaluation: %v", err)
	}
	_, err = fixture.service.Compare(context.Background(), baseEval.ID, tampered.ID)
	if !errors.As(err, &appErr) || appErr.Status != 422 {
		t.Fatalf("hash mismatch should return 422, got %v", err)
	}
}

func TestCompareRejectsIdenticalAndMissingVersions(t *testing.T) {
	fixture := newComparisonFixture(t)
	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), fixture.reference)

	if _, err := fixture.service.Compare(context.Background(), baseEval.ID, baseEval.ID); err == nil {
		t.Fatalf("comparing an evaluation with itself must fail")
	}
	var appErr *util.AppError
	if _, err := fixture.service.Compare(context.Background(), baseEval.ID, 9999); !errors.As(err, &appErr) || appErr.Status != 404 {
		t.Fatalf("missing compared evaluation should return 404, got %v", err)
	}
}

func TestCompareIsReadOnly(t *testing.T) {
	fixture := newComparisonFixture(t)
	verified := fixture.reference.AddDate(0, -1, 0)
	baseEval := persistCompletedEvaluation(t, fixture.db, 1, fixture.scenario, fixture.node,
		nil, string(constants.CoverageCompleted), fixture.reference)
	otherEval := persistCompletedEvaluation(t, fixture.db, 2, fixture.scenario, fixture.node,
		[]model.Safeguard{activeSafeguard(401, "PSV-401", "PSV", 0.9, verified)},
		string(constants.CoverageCompleted), fixture.reference.Add(time.Hour))

	beforeCount := countAuditRows(t, fixture.db)
	beforeState := loadEvaluationState(t, fixture.db, baseEval.ID)
	otherBeforeState := loadEvaluationState(t, fixture.db, otherEval.ID)

	if _, err := fixture.service.Compare(context.Background(), baseEval.ID, otherEval.ID); err != nil {
		t.Fatalf("compare: %v", err)
	}

	if countAuditRows(t, fixture.db) != beforeCount {
		t.Fatalf("comparison must not write audit rows")
	}
	if got := loadEvaluationState(t, fixture.db, baseEval.ID); got != beforeState {
		t.Fatalf("base evaluation state changed: %s != %s", got, beforeState)
	}
	if got := loadEvaluationState(t, fixture.db, otherEval.ID); got != otherBeforeState {
		t.Fatalf("compared evaluation state changed: %s != %s", got, otherBeforeState)
	}
	if countEvaluations(t, fixture.db) != 2 {
		t.Fatalf("comparison must not create new evaluations")
	}
}

func countAuditRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	return count
}

func countEvaluations(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.CoverageEvaluation{}).Count(&count).Error; err != nil {
		t.Fatalf("count evaluations: %v", err)
	}
	return count
}

func loadEvaluationState(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	var evaluation model.CoverageEvaluation
	if err := db.First(&evaluation, id).Error; err != nil {
		t.Fatalf("load evaluation %d: %v", id, err)
	}
	return evaluation.EvaluationState
}
