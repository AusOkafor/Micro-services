package shop

import "time"

type Shop struct {
	ID          string
	Domain      string
	AccessToken string
	Plan        string
	Status      string
	InstalledAt time.Time
}


