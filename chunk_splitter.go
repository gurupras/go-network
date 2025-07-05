package network

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type ChunkSplitter struct {
	name            string
	MaxPacketSize   int
	marshaler       Marshaler
	outChan         chan<- WritePacket
	writePacketPool *sync.Pool
	packetIdx       uint64
}

func NewChunkSplitter(name string, maxPacketSize int, marshaler Marshaler, outChan chan<- WritePacket) *ChunkSplitter {
	writePacketPool := &sync.Pool{
		New: func() any {
			return &writePacket{}
		},
	}

	return &ChunkSplitter{
		name:            name,
		MaxPacketSize:   maxPacketSize,
		marshaler:       marshaler,
		outChan:         outChan,
		writePacketPool: writePacketPool,
		packetIdx:       0,
	}
}

func (c *ChunkSplitter) nextPacketID() uint64 {
	val := atomic.AddUint64(&c.packetIdx, 1)
	return val - 1
}

func (c *ChunkSplitter) SplitToChannel(v interface{}, outChan chan<- WritePacket) error {
	encodedBytes, err := c.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	numBytes := len(encodedBytes)
	if numBytes == 0 {
		return nil
	}

	numChunks := (numBytes + c.MaxPacketSize - 1) / c.MaxPacketSize
	packetID := c.nextPacketID()

	var wg sync.WaitGroup
	wg.Add(numChunks)

	errChan := make(chan error, numChunks)

	callback := func(writePkt WritePacket, e error) {
		c.writePacketPool.Put(writePkt)
		if e != nil {
			errChan <- e
		}
		wg.Done()
	}

	for i := 0; i < numChunks; i++ {
		start := i * c.MaxPacketSize
		end := start + c.MaxPacketSize
		if end > numBytes {
			end = numBytes
		}

		chunk := &Chunk{
			ID:   packetID,
			Seq:  uint64(i),
			End:  i == numChunks-1,
			Data: encodedBytes[start:end],
		}

		writePkt := c.writePacketPool.Get().(WritePacket)
		writePkt.SetData(chunk)
		writePkt.SetCallback(callback)
		outChan <- writePkt
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for e := range errChan {
		errs = append(errs, e)
	}

	if len(errs) > 0 {
		return fmt.Errorf("[%v]: encountered %d error(s) during split, first error: %w", c.name, len(errs), errs[0])
	}

	return nil
}

func (c *ChunkSplitter) Encode(v interface{}) error {
	return c.SplitToChannel(v, c.outChan)
}
