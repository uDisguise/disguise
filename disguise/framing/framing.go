package framing

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/rand"
	"sync"
	"time"

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
			Timestamp: time.Now().UnixNano() / 1e6,
		}
		
		payloadLen := f.profile.GetNextPayloadLength()
		if payloadOffset+payloadLen > len(data) {
			payloadLen = len(data) - payloadOffset
			cell.Flags |= 0x01
		}

		cell.PayloadLen = uint16(payloadLen)
		cell.Payload = data[payloadOffset : payloadOffset+payloadLen]
		cell.Seq = f.seq
		f.seq++

		totalCellSize := f.profile.GetNextCellSize()
		paddingLen := totalCellSize - CellHeaderLen - payloadLen
		
		if paddingLen < 0 {
			paddingLen = 0
		}
		
		cell.PaddingLen = uint16(paddingLen)
		
		// Infer profile type without calling a non-existent method
		var currentProfileType profile.TrafficType
		if len(f.profile.TrafficWeights) == 1 {
			for t := range f.profile.TrafficWeights {
				currentProfileType = t
			}
		} else {
			// For dynamic profile, we assume WebBrowsing as a heuristic
			currentProfileType = profile.WebBrowsing
		}

		cell.Padding = f.generatePadding(paddingLen, currentProfileType)

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

	var currentProfileType profile.TrafficType
	if len(f.profile.TrafficWeights) == 1 {
		for t := range f.profile.TrafficWeights {
			currentProfileType = t
		}
	} else {
		currentProfileType = profile.WebBrowsing
	}

	padding := f.generatePadding(paddingLen, currentProfileType)

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

// generatePadding creates content-aware or random padding.
func (f *Framer) generatePadding(length int, profileType profile.TrafficType) []byte {
	if length <= 0 {
		return []byte{}
	}

	if profileType == profile.WebBrowsing {
		switch rand.Intn(2) {
		case 0:
			data := make([]byte, (length/4)*3)
			crypto_rand.Read(data)
			encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(encoded, data)
			if len(encoded) > length {
				return encoded[:length]
			}
			padding := make([]byte, length)
			copy(padding, encoded)
			crypto_rand.Read(padding[len(encoded):])
			return padding
		case 1:
			padding := make([]byte, length)
			crypto_rand.Read(padding)
			for i := 0; i < len(padding); i += 10 {
				padding[i] = 0x00
			}
			return padding
		}
	}

	padding := make([]byte, length)
	crypto_rand.Read(padding)
	return padding
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
	
	totalContent := make([]byte, cell.PayloadLen + cell.PaddingLen)
	
	copy(totalContent[cell.RandOffset:], cell.Payload)
	copy(totalContent, cell.Padding[:cell.RandOffset])
	copy(totalContent[cell.RandOffset+cell.PayloadLen:], cell.Padding[cell.RandOffset:])

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
	
	cell.Payload = make([]byte, cell.PayloadLen)
	cell.Padding = make([]byte, cell.PaddingLen)
	
	copy(cell.Payload, payloadAndPadding[cell.RandOffset:int(cell.RandOffset+cell.PayloadLen)])
	copy(cell.Padding, payloadAndPadding[:cell.RandOffset])
	copy(cell.Padding[cell.RandOffset:], payloadAndPadding[int(cell.RandOffset+cell.PayloadLen):])

	return cell, nil
}

// generateCellID creates a cryptographically secure random CellID.
func (f *Framer) generateCellID() uint16 {
	var id [2]byte
	_, err := crypto_rand.Read(id[:])
	if err != nil {
		return 0
	}
	return binary.BigEndian.Uint16(id[:])
}

// generateRandomOffset creates a random offset for payload within the cell.
func (f *Framer) generateRandomOffset(max uint16) uint16 {
	if max <= 1 {
		return 0
	}
	return uint16(rand.Intn(int(max - 1)))
}
