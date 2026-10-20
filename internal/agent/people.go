package agent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const maxCandidateProfiles = 10
const maxProfileSearchResults = 5
const minSummaryItems = 2
const maxSummaryItems = 3
const maxConversationTips = 3
const maxCitedTextBytes = 500

var (
	ErrInvalidProfileURL        = errors.New("profile URL must be an absolute HTTPS URL")
	ErrInvalidPersonLookup      = errors.New("person lookup requires a first name, last name, and work field or broad location")
	ErrTooManyProfiles          = errors.New("too many candidate profile URLs")
	ErrTooManySearchResults     = errors.New("profile search returned too many candidates")
	ErrProfileSearchUnavailable = errors.New("public profile search provider is not configured")
	ErrResearchUnavailable      = errors.New("person research providers are not configured")
)

type PublicBrowser interface {
	OpenPublicProfile(context.Context, string) (ProfilePage, error)
}

type PublicProfileSearcher interface {
	SearchPublicProfiles(context.Context, PersonLookup) ([]ProfilePage, error)
}

type ProfilePage struct {
	URL                string   `json:"url"`
	Platform           string   `json:"platform"`
	DisplayName        string   `json:"display_name"`
	PublicBio          string   `json:"public_bio"`
	SelfLinkedProfiles []string `json:"self_linked_profiles"`
}

type PersonResearchRequest struct {
	PrimaryProfileURL    string   `json:"primary_profile_url"`
	CandidateProfileURLs []string `json:"candidate_profile_urls"`
	Question             string   `json:"question,omitempty"`
}

type PersonLookup struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	WorkField string `json:"work_field,omitempty"`
	Location  string `json:"location,omitempty"`
}

type ProfileCandidate struct {
	URL               string `json:"url"`
	Platform          string `json:"platform"`
	DisplayName       string `json:"display_name"`
	IdentityConfirmed bool   `json:"identity_confirmed"`
}

type ProfileSearchReport struct {
	Candidates []ProfileCandidate `json:"candidates"`
	Note       string             `json:"note"`
}

type CitedText struct {
	Text      string `json:"text"`
	SourceURL string `json:"source_url"`
}

type PersonAnalysis struct {
	Summary          []CitedText `json:"summary"`
	ConversationTips []CitedText `json:"conversation_tips"`
}

type PersonAnalyst interface {
	Analyze(context.Context, []ProfilePage, string) (PersonAnalysis, error)
}

type PersonResearchReport struct {
	Profiles         []ProfilePage `json:"profiles"`
	UnverifiedURLs   []string      `json:"unverified_urls,omitempty"`
	Summary          []CitedText   `json:"summary"`
	ConversationTips []CitedText   `json:"conversation_tips"`
}

func researchProfiles(ctx context.Context, browser PublicBrowser, request PersonResearchRequest) ([]ProfilePage, []string, error) {
	if len(request.CandidateProfileURLs) > maxCandidateProfiles {
		return nil, nil, ErrTooManyProfiles
	}
	primaryURL, err := normalizeProfileURL(request.PrimaryProfileURL)
	if err != nil {
		return nil, nil, err
	}

	primary, err := browser.OpenPublicProfile(ctx, primaryURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open primary profile: %w", err)
	}
	primary.URL, err = normalizeProfileURL(primary.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("browser returned invalid primary profile URL: %w", err)
	}
	if primary.URL != primaryURL {
		return nil, nil, errors.New("browser returned a different primary profile URL")
	}

	profiles := []ProfilePage{primary}
	unverified := make([]string, 0)
	seen := map[string]struct{}{primaryURL: {}}
	primaryLinks := normalizeLinks(primary.SelfLinkedProfiles)
	for _, candidateURL := range request.CandidateProfileURLs {
		candidateURL, err = normalizeProfileURL(candidateURL)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := seen[candidateURL]; ok {
			continue
		}
		seen[candidateURL] = struct{}{}

		candidate, err := browser.OpenPublicProfile(ctx, candidateURL)
		if err != nil {
			return nil, nil, fmt.Errorf("open candidate profile: %w", err)
		}
		candidate.URL, err = normalizeProfileURL(candidate.URL)
		if err != nil || candidate.URL != candidateURL {
			unverified = append(unverified, candidateURL)
			continue
		}

		_, primaryLinksCandidate := primaryLinks[candidateURL]
		_, candidateLinksPrimary := normalizeLinks(candidate.SelfLinkedProfiles)[primaryURL]
		if primaryLinksCandidate || candidateLinksPrimary {
			profiles = append(profiles, candidate)
		} else {
			unverified = append(unverified, candidateURL)
		}
	}
	return profiles, unverified, nil
}

func normalizeProfileURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", ErrInvalidProfileURL
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func normalizeLinks(links []string) map[string]struct{} {
	normalized := make(map[string]struct{}, len(links))
	for _, link := range links {
		if value, err := normalizeProfileURL(link); err == nil {
			normalized[value] = struct{}{}
		}
	}
	return normalized
}

func validateCitations(analysis PersonAnalysis, profiles []ProfilePage) error {
	if len(analysis.Summary) < minSummaryItems || len(analysis.Summary) > maxSummaryItems {
		return fmt.Errorf("summary must contain %d to %d cited sentences", minSummaryItems, maxSummaryItems)
	}
	if len(analysis.ConversationTips) > maxConversationTips {
		return fmt.Errorf("conversation tips exceed %d items", maxConversationTips)
	}
	allowed := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		allowed[profile.URL] = struct{}{}
	}
	for _, item := range append(analysis.Summary, analysis.ConversationTips...) {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			return errors.New("analysis items must include text")
		}
		if len(text) > maxCitedTextBytes {
			return fmt.Errorf("analysis items must not exceed %d bytes", maxCitedTextBytes)
		}
		sourceURL, err := normalizeProfileURL(item.SourceURL)
		if err != nil {
			return fmt.Errorf("analysis citation is invalid: %w", err)
		}
		if _, ok := allowed[sourceURL]; !ok {
			return fmt.Errorf("analysis citation is not a verified profile source: %s", sourceURL)
		}
	}
	return nil
}

func findPersonProfiles(ctx context.Context, searcher PublicProfileSearcher, lookup PersonLookup) (ProfileSearchReport, error) {
	lookup = PersonLookup{
		FirstName: strings.TrimSpace(lookup.FirstName),
		LastName:  strings.TrimSpace(lookup.LastName),
		WorkField: strings.TrimSpace(lookup.WorkField),
		Location:  strings.TrimSpace(lookup.Location),
	}
	if lookup.FirstName == "" || lookup.LastName == "" || (lookup.WorkField == "" && lookup.Location == "") || len(lookup.FirstName) > 100 || len(lookup.LastName) > 100 || len(lookup.WorkField) > 160 || len(lookup.Location) > 160 {
		return ProfileSearchReport{}, ErrInvalidPersonLookup
	}

	profiles, err := searcher.SearchPublicProfiles(ctx, lookup)
	if err != nil {
		return ProfileSearchReport{}, fmt.Errorf("search public profiles: %w", err)
	}
	if len(profiles) > maxCandidateProfiles {
		return ProfileSearchReport{}, ErrTooManySearchResults
	}

	candidates := make([]ProfileCandidate, 0, min(len(profiles), maxProfileSearchResults))
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		profileURL, err := normalizeProfileURL(profile.URL)
		if err != nil {
			continue
		}
		if _, exists := seen[profileURL]; exists {
			continue
		}
		seen[profileURL] = struct{}{}
		candidates = append(candidates, ProfileCandidate{
			URL:               profileURL,
			Platform:          strings.TrimSpace(profile.Platform),
			DisplayName:       strings.TrimSpace(profile.DisplayName),
			IdentityConfirmed: false,
		})
		if len(candidates) == maxProfileSearchResults {
			break
		}
	}
	return ProfileSearchReport{
		Candidates: candidates,
		Note:       "Search results are unverified candidates. Confirm a profile URL before researching it; name, work field, and location do not prove identity.",
	}, nil
}
