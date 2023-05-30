package network

type Chunk struct {
	ID   uint64 `msgpack:"id"`
	Seq  uint64 `msgpack:"seq"`
	End  bool   `msgpack:"end"`
	Data []byte `msgpack:"data"`
}

type WritePacketCallback func(w WritePacket, err error)

type WritePacket interface {
	SetData(data interface{})
	GetData() interface{}
	SetCallback(cb WritePacketCallback)
	GetCallback() WritePacketCallback
}

type writePacket struct {
	data interface{}
	cb   WritePacketCallback
}

func (w *writePacket) SetData(data interface{}) {
	w.data = data
}

func (w *writePacket) GetData() interface{} {
	return w.data
}

func (w *writePacket) SetCallback(cb WritePacketCallback) {
	w.cb = cb
}

func (w *writePacket) GetCallback() WritePacketCallback {
	return w.cb
}
