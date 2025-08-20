package profile

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// TrafficType represents the type of traffic to simulate.
type TrafficType int

const (
	WebBrowsing TrafficType = iota
	VideoStreaming
	FileDownload
	// New dynamic profile mode
	Dynamic
)

// CellHeaderLen constant is needed by the Profile to calculate payload size.
const CellHeaderLen = 20

// Profile defines the parameters for a traffic simulation profile.
type Profile struct {
	MinCellSize       int
	MaxCellSize       int
	ProbingInterval   time.Duration
	LatencyJitter     time.Duration
	EWMAAlpha         float64
	TrafficWeights    map[TrafficType]float64
	PayloadDistributions map[TrafficType]distribution

	mu          sync.Mutex
	currentLoad float64
}

// distribution is an interface for a statistical distribution.
type distribution interface {
	Sample() int
}

// bimodalDistribution simulates two distinct peaks,
// e.g., small packets for headers and large packets for data.
type bimodalDistribution struct {
	mode1Mean   float64
	mode1StdDev float64
	mode1Weight float64
	mode2Mean   float64
	mode2StdDev float64
}

func (d *bimodalDistribution) Sample() int {
	if rand.Float64() < d.mode1Weight {
		return int(math.Max(1, rand.NormFloat64()*d.mode1StdDev+d.mode1Mean))
	}
	return int(math.Max(1, rand.NormFloat64()*d.mode2StdDev+d.mode2Mean))
}

// paretoDistribution simulates a "heavy-tailed" distribution.
type paretoDistribution struct {
	alpha float64
	xm    float64
}

func (d *paretoDistribution) Sample() int {
	return int(math.Max(1, d.xm/math.Pow(rand.Float64(), 1/d.alpha)))
}

// GetProfile returns a pre-configured profile instance.
func GetProfile(t TrafficType) *Profile {
	switch t {
	case WebBrowsing:
		return &Profile{
			MinCellSize:     64,
			MaxCellSize:     1400,
			ProbingInterval: 15 * time.Second,
			LatencyJitter:   20 * time.Millisecond,
			EWMAAlpha:       0.1,
			TrafficWeights: map[TrafficType]float64{WebBrowsing: 1.0},
			PayloadDistributions: map[TrafficType]distribution{
				WebBrowsing: &bimodalDistribution{
					mode1Mean:   100,
					mode1StdDev: 20,
					mode1Weight: 0.8,
					mode2Mean:   1000,
					mode2StdDev: 150,
				},
			},
		}
	case VideoStreaming:
		return &Profile{
			MinCellSize:     64,
			MaxCellSize:     1400,
			ProbingInterval: 10 * time.Second,
			LatencyJitter:   10 * time.Millisecond,
			EWMAAlpha:       0.2,
			TrafficWeights: map[TrafficType]float64{VideoStreaming: 1.0},
			PayloadDistributions: map[TrafficType]distribution{
				VideoStreaming: &bimodalDistribution{
					mode1Mean:   64,
					mode1StdDev: 10,
					mode1Weight: 0.2,
					mode2Mean:   1300,
					mode2StdDev: 50,
				},
			},
		}
	case FileDownload:
		return &Profile{
			MinCellSize:     64,
			MaxCellSize:     1400,
			ProbingInterval: 30 * time.Second,
			LatencyJitter:   50 * time.Millisecond,
			EWMAAlpha:       0.05,
			TrafficWeights: map[TrafficType]float64{FileDownload: 1.0},
			PayloadDistributions: map[TrafficType]distribution{
				FileDownload: &paretoDistribution{
					alpha: 1.5,
					xm:    500,
				},
			},
		}
	default:
		// Dynamic profile acts as a meta-profile that manages weights
		return &Profile{
			MinCellSize:     64,
			MaxCellSize:     1400,
			ProbingInterval: 15 * time.Second,
			LatencyJitter:   20 * time.Millisecond,
			EWMAAlpha:       0.1,
			TrafficWeights: map[TrafficType]float64{
				WebBrowsing:    0.7,
				VideoStreaming: 0.2,
				FileDownload:   0.1,
			},
			PayloadDistributions: map[TrafficType]distribution{
				WebBrowsing: &bimodalDistribution{
					mode1Mean:   100,
					mode1StdDev: 20,
					mode1Weight: 0.8,
					mode2Mean:   1000,
					mode2StdDev: 150,
				},
				VideoStreaming: &bimodalDistribution{
					mode1Mean:   64,
					mode1StdDev: 10,
					mode1Weight: 0.2,
					mode2Mean:   1300,
					mode2StdDev: 50,
				},
				FileDownload: &paretoDistribution{
					alpha: 1.5,
					xm:    500,
				},
			},
		}
	}
}

// GetNextPayloadLength returns a simulated payload length based on the active profile.
func (p *Profile) GetNextPayloadLength() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	trafficType := p.selectTrafficType()
	dist := p.PayloadDistributions[trafficType]
	length := dist.Sample()

	if length > p.MaxCellSize-CellHeaderLen {
		length = p.MaxCellSize - CellHeaderLen
	}
	if length < 1 {
		length = 1
	}
	
	p.updateLoad(length)

	return length
}

// GetNextCellSize returns a simulated total cell size.
func (p *Profile) GetNextCellSize() int {
	return rand.Intn(p.MaxCellSize-p.MinCellSize) + p.MinCellSize
}

// selectTrafficType selects a traffic type based on weighted probabilities.
func (p *Profile) selectTrafficType() TrafficType {
	if len(p.TrafficWeights) == 1 {
		for t := range p.TrafficWeights {
			return t
		}
	}
	
	r := rand.Float64()
	cumulativeWeight := 0.0
	for typ, weight := range p.TrafficWeights {
		cumulativeWeight += weight
		if r <= cumulativeWeight {
			return typ
		}
	}
	return WebBrowsing // Fallback
}

// updateLoad simulates an adaptive mechanism by updating the current load
// using an Exponentially Weighted Moving Average (EWMA).
func (p *Profile) updateLoad(latest int) {
	normalized := float64(latest) / float64(p.MaxCellSize)
	p.currentLoad = (p.currentLoad * (1 - p.EWMAAlpha)) + (normalized * p.EWMAAlpha)
}
