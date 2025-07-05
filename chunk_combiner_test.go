package network

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
)

// Mock Deserializer for testing
type mockDeserializer struct{}

func (m *mockDeserializer) Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}

func (m *mockDeserializer) CreateDecoder(r io.Reader) Decoder {
	return msgpack.NewDecoder(r)
}

func TestChunkCombiner(t *testing.T) {
	outChan := make(chan io.Reader, 1)
	deserializer := &mockDeserializer{}
	combiner := NewChunkCombiner("test_combiner", deserializer, outChan)

	data1 := []byte("hello")
	data2 := []byte("world")

	chunk1 := &Chunk{ID: 1, Seq: 0, End: false, Data: data1}
	chunk2 := &Chunk{ID: 1, Seq: 1, End: true, Data: data2}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		combiner.addChunk(chunk1)
		combiner.addChunk(chunk2)
	}()

	wg.Wait()

	select {
	case reader := <-outChan:
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		assert.Equal(t, "helloworld", buf.String())
	default:
		t.Fatal("Expected a combined packet on the output channel")
	}

	combiner.Close()
}
