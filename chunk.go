package network

type Chunk struct {
	ID   uint64
	Seq  uint64
	End  bool
	Data []byte
}

type WritePacketCallback func(w WritePacket, err error)

type WritePacket interface {
	SetData(data interface{})
	GetData() interface{}
	SetCallback(cb WritePacketCallback)
	GetCallback() WritePacketCallback
}
