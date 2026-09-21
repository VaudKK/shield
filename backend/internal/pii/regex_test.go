package pii

import "testing"

func TestDetect(t *testing.T) {
	text := "Contact me at jane.doe@example.com or call +1 555-123-4567. Ref number 900123456789."

	got := Detect(text)

	types := map[string]int{}
	for _, d := range got {
		types[d.Type]++
	}

	if types["email"] != 1 {
		t.Errorf("expected 1 email detection, got %d", types["email"])
	}
	if types["phone_number"] < 1 {
		t.Errorf("expected at least 1 phone detection, got %d", types["phone_number"])
	}
	if types["possible_id_number"] != 1 {
		t.Errorf("expected 1 possible_id_number detection, got %d", types["possible_id_number"])
	}

	for _, d := range got {
		if d.Type == "email" && d.Value != "jane.doe@example.com" {
			t.Errorf("unexpected email value: %q", d.Value)
		}
	}
}

func TestDetect_Deduplicates(t *testing.T) {
	text := "Email a@example.com again: a@example.com"
	got := Detect(text)

	count := 0
	for _, d := range got {
		if d.Type == "email" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected duplicate emails to be deduplicated, got %d entries", count)
	}
}

func TestDetect_DateNotMisreportedAsPhone(t *testing.T) {
	text := "Meeting on 2026-01-15 with the landlord about the lease."
	got := Detect(text)

	for _, d := range got {
		if d.Type == "phone_number" {
			t.Errorf("expected the date not to be detected as a phone number, got %+v", d)
		}
	}
}

func TestDetect_DecimalTableNumbersNotMisreportedAsPhone(t *testing.T) {
	// A stock/spreadsheet table row OCR'd as plain text: 52-week high/low
	// prices and a last/change column, all decimal numbers separated by
	// whitespace. Previously matched the loose phone pattern since it's
	// just digits with separators in the right length range.
	text := "212.25 131.03 BiotechT 204.66 -0.84\n68.88 50.65 Biosite 50.05 -4.57"
	got := Detect(text)

	for _, d := range got {
		if d.Type == "phone_number" {
			t.Errorf("expected decimal table numbers not to be detected as a phone number, got %+v", d)
		}
	}
}

func TestDetect_NoFalsePositivesOnPlainText(t *testing.T) {
	text := "This is a short note with no personal information in it at all."
	got := Detect(text)
	if len(got) != 0 {
		t.Errorf("expected no detections, got %v", got)
	}
}
