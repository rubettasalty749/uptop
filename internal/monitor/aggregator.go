package monitor

import "time"

type AggregationStrategy string

const (
	AggAnyDown      AggregationStrategy = "any-down"
	AggMajorityDown AggregationStrategy = "majority-down"
	AggAllDown      AggregationStrategy = "all-down"
)

type NodeResult struct {
	NodeID    string
	IsUp      bool
	LatencyNs int64
	CheckedAt time.Time
}

func AggregateStatus(results []NodeResult, strategy AggregationStrategy) (isUp bool, avgLatencyNs int64) {
	if len(results) == 0 {
		return true, 0
	}

	upCount := 0
	var totalLatency int64
	for _, r := range results {
		if r.IsUp {
			upCount++
		}
		totalLatency += r.LatencyNs
	}
	avgLatencyNs = totalLatency / int64(len(results))

	switch strategy {
	case AggMajorityDown:
		isUp = upCount > len(results)/2
	case AggAllDown:
		isUp = upCount > 0
	default:
		isUp = upCount == len(results)
	}
	return
}
