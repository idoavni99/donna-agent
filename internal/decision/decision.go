package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

type Action string

const (
	AnswerDirectly   Action = "answer_directly"
	StructuredLookup Action = "structured_lookup"
	SemanticSearch   Action = "semantic_search"
	HybridSearch     Action = "hybrid_search"
	Clarify          Action = "clarify"
)

type Scores struct {
	AnswerDirectly   float64 `json:"answer_directly"`
	StructuredLookup float64 `json:"structured_lookup"`
	SemanticSearch   float64 `json:"semantic_search"`
	HybridSearch     float64 `json:"hybrid_search"`
	Clarify          float64 `json:"clarify"`
}

type Classifier interface {
	Classify(context.Context, string) (Scores, error)
}

type Result struct {
	Action     Action  `json:"action"`
	Confidence float64 `json:"confidence"`
}

var ErrInvalidScores = errors.New("invalid decision scores")

func DecodeScores(payload []byte) (Scores, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return Scores{}, fmt.Errorf("%w: decode decision JSON: %v", ErrInvalidScores, err)
	}
	if len(fields) != 5 {
		return Scores{}, fmt.Errorf("%w: expected exactly five decision probabilities", ErrInvalidScores)
	}
	for _, name := range [...]string{"answer_directly", "structured_lookup", "semantic_search", "hybrid_search", "clarify"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Scores{}, fmt.Errorf("%w: missing or null probability %q", ErrInvalidScores, name)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var scores Scores
	if err := decoder.Decode(&scores); err != nil {
		return Scores{}, fmt.Errorf("%w: decode decision JSON: %v", ErrInvalidScores, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Scores{}, fmt.Errorf("%w: unexpected trailing JSON data", ErrInvalidScores)
	}
	if err := scores.Validate(); err != nil {
		return Scores{}, err
	}
	return scores, nil
}

func Route(scores Scores, minimumConfidence float64) (Result, error) {
	if err := scores.Validate(); err != nil {
		return Result{}, err
	}
	if err := validateMinimumConfidence(minimumConfidence); err != nil {
		return Result{}, err
	}

	choices := [...]struct {
		action Action
		score  float64
	}{
		{AnswerDirectly, scores.AnswerDirectly},
		{StructuredLookup, scores.StructuredLookup},
		{SemanticSearch, scores.SemanticSearch},
		{HybridSearch, scores.HybridSearch},
		{Clarify, scores.Clarify},
	}

	best := choices[0]
	tied := false
	for _, choice := range choices[1:] {
		if choice.score > best.score {
			best = choice
			tied = false
		} else if choice.score == best.score {
			tied = true
		}
	}

	if tied || best.action == Clarify || best.score < minimumConfidence {
		return Result{Action: Clarify, Confidence: best.score}, nil
	}
	return Result{Action: best.action, Confidence: best.score}, nil
}

func validateMinimumConfidence(minimumConfidence float64) error {
	if math.IsNaN(minimumConfidence) || math.IsInf(minimumConfidence, 0) || minimumConfidence < 0 || minimumConfidence > 1 {
		return fmt.Errorf("minimum confidence must be between 0 and 1")
	}
	return nil
}

func (scores Scores) Validate() error {
	values := [...]float64{
		scores.AnswerDirectly,
		scores.StructuredLookup,
		scores.SemanticSearch,
		scores.HybridSearch,
		scores.Clarify,
	}
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
