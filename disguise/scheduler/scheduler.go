package scheduler

import (
	"math/rand"
	"sync"
	"time"

	"github.com/uDisguise/disguise/disguise/framing"
	"github.com/uDisguise/disguise/disguise/profile"
)

// Scheduler manages the transmission order and timing of cells.
type Scheduler struct {
	mu           sync.Mutex
	profile      *profile.Profile
	queue        []*framing.Cell
	lastSendTime time.Time
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler() *Scheduler {
	return &Scheduler{
		profile:      profile.WebBrowsingProfile(), // Default profile
		queue:        make([]*framing.Cell, 0),
		lastSendTime: time.Now(),
	}
}

// SetProfile updates the active traffic profile.
func (s *Scheduler) SetProfile(p *profile.Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = p
}

// ScheduleCell adds a cell to the transmission queue with a randomized delay.
func (s *Scheduler) ScheduleCell(cell *framing.Cell) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use a small, random jitter to the send time
	jitter := time.Duration(rand.Int63n(int64(s.profile.LatencyJitter)))
	sendTime := time.Now().Add(jitter)

	// In a more complex implementation, we would use a priority queue
	// and more sophisticated scheduling. For simplicity, we just append.
	s.queue = append(s.queue, cell)
	s.lastSendTime = sendTime
}

// GetNextCell returns the next cell to be sent from the queue.
func (s *Scheduler) GetNextCell() *framing.Cell {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Wait for the scheduled send time to pass
	if time.Now().Before(s.lastSendTime) {
		return nil
	}

	// Simple queue pop
	if len(s.queue) > 0 {
		cell := s.queue[0]
		s.queue = s.queue[1:]
		return cell
	}

	return nil
}
