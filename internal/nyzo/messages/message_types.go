package messages

const (
	TypeNodeJoinLegacy           int16 = 3
	TypeNodeJoinResponseLegacy   int16 = 4
	TypeBlockRequest             int16 = 11
	TypeBlockResponse            int16 = 12
	TypeMeshRequest              int16 = 15 // request node-information for in-cycle nodes
	TypeMeshResponse             int16 = 16 // a (capped) list of in-cycle nodes
	TypeStatusRequest            int16 = 17
	TypeStatusResponse           int16 = 18
	TypeBlockVote                int16 = 19
	TypeMissingBlockVoteRequest  int16 = 23
	TypeMissingBlockVoteResponse int16 = 24
	TypeMissingBlockRequest      int16 = 25
	TypeMissingBlockResponse     int16 = 26
	TypeBootstrapRequest         int16 = 35 // request "starter" information about the frozen edge and the cycle
	TypeBootstrapResponse        int16 = 36 // frozen edge height, frozen edge block hash, a list of all in-cycle verifier IDs
	TypeBlockWithVotesRequest    int16 = 37
	TypeBlockWithVotesResponse   int16 = 38
	TypeFullMeshRequest          int16 = 41 // request node-information for all nodes
	TypeFullMeshResponse         int16 = 42 // a (capped) list of all known nodes
	TypeNodeJoin                 int16 = 43
	TypeNodeJoinResponse         int16 = 44
	TypeIpAddressRequest         int16 = 53
	TypeIpAddressResponse        int16 = 54
	TypeWhitelistRequest         int16 = 424
	TypeWhitelistResponse        int16 = 425
)
