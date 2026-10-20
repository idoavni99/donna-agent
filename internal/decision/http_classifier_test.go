package decision

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClassifierSendsQueryAndDecodesScores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: method=%s content-type=%q", request.Method, request.Header.Get("Content-Type"))
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"query":"find my note"}` {
			t.Errorf("unexpected request body: %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"answer_directly":0.02,"structured_lookup":0.08,"semantic_search":0.7,"hybrid_search":0.15,"clarify":0.05}`))
	}))
	defer server.Close()

	classifier, err := NewHTTPClassifier(server.URL, server.Client(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	scores, err := classifier.Classify(context.Background(), "find my note")
	if err != nil {
		t.Fatal(err)
	}
	if scores.SemanticSearch != 0.7 {
		t.Fatalf("unexpected scores: %+v", scores)
	}
}

func TestHTTPClassifierRejectsInvalidModelOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"semantic_search":1}`))
	}))
	defer server.Close()

	classifier, err := NewHTTPClassifier(server.URL, server.Client(), "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = classifier.Classify(context.Background(), "query")
	if !errors.Is(err, ErrInvalidScores) {
		t.Fatalf("expected ErrInvalidScores, got %v", err)
	}
}

func TestHTTPClassifierEvaluatesYesNoQuestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"question":"Is it saved?","evidence":"project notes"}` {
			t.Errorf("unexpected yes/no request body: %s", body)
		}
		_, _ = writer.Write([]byte(`{"yes":0.9,"no":0.1}`))
	}))
	defer server.Close()

	classifier, err := NewHTTPClassifier(server.URL, server.Client(), "")
	if err != nil {
		t.Fatal(err)
	}
	scores, err := classifier.Evaluate(context.Background(), "Is it saved?", "project notes")
	if err != nil {
		t.Fatal(err)
	}
	if scores.Yes != 0.9 || scores.No != 0.1 {
		t.Fatalf("unexpected yes/no scores: %+v", scores)
	}
}

func TestNewHTTPClassifierRequiresAbsoluteHTTPURL(t *testing.T) {
	if _, err := NewHTTPClassifier("/classify", nil, ""); err == nil {
		t.Fatal("expected invalid endpoint to be rejected")
	}
}
