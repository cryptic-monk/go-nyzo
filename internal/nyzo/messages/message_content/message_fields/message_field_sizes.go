/*
Byte level message field sizes
*/
package message_fields

const (
	SizeNodeIdentifier         = 32
	SizeSignature              = 64
	SizeIPAddress              = 4
	SizePort                   = 4
	SizeTimestamp              = 8
	SizeNodeListLength         = 4
	SizeMessageLength          = 4
	SizeMessageType            = 2
	SizeStringLength           = 2
	SizeVoteListLength         = 1
	SizeHash                   = 32
	SizeSeed                   = 32
	SizeBlockHeight            = 8
	SizeTransactionType        = 1
	SizeTransactionAmount      = 8
	SizeBlocksUntilFee         = 2
	SizeRolloverTransactionFee = 1
	SizeBalanceListLength      = 4
	SizeUnnamedByte            = 1
	SizeUnnamedInt16           = 2
	SizeUnnamedInt32           = 4
	SizeUnnamedInt64           = 8
	SizeShlong                 = 8
	SizeFrozenBlockListLength  = 2
	SizeBool                   = 1
)
