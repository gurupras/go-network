package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockSerDe struct{}

func (m *MockSerDe) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (m *MockSerDe) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (m *MockSerDe) CreateEncoder(w io.Writer) Encoder {
	return json.NewEncoder(w)
}

func (m *MockSerDe) CreateDecoder(r io.Reader) Decoder {
	return json.NewDecoder(r)
}

func TestChunkedTransport(t *testing.T) {
	var (
		transport1 *ChunkedTransport
		transport2 *ChunkedTransport
	)
	serde := &MockSerDe{}

	writeCallback1 := func(data interface{}) error {
		pkt := data.(WritePacket)
		chunkBytes, err := serde.Marshal(pkt.GetData())
		require.NoError(t, err)
		return transport2.OnData(chunkBytes)
	}

	writeCallback2 := func(data interface{}) error {
		pkt := data.(WritePacket)
		chunkBytes, err := serde.Marshal(pkt.GetData())
		require.NoError(t, err)
		return transport1.OnData(chunkBytes)
	}

	transport1 = New("transport1", 3, serde, writeCallback1)
	transport2 = New("transport2", 5, serde, writeCallback2)

	defer transport1.Close()
	defer transport2.Close()

	t.Run("TestSendReceive", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				msg := fmt.Sprintf("test-message-%d", i)
				err := transport1.Send(msg)
				require.NoError(t, err)
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				var msg string
				err := transport2.ReadPacket(&msg)
				require.NoError(t, err)
				expected := fmt.Sprintf("test-message-%d", i)
				require.Equal(t, expected, msg)
			}
		}()
		wg.Wait()
	})

	t.Run("TestLargeSendReceive", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)

		largeData := make([]byte, 2048)
		for i := 0; i < 2048; i++ {
			largeData[i] = byte(i)
		}

		go func() {
			defer wg.Done()
			err := transport1.Send(largeData)
			require.NoError(t, err)
		}()

		go func() {
			defer wg.Done()
			var receivedData []byte
			err := transport2.ReadPacket(&receivedData)
			require.NoError(t, err)
			require.True(t, bytes.Equal(largeData, receivedData))
		}()
		wg.Wait()
	})
}
