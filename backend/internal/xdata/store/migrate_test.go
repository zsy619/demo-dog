package store

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestMigrator_Head(t *testing.T) {
	m := NewMigrator(DefaultChain())
	if m.Head() != 3 {
		t.Fatalf("head: %d", m.Head())
	}
}

func TestMigrator_ApplyFromZero(t *testing.T) {
	m := NewMigrator(DefaultChain())
	input := []byte(`{"Logs":[],"Metrics":[],"Spans":[]}`)
	out, head, err := m.Apply(input, 1)
	if err != nil {
		t.Fatal(err)
	}
	if head != 3 {
		t.Fatalf("head: %d", head)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	if _, ok := v["Histograms"]; !ok {
		t.Fatal("v3 should have Histograms")
	}
	if _, ok := v["TDigests"]; !ok {
		t.Fatal("v3 should have TDigests")
	}
}

func TestMigrator_ApplyFromLatest(t *testing.T) {
	m := NewMigrator(DefaultChain())
	input := []byte(`{"Logs":[],"Histograms":[],"TDigests":[]}`)
	out, head, err := m.Apply(input, m.Head())
	if err != nil {
		t.Fatal(err)
	}
	if head != 3 {
		t.Fatalf("head: %d", head)
	}
	if !bytes.Equal(out, input) {
		t.Fatal("no-op migration should leave payload unchanged")
	}
}

func TestMigrator_ApplySourceBeyondHead(t *testing.T) {
	m := NewMigrator(DefaultChain())
	_, _, err := m.Apply([]byte("x"), 99)
	if err == nil {
		t.Fatal("expected error for source > head")
	}
}

func TestMigrator_PropagatesErrors(t *testing.T) {
	m := NewMigrator([]Migration{
		{Version: 1, Name: "baseline", Apply: func(p []byte) ([]byte, error) { return p, nil }},
		{Version: 2, Name: "broken", Apply: func(p []byte) ([]byte, error) { return nil, errBrokenMigration }},
	})
	_, _, err := m.Apply([]byte("anything"), 1)
	if err == nil {
		t.Fatal("expected error from broken migration")
	}
}

var errBrokenMigration = errBroken{}

type errBroken struct{}

func (errBroken) Error() string { return "broken migration" }

func TestEncodeDecodeMigrationHeader(t *testing.T) {
	payload := []byte("hello world")
	encoded := EncodeMigrationHeader(7, payload)
	v, body, err := DecodeMigrationHeader(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if v != 7 {
		t.Fatalf("version: %d", v)
	}
	if !bytes.Equal(body, payload) {
		t.Fatal("payload mismatch")
	}
}

func TestDecodeMigrationHeader_TooShort(t *testing.T) {
	if _, _, err := DecodeMigrationHeader([]byte("abc")); err == nil {
		t.Fatal("expected error")
	}
}

func TestV1ToV2_AddsHistograms(t *testing.T) {
	in := []byte(`{"Logs":[]}`)
	out, err := V1ToV2(in)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	json.Unmarshal(out, &v)
	if _, ok := v["Histograms"]; !ok {
		t.Fatal("expected Histograms key")
	}
}

func TestV1ToV2_PreservesExisting(t *testing.T) {
	in := []byte(`{"Logs":[],"Histograms":[{"x":1}]}`)
	out, err := V1ToV2(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"x":1`)) {
		t.Fatal("should not overwrite existing Histograms")
	}
}

func TestV1ToV2_BadJSON(t *testing.T) {
	if _, err := V1ToV2([]byte("not json")); err == nil {
		t.Fatal("expected error")
	}
}

func TestV2ToV3_AddsTDigests(t *testing.T) {
	in := []byte(`{"Logs":[]}`)
	out, err := V2ToV3(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("TDigests")) {
		t.Fatal("expected TDigests key")
	}
}

func TestV2ToV3_PreservesExisting(t *testing.T) {
	in := []byte(`{"Logs":[],"TDigests":[{"x":1}]}`)
	out, _ := V2ToV3(in)
	if !bytes.Contains(out, []byte(`"x":1`)) {
		t.Fatal("should preserve existing TDigests")
	}
}

func TestV2ToV3_BadJSON(t *testing.T) {
	if _, err := V2ToV3([]byte("not json")); err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultChain_Ordering(t *testing.T) {
	chain := DefaultChain()
	for i := 1; i < len(chain); i++ {
		if chain[i].Version <= chain[i-1].Version {
			t.Errorf("chain not sorted at %d: %d <= %d", i, chain[i].Version, chain[i-1].Version)
		}
	}
}

func TestMigrator_EmptyChain(t *testing.T) {
	m := NewMigrator(nil)
	if m.Head() != 0 {
		t.Fatal("empty chain head should be 0")
	}
	out, head, err := m.Apply([]byte("x"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if head != 0 || string(out) != "x" {
		t.Fatal("empty chain should be a no-op")
	}
}
