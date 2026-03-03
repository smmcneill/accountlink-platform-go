package db

import "testing"

func TestParseVersionedMigration(t *testing.T) {
	m, err := parseVersionedMigration("V1_2__add_index.sql", "migrations")
	if err != nil {
		t.Fatalf("parseVersionedMigration returned error: %v", err)
	}

	if m.path != "migrations/V1_2__add_index.sql" {
		t.Fatalf("unexpected path: %s", m.path)
	}

	if m.version != "1_2" {
		t.Fatalf("unexpected version: %s", m.version)
	}

	if m.description != "add index" {
		t.Fatalf("unexpected description: %q", m.description)
	}
}

func TestParseVersionedMigrationRejectsInvalidName(t *testing.T) {
	if _, err := parseVersionedMigration("bad.sql", "migrations"); err == nil {
		t.Fatalf("expected parseVersionedMigration to fail for invalid name")
	}
}

func TestCompareVersionPartsNumericOrder(t *testing.T) {
	a, err := parseVersionParts("2")
	if err != nil {
		t.Fatalf("parseVersionParts returned error: %v", err)
	}

	b, err := parseVersionParts("10")
	if err != nil {
		t.Fatalf("parseVersionParts returned error: %v", err)
	}

	if compareVersionParts(a, b) >= 0 {
		t.Fatalf("expected 2 < 10")
	}
}

func TestCompareVersionPartsWithSegments(t *testing.T) {
	a, err := parseVersionParts("1.2")
	if err != nil {
		t.Fatalf("parseVersionParts returned error: %v", err)
	}

	b, err := parseVersionParts("1.10")
	if err != nil {
		t.Fatalf("parseVersionParts returned error: %v", err)
	}

	if compareVersionParts(a, b) >= 0 {
		t.Fatalf("expected 1.2 < 1.10")
	}
}
