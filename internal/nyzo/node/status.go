/*
Used to track a node's status as encapsulated in Java's StatusResponse message content class.
Informational purposes only, so some info duplication is OK here.
*/
package node

import (
	"strconv"
	"strings"
)

type Status struct {
	Identifier        []byte
	Timestamp         int64
	Ip                []byte
	Nickname          string
	Version           int
	MeshTotal         int
	MeshCycle         int
	CycleLength       int
	Transactions      int
	RetentionEdge     int64
	TrailingEdge      int64
	FrozenEdge        int64
	OpenEdge          int64
	BlocksTransmitted int64
	BlocksCreated     int64
	VoteStats         string
	LastJoinHeight    int64
	LastRemovalHeight int64
	ReceivingUDP      bool
}

// Convert Java's nasty status response format to something more palpable.
func StatusFromStatusResponse(identifier, ip []byte, timestamp int64, lines []string) *Status {
	status := &Status{}
	status.Identifier = identifier
	status.Ip = ip
	status.Timestamp = timestamp
	for _, s := range lines {
		if strings.HasPrefix(s, "nickname: ") {
			status.Nickname = s[len("nickname: "):]
		} else if strings.HasPrefix(s, "version: ") {
			status.Version, _ = strconv.Atoi(s[len("version: "):])
		} else if strings.HasPrefix(s, "mesh: ") {
			s = s[len("mesh: "):]
			if strings.HasSuffix(s, " in cycle") {
				s = s[:len(s)-len(" in cycle")]
			}
			result := strings.Split(s, " total, ")
			if len(result) >= 2 {
				status.MeshTotal, _ = strconv.Atoi(result[0])
				status.MeshCycle, _ = strconv.Atoi(result[1])
			}
		} else if strings.HasPrefix(s, "cycle length: ") {
			s = s[len("cycle length: "):]
			if strings.HasSuffix(s, "(G)") {
				s = s[:len(s)-3]
			}
			status.CycleLength, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(s, "transactions: ") {
			status.Transactions, _ = strconv.Atoi(s[len("transactions: "):])
		} else if strings.HasPrefix(s, "retention edge: ") {
			status.RetentionEdge, _ = strconv.ParseInt(s[len("retention edge: "):], 10, 64)
		} else if strings.HasPrefix(s, "trailing edge: ") {
			status.TrailingEdge, _ = strconv.ParseInt(s[len("trailing edge: "):], 10, 64)
		} else if strings.HasPrefix(s, "open edge: ") {
			status.OpenEdge, _ = strconv.ParseInt(s[len("open edge: "):], 10, 64)
		} else if strings.HasPrefix(s, "frozen edge: ") {
			s = s[len("frozen edge: "):]
			result := strings.Split(s, " (")
			status.FrozenEdge, _ = strconv.ParseInt(result[0], 10, 64)
		} else if strings.HasPrefix(s, "blocks transmitted/created: ") {
			s = s[len("blocks transmitted/created: "):]
			result := strings.Split(s, "/")
			if len(result) >= 2 {
				status.BlocksTransmitted, _ = strconv.ParseInt(result[0], 10, 64)
				status.BlocksCreated, _ = strconv.ParseInt(result[1], 10, 64)
			}
		} else if strings.HasPrefix(s, "block vote: ") {
			status.VoteStats = s[len("block vote: "):]
		} else if strings.HasPrefix(s, "last join height: ") {
			status.LastJoinHeight, _ = strconv.ParseInt(s[len("last join height: "):], 10, 64)
		} else if strings.HasPrefix(s, "last removal height: ") {
			status.LastRemovalHeight, _ = strconv.ParseInt(s[len("last removal height: "):], 10, 64)
		} else if strings.HasPrefix(s, "receiving UDP: ") && strings.HasSuffix(s, "yes") {
			status.ReceivingUDP = true
		}
	}
	return status
}
