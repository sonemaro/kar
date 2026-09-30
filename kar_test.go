package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIntervalValue(t *testing.T) {
	got := intervalValue("2027-01-06")
	want := `{"end":"2027-01-06","start":"2027-01-06"}`
	if got != want {
		// order-insensitive: just decode and compare
		var m map[string]string
		if err := json.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("intervalValue not valid JSON object: %q", got)
		}
		if m["start"] != "2027-01-06" || m["end"] != "2027-01-06" {
			t.Fatalf("intervalValue = %q, want start/end 2027-01-06", got)
		}
	}
}

func TestTextToADF(t *testing.T) {
	doc := textToADF("first paragraph\nstill first.\n\nsecond paragraph")
	if doc["type"] != "doc" || doc["version"] != 1 {
		t.Fatalf("doc envelope wrong: %v", doc)
	}
	content := doc["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("want 2 paragraphs, got %d", len(content))
	}
	p := content[0].(map[string]any)
	texts := p["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.HasPrefix(texts, "first paragraph") || !strings.Contains(texts, "still first.") {
		t.Fatalf("paragraph text wrong: %q", texts)
	}
}

func TestTextToADFEmpty(t *testing.T) {
	doc := textToADF("   ")
	content := doc["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("empty text should yield one empty paragraph, got %d", len(content))
	}
}

func TestValidDate(t *testing.T) {
	for good := range map[string]bool{"2027-01-06": true, "2026-09-30": true} {
		if err := validDate(good); err != nil {
			t.Errorf("validDate(%q) = %v, want nil", good, err)
		}
	}
	for _, bad := range []string{"2027-1-6", "27-01-06", "2027/01/06", "2027-01-06T00:00:00Z"} {
		if err := validDate(bad); err == nil {
			t.Errorf("validDate(%q) = nil, want error", bad)
		}
	}
}

func TestLinkedWorkIssue(t *testing.T) {
	raw := rawIssue{
		Key: "HAMROADMAP-11",
		Fields: json.RawMessage(`{
			"summary": "v0.4 Calendar & Files",
			"status": {"name": "Delivery"},
			"customfield_10059": {"start": "2026-09-09", "end": "2026-09-09"},
			"issuelinks": [
				{"outwardIssue": {"key": "HAMN-43"}},
				{"inwardIssue": {"key": "HAMN-100"}}
			]
		}`),
	}
	got := linkedWorkIssue(raw, "HAMN-")
	if got != "HAMN-43" {
		t.Fatalf("linkedWorkIssue = %q, want HAMN-43", got)
	}
	f, err := raw.decode()
	if err != nil || f.Status.Name != "Delivery" {
		t.Fatalf("decode: %v %+v", err, f.Status)
	}
	if !raw.hasField("customfield_10059") {
		t.Fatal("hasField should be false only for absent/null fields")
	}
	if raw.hasField("customfield_10053") {
		t.Fatal("customfield_10053 absent from fixture; hasField should be false")
	}
}
