package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"donna/internal/decision"
)

type DecisionLayer interface {
	Decide(context.Context, string) (decision.Result, error)
	Check(context.Context, string, string) (decision.CheckResult, error)
}

type Brain struct {
	browser         PublicBrowser
	analyst         PersonAnalyst
	profileSearcher PublicProfileSearcher
	decision        DecisionLayer
}

func NewBrain(browser PublicBrowser, analyst PersonAnalyst, decisionLayer DecisionLayer) (*Brain, error) {
	return NewBrainWithProfileSearch(browser, analyst, nil, decisionLayer)
}

func NewBrainWithProfileSearch(browser PublicBrowser, analyst PersonAnalyst, profileSearcher PublicProfileSearcher, decisionLayer DecisionLayer) (*Brain, error) {
	if decisionLayer == nil {
		return nil, errors.New("decision layer is required")
	}
	return &Brain{browser: browser, analyst: analyst, profileSearcher: profileSearcher, decision: decisionLayer}, nil
}

func (brain *Brain) Decide(ctx context.Context, query string) (decision.Result, error) {
	return brain.decision.Decide(ctx, query)
}

func (brain *Brain) Check(ctx context.Context, question, evidence string) (decision.CheckResult, error) {
	return brain.decision.Check(ctx, question, evidence)
}

func (brain *Brain) ResearchPerson(ctx context.Context, request PersonResearchRequest) (PersonResearchReport, error) {
	started := time.Now()
	slog.InfoContext(ctx, "public profile research started", "candidate_url_count", len(request.CandidateProfileURLs))
	if brain.browser == nil || brain.analyst == nil {
		slog.WarnContext(ctx, "public profile research unavailable", "reason", "browser_or_analyst_not_configured")
		return PersonResearchReport{}, ErrResearchUnavailable
	}
	profiles, unverifiedURLs, err := researchProfiles(ctx, brain.browser, request)
	if err != nil {
		slog.WarnContext(ctx, "public profile retrieval failed", "error_type", fmt.Sprintf("%T", err))
		return PersonResearchReport{}, err
	}

	analysis, err := brain.analyst.Analyze(ctx, profiles, strings.TrimSpace(request.Question))
	if err != nil {
		slog.WarnContext(ctx, "public profile analysis failed", "verified_profile_count", len(profiles), "error_type", fmt.Sprintf("%T", err))
		return PersonResearchReport{}, fmt.Errorf("analyze public profile sources: %w", err)
	}
	if err := validateCitations(analysis, profiles); err != nil {
		slog.WarnContext(ctx, "public profile analysis failed citation validation", "verified_profile_count", len(profiles), "error_type", fmt.Sprintf("%T", err))
		return PersonResearchReport{}, err
	}
	slog.InfoContext(ctx, "public profile research completed", "verified_profile_count", len(profiles), "unverified_profile_count", len(unverifiedURLs), "summary_count", len(analysis.Summary), "conversation_tip_count", len(analysis.ConversationTips), "duration_ms", time.Since(started).Milliseconds())
	return PersonResearchReport{
		Profiles:         profiles,
		UnverifiedURLs:   unverifiedURLs,
		Summary:          analysis.Summary,
		ConversationTips: analysis.ConversationTips,
	}, nil
}

func (brain *Brain) FindPersonProfiles(ctx context.Context, lookup PersonLookup) (ProfileSearchReport, error) {
	if brain.profileSearcher == nil {
		slog.WarnContext(ctx, "public profile search unavailable", "reason", "search_provider_not_configured")
		return ProfileSearchReport{}, ErrProfileSearchUnavailable
	}
	started := time.Now()
	slog.InfoContext(ctx, "public profile search started", "has_work_field", strings.TrimSpace(lookup.WorkField) != "", "has_location", strings.TrimSpace(lookup.Location) != "")
	report, err := findPersonProfiles(ctx, brain.profileSearcher, lookup)
	if err != nil {
		slog.WarnContext(ctx, "public profile search failed", "error_type", fmt.Sprintf("%T", err), "duration_ms", time.Since(started).Milliseconds())
		return ProfileSearchReport{}, err
	}
	slog.InfoContext(ctx, "public profile search completed", "candidate_count", len(report.Candidates), "duration_ms", time.Since(started).Milliseconds())
	return report, nil
}
