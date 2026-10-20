package command

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"digital-contracting-service/internal/base/datatype"
)

// Ontology keys on dcs:ContractMetadata (optional except title). These travel
// inside the shipped PDF's embedded JSON-LD so the peer can fill its contracts
// row on initial replication.
const (
	metaTitle                  = "dcs:title"
	metaDescription            = "dcs:description"
	metaStartDate              = "dcs:startDate"
	metaExpirationDate         = "dcs:expirationDate"
	metaExpirationNoticePeriod = "dcs:expirationNoticePeriod"
	metaExpirationPolicy       = "dcs:expirationPolicy"
)

// sharedContractDetails mirrors the contracts-row Shared Contract Details that
// must agree across DCS instances after Offer.
type sharedContractDetails struct {
	Name            *string
	Description     *string
	StartDate       *time.Time
	ExpDate         *time.Time
	ExpPolicy       *string
	ExpNoticePeriod *int
}

// parseShippedSharedDetails reads Shared Contract Details from dcs:metadata.
func parseShippedSharedDetails(payload []byte) sharedContractDetails {
	var out sharedContractDetails
	var doc map[string]any

	err := json.Unmarshal(payload, &doc)
	if err != nil {
		return out
	}

	meta, ok := doc["dcs:metadata"].(map[string]any)
	if !ok {
		return out
	}

	if t, ok := stringFromMeta(meta, metaTitle); ok {
		out.Name = &t
	}

	if d, ok := stringFromMeta(meta, metaDescription); ok {
		out.Description = &d
	}

	if s, ok := stringFromMeta(meta, metaStartDate); ok {
		ts, err := parseSharedTime(s)
		if err == nil {
			out.StartDate = &ts
		}
	}

	if s, ok := stringFromMeta(meta, metaExpirationDate); ok {
		ts, err := parseSharedTime(s)
		if err == nil {
			out.ExpDate = &ts
		}
	}

	if s, ok := stringFromMeta(meta, metaExpirationPolicy); ok {
		out.ExpPolicy = &s
	}

	if n, ok := intFromMeta(meta, metaExpirationNoticePeriod); ok {
		out.ExpNoticePeriod = &n
	}

	return out
}

// stampSharedDetailsIntoContractData writes Shared Contract Details into
// dcs:metadata so the next PDF embed carries them. Returns whether metadata changed.
func stampSharedDetailsIntoContractData(payload *datatype.JSON, details sharedContractDetails) (*datatype.JSON, bool, error) {
	if payload == nil || !payload.IsNotNullValue() {
		return payload, false, nil
	}

	var doc map[string]any
	err := json.Unmarshal([]byte(*payload), &doc)
	if err != nil {
		return nil, false, fmt.Errorf("could not parse contract data for shared details: %w", err)
	}

	meta, _ := doc["dcs:metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{"@type": "dcs:ContractMetadata"}
	}

	before, _ := json.Marshal(meta)

	if details.Name != nil {
		if *details.Name == "" {
			delete(meta, metaTitle)
		} else {
			meta[metaTitle] = *details.Name
		}
	}

	if details.Description != nil {
		if *details.Description == "" {
			delete(meta, metaDescription)
		} else {
			meta[metaDescription] = *details.Description
		}
	}

	setOptionalTime(meta, metaStartDate, details.StartDate)
	setOptionalTime(meta, metaExpirationDate, details.ExpDate)

	if details.ExpPolicy != nil {
		if *details.ExpPolicy == "" {
			delete(meta, metaExpirationPolicy)
		} else {
			meta[metaExpirationPolicy] = *details.ExpPolicy
		}
	}

	if details.ExpNoticePeriod != nil {
		meta[metaExpirationNoticePeriod] = *details.ExpNoticePeriod
	}

	after, _ := json.Marshal(meta)
	if string(before) == string(after) {
		return payload, false, nil
	}

	doc["dcs:metadata"] = meta
	raw, err := json.Marshal(doc)

	if err != nil {
		return nil, false, fmt.Errorf("could not marshal contract data with shared details: %w", err)
	}

	stamped := datatype.JSON(raw)

	return &stamped, true, nil
}

func stringFromMeta(meta map[string]any, key string) (string, bool) {
	v, ok := meta[key]

	if !ok || v == nil {
		return "", false
	}

	switch typed := v.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	case map[string]any:
		s, ok := typed["@value"].(string)
		if !ok || s == "" {
			return "", false
		}
		return s, true
	default:
		return "", false
	}
}

func intFromMeta(meta map[string]any, key string) (int, bool) {
	v, ok := meta[key]

	if !ok || v == nil {
		return 0, false
	}

	switch n := v.(type) {
	case float64:
		if math.Trunc(n) != n {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func parseSharedTime(s string) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, s)

	if err != nil {
		ts, err = time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("unrecognized shared detail time %q", s)
		}
	}

	return ts.UTC(), nil
}

func setOptionalTime(meta map[string]any, key string, t *time.Time) {
	if t == nil {
		return
	}

	meta[key] = map[string]any{
		"@value": t.UTC().Format(time.RFC3339),
		"@type":  "xsd:dateTime",
	}
}
