/*
An individual cycle transaction signature (V1 blockchain).
*/
package blockchain_data

type CycleSignature struct {
	Id        []byte // node ID
	Signature []byte // the node's signature
}
