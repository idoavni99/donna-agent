package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

type YesNoScores struct {
	Yes float64 `json:"yes"`
	No  float64 `json:"no"`
}

type YesNoClassifier interface {
	Evaluate(context.Context, string, string) (YesNoScores, error)
}

type CheckResult struct {
	Answer     *bool   `json:"answer"`
	Confidence float64 `json:"confidence"`
}

func DecodeYesNoScores(payload []byte) (YesNoScores, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return YesNoScores{}, fmt.Errorf("%w: decode yes/no JSON: %v", ErrInvalidScores, err)
	}
	if len(fields) != 2 {
		return YesNoScores{}, fmt.Errorf("%w: expected exactly yes and no probabilities", ErrInvalidScores)
	}
	for _, name := range [...]string{"yes", "no"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return YesNoScores{}, fmt.Errorf("%w: missing or null probability %q", ErrInvalidScores, name)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var scores YesNoScores
	if err := decoder.Decode(&scores); err != nil {
		return YesNoScores{}, fmt.Errorf("%w: decode yes/no JSON: %v", ErrInvalidScores, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return YesNoScores{}, fmt.Errorf("%w: unexpected trailing JSON data", ErrInvalidScores)
	}
	if err := scores.Validate(); err != nil {
		return YesNoScores{}, err
	}
	return scores, nil
}

func (scores YesNoScores) Validate() error {
	values := [...]float64{scores.Yes, scores.No}
	total := 0.0
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("%w: each probability must be finite and between 0 and 1", ErrInvalidScores)
		}
		total += value
	}
	if math.Abs(total-1) > 0.01 {
		return fmt.Errorf("%w: probabilities must sum to 1", ErrInvalidScores)
	}
	return nil
}

func Evaluate(scores YesNoScores, minimumConfidence float64) (CheckResult, error) {
	if err := scores.Validate(); err != nil {
		return CheckResult{}, err
	}
	if err := validateMinimumConfidence(minimumConfidence); err != nil {
		return CheckResult{}, err
	}
	if scores.Yes == scores.No {
		return CheckResult{Confidence: scores.Yes}, nil
	}

	answer := scores.Yes > scores.No
	confidence := scores.No
	if answer {
		confidence = scores.Yes
	}
	if confidence < minimumConfidence {
		return CheckResult{Confidence: confidence}, nil
	}
	return CheckResult{Answer: &answer, Confidence: confidence}, nil
}
