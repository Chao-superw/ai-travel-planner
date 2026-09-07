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

func TestAmapSearchFiltersAndMapsPOIs(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/v5/place/text" || q.Get("key") != "secret" || q.Get("region") != "北京" || q.Get("types") != "110000|140100" || q.Get("city_limit") != "true" || q.Get("page_size") != "5" || q.Get("page_num") != "1" || q.Get("show_fields") != "business" {
			t.Fatalf("bad request: %s %v", r.URL.Path, q)
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "1", "infocode": "10000", "pois": []any{
			map[string]any{"id": "a", "name": "故宫", "cityname": "北京市", "citycode": "010", "adcode": "110101", "location": "116.397,39.918", "address": "景山前街", "typecode": "110100", "type": "风景名胜;世界遗产"},
			map[string]any{"id": "a", "name": "重复", "cityname": "北京市", "location": "116.397,39.918", "typecode": "110100"},
			map[string]any{"id": "b", "name": "异地", "cityname": "上海市", "location": "121.4,31.2", "typecode": "110100"},
			map[string]any{"id": "c", "name": "餐馆", "cityname": "北京市", "location": "116.4,39.9", "typecode": "050100"},
			map[string]any{"id": "m", "name": "北京博物馆", "cityname": "北京市", "location": "116.3,39.9", "typecode": "140100", "type": "科教文化服务;博物馆"},
			map[string]any{"id": "p", "name": "停车场", "cityname": "北京市", "location": "116.3,39.8", "typecode": "150900", "type": "交通设施服务;停车场"},
		}})
	}))
	defer s.Close()
	got, more, err := (&Amap{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Search(context.Background(), "北京", "", 0, 5)
	if err != nil || !more || len(got) != 2 {
		t.Fatalf("got=%+v more=%v err=%v", got, more, err)
	}
	p := got[0]
	if p.ID != "amap:a" || p.CoordinateSystem != "GCJ-02" || p.DurationMinutes != 90 || p.FeeCents != nil || !p.Active || p.FetchedAt == "" {
		t.Fatalf("bad place: %+v", p)
	}
	if got[1].ID != "amap:m" || !containsAll(got[1].Tags, "室内", "博物馆") {
		t.Fatalf("bad museum: %+v", got[1])
	}
}

func TestAmapRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, mode, body   string
		distance, duration int64
		fee                *int64
		poly               string
	}{
		{"walking", "walking", `{"status":"1","infocode":"10000","route":{"paths":[{"distance":"4874","cost":{"duration":"3899"},"steps":[{"polyline":"a;b"},{"polyline":"c;d"}]}]}}`, 4874, 3899, int64p(0), "a;b;c;d"},
		{"transit", "transit", `{"status":"1","infocode":"10000","route":{"transits":[{"distance":"5599","walking_distance":"1187","cost":{"duration":"2774","transit_fee":"2.0"},"segments":[{"bus":{"buslines":[{"name":"1路","polyline":"a;b"}]},"walking":{"steps":[{"polyline":"c;d"}]}}]}]}}`, 5599, 2774, int64p(200), "c;d;a;b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("key") != "secret" {
					t.Fatal("missing key")
				}
				w.Write([]byte(tc.body))
			}))
			defer s.Close()
			from := domain.Place{ID: "x", Location: "116,39", CoordinateSystem: "GCJ-02", CityCode: "010"}
			to := domain.Place{ID: "y", Location: "117,40", CoordinateSystem: "GCJ-02", CityCode: "010"}
			got, err := (&Amap{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Route(context.Background(), from, to, tc.mode)
			if err != nil || got.DistanceM != tc.distance || got.DurationS != tc.duration || got.FeeCents == nil || *got.FeeCents != *tc.fee || got.Polyline != tc.poly {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
}

func TestAmapErrorsAreBoundedAndNoRetry(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"status":"0","infocode":"10003","info":"secret leaked detail"}`))
	}))
	defer s.Close()
	_, _, err := (&Amap{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Search(context.Background(), "北京", "x", 1, 25)
	if err == nil || calls != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	_, err = (&Amap{BaseURL: s.URL, APIKey: "secret", Client: s.Client()}).Route(context.Background(), domain.Place{Location: "x", CoordinateSystem: "GCJ-02"}, domain.Place{Location: "y", CoordinateSystem: "GCJ-02"}, "walking")
	if err == nil {
		t.Fatal("expected invalid location")
	}
}

func TestAmapRejectsNonFiniteCoordinates(t *testing.T) {
	for _, location := range []string{"NaN,NaN", "+Inf,39", "116,-Inf"} {
		_, err := (&Amap{APIKey: "k"}).Route(context.Background(), domain.Place{Location: location, CoordinateSystem: "GCJ-02"}, domain.Place{Location: "116,39", CoordinateSystem: "GCJ-02"}, "walking")
		if err == nil {
			t.Fatalf("accepted %q", location)
		}
	}
}

func TestAmapRouteRejectsMissingAndInvalidDataButKeepsAbsentTransitFeeUnknown(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
		wantNilFee bool
	}{
		{"missing", `{"status":"1","infocode":"10000","route":{"paths":[]}}`, true, false},
		{"invalid numeric", `{"status":"1","infocode":"10000","route":{"paths":[{"distance":"oops","cost":{"duration":"2"}}]}}`, true, false},
		{"absent transit fee", `{"status":"1","infocode":"10000","route":{"transits":[{"distance":"10","cost":{"duration":"20"},"segments":[]}]}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(tc.body)) }))
			defer s.Close()
			mode := "walking"
			if tc.wantNilFee {
				mode = "transit"
			}
			p := domain.Place{ID: "x", ProviderPOIID: "p", Location: "116,39", CoordinateSystem: "GCJ-02", CityCode: "010"}
			got, err := (&Amap{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Route(context.Background(), p, p, mode)
			if (err != nil) != tc.wantErr || (tc.wantNilFee && got.FeeCents != nil) {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
}

func TestAmapTransitPolylineShapes(t *testing.T) {
	for _, tc := range []struct {
		name, bus, walking, want string
		wantErr                  bool
	}{
		{"nested real v5 shape", `{"polyline":"120.2,30.2;120.3,30.3"}`, `{"polyline":"120.1,30.1;120.2,30.2"}`, "120.1,30.1;120.2,30.2;120.2,30.2;120.3,30.3", false},
		{"legacy strings", `"c;d"`, `"a;b"`, "a;b;c;d", false},
		{"null", `null`, `null`, "", false},
		{"missing", `__missing__`, `__missing__`, "", false},
		{"malformed object", `{"coordinates":[]}`, `null`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			busField, walkingField := `,"polyline":`+tc.bus, `"polyline":`+tc.walking
			if tc.bus == "__missing__" {
				busField = ""
			}
			if tc.walking == "__missing__" {
				walkingField = `"instruction":"步行"`
			}
			body := `{"status":"1","infocode":"10000","route":{"transits":[{"distance":"5599","cost":{"duration":"2406","transit_fee":"2.0"},"segments":[{"bus":{"buslines":[{"name":"地铁"` + busField + `},{"name":"平行备选","polyline":"alt;route"}]},"walking":{"steps":[{` + walkingField + `}]}}]}]}}`
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer s.Close()
			p := domain.Place{ID: "x", ProviderPOIID: "p", Location: "120,30", CoordinateSystem: "GCJ-02", CityCode: "0571"}
			got, err := (&Amap{BaseURL: s.URL, APIKey: "k", Client: s.Client()}).Route(context.Background(), p, p, "transit")
			if (err != nil) != tc.wantErr {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if !tc.wantErr && (got.Polyline != tc.want || got.DurationS != 2406 || got.FeeCents == nil || *got.FeeCents != 200) {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestYuanCentsRejectsNegativeAndOverflowValues(t *testing.T) {
	for _, input := range []string{"-1", "1.-1", "92233720368547758.08"} {
		if got, err := yuanCents(input); err == nil {
			t.Fatalf("yuanCents(%q)=%d, want error", input, got)
		}
	}
	for input, want := range map[string]int64{"0": 0, "2.0": 200, "12.34": 1234} {
		got, err := yuanCents(input)
		if err != nil || got != want {
			t.Fatalf("yuanCents(%q)=%d,%v want %d", input, got, err, want)
		}
	}
}

func int64p(v int64) *int64 { return &v }
func containsAll(values []string, wants ...string) bool {
	for _, want := range wants {
		found := false
		for _, value := range values {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
