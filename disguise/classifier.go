package disguise

import (
	"errors"
	"github.com/uDisguise/disguise/disguise/profile"
	"math"
)

// HMMClassifier encapsulates the Hidden Markov Model for traffic classification.
type HMMClassifier struct {
	// States are our traffic profiles.
	States []profile.TrafficType
	// Emission probabilities: P(observation | state).
	// For simplicity, we discretize payload sizes into a few buckets.
	EmissionProbs map[profile.TrafficType][]float64
	// Transition probabilities: P(next state | current state).
	TransitionProbs map[profile.TrafficType]map[profile.TrafficType]float64
	
	// A small value to prevent log(0) errors.
	Epsilon float64
}

// NewHMMClassifier creates and initializes a new HMM classifier.
func NewHMMClassifier() *HMMClassifier {
	// Define the traffic types (states) we are interested in.
	states := []profile.TrafficType{
		profile.WebBrowsing,
		profile.VideoStreaming,
		profile.FileDownload,
	}

	// Initialize with some sensible, smoothed prior probabilities.
	// In a real-world scenario, these would be learned from a large dataset.
	epsilon := 1e-9
	emissionProbs := make(map[profile.TrafficType][]float64)
	transitionProbs := make(map[profile.TrafficType]map[profile.TrafficType]float64)

	// Web Browsing: prefers small and medium packets.
	emissionProbs[profile.WebBrowsing] = []float64{0.7, 0.25, 0.05}
	// Video Streaming: prefers large packets.
	emissionProbs[profile.VideoStreaming] = []float64{0.1, 0.3, 0.6}
	// File Download: prefers consistently large packets.
	emissionProbs[profile.FileDownload] = []float64{0.05, 0.05, 0.9}

	// Transition probabilities (smoothed to avoid zero-probability).
	for _, s1 := range states {
		transitionProbs[s1] = make(map[profile.TrafficType]float64)
		for _, s2 := range states {
			transitionProbs[s1][s2] = 1.0 / float64(len(states)) // Uniform prior
		}
	}

	// Add smoothing
	for _, emissions := range emissionProbs {
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
		States:        states,
		EmissionProbs: emissionProbs,
		TransitionProbs: transitionProbs,
		Epsilon:       epsilon,
	}
}

// Predict takes a sequence of observations (discretized cell lengths) and
// returns the most likely hidden state (traffic type) sequence using the Viterbi algorithm.
func (h *HMMClassifier) Predict(observations []int) (profile.TrafficType, error) {
	if len(observations) == 0 {
		return 0, errors.New("observations cannot be empty")
	}

	numStates := len(h.States)
	numObservations := len(observations)
	
	// State mapping for easy lookup
	stateMap := make(map[profile.TrafficType]int)
	for i, s := range h.States {
		stateMap[s] = i
	}
	
	// Viterbi path and probability matrix
	viterbi := make([][]float64, numStates)
	backpointer := make([][]int, numStates)
	for i := range viterbi {
		viterbi[i] = make([]float64, numObservations)
		backpointer[i] = make([]int, numObservations)
	}

	// Initialization step (t=0)
	for i, state := range h.States {
		obs := observations[0]
		initialProb := 1.0 / float64(numStates)
		viterbi[i][0] = math.Log(initialProb) + math.Log(h.EmissionProbs[state][obs])
	}
	
	// Recursion step (t=1 to T-1)
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

	// Termination step
	maxProb := -math.MaxFloat64
	lastState := 0
	for i := range h.States {
		if viterbi[i][numObservations-1] > maxProb {
			maxProb = viterbi[i][numObservations-1]
			lastState = i
		}
	}

	// Return the most likely final state
	return h.States[lastState], nil
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
