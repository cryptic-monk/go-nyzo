/*
A node in the mesh, this is how we keep track of an individual peer. Due to a circular import SNAFU,
this isn't part of the 'networking' package.
*/
package node

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
	"time"
)

type Node struct {
	Identifier     []byte // wallet public key (32 bytes)
	IpAddress      []byte // IPv4 address, stored as bytes to keep memory predictable (4 bytes)
	IpString       string // IP copy in string format for convenience
	PortTcp        int32  // TCP port number
	PortUdp        int32  // UDP port number, if available
	QueueTimestamp int64  // this is the timestamp that determines queue placement -- it is when the verifier joined the mesh or when the verifier was last updated
	Nickname       string // we keep those here too, no need to have a separate nickname manager IMO

	IdentifierChangeTimestamp int64 // when the identifier at this IP was last changed
	InactiveTimestamp         int64 // when the verifier was marked as inactive; -1 for active verifiers
	LastNodeJoinBlock         int64 // number of the last frozen edge block when we sent this node a node join message, set to 0 if it should receive a message

	FailedConnectionCount int // number of failed connection attempts to this node
}

// Create a new node entry
func NewNode(id, ip []byte, portTcp, portUdp int32) *Node {
	n := new(Node)
	n.Identifier = id
	n.IpAddress = ip
	n.IpString = message_fields.IP4BytesToString(ip)
	n.PortTcp = portTcp
	n.PortUdp = portUdp
	n.QueueTimestamp = time.Now().UnixNano() / 1000000
	n.IdentifierChangeTimestamp = time.Now().UnixNano() / 1000000
	n.InactiveTimestamp = -1
	n.FailedConnectionCount = 0
	return n
}

// Convenience to create a new node from bytes
func ReadNewNode(r io.Reader) (*Node, error) {
	newNode := new(Node)
	err := newNode.Read(r)
	if err != nil {
		return nil, err
	} else {
		return newNode, nil
	}
}

// Serializable Interface: How many bytes will ToBytes return?
func (n *Node) GetSerializedLength() int {
	// UDP port is not serialized
	return message_fields.SizeNodeIdentifier + message_fields.SizeIPAddress + message_fields.SizePort + message_fields.SizeTimestamp
}

// Serializable Interface: Dump node info to bytes.
func (n *Node) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, n.Identifier...)
	serialized = append(serialized, n.IpAddress...)
	serialized = append(serialized, message_fields.SerializeInt32(n.PortTcp)...)
	serialized = append(serialized, message_fields.SerializeInt64(n.QueueTimestamp)...)
	// something went wrong, better bail us out with a zeroed out message
	if len(serialized) != n.GetSerializedLength() {
		serialized = make([]byte, n.GetSerializedLength())
	}
	return serialized
}

// Serializable Interface: Construct node info from bytes.
func (n *Node) Read(r io.Reader) error {
	var err error
	n.Identifier, err = message_fields.ReadNodeId(r)
	if err != nil {
		return err
	}
	if message_fields.AllZeroes(n.Identifier) {
		return errors.New("cannot deserialize node info: source ID is all zeroes")
	}
	n.IpAddress, err = message_fields.ReadBytes(r, message_fields.SizeIPAddress)
	if err != nil {
		return err
	}
	n.PortTcp, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	n.PortUdp = -1
	n.QueueTimestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	n.IdentifierChangeTimestamp = time.Now().UnixNano() / 1000000
	n.InactiveTimestamp = -1
	n.FailedConnectionCount = 0
	n.IpString = message_fields.IP4BytesToString(n.IpAddress)
	return nil
}
