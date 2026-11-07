package workspace

import (
	"testing"
	"time"
)

func TestDNDWindow(t *testing.T) {
	at := func(hour, minute int) time.Time {
		return time.Date(2026, 10, 30, hour, minute, 0, 0, time.Local)
	}
	cases := []struct {
		name     string
		settings DNDSettings
		now      time.Time
		want     bool
	}{
		{"off", DNDSettings{Start: "22:00", End: "07:00"}, at(23, 0), false},
		{"manual", DNDSettings{Enabled: true, Start: "22:00", End: "07:00"}, at(12, 0), true},
		{"schedule before midnight", DNDSettings{ScheduleEnabled: true, Start: "22:00", End: "07:00"}, at(23, 0), true},
		{"schedule after midnight", DNDSettings{ScheduleEnabled: true, Start: "22:00", End: "07:00"}, at(6, 59), true},
		{"schedule ends", DNDSettings{ScheduleEnabled: true, Start: "22:00", End: "07:00"}, at(7, 0), false},
		{"schedule starts", DNDSettings{ScheduleEnabled: true, Start: "22:00", End: "07:00"}, at(22, 0), true},
		{"schedule daytime", DNDSettings{ScheduleEnabled: true, Start: "09:00", End: "17:00"}, at(12, 0), true},
		{"schedule daytime outside", DNDSettings{ScheduleEnabled: true, Start: "09:00", End: "17:00"}, at(18, 0), false},
		{"schedule full day", DNDSettings{ScheduleEnabled: true, Start: "00:00", End: "00:00"}, at(3, 0), true},
	}
	for _, c := range cases {
		if got := dndWindow(c.settings, c.now); got != c.want {
			t.Errorf("%s: dndWindow = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDNDSettingsValidation(t *testing.T) {
	for _, bad := range []DNDSettings{{Start: "24:00", End: "07:00"}, {Start: "7:00", End: "08:00"}, {Start: "22:00", End: "07:60"}} {
		if err := validateDND(bad); err == nil {
			t.Errorf("%+v diterima", bad)
		}
	}
	if err := validateDND(defaultDND()); err != nil {
		t.Fatalf("bawaan ditolak: %v", err)
	}
}
