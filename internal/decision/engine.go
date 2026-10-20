package decision

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

var ErrEmptyQuery = errors.New("query must not be empty")
var ErrEmptyQuestion = errors.New("question must not be empty")
var ErrCheckUnavailable = errors.New("yes/no checks are not configured")

type Engine struct {
	classifier        Classifier
	checkClassifier   YesNoClassifier
	minimumConfidence float64
}

func NewEngine(classifier Classifier, minimumConfidence float64) (*Engine, error) {
	return NewEngineWithCheck(classifier, nil, minimumConfidence)
}

func NewEngineWithCheck(classifier Classifier, checkClassifier YesNoClassifier, minimumConfidence float64) (*Engine, error) {
	if classifier == nil {
		return nil, errors.New("classifier is required")
	}
	if err := validateMinimumConfidence(minimumConfidence); err != nil {
		return nil, err
	}
	return &Engine{classifier: classifier, checkClassifier: checkClassifier, minimumConfidence: minimumConfidence}, nil
}

func (engine *Engine) Decide(ctx context.Context, query string) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, ErrEmptyQuery
	}

	started := time.Now()
	scores, err := engine.classifier.Classify(ctx, query)
	if err != nil {
		if errors.Is(err, ErrInvalidScores) {
			slog.WarnContext(ctx, "route classifier returned invalid scores", "duration_ms", time.Since(started).Milliseconds())
			return Result{Action: Clarify}, nil
		}
		slog.WarnContext(ctx, "route classification failed", "error_type", fmt.Sprintf("%T", err), "duration_ms", time.Since(started).Milliseconds())
		return Result{}, fmt.Errorf("classify query: %w", err)
	}

	result, err := Route(scores, engine.minimumConfidence)
	if errors.Is(err, ErrInvalidScores) {
		slog.WarnContext(ctx, "route classifier returned invalid scores")
		return Result{Action: Clarify}, nil
	}
	if err == nil {
		slog.InfoContext(ctx, "route decision completed", "action", result.Action, "confidence", result.Confidence, "duration_ms", time.Since(started).Milliseconds())
	}
	return result, err
}

func (engine *Engine) Check(ctx context.Context, question, evidence string) (CheckResult, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return CheckResult{}, ErrEmptyQuestion
	}
	if engine.checkClassifier == nil {
		return CheckResult{}, ErrCheckUnavailable
	}

	started := time.Now()
	scores, err := engine.checkClassifier.Evaluate(ctx, question, strings.TrimSpace(evidence))
	if err != nil {
		if errors.Is(err, ErrInvalidScores) {
			slog.WarnContext(ctx, "yes/no classifier returned invalid scores", "duration_ms", time.Since(started).Milliseconds())
			return CheckResult{}, nil
		}
		slog.WarnContext(ctx, "yes/no evaluation failed", "error_type", fmt.Sprintf("%T", err), "duration_ms", time.Since(started).Milliseconds())
		return CheckResult{}, fmt.Errorf("evaluate yes/no question: %w", err)
	}
	result, err := Evaluate(scores, engine.minimumConfidence)
	if errors.Is(err, ErrInvalidScores) {
		slog.WarnContext(ctx, "yes/no classifier returned invalid scores")
		return CheckResult{}, nil
	}
	if err == nil {
		slog.InfoContext(ctx, "yes/no evaluation completed", "answer_known", result.Answer != nil, "confidence", result.Confidence, "duration_ms", time.Since(started).Milliseconds())
	}
	return result, err
}
