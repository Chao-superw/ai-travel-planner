package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-travel/internal/domain"
)

const maxProviderResponse = 2 << 20

type Amap struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

type amapEnvelope struct {
	Status, InfoCode string
	POIs             []amapPOI `json:"pois"`
	Route            struct {
		Paths    []amapPath    `json:"paths"`
		Transits []amapTransit `json:"transits"`
	} `json:"route"`
}
type amapPOI struct{ ID, Name, CityName, CityCode, AdCode, Location, Address, TypeCode, Type string }
type amapPath struct {
	Distance string
	Cost     struct{ Duration string } `json:"cost"`
	Steps    []struct {
		Polyline string `json:"polyline"`
	} `json:"steps"`
}
type amapTransit struct {
	Distance        string
	WalkingDistance string `json:"walking_distance"`
	Cost            struct {
		Duration   string `json:"duration"`
		TransitFee string `json:"transit_fee"`
	} `json:"cost"`
	Segments []struct {
		Bus struct {
			BusLines []struct {
				Name     string
				Polyline rawPolyline
			} `json:"buslines"`
		} `json:"bus"`
		Walking struct {
			Steps []struct{ Polyline rawPolyline } `json:"steps"`
		} `json:"walking"`
	} `json:"segments"`
}

type rawPolyline string

func (p *rawPolyline) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*p = ""
		return nil
	}
	var direct string
	if json.Unmarshal(data, &direct) == nil {
		*p = rawPolyline(direct)
		return nil
	}
	var wrapped struct {
		Polyline *string `json:"polyline"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wrapped); err != nil || dec.Decode(&struct{}{}) != io.EOF || wrapped.Polyline == nil {
		return fmt.Errorf("invalid polyline")
	}
	*p = rawPolyline(*wrapped.Polyline)
	return nil
}

func (a *Amap) Search(ctx context.Context, city, keyword string, page, size int) ([]domain.Place, bool, error) {
	if a.APIKey == "" {
		return nil, false, apiErr("MAP_UNAVAILABLE", "map dependency is not configured", 503)
	}
	if keyword == "" {
		keyword = "风景名胜"
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 25 {
		size = 25
	}
	q := url.Values{"key": {a.APIKey}, "keywords": {keyword}, "types": {"110000|140100"}, "region": {city}, "city_limit": {"true"}, "page_size": {strconv.Itoa(size)}, "page_num": {strconv.Itoa(page)}, "show_fields": {"business"}}
	var env amapEnvelope
	if err := providerGET(ctx, a.client(), a.base()+"/v5/place/text?"+q.Encode(), &env); err != nil {
		return nil, false, err
	}
	if env.Status != "1" || env.InfoCode != "10000" {
		code := "MAP_UNAVAILABLE"
		if env.InfoCode == "10003" || strings.HasPrefix(env.InfoCode, "1004") {
			code = "MAP_QUOTA"
		}
		return nil, false, apiErr(code, "map provider request failed", 502)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	out := make([]domain.Place, 0, len(env.POIs))
	seen := map[string]bool{}
	for _, p := range env.POIs {
		if p.ID == "" || seen[p.ID] || (!strings.HasPrefix(p.TypeCode, "11") && p.TypeCode != "140100") || normalizeCity(p.CityName) != normalizeCity(city) || !validLocation(p.Location) {
			continue
		}
		seen[p.ID] = true
		tags := splitTags(p.Type)
		if p.TypeCode == "140100" {
			tags = appendUnique(tags, "室内", "博物馆")
		}
		out = append(out, domain.Place{ID: "amap:" + p.ID, Provider: "amap", ProviderPOIID: p.ID, Name: p.Name, City: p.CityName, CityCode: p.CityCode, AdCode: p.AdCode, Location: p.Location, CoordinateSystem: "GCJ-02", Address: p.Address, TypeCode: p.TypeCode, Tags: tags, DurationMinutes: 90, FeeSource: "unknown", Active: true, FetchedAt: now})
	}
	return out, len(env.POIs) >= size, nil
}

func (a *Amap) Route(ctx context.Context, from, to domain.Place, mode string) (domain.Route, error) {
	var zero domain.Route
	if a.APIKey == "" {
		return zero, apiErr("MAP_UNAVAILABLE", "map dependency is not configured", 503)
	}
	if from.CoordinateSystem != "GCJ-02" || to.CoordinateSystem != "GCJ-02" || !validLocation(from.Location) || !validLocation(to.Location) {
		return zero, apiErr("MAP_UNAVAILABLE", "invalid route coordinates", 400)
	}
	q := url.Values{"key": {a.APIKey}, "origin": {from.Location}, "destination": {to.Location}, "origin_id": {from.ProviderPOIID}, "destination_id": {to.ProviderPOIID}, "show_fields": {"cost,polyline"}}
	path := "/v5/direction/walking"
	if mode == "transit" {
		path = "/v5/direction/transit/integrated"
		q.Set("city1", from.CityCode)
		q.Set("city2", to.CityCode)
		q.Set("originpoi", from.ProviderPOIID)
		q.Set("destinationpoi", to.ProviderPOIID)
	} else if mode != "walking" {
		return zero, apiErr("MAP_UNAVAILABLE", "unsupported route mode", 400)
	}
	var env amapEnvelope
	if err := providerGET(ctx, a.client(), a.base()+path+"?"+q.Encode(), &env); err != nil {
		return zero, err
	}
	if env.Status != "1" || env.InfoCode != "10000" {
		return zero, apiErr("MAP_UNAVAILABLE", "map provider request failed", 502)
	}
	r := domain.Route{FromItemID: from.ID, ToItemID: to.ID, Mode: mode, Provider: "amap", QueriedAt: time.Now().UTC().Format(time.RFC3339), FromLocation: from.Location, ToLocation: to.Location}
	if mode == "walking" {
		if len(env.Route.Paths) == 0 {
			return zero, apiErr("MAP_NO_ROUTE", "map provider returned no route", 422)
		}
		p := env.Route.Paths[0]
		var err error
		if r.DistanceM, err = parseNonnegative(p.Distance); err != nil {
			return zero, apiErr("MAP_UNAVAILABLE", "map provider returned invalid route", 502)
		}
		if r.DurationS, err = parseNonnegative(p.Cost.Duration); err != nil {
			return zero, apiErr("MAP_UNAVAILABLE", "map provider returned invalid route", 502)
		}
		z := int64(0)
		r.FeeCents = &z
		var ps []string
		for _, s := range p.Steps {
			if s.Polyline != "" {
				ps = append(ps, s.Polyline)
			}
		}
		r.Polyline = strings.Join(ps, ";")
		r.Summary = "Walking route"
	} else {
		if len(env.Route.Transits) == 0 {
			return zero, apiErr("MAP_NO_ROUTE", "map provider returned no route", 422)
		}
		t := env.Route.Transits[0]
		var err error
		if r.DistanceM, err = parseNonnegative(t.Distance); err != nil {
			return zero, apiErr("MAP_UNAVAILABLE", "map provider returned invalid route", 502)
		}
		if r.DurationS, err = parseNonnegative(t.Cost.Duration); err != nil {
			return zero, apiErr("MAP_UNAVAILABLE", "map provider returned invalid route", 502)
		}
		if t.Cost.TransitFee != "" {
			v, e := yuanCents(t.Cost.TransitFee)
			if e != nil {
				return zero, apiErr("MAP_UNAVAILABLE", "map provider returned invalid route", 502)
			}
			r.FeeCents = &v
		}
		var ps, names []string
		for _, s := range t.Segments {
			for _, w := range s.Walking.Steps {
				if w.Polyline != "" {
					ps = append(ps, string(w.Polyline))
				}
			}
			if len(s.Bus.BusLines) > 0 {
				b := s.Bus.BusLines[0]
				if b.Name != "" {
					names = append(names, b.Name)
				}
				if b.Polyline != "" {
					ps = append(ps, string(b.Polyline))
				}
			}
		}
		r.Polyline = strings.Join(ps, ";")
		r.Summary = strings.Join(names, ", ")
	}
	return r, nil
}

func (a *Amap) base() string {
	if a.BaseURL != "" {
		return strings.TrimRight(a.BaseURL, "/")
	}
	return "https://restapi.amap.com"
}
func (a *Amap) client() *http.Client { return safeClient(a.Client) }
func normalizeCity(s string) string  { return strings.TrimSuffix(strings.TrimSpace(s), "市") }
func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == '|' })
}
func appendUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		found := false
		for _, value := range values {
			if value == addition {
				found = true
				break
			}
		}
		if !found {
			values = append(values, addition)
		}
	}
	return values
}
func validLocation(s string) bool {
	p := strings.Split(s, ",")
	if len(p) != 2 {
		return false
	}
	lng, e1 := strconv.ParseFloat(p[0], 64)
	lat, e2 := strconv.ParseFloat(p[1], 64)
	return e1 == nil && e2 == nil && !math.IsNaN(lng) && !math.IsNaN(lat) && !math.IsInf(lng, 0) && !math.IsInf(lat, 0) && lng >= -180 && lng <= 180 && lat >= -90 && lat <= 90
}
func parseNonnegative(s string) (int64, error) {
	v, e := strconv.ParseInt(s, 10, 64)
	if e != nil || v < 0 {
		return 0, fmt.Errorf("invalid numeric")
	}
	return v, nil
}
func yuanCents(s string) (int64, error) {
	parts := strings.Split(s, ".")
	if len(parts) > 2 || len(parts) == 0 || !decimalDigits(parts[0]) {
		return 0, fmt.Errorf("invalid fee")
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil {
		return 0, fmt.Errorf("invalid fee")
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(parts) == 2 && (!decimalDigits(frac) || len(frac) > 2) {
		return 0, fmt.Errorf("invalid fee")
	}
	for len(frac) < 2 {
		frac += "0"
	}
	f := int64(0)
	if frac != "" {
		f, e = strconv.ParseInt(frac, 10, 64)
		if e != nil {
			return 0, fmt.Errorf("invalid fee")
		}
	}
	maxInt64 := int64(^uint64(0) >> 1)
	if whole > (maxInt64-f)/100 {
		return 0, fmt.Errorf("invalid fee")
	}
	return whole*100 + f, nil
}
func decimalDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
func safeClient(c *http.Client) *http.Client {
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	clone := *c
	if clone.Timeout == 0 {
		clone.Timeout = 15 * time.Second
	}
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	return &clone
}
func providerGET(ctx context.Context, c *http.Client, u string, dst any) error {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return apiErr("MAP_UNAVAILABLE", "map provider request failed", 502)
	}
	resp, e := c.Do(req)
	if e != nil {
		return apiErr("MAP_UNAVAILABLE", "map provider request failed", 502)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "MAP_UNAVAILABLE"
		if resp.StatusCode == http.StatusTooManyRequests {
			code = "MAP_QUOTA"
		}
		return apiErr(code, "map provider request failed", 502)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponse+1))
	if e != nil || len(raw) > maxProviderResponse || json.Unmarshal(raw, dst) != nil {
		return apiErr("MAP_UNAVAILABLE", "map provider returned invalid response", 502)
	}
	return nil
}
func apiErr(code, msg string, status int32) error {
	return &domain.APIError{Code: code, Message: msg, Status: status}
}
