package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

func (e *APIError) Error() string { return e.Code + ": " + e.Message }
func Err(status int32, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}
func ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("secure randomness unavailable")
	}
	return hex.EncodeToString(b[:])
}
func ValidateConstraints(c *Constraints) error {
	c.City = strings.TrimSpace(c.City)
	if c.City == "" || len([]rune(c.City)) > 40 {
		return Err(400, "INVALID_INPUT", "city 必须为一个城市")
	}
	start, e := time.Parse("2006-01-02", c.StartDate)
	if e != nil {
		return Err(400, "INVALID_INPUT", "start_date 格式必须为 YYYY-MM-DD")
	}
	end, e := time.Parse("2006-01-02", c.EndDate)
	if e != nil || end.Before(start) || end.Sub(start) > 6*24*time.Hour {
		return Err(400, "INVALID_INPUT", "行程必须为 1 至 7 天")
	}
	if c.PartySize < 1 || c.PartySize > 8 {
		return Err(400, "INVALID_INPUT", "人数须为 1 至 8")
	}
	if c.BudgetCents < 1 || c.BudgetCents > 100000000 {
		return Err(400, "INVALID_INPUT", "预算金额须为 1 至 100000000 分")
	}
	if c.BudgetScope != "total" && c.BudgetScope != "per_person" {
		return Err(400, "INVALID_INPUT", "budget_scope 须为 total 或 per_person")
	}
	c.BudgetTotalCents = c.BudgetCents
	if c.BudgetScope == "per_person" {
		c.BudgetTotalCents *= int64(c.PartySize)
	}
	if c.Transport != "walking" && c.Transport != "transit" {
		return Err(400, "INVALID_INPUT", "transport 须为 walking 或 transit")
	}
	if c.Pace == "" {
		c.Pace = "relaxed"
	}
	if c.Pace != "relaxed" && c.Pace != "balanced" && c.Pace != "intensive" {
		return Err(400, "INVALID_INPUT", "pace 不合法")
	}
	if len(c.Interests) > 8 {
		return Err(400, "INVALID_INPUT", "兴趣标签最多 8 个")
	}
	for _, s := range c.Interests {
		if len([]rune(s)) > 40 {
			return Err(400, "INVALID_INPUT", "兴趣标签过长")
		}
	}
	return nil
}
func Sorted(items []Activity) []Activity {
	out := append([]Activity(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].StartMinute < out[j].StartMinute
	})
	return out
}
func ValidatePlan(c Constraints, p Plan, places []Place) []string {
	issues := []string{}
	known := map[string]Place{}
	for _, v := range places {
		known[v.ID] = v
	}
	ids := map[string]bool{}
	seenPOI := map[string]bool{}
	sights := map[string]int{}
	meals := map[string]int{}
	hotels := map[string]int{}
	if len(p.Activities) == 0 || len(p.Activities) > 100 {
		return []string{"活动数量须为 1 至 100"}
	}
	for i, a := range Sorted(p.Activities) {
		if a.ID == "" || ids[a.ID] {
			issues = append(issues, "活动编号缺失或重复")
		}
		ids[a.ID] = true
		if a.Date < c.StartDate || a.Date > c.EndDate {
			issues = append(issues, "活动日期超出行程")
		}
		if _, e := time.Parse("2006-01-02", a.Date); e != nil {
			issues = append(issues, "活动日期无效")
		}
		if a.StartMinute < 0 || a.EndMinute > 1440 || a.EndMinute <= a.StartMinute {
			issues = append(issues, "活动时段无效")
		}
		sorted := Sorted(p.Activities)
		if i > 0 && a.Date == sorted[i-1].Date && a.StartMinute < sorted[i-1].EndMinute {
			issues = append(issues, "活动时间重叠")
		}
		switch a.Kind {
		case "sightseeing":
			sights[a.Date]++
			place, ok := known[a.PlaceID]
			if !ok || !place.Active {
				issues = append(issues, "景点编号不在有效候选中: "+a.PlaceID)
				break
			}
			if seenPOI[a.PlaceID] {
				issues = append(issues, "重复安排同一景点")
			}
			seenPOI[a.PlaceID] = true
			if strings.TrimSuffix(place.City, "市") != strings.TrimSuffix(c.City, "市") {
				issues = append(issues, "景点不属于目的地城市")
			}
			if place.DurationMinutes > 0 && a.EndMinute-a.StartMinute < place.DurationMinutes {
				issues = append(issues, "景点停留时间不足: "+a.PlaceID)
			}
			if place.OpenMinute != nil && a.StartMinute < *place.OpenMinute {
				issues = append(issues, "早于已知营业时间")
			}
			if place.CloseMinute != nil && a.EndMinute > *place.CloseMinute {
				issues = append(issues, "晚于已知营业时间")
			}
		case "meal":
			meals[a.Date]++
		case "hotel":
			hotels[a.Date]++
		case "rest":
		default:
			issues = append(issues, "活动类型无效")
		}
		for _, cost := range a.Costs {
			if cost.AmountCents != nil && (*cost.AmountCents < 0 || *cost.AmountCents > 100000000) {
				issues = append(issues, "费用数值无效")
			}
			if cost.Unit != "per_person" && cost.Unit != "group" {
				issues = append(issues, "费用单位无效")
			}
		}
	}
	start, e := time.Parse("2006-01-02", c.StartDate)
	if e == nil {
		for d := start; d.Format("2006-01-02") <= c.EndDate; d = d.AddDate(0, 0, 1) {
			date := d.Format("2006-01-02")
			if sights[date] < 1 || sights[date] > 5 {
				issues = append(issues, date+" 需要 1 至 5 个景点")
			}
			if meals[date] < 1 {
				issues = append(issues, date+" 缺少用餐时段")
			}
			if date < c.EndDate && hotels[date] < 1 {
				issues = append(issues, date+" 缺少住宿安排")
			}
		}
	}
	b := Budget(p, c.PartySize)
	if b.KnownTotalCents > c.BudgetTotalCents {
		issues = append(issues, "已知费用超过预算")
	}
	return issues
}
func ValidateScope(base Plan, s Scope) error {
	if s.StartMinute < 0 || s.EndMinute > 1440 || s.StartMinute >= s.EndMinute || len(s.EditableItemIDs) == 0 {
		return Err(400, "INVALID_SCOPE", "选择有效日期时段及可编辑活动")
	}
	if _, e := time.Parse("2006-01-02", s.Date); e != nil {
		return Err(400, "INVALID_SCOPE", "修改日期无效")
	}
	all := map[string]Activity{}
	for _, a := range base.Activities {
		all[a.ID] = a
	}
	locked := map[string]bool{}
	for _, id := range s.LockedItemIDs {
		if _, ok := all[id]; !ok || locked[id] {
			return Err(400, "INVALID_SCOPE", "锁定活动编号不存在或重复")
		}
		locked[id] = true
	}
	edit := map[string]bool{}
	for _, id := range s.EditableItemIDs {
		a, ok := all[id]
		if !ok || edit[id] || locked[id] || a.Date != s.Date || a.StartMinute < s.StartMinute || a.EndMinute > s.EndMinute {
			return Err(400, "INVALID_SCOPE", "可编辑活动不存在、重复、被锁定或超出范围")
		}
		edit[id] = true
	}
	return nil
}
func MergeRevision(base Plan, s Scope, replacement []Activity) (Plan, error) {
	if e := ValidateScope(base, s); e != nil {
		return Plan{}, e
	}
	out := base
	out.Activities = []Activity{}
	out.Routes = nil
	edit := map[string]bool{}
	used := map[string]bool{}
	for _, id := range s.EditableItemIDs {
		edit[id] = true
	}
	for _, a := range base.Activities {
		if !edit[a.ID] {
			out.Activities = append(out.Activities, a)
			used[a.ID] = true
		}
	}
	for _, a := range replacement {
		if used[a.ID] || a.Date != s.Date || a.StartMinute < s.StartMinute || a.EndMinute > s.EndMinute {
			return Plan{}, Err(422, "SCOPE_CONFLICT", "替换活动超出修改范围或重复编号")
		}
		used[a.ID] = true
		out.Activities = append(out.Activities, a)
	}
	out.Activities = Sorted(out.Activities)
	return out, nil
}
func Unchanged(a, b Activity) bool { return reflect.DeepEqual(a, b) }
func FreeWindowSeconds(p Plan, date string, start, end int32) int64 {
	maxGap := int32(0)
	cursor := start
	for _, a := range Sorted(p.Activities) {
		if a.Date != date || a.EndMinute <= start || a.StartMinute >= end {
			continue
		}
		if a.StartMinute > cursor && a.StartMinute-cursor > maxGap {
			maxGap = a.StartMinute - cursor
		}
		if a.EndMinute > cursor {
			cursor = a.EndMinute
		}
	}
	if end-cursor > maxGap {
		maxGap = end - cursor
	}
	return int64(maxGap) * 60
}
func RouteIssue(p Plan, from, to Activity, r Route) string {
	free := FreeWindowSeconds(p, from.Date, from.EndMinute, to.StartMinute)
	if r.DurationS+300 > free {
		return fmt.Sprintf("%s：%s（ID=%s，结束 minute=%d）到 %s（ID=%s，开始 minute=%d）的最长连续空闲 %d 秒；交通耗时 %d 秒加 300 秒缓冲，至少预留 %d 秒。中间 meal/rest/hotel 等活动占用不算交通留白，不可累加分散空档；请在允许修改范围内调整活动时间，留出足够的连续交通时间。", from.Date, from.Title, from.ID, from.EndMinute, to.Title, to.ID, to.StartMinute, free, r.DurationS, r.DurationS+300)
	}
	return ""
}
