package meta

import (
	"testing"
	"time"
)

func TestParseVideoTime(t *testing.T) {
	tm, has, ok := parseVideoTime("2026-05-03T14:22:31+0900", time.UTC)
	if !ok || !has || tm.Hour() != 14 {
		t.Fatalf("unexpected %v %v %v", tm, has, ok)
	}
}
func TestFindXMLDate(t *testing.T) {
	if got := findXMLDate("<CreationDateValue>2024-01-02T03:04:05+09:00</CreationDateValue>"); got != "2024-01-02T03:04:05+09:00" {
		t.Fatal(got)
	}
}
