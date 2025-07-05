package network

import (
	"fmt"
	"io"
	"sync"
)

type WriteCallback func(data interface{}) error

type ChunkedTransport struct {
	name           string
	MaxPackageSize int
	serde          SerDe
	splitter       *ChunkSplitter
	combiner       ChunkCombinerInterface
	sendChan       chan WritePacket
	recvChan       chan io.Reader
	write          WriteCallback
	wg             sync.WaitGroup
}

func New(name string, packetSize int, serde SerDe, write WriteCallback) *ChunkedTransport {
	sendChan := make(chan WritePacket)
	recvChan := make(chan io.Reader)

	ret := &ChunkedTransport{
		name:           name,
		MaxPackageSize: packetSize,
		serde:          serde,
		splitter:       NewChunkSplitter(name, packetSize, serde, sendChan),
		combiner:       NewChunkCombiner(name, serde, recvChan),
		sendChan:       sendChan,
		recvChan:       recvChan,
		write:          write,
		wg:             sync.WaitGroup{},
	}

	ret.wg.Add(1)

	go func() {
		defer ret.wg.Done()
		ret.processSendChan()
	}()

	return ret
}

func (c *ChunkedTransport) processSendChan() error {
	for pkt := range c.sendChan {
		err := c.write(pkt)
		pkt.GetCallback()(pkt, err)
	}
	return nil
}

// Notify the network of any data that is coming in from the outside
func (c *ChunkedTransport) OnData(b []byte) error {
	return c.combiner.ProcessIncomingBytes(b)
}

func (c *ChunkedTransport) Close() error {
	close(c.sendChan)
	return c.combiner.Close()
}

// Send some data out over the network
func (c *ChunkedTransport) Send(data interface{}) error {
	return c.splitter.Encode(data)
}

// Read a full "packet" out of the network
func (c *ChunkedTransport) ReadPacket(pkt interface{}) error {
	reader, ok := <-c.recvChan
	if !ok {
		return io.EOF
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("[%v]: failed to read incoming packet: %v", c.name, err)
	}
	err = c.serde.Unmarshal(data, &pkt)
	if err != nil {
		return err
	}
	return nil
}
