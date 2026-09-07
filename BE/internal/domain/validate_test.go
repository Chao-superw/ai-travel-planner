package domain

import (
	"strings"
	"testing"
)

func validConstraints() Constraints {
	return Constraints{City: "杭州市", StartDate: "2026-10-01", EndDate: "2026-10-03", PartySize: 2, BudgetCents: 200000, BudgetScope: "per_person", Transport: "walking", Pace: "relaxed"}
}
func TestConstraintsBoundaries(t *testing.T) {
	c := validConstraints()
	if err := ValidateConstraints(&c); err != nil {
		t.Fatal(err)
	}
	if c.BudgetTotalCents != 400000 {
		t.Fatal(c.BudgetTotalCents)
	}
	for _, change := range []func(*Constraints){func(c *Constraints) { c.EndDate = "2026-10-08" }, func(c *Constraints) { c.StartDate = "bad" }, func(c *Constraints) { c.PartySize = 0 }, func(c *Constraints) { c.BudgetCents = 1 << 62 }, func(c *Constraints) { c.Transport = "teleport" }} {
		bad := validConstraints()
		change(&bad)
		if ValidateConstraints(&bad) == nil {
			t.Fatal("invalid constraint accepted", bad)
		}
	}
}
func TestRejectUnknownAndOverlappingPlaces(t *testing.T) {
	c := validConstraints()
	ValidateConstraints(&c)
	p := Plan{Activities: []Activity{{ID: "a", Date: c.StartDate, StartMinute: 540, EndMinute: 600, Kind: "sightseeing", PlaceID: "missing"}}}
	if len(ValidatePlan(c, p, nil)) == 0 {
		t.Fatal("invented POI accepted")
	}
	p.Activities = []Activity{{ID: "a", Date: c.StartDate, StartMinute: 540, EndMinute: 600, Kind: "rest"}, {ID: "b", Date: c.StartDate, StartMinute: 570, EndMinute: 630, Kind: "rest"}}
	if len(ValidatePlan(c, p, nil)) == 0 {
		t.Fatal("overlap accepted")
	}
}
func TestRevisionCannotChangeLockedItems(t *testing.T) {
	p := Plan{Activities: []Activity{{ID: "a", Date: "2026-10-01", StartMinute: 540, EndMinute: 600, Kind: "rest"}, {ID: "b", Date: "2026-10-01", StartMinute: 720, EndMinute: 780, Kind: "rest"}}}
	s := Scope{Date: "2026-10-01", StartMinute: 700, EndMinute: 900, EditableItemIDs: []string{"b"}, LockedItemIDs: []string{"a"}}
	out, err := MergeRevision(p, s, []Activity{{ID: "c", Date: s.Date, StartMinute: 730, EndMinute: 800, Kind: "rest"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Activities[0].ID != "a" || out.Activities[0].StartMinute != 540 {
		t.Fatal(out)
	}
	s.EditableItemIDs = []string{"a"}
	if _, err := MergeRevision(p, s, nil); err == nil {
		t.Fatal("locked edit accepted")
	}
}
func TestUnknownCostsKeepBudgetIncomplete(t *testing.T) {
	amount := int64(1000)
	p := Plan{Activities: []Activity{{Date: "2026-10-01", Costs: []Cost{{Category: "food", AmountCents: &amount, Unit: "per_person"}, {Category: "hotel", Unit: "group"}}}}}
	b := Budget(p, 2)
	if b.KnownTotalCents != 2000 || b.UnknownCount != 2 || b.Complete || b.WithinBudget != nil {
		t.Fatalf("bad budget: %+v", b)
	}
}
func TestUnlocatedDailyTransportKeepsBudgetIncomplete(t *testing.T) {
	free := int64(0)
	p := Plan{Activities: []Activity{{Date: "2026-10-01", Kind: "sightseeing", Costs: []Cost{{Category: "ticket", AmountCents: &free, Unit: "per_person"}}}}}
	b := Budget(p, 1)
	if b.Complete || b.UnknownCount != 1 {
		t.Fatalf("unpriced local transport missing: %+v", b)
	}
}

func TestRouteIssueProvidesReadableRepairContext(t *testing.T) {
	from := Activity{ID: "random-from", Title: "西湖", Date: "2026-10-01", EndMinute: 600}
	to := Activity{ID: "random-to", Title: "博物馆", Date: from.Date, StartMinute: 660}
	for _, kind := range []string{"meal", "rest", "hotel"} {
		t.Run(kind, func(t *testing.T) {
			p := Plan{Activities: []Activity{{Date: from.Date, Kind: kind, StartMinute: 620, EndMinute: 640}}}
			issue := RouteIssue(p, from, to, Route{DurationS: 1000})
			for _, want := range []string{from.Date, from.Title, from.ID, to.Title, to.ID, "结束 minute=600", "开始 minute=660", "最长连续空闲 1200 秒", "1000 秒", "300 秒", "至少预留 1300 秒", "meal/rest/hotel", "不算交通留白"} {
				if !strings.Contains(issue, want) {
					t.Errorf("repair feedback missing %q: %s", want, issue)
				}
			}
			if got := RouteIssue(p, from, to, Route{DurationS: 900}); got != "" {
				t.Errorf("exactly fitting route rejected: %s", got)
			}
			if got := RouteIssue(p, from, to, Route{DurationS: 901}); got == "" {
				t.Error("route exceeding contiguous gap by one second accepted")
			}
		})
	}
}
