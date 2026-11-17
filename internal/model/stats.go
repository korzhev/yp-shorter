package model

// Stats contains totals for stored links and their distinct owners.
type Stats struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}
