package network

import (
	"fmt"
	"math"
	"sync"
)

type ChunkSplitter struct {
	name            string
	MaxPacketSize   int
	marshaler       Marshaler
	outChan         chan<- WritePacket
	writePacketPool *sync.Pool
	packetIdx       uint64
	mutex           sync.Mutex
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
		mutex:           sync.Mutex{},
	}
}

func (c *ChunkSplitter) SplitToChannel(v interface{}, outChan chan<- WritePacket) error {
	encodedBytes, err := c.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	numChunks := int(math.Ceil(float64(len(encodedBytes)) / float64(c.MaxPacketSize)))
	var packetID uint64
	func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		packetID = c.packetIdx
		c.packetIdx++
	}()
	written := 0

	wg := sync.WaitGroup{}
	wg.Add(numChunks)

	mutex := sync.Mutex{}
	errors := make([]error, 0)

	callback := func(writePkt WritePacket, e error) {
		defer wg.Done()
		c.writePacketPool.Put(writePkt)
		if e != nil {
			mutex.Lock()
			defer mutex.Unlock()
			errors = append(errors, e)
		}
	}

	for chunkIdx := 0; chunkIdx < numChunks; chunkIdx++ {
		remaining := len(encodedBytes) - written
		chunkSize := int(math.Min(float64(remaining), float64(c.MaxPacketSize)))
		chunk := &Chunk{
			ID:   packetID,
			Seq:  uint64(chunkIdx),
			End:  chunkIdx == numChunks-1,
			Data: encodedBytes[written : written+chunkSize],
		}
		writePkt := c.writePacketPool.Get().(WritePacket)
		writePkt.SetData(chunk)
		writePkt.SetCallback(callback)
		outChan <- writePkt
		func() {
			mutex.Lock()
			defer mutex.Unlock()
			written += chunkSize
		}()
	}
	wg.Wait()
	if len(errors) != 0 {
		err := fmt.Errorf("[%v]: Encountered errors when encoding: %v", c.name, errors[0])
		return err
	}
	return nil
}

func (c *ChunkSplitter) Encode(v interface{}) error {
	return c.SplitToChannel(v, c.outChan)
}
