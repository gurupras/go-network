package network

import (
	"bytes"
	"encoding/gob"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

// GobSerDe implements the SerDe interface using Go's gob encoding.
// This is a self-contained implementation for testing purposes.
type GobSerDe struct{}

func NewGobSerDe() *GobSerDe {
	return &GobSerDe{}
}

// Marshaler
func (g *GobSerDe) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshaler
func (g *GobSerDe) Unmarshal(data []byte, v interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(v)
}

// CreateEncoder
func (g *GobSerDe) CreateEncoder(w io.Writer) Encoder {
	return gob.NewEncoder(w)
}

// CreateDecoder
func (g *GobSerDe) CreateDecoder(r io.Reader) Decoder {
	return gob.NewDecoder(r)
}

// TestPayload is the data structure we will send across the splitter/combiner.
type TestPayload struct {
	ID      int
	Message string
	Data    []byte
}

func TestSplitterAndCombinerIntegration(t *testing.T) {
	// Register the types that will be encoded/decoded by gob.
	// This is necessary because they are passed as interface{}.
	gob.Register(&Chunk{})
	gob.Register(&TestPayload{})

	serde := NewGobSerDe()

	splitterOutChan := make(chan WritePacket, 100)
	combinerOutChan := make(chan io.Reader, 1)

	// A small max packet size to ensure chunking happens
	splitter := NewChunkSplitter("test-splitter", 50, serde, nil)
	combiner := NewChunkCombiner("test-combiner", serde, combinerOutChan)
	defer combiner.Close()

	var transportWg sync.WaitGroup
	transportWg.Add(1)

	// This goroutine simulates the transport layer.
	go func() {
		defer transportWg.Done()
		for writePkt := range splitterOutChan {
			chunk, ok := writePkt.GetData().(*Chunk)
			if !ok {
				t.Errorf("GetData() did not return a *Chunk")
				if cb := writePkt.GetCallback(); cb != nil {
					cb(writePkt, nil)
				}
				continue
			}
			callback := writePkt.GetCallback()

			// Marshal the chunk to simulate sending it over the network
			chunkBytes, err := serde.Marshal(chunk)
			if err != nil {
				t.Errorf("failed to marshal chunk: %v", err)
				if callback != nil {
					callback(writePkt, err)
				}
				continue
			}

			// Feed the bytes into the combiner
			if err := combiner.ProcessIncomingBytes(chunkBytes); err != nil {
				t.Errorf("combiner failed to process bytes: %v", err)
				if callback != nil {
					callback(writePkt, err)
				}
				continue
			}

			// Signal completion for this chunk to unblock the splitter
			if callback != nil {
				callback(writePkt, nil)
			}
		}
	}()

	// The data we want to send
	payload := &TestPayload{
		ID:      123,
		Message: "This is a test message that is intentionally long to ensure it gets split into multiple chunks by the splitter.",
		Data:    bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 20), // 80 bytes of data
	}

	// Start the splitting process. The splitter will use the provided channel.
	err := splitter.SplitToChannel(payload, splitterOutChan)
	if err != nil {
		t.Fatalf("SplitToChannel failed: %v", err)
	}
	close(splitterOutChan)
	transportWg.Wait()

	// Wait for the combiner to finish and produce the reassembled data
	select {
	case reader := <-combinerOutChan:
		reassembledBytes, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read reassembled data: %v", err)
		}

		var resultPayload TestPayload
		err = serde.Unmarshal(reassembledBytes, &resultPayload)
		if err != nil {
			t.Fatalf("failed to unmarshal reassembled data: %v", err)
		}

		// Verify the reassembled data matches the original
		if !reflect.DeepEqual(payload, &resultPayload) {
			t.Errorf("data mismatch after reassembly.\nGot:  %+v\nWant: %+v", resultPayload, payload)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for combiner to produce result")
	}
}