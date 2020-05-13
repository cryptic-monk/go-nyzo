package messages

const (
	TypeInternalConnectionFailure  int16 = 1   // payload[0] is an IP as a string, payload[1] is the message type as int16
	TypeInternalConnectionSuccess  int16 = 2   // payload[0] is an IP as a string, payload[1] is the message type as int16
	TypeInternalNewFrozenEdgeBlock int16 = 3   // payload[0] * to the block that has been frozen, payload[1] nil, or * to the block's balance list (during bootstrapping)
	TypeInternalNewRetentionEdge   int16 = 4   // payload[0] is the new retention edge as int64
	TypeInternalNewTrailingEdge    int16 = 5   // payload[0] is the new trailing edge as int64
	TypeInternalSendToRandomNode   int16 = 6   // payload[0] is the message to send to a random node
	TypeInternalBootstrapBlock     int16 = 7   // payload[0] is the block height, payload[1] is the hash of this block's balance list
	TypeInternalChainInitialized   int16 = 8   // no payload
	TypeInternalDataStoreHeight    int16 = 9   // payload[0] is the highest height stored in the data store, used to bootstrap archive mode
	TypeInternalTransaction        int16 = 10  // transaction DB storage event, payload: see balance_authority, emitTransaction
	TypeInternalCycleEvent         int16 = 11  // cycle DB storage event, payload: see block_file_handler, emitCycleEvents
	TypeInternalBlock              int16 = 12  // block DB storage event, payload: see block_file_handler, emitBlock
	TypeInternalNodeStatus         int16 = 13  // node status DB storage event, payload: see node.Status
	TypeInternalExiting            int16 = 200 // exit program gracefully
)
