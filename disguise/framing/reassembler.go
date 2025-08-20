package framing

import (
	"bytes"
	"errors"
	"sync"
)

// Reassembler manages the reassembly of fragmented cells.
type Reassembler struct {
	mu         sync.Mutex
	// In a real-world scenario, a map of CellID to a buffer would be used
	// for multi-stream support.
	currentCellID  uint16
	currentSeq     uint32
	buffer         *bytes.Buffer
}

// NewReassembler creates a new Reassembler instance.
func NewReassembler() *Reassembler {
	return &Reassembler{
		buffer: new(bytes.Buffer),
	}
}

// ProcessCell processes an incoming cell and returns the reassembled payload
// if a full message has been received.
func (r *Reassembler) ProcessCell(cell *Cell) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if this is a new message stream
	if r.currentCellID == 0 {
		r.currentCellID = cell.CellID
		r.currentSeq = cell.Seq
	}

	// Simple check for out-of-order or wrong stream cells
	if cell.CellID != r.currentCellID || cell.Seq < r.currentSeq {
		return nil, errors.New("out-of-order or invalid cell received")
	}

	// Append payload to the buffer
	r.buffer.Write(cell.Payload)
	r.currentSeq = cell.Seq

	// If it's the end of the stream, return the full payload
	if cell.Flags&0x01 != 0 {
		payload := r.buffer.Bytes()
		r.buffer.Reset()
		r.currentCellID = 0
		r.currentSeq = 0
		return payload, nil
	}

	return nil, nil // Return nil if the message is not yet complete
}
