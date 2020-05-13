package message_content

// default message content if we don't know how to serialize/deserialize it
type Default struct {
	Content []byte
}

// Serializable interface: data length when serialized.
func (c *Default) GetSerializedLength() int {
	return len(c.Content)
}

// Serializable interface: to data bytes.
func (c *Default) ToBytes() []byte {
	return c.Content
}

// Serializable interface: from data bytes.
func (c *Default) FromBytes(bytes []byte) (int, error) {
	c.Content = bytes
	return len(bytes), nil
}
