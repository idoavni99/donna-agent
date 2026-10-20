package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPProfileSearcherUsesCandidateOnlyContract(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer search-key" {
			t.Errorf("unexpected request: method=%s authorization=%q", request.Method, request.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"first_name":"Alex","last_name":"Lee","work_field":"Design","location":"Toronto"}` {
			t.Errorf("unexpected request body: %s", body)
		}
		_, _ = writer.Write([]byte(`{"candidates":[{"url":"https://network.example/alex","platform":"network","display_name":"Alex Lee"}]}`))
	}))
	defer server.Close()

	searcher, err := NewHTTPProfileSearcher(server.URL, server.Client(), "search-key")
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := searcher.SearchPublicProfiles(context.Background(), PersonLookup{
		FirstName: "Alex",
		LastName:  "Lee",
		WorkField: "Design",
		Location:  "Toronto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].URL != "https://network.example/alex" || profiles[0].SelfLinkedProfiles != nil {
		t.Fatalf("unexpected candidate profiles: %+v", profiles)
	}
}

func TestHTTPProfileSearcherRejectsInsecureEndpointAndExtraFields(t *testing.T) {
	if _, err := NewHTTPProfileSearcher("http://search.example", nil, ""); err == nil {
		t.Fatal("expected non-HTTPS endpoint to be rejected")
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"candidates":[{"url":"https://network.example/alex","platform":"network","display_name":"Alex Lee","age":32}]}`))
	}))
	defer server.Close()
	searcher, err := NewHTTPProfileSearcher(server.URL, server.Client(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := searcher.SearchPublicProfiles(context.Background(), PersonLookup{FirstName: "Alex", LastName: "Lee", WorkField: "Design"}); err == nil {
		t.Fatal("expected unsupported candidate fields to be rejected")
	}
}
