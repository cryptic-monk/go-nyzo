package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/node"
)

const (
	meshResponseMaxNodes = 10000 // max number of nodes that we'll accept in a mesh response
)

type MeshResponse struct {
	Nodes []*node.Node
}

// Create a new mesh response message with the given nodes.
func NewMeshResponse(nodes []*node.Node) *MeshResponse {
	return &MeshResponse{nodes}
}

// Serializable interface: data length when serialized.
func (c *MeshResponse) GetSerializedLength() int {
	var newNode node.Node
	nodeDataLength := newNode.GetSerializedLength()
	return message_fields.SizeNodeListLength + (len(c.Nodes) * nodeDataLength)
}

// Serializable interface: write notes to data bytes.
func (c *MeshResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt32(int32(len(c.Nodes)))...)
	for _, n := range c.Nodes {
		serialized = append(serialized, n.ToBytes()...)
	}
	return serialized
}

// Serializable interface: read nodes from data bytes.
func (c *MeshResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < 4 {
		return 0, errors.New("invalid mesh response content")
	}
	nodeCount := int(message_fields.DeserializeInt32(bytes[0:4]))
	if nodeCount > meshResponseMaxNodes {
		nodeCount = meshResponseMaxNodes
	}
	var newNode *node.Node
	// nil call works here because GetSerializedLength doesn't user the pointer. TODO: this is nasty
	nodeDataLength := newNode.GetSerializedLength()
	// hmm, some mesh responses contain two unsigned 'bonus' nodes, or some other data of this size at the end
	// of the supposed node list, well well, we'll just ignore that data for now
	consumed := 4
	for count, position := 0, 4; position <= len(bytes)+nodeDataLength && count < nodeCount; position += nodeDataLength {
		newNode, err := node.NewNodeFromBytes(bytes[position : position+nodeDataLength])
		if err != nil {
			return 0, err
		}
		c.Nodes = append(c.Nodes, newNode)
		consumed += nodeDataLength
		count++
	}
	return consumed, nil
}
