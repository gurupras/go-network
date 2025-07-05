package network

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
)

// Mock Marshaler for testing
type mockMarshaler struct{}

func (m *mockMarshaler) Marshal(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

func TestChunkSplitter(t *testing.T) {
	outChan := make(chan WritePacket)
	marshaler := &mockMarshaler{}
	splitter := NewChunkSplitter("test_splitter", 5, marshaler, outChan)

	wg := sync.WaitGroup{}
	wg.Add(1)

	chunks := []string{}
	go func() {
		defer wg.Done()
		for wp := range outChan {
			chunk := wp.GetData().(*Chunk)
			chunks = append(chunks, string(chunk.Data))
			cb := wp.GetCallback()
			if cb != nil {
				cb(wp, nil)
			}
		}
	}()

	data := "helloworld"
	err := splitter.SplitToChannel(data, outChan)
	assert.NoError(t, err)

	close(outChan)

	wg.Wait()

	assert.Len(t, chunks, 3)
}

func TestChunkSplitter_SequentialPacketIDs(t *testing.T) {
	outChan := make(chan WritePacket)
	marshaler := &mockMarshaler{}
	splitter := NewChunkSplitter("test_splitter_seq", 10, marshaler, outChan)

	wg := sync.WaitGroup{}
	wg.Add(1)

	receivedChunks := []*Chunk{}
	go func() {
		defer wg.Done()
		for wp := range outChan {
			chunk := wp.GetData().(*Chunk)
			receivedChunks = append(receivedChunks, chunk)
			cb := wp.GetCallback()
			if cb != nil {
				cb(wp, nil)
			}
		}
	}()

	// First call
	data1 := "some-data-that-is-long"
	err := splitter.SplitToChannel(data1, outChan)
	assert.NoError(t, err)

	// Second call
	data2 := "some-other-data"
	err = splitter.SplitToChannel(data2, outChan)
	assert.NoError(t, err)

	close(outChan)
	wg.Wait()

	marshaled1, _ := marshaler.Marshal(data1)
	numChunks1 := (len(marshaled1) + splitter.MaxPacketSize - 1) / splitter.MaxPacketSize

	marshaled2, _ := marshaler.Marshal(data2)
	numChunks2 := (len(marshaled2) + splitter.MaxPacketSize - 1) / splitter.MaxPacketSize

	assert.Len(t, receivedChunks, numChunks1+numChunks2)

	// Check first batch of chunks
	firstPacketID := receivedChunks[0].ID
	for i := 0; i < numChunks1; i++ {
		assert.Equal(t, firstPacketID, receivedChunks[i].ID, "All chunks from the same split should have the same packet ID")
		assert.Equal(t, uint64(i), receivedChunks[i].Seq, "Sequence should be correct")
	}

	// Check second batch of chunks
	secondPacketID := receivedChunks[numChunks1].ID
	for i := 0; i < numChunks2; i++ {
		assert.Equal(t, secondPacketID, receivedChunks[numChunks1+i].ID, "All chunks from the same split should have the same packet ID")
		assert.Equal(t, uint64(i), receivedChunks[numChunks1+i].Seq, "Sequence should be correct")
	}

	assert.Equal(t, uint64(0), firstPacketID, "First packet ID should be 0")
	assert.Equal(t, uint64(1), secondPacketID, "Second packet ID should be 1")
}