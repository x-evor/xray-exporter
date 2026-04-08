package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRawStatSupportsUserTrafficPattern(t *testing.T) {
	counter, ok := parseRawStat("user>>>acct-1>>>traffic>>>uplink", "42")
	if !ok {
		t.Fatalf("expected stat to parse")
	}
	if counter.UUID != "acct-1" || counter.Direction != "uplink" || counter.Value != 42 {
		t.Fatalf("unexpected counter %#v", counter)
	}
}

func TestParseRawStatSupportsInboundTrafficPattern(t *testing.T) {
	counter, ok := parseRawStat("inbound>>>premium>>>user>>>acct-1>>>traffic>>>downlink", 64.0)
	if !ok {
		t.Fatalf("expected stat to parse")
	}
	if counter.UUID != "acct-1" || counter.InboundTag != "premium" || counter.Direction != "downlink" || counter.Value != 64 {
		t.Fatalf("unexpected counter %#v", counter)
	}
}

func TestParseExpvarCountersFromDebugVarsStylePayload(t *testing.T) {
	payloadPath := filepath.Join("testdata", "debug_vars.sample.json")
	raw, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read sample payload: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	counters := parseExpvarCounters(payload)
	if len(counters) != 4 {
		t.Fatalf("expected 4 counters, got %d", len(counters))
	}
	if counters[0].UUID == "" && counters[1].UUID == "" && counters[2].UUID == "" && counters[3].UUID == "" {
		t.Fatalf("expected parsed uuids in counters %#v", counters)
	}
}
