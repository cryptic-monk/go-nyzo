/*
A message to/from a peer.
*/
package messages

import (
	"crypto/ed25519"
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"net"
	"strings"
	"time"
)

const (
	ConnectionTimeout = time.Second * 5
	ReadTimeout       = time.Second * 5
	WriteTimeout      = time.Second * 5
)

// Any object that can be serialized to and from a byte stream
type Serializable interface {
	GetSerializedLength() int            // how many bytes will ToBytes return?
	ToBytes() []byte                     // convert to bytes, will return GetSerializedLength() of zeroes at worst
	FromBytes(bytes []byte) (int, error) // convert from bytes, returns consumed bytes and an error if the input data is erroneous
}

type Message struct {
	Timestamp    int64 // millisecond precision -- when the message is first generated
	Type         int16
	Content      Serializable  // tbd
	SourceId     []byte        // the public key of the node that created this message
	Signature    []byte        // the signature of all preceding parts
	SourceIP     []byte        // not serialized, source IP in byte format
	ReplyChannel chan *Message // channel to receive an internal reply to this message
}

// Create a new message, local variant (the ones we can sign).
func NewLocal(messageType int16, messageContent Serializable, identity *identity.Identity) *Message {
	message := Message{}
	message.Timestamp = time.Now().UnixNano() / 1000000
	message.Type = messageType
	message.Content = messageContent
	message.SourceId = identity.PublicKey
	message.Signature = identity.Sign(message.SerializeForSigning())
	return &message
}

// Construct a new message from a byte slice
// Format + byte length:
//   Message Length  4
//   Timestamp       8
//   Type            2
//   Content        <variable>
//   Identifier     32
//   Signature      64
func NewFromBytes(bytes []byte, sourceAddress string) (*Message, error) {
	message := Message{}
	position := 0
	nonContentLength := message_fields.SizeMessageLength + message_fields.SizeTimestamp + message_fields.SizeMessageType + message_fields.SizeNodeIdentifier + message_fields.SizeSignature
	contentLength := int(message_fields.DeserializeInt32(bytes[position:position+message_fields.SizeMessageLength])) - nonContentLength
	position += message_fields.SizeMessageLength
	message.Timestamp = message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeTimestamp])
	position += message_fields.SizeTimestamp
	message.Type = message_fields.DeserializeInt16(bytes[position : position+message_fields.SizeMessageType])
	position += message_fields.SizeMessageType
	if contentLength > 0 {
		switch message.Type {
		case TypeBlockRequest:
			message.Content = &message_content.BlockRequest{}
		case TypeBlockResponse:
			message.Content = &message_content.BlockResponse{}
		case TypeMeshResponse:
			message.Content = &message_content.MeshResponse{}
		case TypeStatusResponse:
			message.Content = &message_content.StatusResponse{}
		case TypeMissingBlockVoteRequest:
			message.Content = &message_content.MissingBlockVoteRequest{}
		case TypeMissingBlockRequest:
			message.Content = &message_content.MissingBlockRequest{}
		case TypeBootstrapRequest:
			message.Content = &message_content.BootstrapRequest{}
		case TypeBootstrapResponse:
			message.Content = &message_content.BootstrapResponse{}
		case TypeNodeJoin:
			message.Content = &message_content.NodeJoin{}
		case TypeNodeJoinResponse:
			message.Content = &message_content.NodeJoinResponse{}
		case TypeBlockWithVotesResponse:
			message.Content = &message_content.BlockWithVotesResponse{}
		case TypeIpAddressResponse:
			message.Content = &message_content.IpAddress{}
		case TypeWhitelistResponse:
			message.Content = &message_content.BooleanResponse{}
		default:
			message.Content = &message_content.Default{}
		}
		content := bytes[position : position+contentLength]
		position += contentLength
		_, err := message.Content.FromBytes(content)
		if err != nil {
			return nil, err
		}
	}
	message.SourceId = bytes[position : position+message_fields.SizeNodeIdentifier]
	position += message_fields.SizeNodeIdentifier
	if message_fields.AllZeroes(message.SourceId) {
		return nil, errors.New("cannot convert incoming message, source ID is all zeroes")
	}
	message.Signature = bytes[position : position+message_fields.SizeSignature]
	// TODO: like the Java version, we re-serialize here to check the signature. This is very costly (but safer).
	if !ed25519.Verify(message.SourceId, message.SerializeForSigning(), message.Signature) {
		return nil, errors.New("message signature invalid, source address: " + sourceAddress)
	}
	split := strings.Split(sourceAddress, ":")
	parsed := net.ParseIP(split[0])
	if parsed == nil {
		return nil, errors.New("cannot convert incoming message, source IP cannot be parsed")
	}
	message.SourceIP = parsed[len(parsed)-4:]
	if message_fields.AllZeroes(message.SourceIP) || utilities.IsPrivateIP(parsed) {
		return nil, errors.New("cannot convert incoming message, source IP is zero or from a private range")
	}
	return &message, nil
}

// Serialize for signing.
// Format + byte length:
//   Timestamp       8
//   Type            2
//   Content        <variable>
//   Identifier     32
func (m *Message) SerializeForSigning() []byte {
	var serialized []byte

	serialized = append(serialized, message_fields.SerializeInt64(m.Timestamp)...)
	serialized = append(serialized, message_fields.SerializeInt16(m.Type)...)
	if m.Content != nil {
		serialized = append(serialized, m.Content.ToBytes()...)
	}
	serialized = append(serialized, m.SourceId...)
	return serialized
}

// Serialize for transmission.
// Format + byte length:
//   Message Length  4
//   Timestamp       8
//   Type            2
//   Content        <variable>
//   Identifier     32
//   Signature      64
func (m *Message) SerializeForTransmission() []byte {
	var serialized []byte

	length := message_fields.SizeMessageLength + message_fields.SizeTimestamp + message_fields.SizeMessageType + message_fields.SizeNodeIdentifier + message_fields.SizeSignature
	if m.Content != nil {
		length += m.Content.GetSerializedLength()
	}

	serialized = append(serialized, message_fields.SerializeInt32(int32(length))...)
	serialized = append(serialized, message_fields.SerializeInt64(m.Timestamp)...)
	serialized = append(serialized, message_fields.SerializeInt16(m.Type)...)
	if m.Content != nil {
		serialized = append(serialized, m.Content.ToBytes()...)
	}
	serialized = append(serialized, m.SourceId...)
	serialized = append(serialized, m.Signature...)

	return serialized
}

// Verify this messages's signature and return whether it's valid.
func (m *Message) SignatureIsValid() bool {
	return ed25519.Verify(m.SourceId, m.SerializeForSigning(), m.Signature)
}
