package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"donna/internal/decision"
)

type testBrowser struct {
	pages map[string]ProfilePage
}

func (browser testBrowser) OpenPublicProfile(_ context.Context, profileURL string) (ProfilePage, error) {
	page, ok := browser.pages[profileURL]
	if !ok {
		return ProfilePage{}, errors.New("profile not found")
	}
	return page, nil
}

type testAnalyst struct {
	profiles []ProfilePage
	analysis PersonAnalysis
	question string
}

type testProfileSearcher struct {
	lookup   PersonLookup
	profiles []ProfilePage
	err      error
}

func (searcher *testProfileSearcher) SearchPublicProfiles(_ context.Context, lookup PersonLookup) ([]ProfilePage, error) {
	searcher.lookup = lookup
	return searcher.profiles, searcher.err
}

func (analyst *testAnalyst) Analyze(_ context.Context, profiles []ProfilePage, question string) (PersonAnalysis, error) {
	analyst.profiles = profiles
	analyst.question = question
	return analyst.analysis, nil
}

func TestResearchPersonOnlyIncludesCrossLinkedProfiles(t *testing.T) {
	primaryURL := "https://social.example/alex"
	verifiedURL := "https://network.example/alex"
	unlinkedURL := "https://photos.example/alex"
	browser := testBrowser{pages: map[string]ProfilePage{
		primaryURL: {
			URL:                primaryURL,
			Platform:           "social",
			DisplayName:        "Alex Lee",
			PublicBio:          "Designer",
			SelfLinkedProfiles: []string{verifiedURL + "?from=bio"},
		},
		verifiedURL: {
			URL:         verifiedURL,
			Platform:    "network",
			DisplayName: "Alex Lee",
		},
		unlinkedURL: {
			URL:         unlinkedURL,
			Platform:    "photos",
			DisplayName: "Alex Lee",
		},
	}}
	analyst := &testAnalyst{analysis: PersonAnalysis{
		Summary: []CitedText{
			{Text: "Publicly describes work as design.", SourceURL: primaryURL},
			{Text: "The linked profile lists a recent design project.", SourceURL: verifiedURL},
		},
		ConversationTips: []CitedText{{Text: "Ask about a project they chose to share.", SourceURL: verifiedURL}},
	}}
	brain, err := NewBrain(browser, analyst, nilDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}

	report, err := brain.ResearchPerson(context.Background(), PersonResearchRequest{
		PrimaryProfileURL:    primaryURL + "/?tracking=1",
		CandidateProfileURLs: []string{verifiedURL, unlinkedURL},
		Question:             "What design project did they share?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Profiles) != 2 || report.Profiles[1].URL != verifiedURL {
		t.Fatalf("unexpected verified profiles: %+v", report.Profiles)
	}
	if !reflect.DeepEqual(report.UnverifiedURLs, []string{unlinkedURL}) {
		t.Fatalf("unexpected unverified URLs: %+v", report.UnverifiedURLs)
	}
	if len(analyst.profiles) != 2 {
		t.Fatalf("analyst received unverified profiles: %+v", analyst.profiles)
	}
	if analyst.question != "What design project did they share?" {
		t.Fatalf("analyst did not receive the follow-up question: %q", analyst.question)
	}
}

func TestFindPersonProfilesReturnsUnverifiedCandidates(t *testing.T) {
	searcher := &testProfileSearcher{profiles: []ProfilePage{
		{URL: "https://network.example/alex", Platform: "network", DisplayName: "Alex Lee"},
		{URL: "https://work.example/alex", Platform: "work", DisplayName: "Alex Lee"},
		{URL: "http://unsafe.example/alex", Platform: "unsafe", DisplayName: "Alex Lee"},
	}}
	brain, err := NewBrainWithProfileSearch(nil, nil, searcher, nilDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}

	report, err := brain.FindPersonProfiles(context.Background(), PersonLookup{
		FirstName: " Alex ",
		LastName:  "Lee",
		WorkField: "Product design",
		Location:  "Toronto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if searcher.lookup.FirstName != "Alex" || searcher.lookup.WorkField != "Product design" || searcher.lookup.Location != "Toronto" {
		t.Fatalf("lookup was not normalized: %+v", searcher.lookup)
	}
	if len(report.Candidates) != 2 || report.Candidates[0].IdentityConfirmed || report.Candidates[1].IdentityConfirmed {
		t.Fatalf("expected only unverified HTTPS candidates: %+v", report.Candidates)
	}
}

func TestFindPersonProfilesRequiresProfessionalContext(t *testing.T) {
	searcher := &testProfileSearcher{}
	_, err := findPersonProfiles(context.Background(), searcher, PersonLookup{FirstName: "Alex", LastName: "Lee"})
	if !errors.Is(err, ErrInvalidPersonLookup) {
		t.Fatalf("expected ErrInvalidPersonLookup, got %v", err)
	}
	if searcher.lookup.FirstName != "" {
		t.Fatal("invalid lookup should not reach the search provider")
	}
}

func TestResearchPersonRejectsInvalidProfileURLs(t *testing.T) {
	_, _, err := researchProfiles(context.Background(), testBrowser{}, PersonResearchRequest{
		PrimaryProfileURL: "http://social.example/alex",
	})
	if !errors.Is(err, ErrInvalidProfileURL) {
		t.Fatalf("expected ErrInvalidProfileURL, got %v", err)
	}
}

func TestResearchPersonRejectsUnverifiedCitations(t *testing.T) {
	primaryURL := "https://social.example/alex"
	otherURL := "https://other.example/alex"
	browser := testBrowser{pages: map[string]ProfilePage{
		primaryURL: {URL: primaryURL, DisplayName: "Alex Lee"},
		otherURL:   {URL: otherURL, DisplayName: "Alex Lee"},
	}}
	analyst := &testAnalyst{analysis: PersonAnalysis{
		Summary: []CitedText{
			{Text: "Possibly the same person.", SourceURL: otherURL},
			{Text: "The profile uses a similar name.", SourceURL: otherURL},
		},
	}}
	brain, err := NewBrain(browser, analyst, nilDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = brain.ResearchPerson(context.Background(), PersonResearchRequest{
		PrimaryProfileURL:    primaryURL,
		CandidateProfileURLs: []string{otherURL},
	})
	if err == nil {
		t.Fatal("expected unverified source citation to be rejected")
	}
}

func TestResearchPersonKeepsAnalysisConcise(t *testing.T) {
	primaryURL := "https://social.example/alex"
	browser := testBrowser{pages: map[string]ProfilePage{
		primaryURL: {URL: primaryURL, DisplayName: "Alex Lee"},
	}}
	analyst := &testAnalyst{analysis: PersonAnalysis{
		Summary: []CitedText{
			{Text: string(make([]byte, maxCitedTextBytes+1)), SourceURL: primaryURL},
			{Text: "A second cited sentence.", SourceURL: primaryURL},
		},
	}}
	brain, err := NewBrain(browser, analyst, nilDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := brain.ResearchPerson(context.Background(), PersonResearchRequest{PrimaryProfileURL: primaryURL}); err == nil {
		t.Fatal("expected overly long analysis item to be rejected")
	}
}

func TestValidateCitationsRequiresTwoOrThreeSummarySentences(t *testing.T) {
	profile := ProfilePage{URL: "https://social.example/alex"}
	oneSentence := PersonAnalysis{Summary: []CitedText{{Text: "A cited finding.", SourceURL: profile.URL}}}
	if err := validateCitations(oneSentence, []ProfilePage{profile}); err == nil {
		t.Fatal("expected a one-sentence summary to be rejected")
	}
	twoSentences := PersonAnalysis{Summary: []CitedText{
		{Text: "The profile describes design work.", SourceURL: profile.URL},
		{Text: "It also lists a recent project.", SourceURL: profile.URL},
	}}
	if err := validateCitations(twoSentences, []ProfilePage{profile}); err != nil {
		t.Fatalf("expected two cited sentence-items to pass: %v", err)
	}
}

type nilDecisionLayer struct{}

func (nilDecisionLayer) Decide(context.Context, string) (decision.Result, error) {
	return decision.Result{}, nil
}

func (nilDecisionLayer) Check(context.Context, string, string) (decision.CheckResult, error) {
	return decision.CheckResult{}, nil
}
