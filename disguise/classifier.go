package disguise

import (
	"errors"
	"fmt"
	"github.com/uDisguise/disguise/disguise/profile"
	"math"
	"math/rand"
	"sync"
)

// HMMClassifier encapsulates the Hidden Markov Model for traffic classification.
type HMMClassifier struct {
	mu sync.Mutex

	// States are our traffic profiles.
	States []profile.TrafficType
	
	// Emission and Transition probabilities
	EmissionProbs   map[profile.TrafficType][]float64
	TransitionProbs map[profile.TrafficType]map[profile.TrafficType]float64
	
	// Counters for online learning (to update probabilities)
	EmissionCounts   map[profile.TrafficType][]float64
	TransitionCounts map[profile.TrafficType]map[profile.TrafficType]float64
	
	// A small value to prevent log(0) errors.
	Epsilon float64
}

// NewHMMClassifier creates and initializes a new HMM classifier.
func NewHMMClassifier() *HMMClassifier {
	states := []profile.TrafficType{
		profile.WebBrowsing,
		profile.VideoStreaming,
		profile.FileDownload,
	}

	epsilon := 1e-9
	emissionProbs := make(map[profile.TrafficType][]float64)
	transitionProbs := make(map[profile.TrafficType]map[profile.TrafficType]float64)
	emissionCounts := make(map[profile.TrafficType][]float64)
	transitionCounts := make(map[profile.TrafficType]map[profile.TrafficType]float64)

	// Web Browsing: prefers small and medium packets.
	emissionProbs[profile.WebBrowsing] = []float64{0.7, 0.25, 0.05}
	emissionCounts[profile.WebBrowsing] = []float64{0, 0, 0}
	
	// Video Streaming: prefers large packets.
	emissionProbs[profile.VideoStreaming] = []float64{0.1, 0.3, 0.6}
	emissionCounts[profile.VideoStreaming] = []float64{0, 0, 0}
	
	// File Download: prefers consistently large packets.
	emissionProbs[profile.FileDownload] = []float64{0.05, 0.05, 0.9}
	emissionCounts[profile.FileDownload] = []float64{0, 0, 0}
	
	for _, s1 := range states {
		transitionProbs[s1] = make(map[profile.TrafficType]float64)
		transitionCounts[s1] = make(map[profile.TrafficType]float64)
		for _, s2 := range states {
			transitionProbs[s1][s2] = 1.0 / float64(len(states))
			transitionCounts[s1][s2] = 0
		}
	}

	// Add smoothing to initial probabilities
	for s, emissions := range emissionProbs {
		sum := 0.0
		for i, p := range emissions {
			emissions[i] = p + epsilon
			sum += emissions[i]
		}
		for i := range emissions {
			emissions[i] /= sum
		}
	}

	return &HMMClassifier{
		States:           states,
		EmissionProbs:    emissionProbs,
		TransitionProbs:  transitionProbs,
		EmissionCounts:   emissionCounts,
		TransitionCounts: transitionCounts,
		Epsilon:          epsilon,
	}
}

// Predict takes a sequence of observations and returns the most likely hidden state.
func (h *HMMClassifier) Predict(observations []int) (profile.TrafficType, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if len(observations) == 0 {
		return 0, errors.New("observations cannot be empty")
	}

	numStates := len(h.States)
	numObservations := len(observations)
	stateMap := make(map[profile.TrafficType]int)
	for i, s := range h.States {
		stateMap[s] = i
	}
	
	viterbi := make([][]float64, numStates)
	backpointer := make([][]int, numStates)
	for i := range viterbi {
		viterbi[i] = make([]float64, numObservations)
		backpointer[i] = make([]int, numObservations)
	}

	for i, state := range h.States {
		initialProb := 1.0 / float64(numStates)
		viterbi[i][0] = math.Log(initialProb) + math.Log(h.EmissionProbs[state][observations[0]])
	}
	
	for t := 1; t < numObservations; t++ {
		obs := observations[t]
		for i, currentState := range h.States {
			maxProb := -math.MaxFloat64
			maxState := 0
			for j, prevState := range h.States {
				prob := viterbi[j][t-1] + math.Log(h.TransitionProbs[prevState][currentState])
				if prob > maxProb {
					maxProb = prob
					maxState = j
				}
			}
			viterbi[i][t] = maxProb + math.Log(h.EmissionProbs[currentState][obs])
			backpointer[i][t] = maxState
		}
	}

	maxProb := -math.MaxFloat64
	lastState := 0
	for i := range h.States {
		if viterbi[i][numObservations-1] > maxProb {
			maxProb = viterbi[i][numObservations-1]
			lastState = i
		}
	}
	return h.States[lastState], nil
}

// Train updates the HMM's probabilities based on a sequence of observations and the corresponding ground truth.
func (h *HMMClassifier) Train(observations []int, groundTruth profile.TrafficType) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if len(observations) == 0 {
		return errors.New("observations cannot be empty")
	}

	// For a simplified online learning, we just update the counts.
	// A more robust solution would implement Baum-Welch or Viterbi training.
	
	// Update emission counts
	for _, obs := range observations {
		if obs >= len(h.EmissionCounts[groundTruth]) {
			continue // Skip invalid observations
		}
		h.EmissionCounts[groundTruth][obs]++
	}

	// Update transition counts (assuming a single, consistent state)
	if len(observations) > 1 {
		h.TransitionCounts[groundTruth][groundTruth] += float64(len(observations) - 1)
	}
	
	// Re-normalize probabilities
	h.reNormalizeProbabilities()
	
	return nil
}

func (h *HMMClassifier) reNormalizeProbabilities() {
	// Re-normalize emission probabilities from counts
	for state, counts := range h.EmissionCounts {
		total := 0.0
		for _, count := range counts {
			total += count
		}
		if total == 0 {
			total = 1.0 // Prevent division by zero
		}
		for i := range counts {
			h.EmissionProbs[state][i] = (counts[i] + h.Epsilon) / (total + float64(len(counts))*h.Epsilon)
		}
	}
	
	// Re-normalize transition probabilities
	for state, transitions := range h.TransitionCounts {
		total := 0.0
		for _, count := range transitions {
			total += count
		}
		if total == 0 {
			total = 1.0
		}
		for nextState, count := range transitions {
			h.TransitionProbs[state][nextState] = (count + h.Epsilon) / (total + float64(len(transitions))*h.Epsilon)
		}
	}
}

// DiscretizePayloadSize maps a payload length to a discrete bucket.
func DiscretizePayloadSize(length int) int {
	if length < 200 {
		return 0 // Small packets (e.g., headers, acknowledgments)
	} else if length < 800 {
		return 1 // Medium packets
	}
	return 2 // Large packets (e.g., data chunks)
}
