package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"donna/internal/decision"
)

type toolTestDecisionLayer struct {
	routeResult decision.Result
	checkResult decision.CheckResult
	routeQuery  string
	question    string
	evidence    string
}

func (layer *toolTestDecisionLayer) Decide(_ context.Context, query string) (decision.Result, error) {
	layer.routeQuery = query
	return layer.routeResult, nil
}

func (layer *toolTestDecisionLayer) Check(_ context.Context, question, evidence string) (decision.CheckResult, error) {
	layer.question = question
	layer.evidence = evidence
	return layer.checkResult, nil
}

func newToolTestBrain(t *testing.T, layer *toolTestDecisionLayer) *Brain {
	t.Helper()
	brain, err := NewBrain(testBrowser{pages: map[string]ProfilePage{}}, &testAnalyst{}, layer)
	if err != nil {
		t.Fatal(err)
	}
	return brain
}

func TestToolsReturnProviderNeutralSchemas(t *testing.T) {
	brain := newToolTestBrain(t, &toolTestDecisionLayer{})
	definitions := brain.Tools()
	if len(definitions) != 3 {
		t.Fatalf("expected three tools, got %d", len(definitions))
	}
	for _, definition := range definitions {
		if definition.Name == "" || definition.Description == "" || !json.Valid(definition.InputSchema) {
			t.Errorf("invalid tool definition: %+v", definition)
		}
	}
	definitions[0].InputSchema[0] = 'x'
	if !json.Valid(brain.Tools()[0].InputSchema) {
		t.Fatal("mutating returned tools changed the brain's definitions")
	}
}

func TestDecisionToolsDoNotRequireResearchProviders(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	if tools := brain.Tools(); len(tools) != 2 || tools[0].Name != ToolRouteQuery || tools[1].Name != ToolCheckYesNo {
		t.Fatalf("unexpected available tools: %+v", tools)
	}
	if _, err := brain.InvokeTool(context.Background(), ToolResearchPerson, json.RawMessage(`{"primary_profile_url":"https://example.com/profile"}`)); !errors.Is(err, ErrResearchUnavailable) {
		t.Fatalf("expected ErrResearchUnavailable, got %v", err)
	}
}

func TestProfileSearchToolRequiresSupportedFields(t *testing.T) {
	searcher := &testProfileSearcher{profiles: []ProfilePage{{URL: "https://network.example/alex", DisplayName: "Alex Lee"}}}
	brain, err := NewBrainWithProfileSearch(nil, nil, searcher, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	if tools := brain.Tools(); len(tools) != 3 || tools[2].Name != ToolFindPersonProfiles {
		t.Fatalf("candidate search tool should be advertised: %+v", tools)
	}

	output, err := brain.InvokeTool(context.Background(), ToolFindPersonProfiles, json.RawMessage(`{"first_name":"Alex","last_name":"Lee","work_field":"Design"}`))
	if err != nil {
		t.Fatal(err)
	}
	var report ProfileSearchReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 1 || report.Candidates[0].IdentityConfirmed {
		t.Fatalf("search result must remain unverified: %+v", report)
	}

	_, err = brain.InvokeTool(context.Background(), ToolFindPersonProfiles, json.RawMessage(`{"first_name":"Alex","last_name":"Lee","work_field":"Design","age":32,"appearance":"red hair"}`))
	if !errors.Is(err, ErrInvalidToolInput) {
		t.Fatalf("expected age/appearance fields to be rejected, got %v", err)
	}
}

func TestInvokeToolRoutesQueries(t *testing.T) {
	layer := &toolTestDecisionLayer{routeResult: decision.Result{Action: decision.SemanticSearch, Confidence: 0.9}}
	brain := newToolTestBrain(t, layer)

	output, err := brain.InvokeTool(context.Background(), ToolRouteQuery, json.RawMessage(`{"query":"find my note"}`))
	if err != nil {
		t.Fatal(err)
	}
	if layer.routeQuery != "find my note" || !json.Valid(output) {
		t.Fatalf("unexpected tool result: query=%q output=%s", layer.routeQuery, output)
	}
}

func TestInvokeToolPreservesUnknownYesNoAnswer(t *testing.T) {
	layer := &toolTestDecisionLayer{checkResult: decision.CheckResult{Confidence: 0.5}}
	brain := newToolTestBrain(t, layer)
	output, err := brain.InvokeTool(context.Background(), ToolCheckYesNo, json.RawMessage(`{"question":"Is it saved?","evidence":"No matching item in the notes index"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result decision.CheckResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result.Answer != nil || layer.question != "Is it saved?" || layer.evidence == "" {
		t.Fatalf("expected uncertain answer with evidence, got %+v", result)
	}
}

func TestInvokeToolRejectsUnknownNamesAndFields(t *testing.T) {
	brain := newToolTestBrain(t, &toolTestDecisionLayer{})
	if _, err := brain.InvokeTool(context.Background(), "vendor_private_tool", json.RawMessage(`{}`)); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("expected ErrUnknownTool, got %v", err)
	}
	if _, err := brain.InvokeTool(context.Background(), ToolRouteQuery, json.RawMessage(`{"query":"x","extra":true}`)); err == nil {
		t.Fatal("expected unknown input field to be rejected")
	}
}

func TestInvokeToolRequiresEvidenceForYesNo(t *testing.T) {
	brain := newToolTestBrain(t, &toolTestDecisionLayer{})
	if _, err := brain.InvokeTool(context.Background(), ToolCheckYesNo, json.RawMessage(`{"question":"Is it saved?","evidence":" "}`)); err == nil {
		t.Fatal("expected missing evidence to be rejected")
	}
}
