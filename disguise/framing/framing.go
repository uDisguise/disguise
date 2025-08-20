package framing

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"
	"sync"
	"bytes"
	
	"github.com/uDisguise/disguise/disguise/profile"
)

// Cell structure definitions based on the specification.
const (
	CellHeaderLen = 20
	TypeData      = 0x01
	TypeHandshake = 0x02
	TypeControl   = 0x03
	TypeDummy     = 0x04
)

// Cell represents a Disguise protocol packet.
type Cell struct {
	CellID     uint16
	Type       uint8
	Flags      uint8
	Seq        uint32
	Timestamp  int64
	PayloadLen uint16
	PaddingLen uint16
	RandOffset uint16
	Payload    []byte
	Padding    []byte
}

// Framer handles fragmentation and cell creation.
type Framer struct {
	profile *profile.Profile
	mu      sync.Mutex
	seq     uint32
}

// NewFramer creates a new Framer instance.
func NewFramer(p *profile.Profile) *Framer {
	return &Framer{
		profile: p,
		seq:     0,
	}
}

// SetProfile updates the active traffic profile.
func (f *Framer) SetProfile(p *profile.Profile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profile = p
}

// Fragment takes a byte slice of application data and fragments it into a slice of Cells.
func (f *Framer) Fragment(data []byte) ([]*Cell, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var cells []*Cell
	cellID := f.generateCellID()
	payloadOffset := 0

	for payloadOffset < len(data) {
		cell := &Cell{
			CellID:    cellID,
			Type:      TypeData,
			Flags:     0x00,
			Timestamp: time.Now().UnixNano() / 1e6, // Milliseconds since Unix epoch
		}
		
		// Use a simple fragmentation strategy for now, can be improved with
		// dynamic profiling based on profile.MaxCellSize and others.
		payloadLen := f.profile.GetNextPayloadLength()
		if payloadOffset+payloadLen > len(data) {
			payloadLen = len(data) - payloadOffset
			cell.Flags |= 0x01 // Set End of Stream flag
		}

		cell.PayloadLen = uint16(payloadLen)
		cell.Payload = data[payloadOffset : payloadOffset+payloadLen]
		cell.Seq = f.seq
		f.seq++

		totalCellSize := f.profile.GetNextCellSize()
		paddingLen := totalCellSize - CellHeaderLen - payloadLen
		cell.PaddingLen = uint16(paddingLen)
		cell.Padding = make([]byte, paddingLen)
		_, err := rand.Read(cell.Padding)
		if err != nil {
			return nil, err
		}

		cell.RandOffset = f.generateRandomOffset(uint16(totalCellSize))

		cells = append(cells, cell)
		payloadOffset += payloadLen
	}

	return cells, nil
}

// CreateDummyCell creates a dummy cell for cover traffic.
func (f *Framer) CreateDummyCell() (*Cell, error) {
	totalCellSize := f.profile.GetNextCellSize()
	paddingLen := totalCellSize - CellHeaderLen
	padding := make([]byte, paddingLen)
	_, err := rand.Read(padding)
	if err != nil {
		return nil, err
	}

	cell := &Cell{
		CellID:     0x0000,
		Type:       TypeDummy,
		Flags:      0x00,
		Seq:        0,
		Timestamp:  time.Now().UnixNano() / 1e6,
		PayloadLen: 0,
		PaddingLen: uint16(paddingLen),
		RandOffset: f.generateRandomOffset(uint16(totalCellSize)),
		Payload:    []byte{},
		Padding:    padding,
	}
	return cell, nil
}

// EncodeCell serializes a Cell struct into a byte slice.
func (f *Framer) EncodeCell(cell *Cell) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, cell.CellID); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.Type); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.Flags); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.Seq); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.Timestamp); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.PayloadLen); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.PaddingLen); err != nil { return nil, err }
	if err := binary.Write(buf, binary.BigEndian, cell.RandOffset); err != nil { return nil, err }
	
	// Reorder payload and padding based on RandOffset.
	totalContent := make([]byte, cell.PayloadLen + cell.PaddingLen)
	copy(totalContent[cell.RandOffset:], cell.Payload)
	copy(totalContent, cell.Padding[:cell.RandOffset])
	copy(totalContent[cell.RandOffset + cell.PayloadLen:], cell.Padding[cell.RandOffset:])
	
	buf.Write(totalContent)

	return buf.Bytes(), nil
}

// DecodeCell deserializes a byte slice back into a Cell struct.
func (f *Framer) DecodeCell(data []byte) (*Cell, error) {
	if len(data) < CellHeaderLen {
		return nil, errors.New("cell data too short")
	}

	cell := &Cell{}
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &cell.CellID); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.Type); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.Flags); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.Seq); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.Timestamp); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.PayloadLen); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.PaddingLen); err != nil { return nil, err }
	if err := binary.Read(reader, binary.BigEndian, &cell.RandOffset); err != nil { return nil, err }

	payloadAndPadding := data[CellHeaderLen:]
	if len(payloadAndPadding) != int(cell.PayloadLen + cell.PaddingLen) {
		return nil, errors.New("cell content length mismatch")
	}
	
	// Reorder content to extract payload and padding
	cell.Payload = make([]byte, cell.PayloadLen)
	cell.Padding = make([]byte, cell.PaddingLen)
	
	copy(cell.Payload, payloadAndPadding[cell.RandOffset:])
	copy(cell.Padding, payloadAndPadding[:cell.RandOffset])
	copy(cell.Padding[cell.RandOffset:], payloadAndPadding[cell.RandOffset+cell.PayloadLen:])

	return cell, nil
}

// generateCellID creates a cryptographically secure random CellID.
func (f *Framer) generateCellID() uint16 {
	var id [2]byte
	_, err := rand.Read(id[:])
	if err != nil {
		return 0 // Fallback, though a real implementation should handle this
	}
	return binary.BigEndian.Uint16(id[:])
}

// generateRandomOffset creates a random offset for payload within the cell.
func (f *Framer) generateRandomOffset(max uint16) uint16 {
	if max == 0 {
		return 0
	}
	var offset [2]byte
	_, err := rand.Read(offset[:])
	if err != nil {
		return 0
	}
	return binary.BigEndian.Uint16(offset[:]) % max
}
