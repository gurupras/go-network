package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWritePacket(t *testing.T) {
	data := "test_data"
	var cb WritePacketCallback = func(w WritePacket, err error) {}

	wp := &writePacket{
		data: data,
		cb:   cb,
	}

	assert.Equal(t, data, wp.GetData())
	// Note: Comparing function pointers is not reliable, so we just check if it's not nil.
	assert.NotNil(t, wp.GetCallback())

	newData := "new_test_data"
	wp.SetData(newData)
	assert.Equal(t, newData, wp.GetData())

	var newCb WritePacketCallback = func(w WritePacket, err error) {}
	wp.SetCallback(newCb)
	assert.NotNil(t, wp.GetCallback())
}
