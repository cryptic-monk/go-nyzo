package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/node"
	"io"
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
func (c *MeshResponse) Read(r io.Reader) error {
	nodeCount, err := message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	if nodeCount > meshResponseMaxNodes {
		nodeCount = meshResponseMaxNodes
	}
	// hmm, some mesh responses contain two unsigned 'bonus' nodes, or some other data of this size at the end
	// of the supposed node list, well well, we'll just ignore that data for now
	for count := 0; count < int(nodeCount); count++ {
		newNode, err := node.ReadNewNode(r)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, newNode)
	}
	return nil
}
