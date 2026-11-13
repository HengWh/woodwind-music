package app

import (
	"testing"
	"time"
)

func TestDeviceLocation(t *testing.T) {
	// UTC is still yesterday while the phone in Shanghai has crossed midnight.
	instant := time.Date(2026, 11, 13, 17, 30, 0, 0, time.UTC)
	local := instant.In(deviceLocation("Asia/Shanghai"))
	if local.Day() != 14 || local.Hour() != 1 || local.Minute() != 30 {
		t.Fatalf("device time = %v", local)
	}
	// Loading an IANA zone retains its daylight-saving rules.
	summer := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).In(deviceLocation("America/New_York"))
	if summer.Hour() != 8 {
		t.Fatalf("summer time = %v", summer)
	}
	if deviceLocation("invalid-zone") != time.Local || deviceLocation("") != time.Local {
		t.Fatal("missing zone did not retain system time")
	}
}
