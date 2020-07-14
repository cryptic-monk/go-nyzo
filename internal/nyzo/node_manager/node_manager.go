/*
The node manager keeps track of all our peers's behavior.

In all modes but the archive mode, the node manager starts the bootstrapping process by contacting the mesh via
trusted entry points or managed verifiers.
*/
package node_manager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/node"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	minimumMeshRequestInterval       = 30 // don't send node join messages to a particular node more frequently than every ... blocks
	watchOnlyMeshRequestInterval     = 50 // ca. every 5 minutes
	nodesFilePermissions             = 0644
	consecutiveFailuresBeforeRemoval = 6 // mark a node as inactive after ... consecutive failed connection attempts
	whitelistUpdateInterval          = configuration.DynamicWhitelistInterval / 2
)

type state struct {
	ctxt                       *interfaces.Context
	nodeMap                    map[string]*node.Node           // there can only ever be one node on one particular ip, hence we map nodes by IP
	nodeJoinCache              []*node.Node                    // a cache for nodes that merit a join message (used for performance reasons)
	statusRequestCache         []*node.Node                    // a cache for node status requests (used in archive mode only)
	trustedEntryPoints         []*networking.TrustedEntryPoint // a list of trusted entry points into the mesh
	managedVerifiers           []*networking.ManagedVerifier   // a list of managed verifiers
	messageChannel             chan *messages.Message          // here's where we'll receive the messages we are registering for
	internalMessageChannel     chan *messages.InternalMessage  // channel for internal and local messages
	haveNodeHistory            bool                            // are we confident about our node history?
	meshRequestWait            int64                           // waiting ... blocks until we do the next mesh request to ensure tight mesh integration
	bootstrapPhase             bool
	meshRequestSuccessCount    int          // first two during normal operation will be cycle only requests, then we ask for the full mesh
	activeNodeCount            int          // how many nodes are currently active in the overall mesh?
	inCycleNodes               []*node.Node // a list of all in cycle nodes
	missingIncycleNodesMessage string       // a string message with a list of missing in cycle nodes
	randomNodePosition         int          // instead of finding "random" nodes, we just loop through our node list, I think this should work equally well
	frozenEdgeHeight           int64
	lastWhitelistUpdate        int64
	chainInitialized           bool
}

// Are we at all accepting messages from the given peer? Used to quickly terminate unwanted connections.
// Example: an IP could be blacklisted.
func (s *state) AcceptingMessagesFrom(ip string) bool {
	// TODO: needs to be secured for concurrency if exported.
	return true
}

// Are we accepting the given message type from the given peer? Used to quickly terminate unwanted connections.
// Wrong message types will count as violations, correct message types will go towards the credits this node has.
// Example: an out of cycle verifier sends us an in cycle message.
func (s *state) AcceptingMessageTypeFrom(ip string, messageType int16) bool {
	// TODO: needs to be secured for concurrency if exported.
	return true
}

// Get a list of trusted entry points.
// List is produced once during startup, so there should be no concurrency issues.
func (s *state) GetTrustedEntryPoints() []*networking.TrustedEntryPoint {
	return s.trustedEntryPoints
}

// Get a list of managed verifiers.
// List is produced once during startup, so there are no concurrency issues.
func (s *state) GetManagedVerifiers() []*networking.ManagedVerifier {
	return s.managedVerifiers
}

// Load trusted entry points from configuration file
func (s *state) loadTrustedEntryPoints() error {
	s.trustedEntryPoints = make([]*networking.TrustedEntryPoint, 0)
	fileName := configuration.DataDirectory + "/" + configuration.EntryPointFileName
	f, err := os.Open(fileName)
	if err != nil {
		message := fmt.Sprintf("cannot load trusted entry points: %s", err.Error())
		return errors.New(message)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := fmt.Sprintln(scanner.Text())
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)
		entryPoint := networking.TrustedEntryPointFromString(line)
		if entryPoint != nil {
			s.trustedEntryPoints = append(s.trustedEntryPoints, entryPoint)
		}
	}
	if err := scanner.Err(); err != nil {
		message := fmt.Sprintf("cannot load trusted entry points: %s", err.Error())
		return errors.New(message)
	}
	if len(s.trustedEntryPoints) == 0 {
		return errors.New("no trusted entry points found, cannot connect to mesh")
	}
	return nil
}

// Send node joins to every node known by now.
func (s *state) sendInitialNodeJoins() {
	nodeJoinContent := message_content.NewNodeJoin(configuration.ListeningPortTcp, configuration.ListeningPortUdp, s.ctxt.Identity.Nickname)
	messageNodeJoin := messages.NewLocal(messages.TypeNodeJoin, nodeJoinContent, s.ctxt.Identity)
	for _, n := range s.nodeJoinCache {
		s.nodeMap[n.IpString].LastNodeJoinBlock = s.frozenEdgeHeight
		go networking.FetchTcp(messageNodeJoin, n.IpAddress, n.PortTcp)
	}
	s.nodeJoinCache = make([]*node.Node, 0)
}

// Get initial mesh info, either via managed verifiers (Sentinel), or via TEPs (all other run modes).
// For Verifier run mode, send node joins to TEPs.
func (s *state) bootstrapMesh() {
	// Fetch contact info for the mesh.
	messageMeshRequest := messages.NewLocal(messages.TypeMeshRequest, nil, s.ctxt.Identity)
	if s.ctxt.RunMode() == interfaces.RunModeSentinel || s.ctxt.RunMode() == interfaces.RunModeArchive {
		for _, verifier := range s.managedVerifiers {
			go networking.FetchTcpNamed(messageMeshRequest, verifier.Host, verifier.Port)
		}
	} else {
		for _, ep := range s.trustedEntryPoints {
			go networking.FetchTcpNamed(messageMeshRequest, ep.Host, ep.Port)
		}
	}

	// Only verifiers want to join the mesh. Send node joins to every TEP.
	if s.ctxt.RunMode() == interfaces.RunModeVerifier {
		nodeJoinContent := message_content.NewNodeJoin(configuration.ListeningPortTcp, configuration.ListeningPortUdp, s.ctxt.Identity.Nickname)
		messageNodeJoin := messages.NewLocal(messages.TypeNodeJoin, nodeJoinContent, s.ctxt.Identity)
		for _, ep := range s.trustedEntryPoints {
			go networking.FetchTcpNamed(messageNodeJoin, ep.Host, ep.Port)
		}
	}
}

// Mesh maintenance.
// Sentinel: ask all managed verifiers for mesh info. Others: ask a random node.
// Verifier: enqueue node joins to all known nodes.
func (s *state) maintainMesh() {
	// send a mesh request to a random node.
	var meshRequest *messages.Message
	// Early on, or in archive and sentinel mode, we don't care about the full mesh.
	if s.meshRequestSuccessCount < 2 || s.ctxt.RunMode() == interfaces.RunModeArchive || s.ctxt.RunMode() == interfaces.RunModeSentinel {
		meshRequest = messages.NewLocal(messages.TypeMeshRequest, nil, s.ctxt.Identity)
	} else {
		meshRequest = messages.NewLocal(messages.TypeFullMeshRequest, nil, s.ctxt.Identity)
	}
	if s.ctxt.RunMode() == interfaces.RunModeSentinel {
		for _, verifier := range s.managedVerifiers {
			go networking.FetchTcpNamed(meshRequest, verifier.Host, verifier.Port)
		}
	} else {
		n := s.findRandomCycleNode()
		if n == nil {
			return
		}
		go networking.FetchTcp(meshRequest, n.IpAddress, n.PortTcp)
	}

	if s.ctxt.RunMode() == interfaces.RunModeVerifier {
		// Only verifiers want to join the mesh. Enqueue node joins for every node to keep the mesh tightly integrated.
		var nodes []*node.Node
		if s.meshRequestSuccessCount < 2 {
			nodes = s.getCycle()
		} else {
			nodes = s.getFullMesh()
		}
		for _, n := range nodes {
			s.nodeJoinCache = append(s.nodeJoinCache, n)
		}
	} else if s.ctxt.RunMode() == interfaces.RunModeArchive {
		// Track in-cycle verifier status in archive mode.
		nodes := s.getCycle()
		for _, n := range nodes {
			s.statusRequestCache = append(s.statusRequestCache, n)
		}
	}
}

// Send 10 (maintenance) node join messages from the queue. Regular run mode only.
func (s *state) send10NodeJoins() {
	target := 10
	count := 0
	nodeJoinContent := message_content.NewNodeJoin(configuration.ListeningPortTcp, configuration.ListeningPortUdp, s.ctxt.Identity.Nickname)
	messageNodeJoin := messages.NewLocal(messages.TypeNodeJoin, nodeJoinContent, s.ctxt.Identity)
	for _, n := range s.nodeJoinCache {
		// silently drop nodes that got a recent request from us
		if s.frozenEdgeHeight-n.LastNodeJoinBlock > minimumMeshRequestInterval {
			n.LastNodeJoinBlock = s.frozenEdgeHeight
			// we don't add this one to the wait group, if it dies, it dies
			go networking.FetchTcp(messageNodeJoin, n.IpAddress, n.PortTcp)
		} else {
			target++
		}
		count++
		if count > target {
			break
		}
	}
	if len(s.nodeJoinCache) <= count-1 {
		s.nodeJoinCache = make([]*node.Node, 0)
	} else {
		s.nodeJoinCache = s.nodeJoinCache[count-1:]
	}
}

// Send 10 status requests from the queue. Archive mode only.
func (s *state) send10StatusRequests() {
	messageStatusRequest := messages.NewLocal(messages.TypeStatusRequest, nil, s.ctxt.Identity)
	count := 0
	for _, n := range s.statusRequestCache {
		go networking.FetchTcp(messageStatusRequest, n.IpAddress, n.PortTcp)
		count++
		if count > 10 {
			break
		}
	}
	if len(s.statusRequestCache) <= count-1 {
		s.statusRequestCache = make([]*node.Node, 0)
	} else {
		s.statusRequestCache = s.statusRequestCache[count-1:]
	}
}

// Do some maintenance on mesh data, remove old nodes and update the active cycle ip list, as well as the missing cycle nodes message.
func (s *state) maintainMeshData() {
	cycleLength := int64(s.ctxt.CycleAuthority.GetCurrentCycleLength())
	if cycleLength == 0 {
		cycleLength = 10
	}
	// after this time, we throw out a node from the map
	thresholdTimestamp := utilities.Now() - configuration.BlockDuration*cycleLength*2
	s.activeNodeCount = 0
	s.inCycleNodes = make([]*node.Node, 0)
	separator := ""
	missingMessage := ""
	for ip, n := range s.nodeMap {
		if n.InactiveTimestamp == -1 {
			s.activeNodeCount++
			if s.ctxt.CycleAuthority.VerifierInCurrentCycle(n.Identifier) {
				s.inCycleNodes = append(s.inCycleNodes, n)
			} else if s.ctxt.RunMode() == interfaces.RunModeArchive {
				// in archive mode, we keep track of in-cycle verifiers only
				// we also don't have to be extremely precise about it, as we basically just use them
				// as a peer pool, so we accept some churn during a cycle
				delete(s.nodeMap, ip)
			}
		} else if n.InactiveTimestamp < thresholdTimestamp {
			delete(s.nodeMap, ip)
			logging.TraceLog.Printf("Removed %s from mesh on verifier %s.", n.Nickname, s.ctxt.Identity.Nickname)
		} else if s.ctxt.CycleAuthority.VerifierInCurrentCycle(n.Identifier) {
			missingMessage += separator + n.Nickname
			separator = ","
		}
	}
	if len(missingMessage) == 0 {
		s.missingIncycleNodesMessage = "*** no verifiers missing ***"
	} else {
		s.missingIncycleNodesMessage = missingMessage
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(s.inCycleNodes), func(i, j int) {
		s.inCycleNodes[i], s.inCycleNodes[j] = s.inCycleNodes[j], s.inCycleNodes[i]
	})
}

// Process the response from a mesh request.
func (s *state) processMeshResponse(message *messages.Message) {
	meshResponseContent := *message.Content.(*message_content.MeshResponse)
	if s.ctxt.RunMode() == interfaces.RunModeVerifier {
		initialSize := len(s.nodeMap)
		for _, n := range meshResponseContent.Nodes {
			existingNode, ok := s.nodeMap[n.IpString]
			if ok {
				// only append if there's a chance we'll actually send this request (efficiency)
				if s.frozenEdgeHeight-existingNode.LastNodeJoinBlock > minimumMeshRequestInterval {
					existingNode.LastNodeJoinBlock = 0
					s.nodeJoinCache = append(s.nodeJoinCache, existingNode)
				}
			} else {
				if !message_fields.AllZeroes(n.IpAddress) && !utilities.IsPrivateIP(net.IPv4(n.IpAddress[0], n.IpAddress[1], n.IpAddress[2], n.IpAddress[3])) {
					s.nodeMap[n.IpString] = n
					s.nodeJoinCache = append(s.nodeJoinCache, n)
				}
			}
		}
		finalSize := len(s.nodeMap)
		logging.TraceLog.Printf("Processed mesh response, learned about %d new nodes, %d total nodes in list now.", finalSize-initialSize, finalSize)
	} else {
		for _, n := range meshResponseContent.Nodes {
			s.nodeMap[n.IpString] = n
		}
		logging.TraceLog.Printf("Processed mesh response, we know %d nodes.", len(s.nodeMap))
	}
	if len(s.nodeMap) > 0 {
		s.meshRequestSuccessCount++
	}
	// after bootstrapping, maintain the mesh again after...
	if s.ctxt.RunMode() == interfaces.RunModeArchive || s.ctxt.RunMode() == interfaces.RunModeSentinel {
		s.meshRequestWait = watchOnlyMeshRequestInterval
	} else {
		s.meshRequestWait = minimumMeshRequestInterval
		if s.ctxt.CycleAuthority.GetCurrentCycleLength() > minimumMeshRequestInterval {
			s.meshRequestWait = int64(s.ctxt.CycleAuthority.GetCurrentCycleLength())
		}
	}
}

// Process a status response.
func (s *state) processStatusResponse(message *messages.Message) {
	if s.ctxt.RunMode() != interfaces.RunModeArchive {
		// we should only get those in archive mode, but don't want to trip up if somebody sends us an unsolicited one
		return
	}
	content := *message.Content.(*message_content.StatusResponse)
	status := node.StatusFromStatusResponse(message.SourceId, message.SourceIP, message.Timestamp, content.Lines)
	internalMessage := messages.NewInternalMessage(messages.TypeInternalNodeStatus, status)
	router.Router.RouteInternal(internalMessage)
}

// Process a node join message. Regular run mode only.
func (s *state) processNodeJoinMessage(message *messages.Message) {
	nodeJoinContent := *message.Content.(*message_content.NodeJoin)
	s.updateNode(message.SourceId, message.SourceIP, nodeJoinContent.PortTcp, nodeJoinContent.PortUdp, nodeJoinContent.Nickname)
	// send join reply
	if message.ReplyChannel != nil {
		responseContent := message_content.NewNodeJoinResponse(s.ctxt.Identity.Nickname, configuration.ListeningPortTcp, configuration.ListeningPortUdp)
		response := messages.NewLocal(messages.TypeNodeJoinResponse, responseContent, s.ctxt.Identity)
		message.ReplyChannel <- response
	}
	// Here, Java would also send a node join message to new nodes. We just let this slide and do it as part of the main loop
	logging.TraceLog.Printf("Processed node join message from %s.", nodeJoinContent.Nickname)
}

// Process a legacy node join message. Regular run mode only.
func (s *state) processNodeJoinMessageLegacy(message *messages.Message) {
	nodeJoinContent := *message.Content.(*message_content.NodeJoinLegacy)
	s.updateNode(message.SourceId, message.SourceIP, nodeJoinContent.Port, -1, nodeJoinContent.Nickname)
	// send join reply
	if message.ReplyChannel != nil {
		responseContent := message_content.NewNodeJoinResponseLegacy(s.ctxt.Identity.Nickname, configuration.ListeningPortTcp, message_content.NewVerifierVote{make([]byte, message_fields.SizeNodeIdentifier, message_fields.SizeNodeIdentifier)})
		response := messages.NewLocal(messages.TypeNodeJoinResponseLegacy, responseContent, s.ctxt.Identity)
		message.ReplyChannel <- response
	}
	// Here, Java would also send a node join message to new nodes. We just let this slide and do it as part of the main loop.
	// We ignore the legacy part and just send a new one to ever new node.
	logging.TraceLog.Printf("Processed legacy node join message from %s.", nodeJoinContent.Nickname)
}

// Process a node join response. Regular run mode only.
func (s *state) processNodeJoinResponse(message *messages.Message) {
	nodeJoinResponseContent := *message.Content.(*message_content.NodeJoinResponse)
	s.updateNode(message.SourceId, message.SourceIP, nodeJoinResponseContent.PortTcp, nodeJoinResponseContent.PortUdp, nodeJoinResponseContent.Nickname)
	logging.TraceLog.Printf("Processed node join response from %s.", nodeJoinResponseContent.Nickname)
}

// Process a legacy node join response. Regular run mode only.
func (s *state) processNodeJoinResponseLegacy(message *messages.Message) {
	nodeJoinResponseContent := *message.Content.(*message_content.NodeJoinResponseLegacy)
	s.updateNode(message.SourceId, message.SourceIP, nodeJoinResponseContent.Port, -1, nodeJoinResponseContent.Nickname)
	logging.TraceLog.Printf("Processed legacy node join response from %s.", nodeJoinResponseContent.Nickname)
}

// Update the given node or node's status. Regular run mode only.
func (s *state) updateNode(sourceId, sourceIP []byte, portTcp, portUdp int32, nickname string) {
	existingNode, ok := s.nodeMap[message_fields.IP4BytesToString(sourceIP)]
	if ok {
		existingNode.PortTcp = portTcp
		if portUdp > 0 {
			existingNode.PortUdp = portUdp
		}
		existingNode.Nickname = nickname
		existingNode.InactiveTimestamp = -1
		if !bytes.Equal(existingNode.Identifier, sourceId) {
			now := utilities.Now()
			existingNode.Identifier = sourceId
			existingNode.QueueTimestamp = now
			existingNode.IdentifierChangeTimestamp = now
			if s.frozenEdgeHeight-existingNode.LastNodeJoinBlock > minimumMeshRequestInterval {
				s.nodeJoinCache = append(s.nodeJoinCache, existingNode)
			}
		}
	} else {
		n := node.NewNode(sourceId, sourceIP, portTcp, portUdp)
		// if we don't have enough node history, allow new nodes into the lottery immediately
		if !s.haveNodeHistory {
			n.QueueTimestamp -= configuration.LotteryWaitTime
		}
		s.nodeMap[n.IpString] = n
		s.nodeJoinCache = append(s.nodeJoinCache, n)
	}
}

// A connection to the given IP failed. Regular run mode only.
func (s *state) markFailedConnection(ip string) {
	n, ok := s.nodeMap[ip]
	if ok {
		n.FailedConnectionCount++
		if n.FailedConnectionCount >= consecutiveFailuresBeforeRemoval {
			n.InactiveTimestamp = utilities.Now()
		}
	}
}

// A connection to the given IP was successful. Regular run mode only.
func (s *state) markSuccessfulConnection(ip string) {
	n, ok := s.nodeMap[ip]
	if ok {
		n.FailedConnectionCount = 0
		n.InactiveTimestamp = -1
	}
}

// Returns a slice of all in cycle nodes. Info is deep copied for later use(ability).
func (s *state) getCycle() []*node.Node {
	copiedList := make([]*node.Node, 0, s.ctxt.CycleAuthority.GetCurrentCycleLength())
	for _, n := range s.nodeMap {
		if s.ctxt.CycleAuthority.VerifierInCurrentCycle(n.Identifier) {
			copiedNode := &node.Node{
				Identifier:                append([]byte(nil), n.Identifier...),
				IpAddress:                 append([]byte(nil), n.IpAddress...),
				IpString:                  n.IpString, // probably not needed
				PortTcp:                   n.PortTcp,
				PortUdp:                   n.PortUdp, // probably not needed
				QueueTimestamp:            n.QueueTimestamp,
				Nickname:                  n.Nickname,                  // probably not needed
				IdentifierChangeTimestamp: n.IdentifierChangeTimestamp, // probably not needed
				InactiveTimestamp:         n.InactiveTimestamp,         // probably not needed
				LastNodeJoinBlock:         n.LastNodeJoinBlock,         // probably not needed
			}
			copiedList = append(copiedList, copiedNode)
		}
	}
	return copiedList
}

// Returns slice of all known nodes. Info is deep copied for later use(ability).
func (s *state) getFullMesh() []*node.Node {
	copiedList := make([]*node.Node, 0, len(s.nodeMap))
	for _, n := range s.nodeMap {
		copiedNode := &node.Node{
			Identifier:                append([]byte(nil), n.Identifier...),
			IpAddress:                 append([]byte(nil), n.IpAddress...),
			IpString:                  n.IpString, // probably not needed
			PortTcp:                   n.PortTcp,
			PortUdp:                   n.PortUdp, // probably not needed
			QueueTimestamp:            n.QueueTimestamp,
			Nickname:                  n.Nickname,                  // probably not needed
			IdentifierChangeTimestamp: n.IdentifierChangeTimestamp, // probably not needed
			InactiveTimestamp:         n.InactiveTimestamp,         // probably not needed
			LastNodeJoinBlock:         n.LastNodeJoinBlock,         // probably not needed
		}
		copiedList = append(copiedList, copiedNode)
	}
	return copiedList
}

// Find a random in-cycle node.
// This is not actually random, we just loop through a sorted list of all active in-cycle nodes.
func (s *state) findRandomCycleNode() *node.Node {
	// normally, this loop should just go through one or two iterations
	for i := 0; i < len(s.inCycleNodes); i++ {
		if s.randomNodePosition == -1 {
			rand.Seed(time.Now().UnixNano())
			s.randomNodePosition = rand.Intn(len(s.inCycleNodes) - 1)
		}
		if s.randomNodePosition >= len(s.inCycleNodes) {
			s.randomNodePosition = 0
		}
		n := s.inCycleNodes[s.randomNodePosition]
		s.randomNodePosition++
		if !bytes.Equal(n.Identifier, s.ctxt.Identity.PublicKey) {
			return n
		}
	}
	return nil
}

// Every 100 blocks, Nyzo demotes in cycle nodes and perists them to a file.
// I don't really get the demoting part (except for persisting them), but we'll see.
func (s *state) demoteInCycleNodes() {
	for _, n := range s.nodeMap {
		if s.ctxt.CycleAuthority.VerifierInCurrentCycle(n.Identifier) {
			n.QueueTimestamp = utilities.Now()
		}
	}
}

// Write the node persist file, this cannot happen concurrently by design.
func (s *state) writePersistFile(fileName string, list []*node.Node) {
	defer s.ctxt.WaitGroup.Done()
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, nodesFilePermissions)
	if err == nil {
		defer file.Close()
		writer := bufio.NewWriter(file)
		for _, n := range list {
			_, _ = writer.WriteString(fmt.Sprintf("%s:%s:%d:%d:%d:%d:%d\n", identity.BytesToNyzoHex(n.Identifier), n.IpString, n.PortTcp, n.PortUdp, n.QueueTimestamp, n.IdentifierChangeTimestamp, n.InactiveTimestamp))
		}
		_ = writer.Flush()
	}
}

// Persist node map to a file, this is called every 100 blocks.
func (s *state) persistNodes(directory string) {
	// we make a deep copy of the list to hand over to the async routine, this seems to be fairly quick
	copiedList := make([]*node.Node, 0, len(s.nodeMap))
	for _, n := range s.nodeMap {
		copiedNode := &node.Node{
			Identifier:                append([]byte(nil), n.Identifier...),
			IpAddress:                 append([]byte(nil), n.IpAddress...),
			IpString:                  n.IpString,
			PortTcp:                   n.PortTcp,
			PortUdp:                   n.PortUdp,
			QueueTimestamp:            n.QueueTimestamp,
			Nickname:                  "", // not needed
			IdentifierChangeTimestamp: n.IdentifierChangeTimestamp,
			InactiveTimestamp:         n.InactiveTimestamp,
			LastNodeJoinBlock:         0, // not needed
		}
		copiedList = append(copiedList, copiedNode)
	}
	// now, this won't touch any core data any more
	s.ctxt.WaitGroup.Add(1)
	go s.writePersistFile(directory, copiedList)
}

// This happens pre- main loop, so no concurrency issues.
func (s *state) loadPersistedNodes(directory string) {
	data, err := ioutil.ReadFile(directory)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			details := strings.Split(line, ":")
			if len(details) == 7 {
				_, ok := s.nodeMap[details[1]]
				if !ok {
					id, err := identity.NyzoHexToBytes([]byte(details[0]), message_fields.SizeNodeIdentifier)
					if err != nil || message_fields.AllZeroes(id) {
						continue
					}
					tcp, _ := strconv.Atoi(details[2])
					if tcp <= 0 {
						continue
					}
					udp, _ := strconv.Atoi(details[3])
					ip := message_fields.IP4StringToBytes(details[1])
					if message_fields.AllZeroes(ip) || utilities.IsPrivateIP(net.ParseIP(details[1])) {
						continue
					}
					n := node.NewNode(id, ip, int32(tcp), int32(udp))
					queue, _ := strconv.Atoi(details[4])
					identifier, _ := strconv.Atoi(details[5])
					inactive, _ := strconv.Atoi(details[6])
					n.QueueTimestamp = int64(queue)
					n.IdentifierChangeTimestamp = int64(identifier)
					n.InactiveTimestamp = int64(inactive)
					s.nodeMap[details[1]] = n
				}
			}
		}
	}
}

// Main loop
// Perform startup actions, register to message router and enter main loop.
func (s *state) Start() {
	defer logging.InfoLog.Print("Main loop of node manager exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	logging.InfoLog.Print("Starting main loop of node manager.")
	blockDurationTicker := time.NewTicker(configuration.BlockDuration * time.Millisecond)
	if s.ctxt.RunMode() != interfaces.RunModeArchive {
		// archive mode will only start contacting the mesh once we loaded the blocks in the online repo
		s.meshRequestSuccessCount = 0
		s.bootstrapMesh()
		s.bootstrapPhase = true
	}
	// Try to get whitelisted by managed verifiers. 1st step: send an ip request.
	if s.ctxt.RunMode() == interfaces.RunModeSentinel || s.ctxt.RunMode() == interfaces.RunModeArchive {
		s.startWhitelistUpdate()
	}
	done := false
	for !done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalConnectionFailure:
				ip := m.Payload[0].(string)
				s.markFailedConnection(ip)
			case messages.TypeInternalConnectionSuccess:
				ip := m.Payload[0].(string)
				s.markSuccessfulConnection(ip)
			case messages.TypeInternalSendToRandomNode:
				n := s.findRandomCycleNode()
				if n != nil {
					go networking.FetchTcp(m.Payload[0].(*messages.Message), n.IpAddress, n.PortTcp)
				}
			case messages.TypeInternalSendToCycle:
				for _, n := range s.inCycleNodes {
					if !bytes.Equal(n.Identifier, s.ctxt.Identity.PublicKey) {
						go networking.FetchTcp(m.Payload[0].(*messages.Message), n.IpAddress, n.PortTcp)
					}
				}
			case messages.TypeInternalNewFrozenEdgeBlock:
				block := m.Payload[0].(*blockchain_data.Block)
				s.frozenEdgeHeight = block.Height
			case messages.TypeInternalChainInitialized:
				if s.ctxt.RunMode() == interfaces.RunModeArchive {
					s.meshRequestSuccessCount = 0
					s.bootstrapPhase = true
					s.startWhitelistUpdate()
				}
				s.chainInitialized = true
			case messages.TypeInternalExiting:
				done = true
			}
		case m := <-s.messageChannel:
			switch m.Type {
			case messages.TypeMeshResponse:
				s.processMeshResponse(m)
			case messages.TypeFullMeshResponse:
				s.processMeshResponse(m)
			case messages.TypeStatusResponse:
				s.processStatusResponse(m)
			case messages.TypeNodeJoin:
				s.processNodeJoinMessage(m)
			case messages.TypeNodeJoinResponse:
				s.processNodeJoinResponse(m)
			case messages.TypeNodeJoinLegacy:
				s.processNodeJoinMessageLegacy(m)
			case messages.TypeNodeJoinResponseLegacy:
				s.processNodeJoinResponseLegacy(m)
			case messages.TypeMissingBlockVoteRequest:
				existingNode, ok := s.nodeMap[message_fields.IP4BytesToString(m.SourceIP)]
				if ok {
					existingNode.InactiveTimestamp = -1
				}
			case messages.TypeMissingBlockRequest:
				existingNode, ok := s.nodeMap[message_fields.IP4BytesToString(m.SourceIP)]
				if ok {
					existingNode.InactiveTimestamp = -1
				}
			case messages.TypeMeshRequest:
				if m.ReplyChannel != nil {
					content := message_content.NewMeshResponse(s.getCycle())
					m.ReplyChannel <- messages.NewLocal(messages.TypeMeshResponse, content, s.ctxt.Identity)
				}
			case messages.TypeFullMeshRequest:
				if m.ReplyChannel != nil {
					content := message_content.NewMeshResponse(s.getFullMesh())
					m.ReplyChannel <- messages.NewLocal(messages.TypeFullMeshResponse, content, s.ctxt.Identity)
				}
			case messages.TypeIpAddressResponse:
				s.processIpAddressResponse(m)
			case messages.TypeWhitelistResponse:
				s.processWhitelistResponse(m)
			}
		case <-blockDurationTicker.C:
			// In archive mode, only maintain mesh relationship after chain initialization.
			if s.ctxt.RunMode() == interfaces.RunModeArchive && !s.chainInitialized {
				continue
			}
			// update mesh data, Java does this more often, but I'd say it's pretty expensive and 7s should still be OK given what it's used for
			s.maintainMeshData()
			if s.bootstrapPhase {
				if s.meshRequestSuccessCount > 0 {
					logging.InfoLog.Printf("Node manager finished bootstrap phase, got %d mesh responses.", s.meshRequestSuccessCount)
					s.bootstrapPhase = false
					s.meshRequestSuccessCount = 0
					if s.ctxt.RunMode() == interfaces.RunModeVerifier {
						s.sendInitialNodeJoins()
					}
				} else {
					s.bootstrapMesh()
					continue
				}
			} else {
				// force a re-entry of the bootstrap phase if we couldn't connect to enough active verifiers
				if len(s.inCycleNodes) < s.ctxt.CycleAuthority.GetCurrentCycleLength()*3/4 && s.meshRequestSuccessCount == 0 {
					logging.InfoLog.Printf("Re-entereing bootstrap phase, not enough active verifiers found.")
					s.bootstrapMesh()
					s.bootstrapPhase = true
					continue
				}
			}
			// maintain mesh integration, actual behavior depends on run mode
			s.meshRequestWait--
			if s.meshRequestWait <= 0 && len(s.nodeJoinCache) == 0 && len(s.nodeJoinCache) == 0 {
				s.maintainMesh()
			}
			// send 10 node joins if we have pending ones
			if len(s.nodeJoinCache) > 0 {
				s.send10NodeJoins()
			}
			// send 10 status requests if we have pending ones (archive mode only)
			if len(s.statusRequestCache) > 0 {
				s.send10StatusRequests()
			}
			// persist nodes every 100 blocks
			if s.frozenEdgeHeight%100 == 0 {
				s.demoteInCycleNodes()
				s.persistNodes(configuration.DataDirectory + "/" + configuration.NodesFileName)
			}
			// do we have enough node history?
			if !s.haveNodeHistory && s.meshRequestSuccessCount > 4 {
				s.haveNodeHistory = true
				s.ctxt.PersistentData.Store(configuration.HaveNodeHistoryKey, "1")
			}
			// periodically refresh whitelisting
			if s.ctxt.RunMode() == interfaces.RunModeSentinel || s.ctxt.RunMode() == interfaces.RunModeArchive {
				if utilities.Now()-s.lastWhitelistUpdate > whitelistUpdateInterval {
					s.startWhitelistUpdate()
				}
			}
		}
	}
}

// Initialization function
func (s *state) Initialize() error {
	// set message routes
	s.messageChannel = make(chan *messages.Message, 1500)
	router.Router.AddRoute(messages.TypeMeshResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeFullMeshResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeStatusResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeNodeJoin, s.messageChannel)
	router.Router.AddRoute(messages.TypeNodeJoinResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeNodeJoinLegacy, s.messageChannel)
	router.Router.AddRoute(messages.TypeNodeJoinResponseLegacy, s.messageChannel)
	router.Router.AddRoute(messages.TypeMissingBlockVoteRequest, s.messageChannel)
	router.Router.AddRoute(messages.TypeMissingBlockRequest, s.messageChannel)
	router.Router.AddRoute(messages.TypeMeshRequest, s.messageChannel)
	router.Router.AddRoute(messages.TypeFullMeshRequest, s.messageChannel)
	router.Router.AddRoute(messages.TypeIpAddressResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeWhitelistResponse, s.messageChannel)
	s.internalMessageChannel = make(chan *messages.InternalMessage, 1500)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalNewFrozenEdgeBlock, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalSendToRandomNode, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalConnectionFailure, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalConnectionSuccess, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalChainInitialized, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalSendToCycle, s.internalMessageChannel)
	s.haveNodeHistory = s.ctxt.PersistentData.Retrieve(configuration.HaveNodeHistoryKey, "0") == "1"
	s.activeNodeCount = 0
	s.randomNodePosition = -1
	s.missingIncycleNodesMessage = "*** no verifiers missing ***"
	// load persisted nodes file
	s.loadPersistedNodes(configuration.DataDirectory + "/" + configuration.NodesFileName)
	err := s.loadTrustedEntryPoints()
	if err != nil {
		return err
	}
	err = s.loadManagedVerifiers()
	if err != nil && s.ctxt.RunMode() != interfaces.RunModeSentinel && s.ctxt.RunMode() != interfaces.RunModeArchive {
		// only the sentinel and archive modes need managed verifiers
		// TODO: make a "trustful" archive mode that doesn't require a managed verifier
		err = nil
	}
	return err
}

// Create a node manager.
func NewNodeManager(ctxt *interfaces.Context) interfaces.NodeManagerInterface {
	s := &state{}
	s.ctxt = ctxt
	s.nodeMap = make(map[string]*node.Node)
	s.nodeJoinCache = make([]*node.Node, 0)
	s.statusRequestCache = make([]*node.Node, 0)
	return s
}
