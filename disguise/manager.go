package disguise

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/uDisguise/disguise/disguise/profile"
	"github.com/uDisguise/disguise/disguise/scheduler"
	"github.com/uDisguise/disguise/disguise/framing"
)

// ErrNoOutboundTraffic indicates there's no more traffic to send.
var ErrNoOutboundTraffic = errors.New("no outbound traffic available")

// Manager handles the full lifecycle of Disguise protocol.
type Manager struct {
	mu           sync.Mutex
	profile      *profile.Profile
	framer       *framing.Framer
	reassembler  *framing.Reassembler
	scheduler    *scheduler.Scheduler
	inboundQueue *bytes.Buffer
}

// NewManager initializes a new Disguise Manager.
func NewManager() *Manager {
	// Default to a Web Browsing profile. In a real-world scenario, this
	// would be set dynamically or via an API call.
	p := profile.WebBrowsingProfile()
	s := scheduler.NewScheduler()

	m := &Manager{
		profile:      p,
		framer:       framing.NewFramer(p),
		reassembler:  framing.NewReassembler(),
		scheduler:    s,
		inboundQueue: new(bytes.Buffer),
	}

	// Start the cover traffic generation loop
	go m.startCoverTrafficLoop()

	return m
}

// SetProfile dynamically changes the active traffic profile.
func (m *Manager) SetProfile(p *profile.Profile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profile = p
	m.framer.SetProfile(p)
	m.scheduler.SetProfile(p)
}

// QueueApplicationData takes application data and fragments it into cells.
func (m *Manager) QueueApplicationData(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cells, err := m.framer.Fragment(data)
	if err != nil {
		return err
	}

	// Schedule the cells for transmission
	for _, cell := range cells {
		m.scheduler.ScheduleCell(cell)
	}

	return nil
}

// GetOutboundTraffic fetches the next cell to be sent based on the scheduler.
func (m *Manager) GetOutboundTraffic() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cell := m.scheduler.GetNextCell()
	if cell == nil {
		// No real data or cover traffic to send right now
		return nil, ErrNoOutboundTraffic
	}

	return cell, nil
}

// ProcessInboundTraffic takes an inbound cell and reassembles it.
func (m *Manager) ProcessInboundTraffic(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cell, err := m.framer.DecodeCell(data)
	if err != nil {
		return err
	}

	if cell.Type == framing.TypeData {
		reassembled, err := m.reassembler.ProcessCell(cell)
		if err != nil {
			return err
		}
		if reassembled != nil {
			m.inboundQueue.Write(reassembled)
		}
	} else {
		// Process other cell types like Handshake, Control, Dummy etc.
		// For now, we simply ignore them.
		return nil
	}

	return nil
}

// ReadApplicationData reads reassembled application data from the internal buffer.
func (m *Manager) ReadApplicationData() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inboundQueue.Len() == 0 {
		return nil, nil // No data available
	}

	// Read all available data and reset buffer
	data := m.inboundQueue.Bytes()
	m.inboundQueue.Reset()
	return data, nil
}

// startCoverTrafficLoop periodically generates and schedules dummy traffic
func (m *Manager) startCoverTrafficLoop() {
	ticker := time.NewTicker(m.profile.ProbingInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		m.mu.Lock()
		dummyCell, err := m.framer.CreateDummyCell()
		if err == nil {
			m.scheduler.ScheduleCell(dummyCell)
		}
		m.mu.Unlock()
	}
}
