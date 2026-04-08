package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"xray-exporter/internal/model"
)

type Client struct {
	url        string
	token      string
	httpClient *http.Client
}

func NewClient(url, token string) *Client {
	return &Client{
		url:        strings.TrimSpace(url),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) FetchCounters(ctx context.Context) ([]model.RawCounter, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build xray stats request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch xray stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch xray stats: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read xray stats payload: %w", err)
	}

	var payload struct {
		Samples []struct {
			UUID               string `json:"uuid"`
			InboundTag         string `json:"inbound_tag"`
			UplinkBytesTotal   int64  `json:"uplink_bytes_total"`
			DownlinkBytesTotal int64  `json:"downlink_bytes_total"`
		} `json:"samples"`
		Stats []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode xray stats payload: %w", err)
	}

	if len(payload.Samples) > 0 {
		counters := make([]model.RawCounter, 0, len(payload.Samples)*2)
		for _, sample := range payload.Samples {
			counters = append(counters,
				model.RawCounter{
					UUID:       strings.TrimSpace(sample.UUID),
					InboundTag: strings.TrimSpace(sample.InboundTag),
					Direction:  "uplink",
					Value:      sample.UplinkBytesTotal,
				},
				model.RawCounter{
					UUID:       strings.TrimSpace(sample.UUID),
					InboundTag: strings.TrimSpace(sample.InboundTag),
					Direction:  "downlink",
					Value:      sample.DownlinkBytesTotal,
				},
			)
		}
		return counters, nil
	}

	counters := make([]model.RawCounter, 0, len(payload.Stats))
	for _, stat := range payload.Stats {
		counter, ok := parseRawStat(stat.Name, stat.Value)
		if !ok {
			continue
		}
		counters = append(counters, counter)
	}
	if len(counters) > 0 {
		return counters, nil
	}

	var expvarPayload map[string]json.RawMessage
	if err := json.Unmarshal(body, &expvarPayload); err == nil {
		return parseExpvarCounters(expvarPayload), nil
	}
	return counters, nil
}

func parseExpvarCounters(payload map[string]json.RawMessage) []model.RawCounter {
	var statsMap map[string]interface{}
	if rawStats, ok := payload["stats"]; ok {
		_ = json.Unmarshal(rawStats, &statsMap)
	} else {
		statsMap = make(map[string]interface{}, len(payload))
		for key, rawValue := range payload {
			var value interface{}
			if err := json.Unmarshal(rawValue, &value); err != nil {
				continue
			}
			statsMap[key] = value
		}
	}

	counters := make([]model.RawCounter, 0, len(statsMap))
	for key, value := range statsMap {
		counter, ok := parseRawStat(key, value)
		if !ok {
			continue
		}
		counters = append(counters, counter)
	}
	return counters
}

func parseRawStat(name string, value interface{}) (model.RawCounter, bool) {
	parsedValue, ok := parseCounterValue(value)
	if !ok {
		return model.RawCounter{}, false
	}

	parts := strings.Split(strings.TrimSpace(name), ">>>")
	if len(parts) < 4 {
		return model.RawCounter{}, false
	}

	if parts[0] == "user" && len(parts) == 4 && parts[2] == "traffic" {
		return model.RawCounter{
			UUID:       strings.TrimSpace(parts[1]),
			InboundTag: "",
			Direction:  strings.TrimSpace(parts[3]),
			Value:      parsedValue,
		}, true
	}

	if parts[0] == "inbound" && len(parts) == 6 && parts[2] == "user" && parts[4] == "traffic" {
		return model.RawCounter{
			UUID:       strings.TrimSpace(parts[3]),
			InboundTag: strings.TrimSpace(parts[1]),
			Direction:  strings.TrimSpace(parts[5]),
			Value:      parsedValue,
		}, true
	}

	return model.RawCounter{}, false
}

func parseCounterValue(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
