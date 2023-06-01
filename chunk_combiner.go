package network

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gurupras/go-fragmentedbuf"
	log "github.com/sirupsen/logrus"
)

type PacketChunks struct {
	data        map[uint64]*Chunk
	totalChunks *uint64
	mutex       sync.Mutex
}

func (c *PacketChunks) AddChunk(chunk *Chunk) {
	c.data[chunk.Seq] = chunk
	if chunk.End {
		// Last chunk. Make note of the sequence number as this is the total number of chunks
		total := chunk.Seq + 1 // We add one since Seq starts from 0
		c.totalChunks = &total
	}
}

func (c *PacketChunks) Total() (uint64, error) {
	if c.totalChunks == nil {
		return 0, fmt.Errorf("unknown total number of chunks")
	}
	return *c.totalChunks, nil
}

func (c *PacketChunks) Complete() bool {
	if c.totalChunks == nil {
		return false
	}
	return *c.totalChunks == uint64(len(c.data))
}

type ChunkCombiner struct {
	name           string
	deserializer   Deserializer
	mutex          sync.Mutex
	partialPackets map[uint64]*PacketChunks
	outChan        chan<- io.Reader
	stopped        bool
}

func NewChunkCombiner(name string, deserializer Deserializer, outChan chan<- io.Reader) *ChunkCombiner {
	return &ChunkCombiner{
		name:           name,
		deserializer:   deserializer,
		mutex:          sync.Mutex{},
		partialPackets: make(map[uint64]*PacketChunks),
		outChan:        outChan,
		stopped:        false,
	}
}

func (c *ChunkCombiner) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.stopped = true
	close(c.outChan)
}

// Ideally, you will run this within a goroutine
func (c *ChunkCombiner) AddReader(reader io.Reader, name string) error {
	chunkDecoder := c.deserializer.CreateDecoder(reader)
	breakLoop := false
	for {
		var chunk Chunk
		err := chunkDecoder.Decode(&chunk)
		if breakLoop {
			break
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Debugf("[%v]: Error: %v", name, err)
			} else {
				log.Errorf("[%v]: Error: %v", name, err)
			}
			return err
		}
		c.AddChunk(&chunk)
		func() {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			if c.stopped {
				breakLoop = true
				return
			}
		}()
	}
	return nil
}

func (c *ChunkCombiner) ProcessIncomingBytes(b []byte) error {
	var chunk Chunk
	err := c.deserializer.Unmarshal(b, &chunk)
	if err != nil {
		return err
	}
	c.AddChunk(&chunk)
	return nil
}

func (c *ChunkCombiner) AddChunk(chunk *Chunk) {
	isComplete := false
	var packetChunks *PacketChunks
	func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		if c.stopped {
			return
		}
		var ok bool
		packetChunks, ok = c.partialPackets[chunk.ID]
		if !ok {
			// Create a new partial packet
			packetChunks = &PacketChunks{
				data:  make(map[uint64]*Chunk),
				mutex: sync.Mutex{},
			}
			c.partialPackets[chunk.ID] = packetChunks
		}
	}()

	func() {
		// Add current chunk to the partial packet's chunks
		packetChunks.mutex.Lock()
		defer packetChunks.mutex.Unlock()
		packetChunks.AddChunk(chunk)

		if packetChunks.Complete() {
			isComplete = true
		}
	}()
	if isComplete {
		func() {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			delete(c.partialPackets, chunk.ID)
		}()
		// We don't need to mutex for this because nobody else is going to be accessing this
		fragmentedBytesBuffer := fragmentedbuf.New()
		// We need to combine all the chunks
		total, _ := packetChunks.Total()
		for idx := uint64(0); idx < total; idx++ {
			c := packetChunks.data[idx]
			fragmentedBytesBuffer.Write(c.Data)
		}
		c.outChan <- fragmentedBytesBuffer
	}
}
