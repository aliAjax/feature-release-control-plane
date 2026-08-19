package scheduler

import "time"

// Window is a half-open time interval [StartsAt, EndsAt). A nil bound means the
// interval is unbounded on that side, which is how "start now" and "no end"
// scheduling windows are expressed by operators.
type Window struct {
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

func (w Window) Valid() bool {
	if w.StartsAt != nil && w.EndsAt != nil && !w.StartsAt.Before(*w.EndsAt) {
		return false
	}
	return true
}

// Active reports whether the instant at falls inside the half-open window.
func (w Window) Active(at time.Time) bool {
	if !w.Valid() {
		return false
	}
	if w.StartsAt != nil && at.Before(*w.StartsAt) {
		return false
	}
	if w.EndsAt != nil && !at.Before(*w.EndsAt) {
		return false
	}
	return true
}

// ActiveWindow returns the first active window in the ordered slice, or nil
// when none is active. Operators define windows in priority order, so the first
// match wins exactly like targeting rules.
func ActiveWindow(windows []Window, at time.Time) *Window {
	for i := range windows {
		if windows[i].Active(at) {
			return &windows[i]
		}
	}
	return nil
}

// NextTransition returns the earliest future instant at which the active-window
// result for the given instant could change, plus whether such an instant
// exists. A false result means the outcome is stable until a bound is edited.
func NextTransition(windows []Window, at time.Time) (time.Time, bool) {
	var next time.Time
	found := false
	for _, w := range windows {
		if w.StartsAt != nil && !at.Before(*w.StartsAt) {
			continue
		}
		for _, candidate := range []*time.Time{w.StartsAt, w.EndsAt} {
			if candidate == nil {
				continue
			}
			if candidate.After(at) && (!found || candidate.Before(next)) {
				next = *candidate
				found = true
			}
		}
	}
	return next, found
}
