package app

import (
	"time"
	_ "time/tzdata" // Android's Go process has no system zoneinfo directory.
)

func deviceLocation(zone string) *time.Location {
	if zone != "" {
		if location, err := time.LoadLocation(zone); err == nil {
			return location
		}
	}
	return time.Local
}

func (a *app) localTime() time.Time {
	if a.location != nil {
		return time.Now().In(a.location)
	}
	return time.Now()
}
