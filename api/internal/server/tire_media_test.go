package server

import "testing"

func TestValidPhotoDataURL(t *testing.T) {
	if !validPhotoDataURL("", 200000) {
		t.Fatal("empty photo should be allowed")
	}
	if !validPhotoDataURL("data:image/webp;base64,UklGRh4AAABXRUJQVlA4TBEAAAAvAAAAAAfQ//73v/+BiOh/AAA=", 200000) {
		t.Fatal("valid webp data URL should be allowed")
	}
	if !validPhotoDataURL("data:image/jpeg;base64,/9j/4AAQSkZJRg==", 200000) {
		t.Fatal("valid jpeg data URL should be allowed")
	}
	if validPhotoDataURL("data:image/svg+xml;base64,PHN2Zz4=", 200000) {
		t.Fatal("svg data URL must be rejected")
	}
	if validPhotoDataURL("https://example.com/a.jpg", 200000) {
		t.Fatal("external URL must be rejected")
	}
	if validPhotoDataURL("data:image/webp;base64,AAAA", 10) {
		t.Fatal("oversized photo must be rejected")
	}
}

func TestValidOptionalPressureAndText(t *testing.T) {
	if !validOptionalPressure(nil) {
		t.Fatal("nil pressure should be valid")
	}
	p := 110.5
	if !validOptionalPressure(&p) {
		t.Fatal("110.5 PSI should be valid")
	}
	bad := 300.0
	if validOptionalPressure(&bad) {
		t.Fatal("300 PSI should be rejected")
	}
	clean, ok := cleanOptionalText("  Fornecedor X  ", 150)
	if !ok || clean != "Fornecedor X" {
		t.Fatalf("unexpected clean text: %q %v", clean, ok)
	}
}
