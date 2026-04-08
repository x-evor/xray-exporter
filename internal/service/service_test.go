package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"xray-exporter/internal/model"
)

type stubCounterSource struct {
	counters []model.RawCounter
	err      error
}

func (s stubCounterSource) FetchCounters(context.Context) ([]model.RawCounter, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.counters, nil
}

type stubIdentitySource struct {
	identities map[string]model.Identity
	err        error
}

func (s stubIdentitySource) FetchIdentities(context.Context) (map[string]model.Identity, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.identities, nil
}

func TestCollectFallsBackToUnknownEmailOnCacheMiss(t *testing.T) {
	svc := New(
		"jp-node",
		"prod",
		time.Minute,
		stubCounterSource{counters: []model.RawCounter{
			{UUID: "acct-1", InboundTag: "premium", Direction: "uplink", Value: 10},
			{UUID: "acct-1", InboundTag: "premium", Direction: "downlink", Value: 20},
		}},
		stubIdentitySource{err: errors.New("accounts unavailable")},
	)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("collect: %v", err)
	}

	snapshot := svc.Snapshot()
	if len(snapshot.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(snapshot.Samples))
	}
	if snapshot.Samples[0].Email != "unknown" {
		t.Fatalf("expected fallback email unknown, got %q", snapshot.Samples[0].Email)
	}
	ok, message := svc.Health()
	if !ok {
		t.Fatalf("expected degraded identity lookup to remain healthy")
	}
	if !strings.Contains(message, "identity lookup degraded") {
		t.Fatalf("expected degradation message, got %q", message)
	}
}

func TestCollectFailsGracefullyWhenXrayUnavailable(t *testing.T) {
	svc := New(
		"jp-node",
		"prod",
		time.Minute,
		stubCounterSource{err: errors.New("xray down")},
		stubIdentitySource{},
	)

	if err := svc.Collect(context.Background()); err == nil {
		t.Fatalf("expected collect error")
	}

	ok, message := svc.Health()
	if ok {
		t.Fatalf("expected unhealthy service after xray failure")
	}
	if !strings.Contains(message, "xray down") {
		t.Fatalf("expected xray failure in health message, got %q", message)
	}
}

func TestMetricsIncludeRequiredLabels(t *testing.T) {
	svc := New(
		"jp-node",
		"prod",
		time.Minute,
		stubCounterSource{counters: []model.RawCounter{
			{UUID: "acct-1", InboundTag: "premium", Direction: "uplink", Value: 10},
			{UUID: "acct-1", InboundTag: "premium", Direction: "downlink", Value: 20},
		}},
		stubIdentitySource{identities: map[string]model.Identity{
			"acct-1": {UUID: "acct-1", Email: "user@example.com", AccountUUID: "acct-1"},
		}},
	)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("collect: %v", err)
	}

	metrics := svc.MetricsText()
	for _, fragment := range []string{
		`uuid="acct-1"`,
		`email="user@example.com"`,
		`node_id="jp-node"`,
		`env="prod"`,
		`inbound_tag="premium"`,
	} {
		if !strings.Contains(metrics, fragment) {
			t.Fatalf("expected metrics to contain %s, got:\n%s", fragment, metrics)
		}
	}
}

func TestSnapshotContractJSON(t *testing.T) {
	svc := New(
		"jp-node",
		"prod",
		time.Minute,
		stubCounterSource{counters: []model.RawCounter{
			{UUID: "acct-1", InboundTag: "premium", Direction: "uplink", Value: 10},
			{UUID: "acct-1", InboundTag: "premium", Direction: "downlink", Value: 20},
		}},
		stubIdentitySource{identities: map[string]model.Identity{
			"acct-1": {UUID: "acct-1", Email: "user@example.com", AccountUUID: "acct-1"},
		}},
	)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("collect: %v", err)
	}

	payload, err := svc.SnapshotJSON()
	if err != nil {
		t.Fatalf("snapshot json: %v", err)
	}
	text := string(payload)
	for _, fragment := range []string{
		`"collected_at"`,
		`"node_id":"jp-node"`,
		`"env":"prod"`,
		`"uuid":"acct-1"`,
		`"email":"user@example.com"`,
		`"inbound_tag":"premium"`,
		`"uplink_bytes_total":10`,
		`"downlink_bytes_total":20`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected snapshot json to contain %s, got %s", fragment, text)
		}
	}
}
