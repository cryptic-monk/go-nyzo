/*
Internal messages used for data sharing between components, and locally, to avoid race conditions with data access.
Local message numbers should start at 1000, message numbers internal to a single package should be negative.
*/
package messages

type InternalMessage struct {
	Type         int16
	Payload      []interface{}
	ReplyChannel chan *InternalMessage
}

func NewInternalMessage(messageType int16, a ...interface{}) *InternalMessage {
	message := &InternalMessage{}
	message.Type = messageType
	message.Payload = a
	return message
}
