package decision

import (
	"context"
	"errors"
	"testing"
)

type testClassifier struct {
	scores Scores
	err    error
	query  string
}

func (classifier *testClassifier) Classify(_ context.Context, query string) (Scores, error) {
	classifier.query = query
	return classifier.scores, classifier.err
}

type testYesNoClassifier struct {
	scores   YesNoScores
	err      error
	question string
	evidence string
}

func (classifier *testYesNoClassifier) Evaluate(_ context.Context, question, evidence string) (YesNoScores, error) {
	classifier.question = question
	classifier.evidence = evidence
	return classifier.scores, classifier.err
}

func TestDecodeScoresAcceptsValidJSON(t *testing.T) {
	scores, err := DecodeScores([]byte(`{"answer_directly":0.03,"structured_lookup":0.08,"semantic_search":0.72,"hybrid_search":0.12,"clarify":0.05}`))
	if err != nil {
		t.Fatal(err)
	}
	if scores.SemanticSearch != 0.72 {
		t.Fatalf("unexpected semantic search score: %v", scores.SemanticSearch)
	}
}

func TestDecodeScoresRejectsIncompleteAndUnknownFields(t *testing.T) {
	tests := []string{
		`{"semantic_search":1}`,
		`{"answer_directly":0,"structured_lookup":0,"semantic_search":1,"hybrid_search":0,"clarify":0,"extra":0}`,
		`{"answer_directly":null,"structured_lookup":0,"semantic_search":1,"hybrid_search":0,"clarify":0}`,
	}
	for _, payload := range tests {
		if _, err := DecodeScores([]byte(payload)); err == nil {
			t.Errorf("expected invalid payload to be rejected: %s", payload)
		}
	}
}

func TestDecodeScoresRejectsTrailingData(t *testing.T) {
	payload := `{"answer_directly":0,"structured_lookup":0,"semantic_search":1,"hybrid_search":0,"clarify":0} {}`
	if _, err := DecodeScores([]byte(payload)); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestRouteSelectsHighestScore(t *testing.T) {
	result, err := Route(Scores{SemanticSearch: 0.72, HybridSearch: 0.12, StructuredLookup: 0.08, Clarify: 0.05, AnswerDirectly: 0.03}, 0.55)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != SemanticSearch || result.Confidence != 0.72 {
		t.Fatalf("unexpected route: %+v", result)
	}
}

func TestRouteClarifiesBelowConfidenceThreshold(t *testing.T) {
	result, err := Route(Scores{SemanticSearch: 0.4, HybridSearch: 0.3, StructuredLookup: 0.15, Clarify: 0.1, AnswerDirectly: 0.05}, 0.55)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != Clarify || result.Confidence != 0.4 {
		t.Fatalf("unexpected route: %+v", result)
	}
}

func TestRouteRejectsInvalidProbabilityTotal(t *testing.T) {
	_, err := Route(Scores{SemanticSearch: 0.8, HybridSearch: 0.3}, 0.55)
	if !errors.Is(err, ErrInvalidScores) {
		t.Fatalf("expected ErrInvalidScores, got %v", err)
	}
}

func TestRouteClarifiesTiedScores(t *testing.T) {
	result, err := Route(Scores{SemanticSearch: 0.5, HybridSearch: 0.5}, 0.4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != Clarify {
		t.Fatalf("expected clarification for tied scores, got %+v", result)
	}
}

func TestEngineTrimsQueryAndRoutesScores(t *testing.T) {
	classifier := &testClassifier{scores: Scores{SemanticSearch: 0.8, Clarify: 0.2}}
	engine, err := NewEngine(classifier, 0.55)
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Decide(context.Background(), "  where did I save it?  ")
	if err != nil {
		t.Fatal(err)
	}
	if classifier.query != "where did I save it?" || result.Action != SemanticSearch {
		t.Fatalf("unexpected decision: query=%q result=%+v", classifier.query, result)
	}
}

func TestEngineClarifiesOnInvalidModelScores(t *testing.T) {
	classifier := &testClassifier{err: ErrInvalidScores}
	engine, err := NewEngine(classifier, 0.55)
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Decide(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != Clarify {
		t.Fatalf("expected clarification, got %+v", result)
	}
}

func TestEngineRejectsEmptyQuery(t *testing.T) {
	engine, err := NewEngine(&testClassifier{}, 0.55)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Decide(context.Background(), "  "); !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("expected ErrEmptyQuery, got %v", err)
	}
}

func TestDecodeYesNoScoresRequiresExactProbabilities(t *testing.T) {
	scores, err := DecodeYesNoScores([]byte(`{"yes":0.9,"no":0.1}`))
	if err != nil {
		t.Fatal(err)
	}
	if scores.Yes != 0.9 || scores.No != 0.1 {
		t.Fatalf("unexpected yes/no scores: %+v", scores)
	}
	for _, payload := range []string{`{"yes":1}`, `{"yes":1,"no":0,"other":0}`, `{"yes":null,"no":1}`} {
		if _, err := DecodeYesNoScores([]byte(payload)); err == nil {
			t.Errorf("expected invalid yes/no payload to be rejected: %s", payload)
		}
	}
}

func TestEvaluateReturnsYesNoOrUncertain(t *testing.T) {
	tests := []struct {
		name  string
		score YesNoScores
		want  *bool
	}{
		{name: "yes", score: YesNoScores{Yes: 0.9, No: 0.1}, want: boolPointer(true)},
		{name: "no", score: YesNoScores{Yes: 0.1, No: 0.9}, want: boolPointer(false)},
		{name: "uncertain below threshold", score: YesNoScores{Yes: 0.52, No: 0.48}},
		{name: "uncertain tie", score: YesNoScores{Yes: 0.5, No: 0.5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Evaluate(test.score, 0.55)
			if err != nil {
				t.Fatal(err)
			}
			if test.want == nil && result.Answer != nil {
				t.Fatalf("expected uncertain answer, got %+v", result)
			}
			if test.want != nil && (result.Answer == nil || *result.Answer != *test.want) {
				t.Fatalf("unexpected answer: %+v", result)
			}
		})
	}
}

func TestEngineCheckUsesYesNoClassifier(t *testing.T) {
	classifier := &testClassifier{}
	checkClassifier := &testYesNoClassifier{scores: YesNoScores{Yes: 0.9, No: 0.1}}
	engine, err := NewEngineWithCheck(classifier, checkClassifier, 0.55)
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Check(context.Background(), "  Is it saved?  ", "  project notes  ")
	if err != nil {
		t.Fatal(err)
	}
	if result.Answer == nil || !*result.Answer || checkClassifier.question != "Is it saved?" || checkClassifier.evidence != "project notes" {
		t.Fatalf("unexpected check result: %+v; classifier=%+v", result, checkClassifier)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
