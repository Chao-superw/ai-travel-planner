package domain

import "sort"

func Budget(p Plan, party int32) BudgetSummary {
	b := BudgetSummary{ByDay: []AmountGroup{}, ByCategory: []AmountGroup{}}
	days := map[string]*AmountGroup{}
	cats := map[string]*AmountGroup{}
	add := func(date, cat string, amount *int64, mult int64) {
		if days[date] == nil {
			days[date] = &AmountGroup{Key: date}
		}
		if cats[cat] == nil {
			cats[cat] = &AmountGroup{Key: cat}
		}
		if amount == nil {
			b.UnknownCount++
			days[date].UnknownCount++
			cats[cat].UnknownCount++
			return
		}
		v := *amount * mult
		b.KnownTotalCents += v
		days[date].KnownCents += v
		cats[cat].KnownCents += v
	}
	for _, a := range p.Activities {
		for _, c := range a.Costs {
			mult := int64(1)
			if c.Unit == "per_person" {
				mult = int64(party)
			}
			add(a.Date, c.Category, c.AmountCents, mult)
		}
	}
	for _, r := range p.Routes {
		add(r.Date, "transport", r.FeeCents, int64(party))
	}
	// Each day has unlocated city transfers (hotel, meals, first/last leg).
	for date := range days {
		add(date, "transport", nil, 1)
	}
	for _, v := range days {
		b.ByDay = append(b.ByDay, *v)
	}
	for _, v := range cats {
		b.ByCategory = append(b.ByCategory, *v)
	}
	sort.Slice(b.ByDay, func(i, j int) bool { return b.ByDay[i].Key < b.ByDay[j].Key })
	sort.Slice(b.ByCategory, func(i, j int) bool { return b.ByCategory[i].Key < b.ByCategory[j].Key })
	b.Complete = b.UnknownCount == 0
	return b
}
