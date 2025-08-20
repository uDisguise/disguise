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
)

// Profile defines the parameters for a traffic simulation profile.
type Profile struct {
	MinCellSize       int
	MaxCellSize       int
	ProbingInterval   time.Duration
	LatencyJitter     time.Duration
	// EWMAAlpha is the smoothing factor for the adaptive model.
	EWMAAlpha float64

	// Weights for dynamic traffic type selection.
	// The sum of these weights should be 1.0.
	TrafficWeights map[TrafficType]float64

	// The current payload length distribution parameters.
	// This would be dynamically updated by a separate ML component.
	// For this production-grade example, we will use static, pre-defined values.
	PayloadDistributions map[TrafficType]distribution
	
	mu sync.Mutex
	// State for the adaptive model.
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
	mode1Weight float64 // The weight of the first mode (e.g., small packets)
	mode2Mean   float64
	mode2StdDev float64
}

// Sample samples a value from the bimodal distribution.
func (d *bimodalDistribution) Sample() int {
	if rand.Float64() < d.mode1Weight {
		return int(math.Max(1, rand.NormFloat64()*d.mode1StdDev+d.mode1Mean))
	}
	return int(math.Max(1, rand.NormFloat64()*d.mode2StdDev+d.mode2Mean))
}

// paretoDistribution simulates a "heavy-tailed" distribution,
// common for file sizes.
type paretoDistribution struct {
	alpha float64
	xm    float64
}

// Sample samples a value from the Pareto distribution.
func (d *paretoDistribution) Sample() int {
	return int(math.Max(1, d.xm/math.Pow(rand.Float64(), 1/d.alpha)))
}

// NewProfile creates a new Profile instance with production-ready distributions.
func NewProfile() *Profile {
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
				mode1Weight: 0.8, // 80% of packets are small
				mode2Mean:   1000,
				mode2StdDev: 150,
			},
			VideoStreaming: &bimodalDistribution{
				mode1Mean:   64,  // small control packets
				mode1StdDev: 10,
				mode1Weight: 0.2,
				mode2Mean:   1300, // large video data packets
				mode2StdDev: 50,
			},
			FileDownload: &paretoDistribution{
				alpha: 1.5, // Common alpha value for internet traffic
				xm:    500,
			},
		},
		currentLoad: 0.0,
	}
}

// GetNextPayloadLength returns a simulated payload length based on the active profile.
// This is the production-ready version.
func (p *Profile) GetNextPayloadLength() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Step 1: Dynamically select a traffic type based on weights.
	// This can be adapted based on the current load or application API.
	trafficType := p.selectTrafficType()

	// Step 2: Sample a length from the selected distribution.
	dist := p.PayloadDistributions[trafficType]
	length := dist.Sample()

	// Step 3: Clamp the length to the configured size boundaries.
	if length > p.MaxCellSize-CellHeaderLen {
		length = p.MaxCellSize - CellHeaderLen
	}
	if length < 1 {
		length = 1
	}
	
	// Step 4: Update the adaptive model with the newly generated length.
	p.updateLoad(length)

	return length
}

// selectTrafficType selects a traffic type based on weighted probabilities.
func (p *Profile) selectTrafficType() TrafficType {
	r := rand.Float64()
	cumulativeWeight := 0.0
	for typ, weight := range p.TrafficWeights {
		cumulativeWeight += weight
		if r <= cumulativeWeight {
			return typ
		}
	}
	// Fallback to a default type if something goes wrong.
	return WebBrowsing
}

// updateLoad simulates an adaptive mechanism by updating the current load
// using an Exponentially Weighted Moving Average (EWMA).
func (p *Profile) updateLoad(latest int) {
	// Normalize the latest value to a percentage of MaxCellSize.
	normalized := float64(latest) / float64(p.MaxCellSize)
	p.currentLoad = (p.currentLoad * (1 - p.EWMAAlpha)) + (normalized * p.EWMAAlpha)
}
