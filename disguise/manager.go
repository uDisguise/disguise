package disguise

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uDisguise/disguise/disguise/framing"
	"github.com/uDisguise/disguise/disguise/profile"
	"github.com/uDisguise/disguise/disguise/scheduler"
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
	
	// HMM Classifier for dynamic profiling
	classifier   *HMMClassifier
	observationQueue []int
	
	// Dynamic profiling state
	lastProfileSwitch time.Time
}

// NewManager initializes a new Disguise Manager.
func NewManager() *Manager {
	p := profile.GetProfile(profile.Dynamic) // 默认使用动态模式
	s := scheduler.NewScheduler()
	
	m := &Manager{
		profile:      p,
		framer:       framing.NewFramer(p),
		reassembler:  framing.NewReassembler(),
		scheduler:    s,
		inboundQueue: new(bytes.Buffer),
		classifier:   NewHMMClassifier(),
		observationQueue: make([]int, 0, 100),
		lastProfileSwitch: time.Now(),
	}

	go m.startCoverTrafficLoop()
	go m.startDynamicProfilingLoop()

	return m
}

// SetProfile dynamically changes the active traffic profile.
func (m *Manager) SetProfile(p *profile.Profile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profile = p
	m.framer.SetProfile(p)
	m.scheduler.SetProfile(p)
	m.lastProfileSwitch = time.Now()
}

// QueueApplicationData takes application data and fragments it into cells.
func (m *Manager) QueueApplicationData(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cells, err := m.framer.Fragment(data)
	if err != nil {
		return err
	}

	for _, cell := range cells {
		// Collect payload length for HMM classification
		m.observationQueue = append(m.observationQueue, DiscretizePayloadSize(len(cell.Payload)))
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
		return nil, ErrNoOutboundTraffic
	}

	encodedCell, err := m.framer.EncodeCell(cell)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cell: %w", err)
	}

	return encodedCell, nil
}

// ProcessInboundTraffic takes an inbound cell and reassembles it.
func (m *Manager) ProcessInboundTraffic(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cell, err := m.framer.DecodeCell(data)
	if err != nil {
		return fmt.Errorf("failed to decode cell: %w", err)
	}

	if cell.Type == framing.TypeData {
		// Collect payload length for HMM classification
		m.observationQueue = append(m.observationQueue, DiscretizePayloadSize(len(cell.Payload)))
		
		reassembled, err := m.reassembler.ProcessCell(cell)
		if err != nil {
			return fmt.Errorf("failed to reassemble cell: %w", err)
		}
		if reassembled != nil {
			m.inboundQueue.Write(reassembled)
		}
	} else {
		// Process other cell types like Handshake, Control, Dummy etc.
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

// startDynamicProfilingLoop analyzes traffic load and switches profiles accordingly.
func (m *Manager) startDynamicProfilingLoop() {
	ticker := time.NewTicker(5 * time.Second) // Check more frequently
	defer ticker.Stop()

	for {
		<-ticker.C
		m.mu.Lock()
		
		// 确保有足够的观察值来做预测
		if len(m.observationQueue) < 10 {
			m.mu.Unlock()
			continue
		}
		
		// 使用 HMM 预测最可能的流量类型
		predictedType, err := m.classifier.Predict(m.observationQueue)
		if err != nil {
			m.mu.Unlock()
			continue
		}
		
		// 如果预测的类型与当前类型不同，则切换配置文件
		if predictedType != m.profile.GetProfileType() {
			m.SetProfile(profile.GetProfile(predictedType))
			fmt.Printf("动态剖析: 切换到 %v 配置文件。\n", predictedType)
		}
		
		// 清空观察队列，准备下一次预测
		m.observationQueue = m.observationQueue[:0]
		m.mu.Unlock()
	}
}
