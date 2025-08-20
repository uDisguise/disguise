package framing

import (
	"bytes"
	"errors"
	"sync"
)

// ReassemblyStream holds the state for a single data stream.
type ReassemblyStream struct {
	buffer *bytes.Buffer
	lastSeq uint32
}

// Reassembler manages the reassembly of fragmented cells for multiple streams.
type Reassembler struct {
	mu sync.Mutex
	// streams maps a CellID to its corresponding reassembly state.
	streams map[uint16]*ReassemblyStream
}

// NewReassembler creates a new Reassembler instance capable of handling multiple streams.
func NewReassembler() *Reassembler {
	return &Reassembler{
		streams: make(map[uint16]*ReassemblyStream),
	}
}

// ProcessCell processes an incoming cell and returns the reassembled payload
// if a full message has been received for that stream.
func (r *Reassembler) ProcessCell(cell *Cell) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create the reassembly stream for this CellID.
	stream, ok := r.streams[cell.CellID]
	if !ok {
		// New stream, initialize it.
		stream = &ReassemblyStream{
			buffer:  new(bytes.Buffer),
			lastSeq: cell.Seq - 1, // Assume the first cell's sequence number is valid
		}
		r.streams[cell.CellID] = stream
	}

	// Simple check for out-of-order or duplicate cells within the same stream.
	if cell.Seq != stream.lastSeq+1 {
		return nil, errors.New("out-of-order or invalid cell received for stream")
	}

	// Append payload to the stream's buffer.
	stream.buffer.Write(cell.Payload)
	stream.lastSeq = cell.Seq

	// If it's the end of the stream, return the full payload and clean up.
	if cell.Flags&0x01 != 0 {
		payload := stream.buffer.Bytes()
		delete(r.streams, cell.CellID) // Clean up the stream state
		return payload, nil
	}

	// Not the end of the stream, return nil.
	return nil, nil
}
