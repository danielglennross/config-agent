package broadcast

import (
	"bytes"
	"encoding/binary"
)

// PackWriterMessage packs msg ready for redis
func PackWriterMessage(msg *DeliverableTrackedMessage) ([]byte, error) {
	var b [256]byte
	copy(b[:], msg.Data)

	var c [16]byte
	copy(c[:], msg.CorrelationToken)

	data := SerializableRedisMessage{
		CorrelationToken: c,
		Data:             b,
	}

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetReceiverMessage gets msg from redis
func GetReceiverMessage(data []byte, channel string) (*TrackedMessage, error) {
	t := SerializableRedisMessage{}
	err := binary.Read(bytes.NewBuffer(data), binary.BigEndian, &t)
	if err != nil {
		return nil, err
	}
	return &TrackedMessage{
		CorrelationToken: string(clean(t.CorrelationToken[:])),
		Message: Message{
			Channel: channel,
			Data:    clean(t.Data[:]),
		},
	}, nil
}

func clean(b []byte) []byte {
	return bytes.Trim(b[:], "\x00")
}
