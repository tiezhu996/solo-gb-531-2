package service

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"hazop-safeguard-coverage/backend/internal/algorithm"
	"hazop-safeguard-coverage/backend/internal/constants"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/util"
)

// buildEvaluationComparison derives a read-only version diff between two frozen
// evaluations. It never mutates either evaluation and never writes any data.
func buildEvaluationComparison(base, other model.CoverageEvaluation) (dto.EvaluationComparisonResponse, error) {
	if base.ScenarioID != other.ScenarioID {
		return dto.EvaluationComparisonResponse{}, util.NewError(
			http.StatusUnprocessableEntity, util.CodeValidation,
			"only evaluations of the same deviation scenario can be compared",
		)
	}
	if err := ensureComparable(base); err != nil {
		return dto.EvaluationComparisonResponse{}, err
	}
	if err := ensureComparable(other); err != nil {
		return dto.EvaluationComparisonResponse{}, err
	}
	baseAllPaths, err := decodeComparisonPaths(base)
	if err != nil {
		return dto.EvaluationComparisonResponse{}, util.WrapError(
			http.StatusUnprocessableEntity, util.CodeValidation,
			fmt.Sprintf("evaluation #%d snapshot is corrupted: %v", base.ID, err), err,
		)
	}
	otherAllPaths, err := decodeComparisonPaths(other)
	if err != nil {
		return dto.EvaluationComparisonResponse{}, util.WrapError(
			http.StatusUnprocessableEntity, util.CodeValidation,
			fmt.Sprintf("evaluation #%d snapshot is corrupted: %v", other.ID, err), err,
		)
	}
	baseSnapshot, err := decodeComparisonSnapshot(base)
	if err != nil {
		return dto.EvaluationComparisonResponse{}, util.WrapError(
			http.StatusUnprocessableEntity, util.CodeValidation,
			fmt.Sprintf("evaluation #%d snapshot is corrupted: %v", base.ID, err), err,
		)
	}
	otherSnapshot, err := decodeComparisonSnapshot(other)
	if err != nil {
		return dto.EvaluationComparisonResponse{}, util.WrapError(
			http.StatusUnprocessableEntity, util.CodeValidation,
			fmt.Sprintf("evaluation #%d snapshot is corrupted: %v", other.ID, err), err,
		)
	}

	basePaths := uncoveredOnly(baseAllPaths)
	otherPaths := uncoveredOnly(otherAllPaths)
	safeguardChanges := diffSafeguards(baseSnapshot.Safeguards, otherSnapshot.Safeguards)
	added, eliminated := diffUncoveredPaths(basePaths, otherPaths, baseAllPaths, otherAllPaths, baseSnapshot, otherSnapshot)
	pathChanges := make([]dto.UncoveredPathComparisonChange, 0, len(eliminated)+len(added))
	pathChanges = append(pathChanges, eliminated...)
	pathChanges = append(pathChanges, added...)

	return dto.EvaluationComparisonResponse{
		BaseID: base.ID, ComparedID: other.ID,
		Base:                versionSummary(base),
		Compared:            versionSummary(other),
		ScoreDelta:          roundDelta(other.CoverageScore - base.CoverageScore),
		UncoveredPathDelta:  len(otherPaths) - len(basePaths),
		RiskRankChanged:     base.RiskRankAfter != other.RiskRankAfter,
		InputChanged:        base.InputHash != other.InputHash,
		AlgorithmChanged:    base.AlgorithmVersion != other.AlgorithmVersion,
		SafeguardChanges:    safeguardChanges,
		PathChanges:         pathChanges,
		AddedUncovered:      added,
		EliminatedUncovered: eliminated,
	}, nil
}

func ensureComparable(evaluation model.CoverageEvaluation) error {
	switch evaluation.EvaluationState {
	case string(constants.CoverageCompleted), string(constants.CoverageConfirmed):
		return nil
	case string(constants.CoverageVoided):
		// A voided evaluation is comparable only when it stores a finished algorithm
		// result; evaluations voided directly after a failure never produced one.
		if strings.TrimSpace(evaluation.Explanation) == "" || evaluation.Explanation == "{}" {
			return util.NewError(
				http.StatusConflict, util.CodeStateTransition,
				fmt.Sprintf("evaluation #%d was voided after failure and never produced a comparable result", evaluation.ID),
			)
		}
		return nil
	case string(constants.CoverageFailed):
		return util.NewError(
			http.StatusConflict, util.CodeStateTransition,
			fmt.Sprintf("evaluation #%d failed and has no result to compare", evaluation.ID),
		)
	default:
		return util.NewError(
			http.StatusConflict, util.CodeStateTransition,
			fmt.Sprintf("evaluation #%d is still %s; wait until it finishes before comparing versions",
				evaluation.ID, evaluation.EvaluationState),
		)
	}
}

func decodeComparisonSnapshot(evaluation model.CoverageEvaluation) (algorithm.Snapshot, error) {
	var snapshot algorithm.Snapshot
	if strings.TrimSpace(evaluation.InputSnapshot) == "" {
		return algorithm.Snapshot{}, fmt.Errorf("frozen input snapshot is empty")
	}
	if err := json.Unmarshal([]byte(evaluation.InputSnapshot), &snapshot); err != nil {
		return algorithm.Snapshot{}, fmt.Errorf("decode frozen input snapshot: %w", err)
	}
	if snapshot.Scenario.ID == 0 || snapshot.Node.ID == 0 {
		return algorithm.Snapshot{}, fmt.Errorf("frozen input snapshot is missing node or scenario identity")
	}
	if util.HashString(evaluation.InputSnapshot) != evaluation.InputHash {
		return algorithm.Snapshot{}, fmt.Errorf("frozen input snapshot does not match stored input hash")
	}
	return snapshot, nil
}

// decodeComparisonPaths returns all cause-to-consequence paths stored on the
// evaluation. Explanation paths are preferred because they carry protection
// metadata; the uncovered_paths column is used as a fallback (uncovered only).
func decodeComparisonPaths(evaluation model.CoverageEvaluation) ([]dto.CoveragePathResponse, error) {
	var explanation dto.EvaluationExplanation
	if strings.TrimSpace(evaluation.Explanation) != "" && evaluation.Explanation != "{}" {
		if err := json.Unmarshal([]byte(evaluation.Explanation), &explanation); err != nil {
			return nil, fmt.Errorf("decode stored explanation: %w", err)
		}
	}
	if len(explanation.Paths) > 0 {
		return explanation.Paths, nil
	}
	if strings.TrimSpace(evaluation.UncoveredPaths) == "" {
		return nil, fmt.Errorf("stored uncovered path list is empty")
	}
	var uncovered []dto.CoveragePathResponse
	if err := json.Unmarshal([]byte(evaluation.UncoveredPaths), &uncovered); err != nil {
		return nil, fmt.Errorf("decode stored uncovered paths: %w", err)
	}
	return uncovered, nil
}

func uncoveredOnly(paths []dto.CoveragePathResponse) []dto.CoveragePathResponse {
	result := make([]dto.CoveragePathResponse, 0, len(paths))
	for _, path := range paths {
		if !path.Covered {
			result = append(result, path)
		}
	}
	return result
}

func findPath(paths []dto.CoveragePathResponse, identity string) (dto.CoveragePathResponse, bool) {
	for _, path := range paths {
		if pathIdentity(path) == identity {
			return path, true
		}
	}
	return dto.CoveragePathResponse{}, false
}

func versionSummary(evaluation model.CoverageEvaluation) dto.EvaluationVersionSummary {
	return dto.EvaluationVersionSummary{
		ID:               evaluation.ID,
		EvaluatedAt:      evaluation.EvaluatedAt.UTC().Format(time.RFC3339),
		AlgorithmVersion: evaluation.AlgorithmVersion,
		InputHash:        evaluation.InputHash,
		EvaluationState:  evaluation.EvaluationState,
		CoverageScore:    evaluation.CoverageScore,
		RiskRankBefore:   evaluation.RiskRankBefore,
		RiskRankAfter:    evaluation.RiskRankAfter,
		EvaluatedByName:  evaluation.EvaluatedByName,
	}
}

func diffSafeguards(base, other []algorithm.SnapshotSafeguard) []dto.SafeguardComparisonChange {
	baseByID := make(map[uint]algorithm.SnapshotSafeguard, len(base))
	otherByID := make(map[uint]algorithm.SnapshotSafeguard, len(other))
	for _, item := range base {
		baseByID[item.ID] = item
	}
	for _, item := range other {
		otherByID[item.ID] = item
	}
	changes := make([]dto.SafeguardComparisonChange, 0)
	for _, item := range other {
		previous, ok := baseByID[item.ID]
		if !ok {
			changes = append(changes, dto.SafeguardComparisonChange{
				ChangeType: "added", SafeguardID: item.ID,
				Name: item.Name, IndependenceKey: item.IndependenceKey,
			})
			continue
		}
		if fieldChanges := safeguardFieldChanges(previous, item); len(fieldChanges) > 0 {
			changes = append(changes, dto.SafeguardComparisonChange{
				ChangeType: "changed", SafeguardID: item.ID,
				Name: item.Name, IndependenceKey: item.IndependenceKey,
				FieldChanges: fieldChanges,
			})
		}
	}
	for _, item := range base {
		if _, ok := otherByID[item.ID]; !ok {
			changes = append(changes, dto.SafeguardComparisonChange{
				ChangeType: "removed", SafeguardID: item.ID,
				Name: item.Name, IndependenceKey: item.IndependenceKey,
			})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].SafeguardID == changes[j].SafeguardID {
			return changes[i].ChangeType < changes[j].ChangeType
		}
		return changes[i].SafeguardID < changes[j].SafeguardID
	})
	return changes
}

func safeguardFieldChanges(before, after algorithm.SnapshotSafeguard) []dto.ComparisonFieldChange {
	fields := []struct {
		name   string
		before string
		after  string
	}{
		{"name", before.Name, after.Name},
		{"type", before.Type, after.Type},
		{"independence_key", before.IndependenceKey, after.IndependenceKey},
		{"effectiveness", formatEffectiveness(before.Effectiveness), formatEffectiveness(after.Effectiveness)},
		{"test_interval_days", fmt.Sprintf("%d", before.TestIntervalDays), fmt.Sprintf("%d", after.TestIntervalDays)},
		{"last_verified_at", formatSnapshotTime(before.LastVerifiedAt), formatSnapshotTime(after.LastVerifiedAt)},
		{"lifecycle_state", before.LifecycleState, after.LifecycleState},
		{"evidence_note", before.EvidenceNote, after.EvidenceNote},
	}
	changes := make([]dto.ComparisonFieldChange, 0, len(fields))
	for _, field := range fields {
		if field.before != field.after {
			changes = append(changes, dto.ComparisonFieldChange{Field: field.name, Before: field.before, After: field.after})
		}
	}
	return changes
}

func diffUncoveredPaths(
	baseUncovered, otherUncovered []dto.CoveragePathResponse,
	baseAll, otherAll []dto.CoveragePathResponse,
	baseSnapshot, otherSnapshot algorithm.Snapshot,
) (added, eliminated []dto.UncoveredPathComparisonChange) {
	baseSet := make(map[string]dto.CoveragePathResponse, len(baseUncovered))
	otherSet := make(map[string]dto.CoveragePathResponse, len(otherUncovered))
	for _, path := range baseUncovered {
		baseSet[pathIdentity(path)] = path
	}
	for _, path := range otherUncovered {
		otherSet[pathIdentity(path)] = path
	}
	added = make([]dto.UncoveredPathComparisonChange, 0)
	for _, path := range otherUncovered {
		identity := pathIdentity(path)
		if _, existed := baseSet[identity]; !existed {
			change := uncoveredPathChange("added", path, addedUncoveredReason(path, baseSnapshot, otherSnapshot))
			change.AfterCombinedProtection = protectionPtr(path)
			if previous, ok := findPath(baseAll, identity); ok {
				change.BeforeCombinedProtection = protectionPtr(previous)
			}
			added = append(added, change)
		}
	}
	eliminated = make([]dto.UncoveredPathComparisonChange, 0)
	for _, path := range baseUncovered {
		identity := pathIdentity(path)
		if _, remains := otherSet[identity]; !remains {
			change := uncoveredPathChange("eliminated", path, eliminatedUncoveredReason(path, baseSnapshot, otherSnapshot))
			change.BeforeCombinedProtection = protectionPtr(path)
			if current, ok := findPath(otherAll, identity); ok {
				change.AfterCombinedProtection = protectionPtr(current)
			}
			eliminated = append(eliminated, change)
		}
	}
	sort.SliceStable(added, func(i, j int) bool { return added[i].PathID < added[j].PathID })
	sort.SliceStable(eliminated, func(i, j int) bool { return eliminated[i].PathID < eliminated[j].PathID })
	return added, eliminated
}

func protectionPtr(path dto.CoveragePathResponse) *float64 {
	value := path.CombinedProtection
	return &value
}

func uncoveredPathChange(changeType string, path dto.CoveragePathResponse, reason reasonWithCode) dto.UncoveredPathComparisonChange {
	return dto.UncoveredPathComparisonChange{
		ChangeType:       changeType,
		PathID:           path.PathID,
		NodeCode:         path.NodeCode,
		Cause:            path.Cause,
		Consequence:      path.Consequence,
		ReasonCode:       reason.code,
		Reason:           reason.text,
		SafeguardIDs:     append([]uint(nil), path.SafeguardIDs...),
		IndependenceKeys: append([]string(nil), path.IndependenceKeys...),
	}
}

type reasonWithCode struct {
	code string
	text string
}

func addedUncoveredReason(path dto.CoveragePathResponse, base, other algorithm.Snapshot) reasonWithCode {
	if !pathExistsInSnapshot(path, base) {
		return reasonWithCode{
			code: "scenario_changed",
			text: fmt.Sprintf("new cause-to-consequence path introduced by scenario revision (scenario version %d)", other.Scenario.Version),
		}
	}
	degraded := make([]algorithm.SnapshotSafeguard, 0)
	otherByID := snapshotSafeguardMap(other.Safeguards)
	for _, item := range base.Safeguards {
		if !safeguardWasEligible(item, base.ReferenceTime) {
			continue
		}
		current, ok := otherByID[item.ID]
		if !ok || !safeguardWasEligible(current, other.ReferenceTime) {
			degraded = append(degraded, item)
			continue
		}
		if current.Effectiveness < item.Effectiveness {
			degraded = append(degraded, item)
		}
	}
	if len(degraded) > 0 {
		return reasonWithCode{
			code: "protection_lost",
			text: fmt.Sprintf("independent protection became unavailable or weaker: %s", describeSafeguards(degraded)),
		}
	}
	if base.AlgorithmVersion != other.AlgorithmVersion {
		return reasonWithCode{
			code: "algorithm_changed",
			text: fmt.Sprintf("protection recalculated with algorithm %s (was %s)", other.AlgorithmVersion, base.AlgorithmVersion),
		}
	}
	return reasonWithCode{
		code: "below_threshold",
		text: fmt.Sprintf("combined independent protection %.2f%% is below the 50%% threshold in the new version",
			path.CombinedProtection*100),
	}
}

func eliminatedUncoveredReason(path dto.CoveragePathResponse, base, other algorithm.Snapshot) reasonWithCode {
	if !pathExistsInSnapshot(path, other) {
		return reasonWithCode{
			code: "scenario_changed",
			text: fmt.Sprintf("cause-to-consequence path removed or rewritten by scenario revision (scenario version %d)", other.Scenario.Version),
		}
	}
	improved := make([]algorithm.SnapshotSafeguard, 0)
	baseByID := snapshotSafeguardMap(base.Safeguards)
	for _, item := range other.Safeguards {
		if !safeguardWasEligible(item, other.ReferenceTime) {
			continue
		}
		previous, existed := baseByID[item.ID]
		if !existed || !safeguardWasEligible(previous, base.ReferenceTime) || item.Effectiveness > previous.Effectiveness {
			improved = append(improved, item)
		}
	}
	if len(improved) > 0 {
		return reasonWithCode{
			code: "protection_added",
			text: fmt.Sprintf("newly available or strengthened independent protection closed the gap: %s", describeSafeguards(improved)),
		}
	}
	if base.AlgorithmVersion != other.AlgorithmVersion {
		return reasonWithCode{
			code: "algorithm_changed",
			text: fmt.Sprintf("protection recalculated with algorithm %s (was %s)", other.AlgorithmVersion, base.AlgorithmVersion),
		}
	}
	return reasonWithCode{
		code: "threshold_reached",
		text: "combined independent protection reached or exceeded the 50% threshold in the new version",
	}
}

func pathExistsInSnapshot(path dto.CoveragePathResponse, snapshot algorithm.Snapshot) bool {
	if path.NodeCode != "" && path.NodeCode != snapshot.Node.NodeCode {
		return false
	}
	return snapshotContainsStatement(snapshot.Scenario.Cause, path.Cause) &&
		snapshotContainsStatement(snapshot.Scenario.Consequence, path.Consequence)
}

func snapshotContainsStatement(statements, want string) bool {
	for _, statement := range algorithm.SplitStatements(statements) {
		if statement == want {
			return true
		}
	}
	return false
}

func snapshotSafeguardMap(items []algorithm.SnapshotSafeguard) map[uint]algorithm.SnapshotSafeguard {
	result := make(map[uint]algorithm.SnapshotSafeguard, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func safeguardWasEligible(item algorithm.SnapshotSafeguard, reference time.Time) bool {
	resolved := algorithm.ResolveIndependence([]algorithm.SnapshotSafeguard{item}, reference)
	return len(resolved.Retained) == 1
}

func describeSafeguards(items []algorithm.SnapshotSafeguard) string {
	descriptions := make([]string, 0, len(items))
	for _, item := range items {
		descriptions = append(descriptions, fmt.Sprintf("#%d %s(%s)", item.ID, item.Name, item.IndependenceKey))
	}
	return strings.Join(descriptions, ", ")
}

func pathIdentity(path dto.CoveragePathResponse) string {
	return strings.ToLower(strings.TrimSpace(path.NodeCode)) + "\x00" +
		strings.ToLower(strings.TrimSpace(path.Cause)) + "\x00" +
		strings.ToLower(strings.TrimSpace(path.Consequence))
}

func formatEffectiveness(value float64) string {
	return fmt.Sprintf("%.4f", value)
}

func formatSnapshotTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func roundDelta(value float64) float64 {
	return math.Round(value*10000) / 10000
}
