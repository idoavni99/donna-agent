package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

const (
	ToolRouteQuery         = "donna_route_query"
	ToolCheckYesNo         = "donna_check_yes_no"
	ToolFindPersonProfiles = "donna_find_person_profiles"
	ToolResearchPerson     = "donna_research_person"
)

var (
	ErrUnknownTool      = errors.New("unknown Donna tool")
	ErrInvalidToolInput = errors.New("invalid Donna tool input")
)

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

var toolDefinitions = []ToolDefinition{
	{
		Name:        ToolRouteQuery,
		Description: "Choose how Donna should handle a user query before retrieving memory.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"The user's query"}},"required":["query"],"additionalProperties":false}`),
	},
	{
		Name:        ToolCheckYesNo,
		Description: "Evaluate a yes/no question against the supplied evidence. A null answer means uncertain.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"},"evidence":{"type":"string"}},"required":["question","evidence"],"additionalProperties":false}`),
	},
	{
		Name:        ToolFindPersonProfiles,
		Description: "Find up to five candidate public professional profiles by full name and work field or broad location. Results are unverified; do not treat similarity as identity proof.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"first_name":{"type":"string","minLength":1,"maxLength":100},"last_name":{"type":"string","minLength":1,"maxLength":100},"work_field":{"type":"string","maxLength":160},"location":{"type":"string","maxLength":160}},"required":["first_name","last_name"],"anyOf":[{"required":["work_field"]},{"required":["location"]}],"additionalProperties":false}`),
	},
	{
		Name:        ToolResearchPerson,
		Description: "Research a user-selected public profile, or dig deeper into that same profile by providing a follow-up question. Only explicitly cross-linked profiles are included.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"primary_profile_url":{"type":"string","format":"uri"},"candidate_profile_urls":{"type":"array","items":{"type":"string","format":"uri"},"maxItems":10},"question":{"type":"string","maxLength":1000,"description":"Optional focus or follow-up question about the selected profile"}},"required":["primary_profile_url"],"additionalProperties":false}`),
	},
}

func (brain *Brain) Tools() []ToolDefinition {
	definitions := append([]ToolDefinition(nil), toolDefinitions[:2]...)
	if brain.profileSearcher != nil {
		definitions = append(definitions, toolDefinitions[2])
	}
	if brain.browser == nil || brain.analyst == nil {
		return cloneToolDefinitions(definitions)
	}
	definitions = append(definitions, toolDefinitions[3])
	return cloneToolDefinitions(definitions)
}

func cloneToolDefinitions(definitions []ToolDefinition) []ToolDefinition {
	cloned := make([]ToolDefinition, len(definitions))
	for index, definition := range definitions {
		cloned[index] = definition
		cloned[index].InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	}
	return cloned
}

func (brain *Brain) InvokeTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	started := time.Now()
	slog.InfoContext(ctx, "Donna tool execution started", "tool", name)
	output, err := brain.invokeTool(ctx, name, input)
	if err != nil {
		slog.WarnContext(ctx, "Donna tool execution failed", "tool", name, "error_type", fmt.Sprintf("%T", err), "duration_ms", time.Since(started).Milliseconds())
		return nil, err
	}
	slog.InfoContext(ctx, "Donna tool execution completed", "tool", name, "result_bytes", len(output), "duration_ms", time.Since(started).Milliseconds())
	return output, nil
}

func (brain *Brain) invokeTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	var result any
	switch name {
	case ToolRouteQuery:
		var arguments struct {
			Query string `json:"query"`
		}
		if err := decodeToolInput(input, &arguments); err != nil {
			return nil, err
		}
		decision, err := brain.Decide(ctx, arguments.Query)
		if err != nil {
			return nil, err
		}
		result = decision
	case ToolCheckYesNo:
		var arguments struct {
			Question string `json:"question"`
			Evidence string `json:"evidence"`
		}
		if err := decodeToolInput(input, &arguments); err != nil {
			return nil, err
		}
		if strings.TrimSpace(arguments.Evidence) == "" {
			return nil, fmt.Errorf("%w: evidence is required for a yes/no check", ErrInvalidToolInput)
		}
		check, err := brain.Check(ctx, arguments.Question, arguments.Evidence)
		if err != nil {
			return nil, err
		}
		result = check
	case ToolFindPersonProfiles:
		if brain.profileSearcher == nil {
			return nil, ErrProfileSearchUnavailable
		}
		var lookup PersonLookup
		if err := decodeToolInput(input, &lookup); err != nil {
			return nil, err
		}
		candidates, err := brain.FindPersonProfiles(ctx, lookup)
		if err != nil {
			return nil, err
		}
		result = candidates
	case ToolResearchPerson:
		if brain.browser == nil || brain.analyst == nil {
			return nil, ErrResearchUnavailable
		}
		var request PersonResearchRequest
		if err := decodeToolInput(input, &request); err != nil {
			return nil, err
		}
		report, err := brain.ResearchPerson(ctx, request)
		if err != nil {
			return nil, err
		}
		result = report
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}

	output, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode Donna tool result: %w", err)
	}
	return output, nil
}

func decodeToolInput(input []byte, target any) error {
	if len(bytes.TrimSpace(input)) == 0 {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidToolInput)
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidToolInput, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: input must contain one JSON value", ErrInvalidToolInput)
	}
	if strings.TrimSpace(string(input)) == "null" {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidToolInput)
	}
	return nil
}
