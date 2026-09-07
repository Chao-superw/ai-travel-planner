package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-travel/internal/domain"
)

func TestGLMGenerateRequestAndCandidate(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/paas/v4/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("bad auth/path")
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		b, _ := json.Marshal(req)
		text := string(b)
		if req["model"] != "glm-5.3-flash" || req["stream"] != false || !strings.Contains(text, "json_object") || strings.Contains(text, "SERVICE-SECRET") || strings.Contains(text, "service_token") {
			t.Fatalf("bad body %s", text)
		}
		messages := req["messages"].([]any)
		promptText := messages[0].(map[string]any)["content"].(string)
		var userData map[string]any
		if err := json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &userData); err != nil {
			t.Fatal(err)
		}
		if userData["operation"] != "generate" {
			t.Fatalf("operation=%v", userData["operation"])
		}
		for _, phrase := range []string{"This is Generate, including repair", "complete plan for ALL dates", "including unchanged activities", "never return a patch", "longitude and latitude", "no route data", "relaxed: prefer 1-2", "balanced: prefer 2-3", "intensive: prefer 3-4", "soft targets", "when the allowed window permits", "at least 90 minutes of unoccupied continuous travel time", "soft conservative recommendation", "not a reason by itself", "actual route duration", "plus exactly 300 seconds", "Chinese", "For Generate only", "at least 1 sightseeing", "at least 1 meal", "90 minutes", "unoccupied continuous gap", "route duration plus 300 seconds", "geographically close", `{"outcome":"unsatisfied","plan":null,"issues":["reason"]}`} {
			if !strings.Contains(promptText, phrase) {
				t.Fatalf("prompt missing %q: %s", phrase, promptText)
			}
		}
		w.Header().Set("X-Request-Id", "rid")
		w.Write([]byte(`{"id":"rid","model":"glm","choices":[{"finish_reason":"stop","message":{"content":"{\"outcome\":\"candidate\",\"plan\":{\"title\":\"北京\",\"summary\":\"行程\",\"activities\":[],\"warnings\":[]},\"issues\":[]}"}}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`))
	}))
	defer s.Close()
	got, err := (&GLM{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{ServiceToken: "SERVICE-SECRET", JobID: "j"})
	if err != nil || got.Outcome != "candidate" || got.Plan == nil || got.ModelRun == nil || got.ModelRun.ProviderRequestID != "rid" || got.ModelRun.PromptTokens == nil || *got.ModelRun.PromptTokens != 10 || got.RawOutput != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestGLMRevisionPromptRequiresCompleteScopedReplacementAndFreeGaps(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("messages=%+v", req.Messages)
		}
		prompt := req.Messages[0].Content
		for _, phrase := range []string{"editable_item_ids", "complete replacement", "unselected base activities", "time window is only", "must not be returned", "unoccupied continuous gap", "meal, rest, or hotel", "reduce the number", "replace them with closer", "adjust start times within", "Never shorten a known required stay", "never exceed the scope"} {
			if !strings.Contains(prompt, phrase) {
				t.Fatalf("revision prompt missing %q: %s", phrase, prompt)
			}
		}
		var userData map[string]any
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &userData); err != nil {
			t.Fatal(err)
		}
		if userData["operation"] != "revise" {
			t.Fatalf("operation=%v", userData["operation"])
		}
		w.Write([]byte(`{"model":"glm","choices":[{"finish_reason":"stop","message":{"content":"{\"outcome\":\"unsatisfied\",\"issues\":[\"none\"]}"}}]}`))
	}))
	defer s.Close()
	_, err := (&GLM{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Revise(context.Background(), domain.PlanningRequest{Scope: &domain.Scope{Date: "2026-09-08"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGLMRejectsBackendOwnedAndTrailingFields(t *testing.T) {
	for _, content := range []string{
		`{"outcome":"candidate","plan":{"title":"x","summary":"x","activities":[],"warnings":[],"routes":[]},"issues":[]}`,
		`{"outcome":"candidate","plan":{"title":"x","summary":"x","activities":[{"id":"a","date":"2026-09-08","start_minute":60,"end_minute":120,"kind":"sightseeing","place_id":"amap:p","title":"x","reason":"x","costs":[] }],"warnings":[]},"issues":[]}`,
		`{"outcome":"candidate","plan":{"title":"x","summary":"x","activities":[{"id":"a","date":"2026-09-08","start_minute":60,"end_minute":120,"kind":"sightseeing","place_id":"amap:p","title":"x","reason":"x","place":{} }],"warnings":[]},"issues":[]}`,
		`{"outcome":"candidate","plan":{"title":"x","summary":"x","activities":[],"warnings":[],"do-not-leak-this-field":true},"issues":[]}`,
		`{"outcome":"unsatisfied","issues":["none"]} {"extra":"value"}`,
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"model": "glm", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}}})
		}))
		got, err := (&GLM{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{})
		s.Close()
		if err != nil || got.Outcome != "invalid_output" || len(got.Issues) == 0 {
			t.Fatalf("content=%s got=%+v err=%v", content, got, err)
		}
		if !strings.Contains(content, `} {`) && got.Issues[0] != "model output contains an unsupported field" {
			t.Fatalf("unsafe or vague issue: %q", got.Issues[0])
		}
		if strings.Contains(got.Issues[0], "do-not-leak") {
			t.Fatalf("issue leaked provider field: %q", got.Issues[0])
		}
	}
}

func TestGLMExplainsInvalidUnsatisfiedShapeWithoutLeakingRawContent(t *testing.T) {
	content := `{"outcome":"unsatisfied","plan":{"title":"x","summary":"x","activities":[],"warnings":[]},"issues":[]}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"model": "glm", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}}})
	}))
	defer s.Close()
	got, err := (&GLM{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{})
	if err != nil || got.Outcome != "invalid_output" || len(got.Issues) != 1 || got.Issues[0] != "unsatisfied output must omit plan and include at least one issue" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if strings.Contains(got.Issues[0], "secret") || strings.Contains(got.Issues[0], content) {
		t.Fatalf("unsafe issue: %q", got.Issues[0])
	}
}

func TestGLMAbsentUsageRemainsUnknown(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"model":"glm","choices":[{"finish_reason":"stop","message":{"content":"{\"outcome\":\"unsatisfied\",\"issues\":[\"none\"]}"}}]}`))
	}))
	defer s.Close()
	got, err := (&GLM{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{})
	if err != nil || got.ModelRun.PromptTokens != nil || got.ModelRun.CompletionTokens != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestGLMProjectsRepairPlansWithoutMutatingRequest(t *testing.T) {
	fee := int64(250)
	plan := domain.Plan{Title: "杭州", Summary: "三日", Activities: []domain.Activity{{ID: "a", Date: "2026-10-01", StartMinute: 540, EndMinute: 630, Kind: "sightseeing", PlaceID: "amap:p", Title: "馆", Reason: "近", Place: &domain.Place{ID: "amap:p", Name: "重复快照", Location: "120,30"}, Costs: []domain.Cost{{Category: "ticket", AmountCents: &fee, Unit: "person", Source: "manual", Estimated: false}}}}, Routes: []domain.Route{{FromItemID: "a", ToItemID: "b", Date: "2026-10-01", Mode: "walking", DistanceM: 123, DurationS: 456, FeeCents: &fee, Provider: "amap", Polyline: strings.Repeat("120,30;", 1000), FromLocation: "120,30", ToLocation: "120.1,30.1"}}, Warnings: []string{"w"}}
	previousBytes, _ := json.Marshal(plan)
	originalPolyline := plan.Routes[0].Polyline
	originalPlace := plan.Activities[0].Place
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var outer struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&outer); err != nil {
			t.Fatal(err)
		}
		var user struct {
			Base           map[string]any `json:"base"`
			PreviousOutput string         `json:"previous_output"`
		}
		if err := json.Unmarshal([]byte(outer.Messages[1].Content), &user); err != nil {
			t.Fatal(err)
		}
		baseBytes, _ := json.Marshal(user.Base)
		projected := string(baseBytes) + user.PreviousOutput
		for _, forbidden := range []string{"polyline", "from_location", "to_location", "重复快照", `"place"`} {
			if strings.Contains(projected, forbidden) {
				t.Fatalf("projection retained %q: %s", forbidden, projected)
			}
		}
		for _, required := range []string{"start_minute", "end_minute", "costs", "amount_cents", "duration_s", "distance_m", "fee_cents"} {
			if !strings.Contains(projected, required) {
				t.Fatalf("projection dropped %q: %s", required, projected)
			}
		}
		if len(user.PreviousOutput) >= len(previousBytes) {
			t.Fatalf("previous output was not reduced: before=%d after=%d", len(previousBytes), len(user.PreviousOutput))
		}
		t.Logf("synthetic previous_output bytes: before=%d after=%d", len(previousBytes), len(user.PreviousOutput))
		w.Write([]byte(`{"model":"glm","choices":[{"finish_reason":"stop","message":{"content":"{\"outcome\":\"unsatisfied\",\"plan\":null,\"issues\":[\"none\"]}"}}]}`))
	}))
	defer s.Close()
	req := domain.PlanningRequest{Base: &plan, PreviousOutput: string(previousBytes)}
	got, err := (&GLM{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Generate(context.Background(), req)
	if err != nil || got.ModelRun == nil || got.ModelRun.PromptVersion != promptVersion {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if plan.Routes[0].Polyline != originalPolyline || plan.Activities[0].Place != originalPlace {
		t.Fatal("input request was mutated")
	}
}

func TestGLMKeepsBoundedUnparseablePreviousOutput(t *testing.T) {
	input := strings.Repeat("invalid-model-text", 400)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var outer struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&outer)
		var user struct {
			PreviousOutput string `json:"previous_output"`
		}
		json.Unmarshal([]byte(outer.Messages[1].Content), &user)
		if len(user.PreviousOutput) != 4096 || user.PreviousOutput != input[:4096] {
			t.Fatalf("len=%d", len(user.PreviousOutput))
		}
		w.Write([]byte(`{"model":"glm","choices":[{"finish_reason":"stop","message":{"content":"{\"outcome\":\"unsatisfied\",\"plan\":null,\"issues\":[\"none\"]}"}}]}`))
	}))
	defer s.Close()
	_, err := (&GLM{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{PreviousOutput: input})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGLMInvalidAndUnsatisfiedOutputs(t *testing.T) {
	for _, tc := range []struct{ name, content, finish, want string }{
		{"invalid", "not json", "stop", "invalid_output"},
		{"truncated", `{"outcome":"candidate"}`, "length", "invalid_output"},
		{"unsatisfied", `{"outcome":"unsatisfied","issues":["没有候选景点"]}`, "stop", "unsatisfied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				json.NewEncoder(w).Encode(map[string]any{"model": "glm", "choices": []any{map[string]any{"finish_reason": tc.finish, "message": map[string]any{"content": tc.content}}}})
			}))
			defer s.Close()
			got, err := (&GLM{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Generate(context.Background(), domain.PlanningRequest{})
			if err != nil || got.Outcome != tc.want || calls != 1 {
				t.Fatalf("got=%+v err=%v calls=%d", got, err, calls)
			}
		})
	}
}

func TestGLMProviderFailureNoSecretOrRetry(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "secret provider body", 429) }))
	defer s.Close()
	_, err := (&GLM{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Revise(context.Background(), domain.PlanningRequest{})
	if err == nil || calls != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
