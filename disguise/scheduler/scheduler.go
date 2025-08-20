package scheduler

import (
	"container/heap"
	"sync"
	"time"

	"github.com/uDisguise/disguise/disguise/framing"
	"github.com/uDisguise/disguise/disguise/profile"
)

// cellItem is a wrapper for a Cell with a priority and index.
type cellItem struct {
	cell *framing.Cell
	// The priority determines the order of the item in the queue.
	// A lower value means higher priority.
	priority int64
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int
}

// cellPriorityQueue implements heap.Interface and holds cellItems.
type cellPriorityQueue []*cellItem

func (pq cellPriorityQueue) Len() int { return len(pq) }

func (pq cellPriorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq cellPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *cellPriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*cellItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *cellPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// Scheduler manages the transmission order and timing of cells.
type Scheduler struct {
	mu           sync.Mutex
	profile      *profile.Profile
	queue        cellPriorityQueue // Use the priority queue
	lastSendTime time.Time
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		profile:      profile.GetProfile(profile.WebBrowsing),
		queue:        make(cellPriorityQueue, 0),
		lastSendTime: time.Now(),
	}
	heap.Init(&s.queue)
	return s
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

	priority := time.Now().UnixNano()
	if cell.Type == framing.TypeDummy {
		// Dummy cells have lower priority to prioritize real data.
		// We add a large offset to their timestamp.
		priority = time.Now().Add(s.profile.ProbingInterval).UnixNano()
	}

	heap.Push(&s.queue, &cellItem{
		cell:     cell,
		priority: priority,
	})
}

// GetNextCell returns the next cell to be sent from the queue.
func (s *Scheduler) GetNextCell() *framing.Cell {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return nil
	}
	
	item := s.queue[0]
	if time.Now().UnixNano() < item.priority {
		return nil // Not yet time to send the highest-priority cell.
	}

	heap.Pop(&s.queue)
	return item.cell
}
