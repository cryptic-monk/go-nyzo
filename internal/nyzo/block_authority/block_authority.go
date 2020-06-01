/*
The block authority handles task related to validating blocks and the blockchain. It is the ultimate instance
that watches over the blockchain up to the frozen edge.

In archive mode, the block authority will start the bootstrap from disk, in other modes, it will react to
bootstrap info received from the node manager.
*/
package block_authority

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/block_handler"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"time"
)

const (
	blockUpdateIntervalFast     = 1000
	blockUpdateIntervalStandard = 2000
	queryHistoryLength          = 10
)

type state struct {
	ctxt                      *interfaces.Context
	genesisHash               []byte
	frozenEdgeHeight          int64
	frozenEdgeBlock           *blockchain_data.Block
	bootstrapFrozenEdgeHeight int64
	bootstrapFrozenEdgeHash   []byte
	messageChannel            chan *messages.Message              // here's where we'll receive the messages we are registering for
	internalMessageChannel    chan *messages.InternalMessage      // channel for internal and local messages
	chainLoaded               bool                                // true after we loaded any available historical chain info (from disk or online repo)
	chainInitialized          bool                                // true once the chain is ready for operation (ready to be added to)
	managedVerifiers          []*networking.ManagedVerifier       // handled by node manager, copied here for mere convenience
	managedVerifierStatus     []*networking.ManagedVerifierStatus // status tracked by block fetching process
	lastBlockRequestedTime    int64                               // last block with votes request
	blockSpeedTracker         int64                               // tracks block speed (important e.g. during historical loading/catchup)
	lastSpeedTrackingBlock    int64
	fastChainInitialization   bool // read from preferences file, set to true to speedup chain initialization (especially historical loading in archive mode), at the cost of some (pretty theoretical) security
}

// Do a full verification of this block: signature (including transactions) and continuity.
func (s *state) BlockIsValid(block *blockchain_data.Block) bool {
	// Note: balance list calculation is checked in StoreBlock only, as the balance list is not technically part of the Block itself
	if s.chainInitialized || !s.fastChainInitialization {
		s.addPreviousBlockHashesToTransactions(block)
		if block.SignatureState == blockchain_data.Undetermined {
			lengthBefore := len(block.Transactions)
			result := s.ctxt.TransactionManager.ValidTransactionsOnly(block.Transactions, block.StartTimestamp)
			if len(result) == lengthBefore {
				block.SignatureState = blockchain_data.Valid
			} else {
				block.SignatureState = blockchain_data.Invalid
			}
		}
	} else {
		block.SignatureState = blockchain_data.Valid
	}
	if block.SignatureState == blockchain_data.Valid {
		if ed25519.Verify(block.VerifierIdentifier, block.Serialize(true), block.VerifierSignature) {
			block.SignatureState = blockchain_data.Valid
		} else {
			block.SignatureState = blockchain_data.Invalid
		}
	}
	s.ctxt.CycleAuthority.DetermineContinuityForBlock(block)
	return block.SignatureState == blockchain_data.Valid && block.ContinuityState == blockchain_data.Valid
}

// Get the genesis block hash, used for seed transactions.
func (s *state) GetGenesisBlockHash() []byte {
	return s.genesisHash
}

// Output frozen edge plus block processing speed stats.
func (s *state) printFrozenEdgeStats(block *blockchain_data.Block) {
	milliseconds := (time.Now().UnixNano() / 1000000) - s.blockSpeedTracker
	blockCount := block.Height - s.lastSpeedTrackingBlock
	if blockCount > 0 {
		milliseconds = milliseconds / blockCount
	} else {
		milliseconds = -1
	}
	s.blockSpeedTracker = time.Now().UnixNano() / 1000000
	s.lastSpeedTrackingBlock = block.Height
	if milliseconds > 0 {
		logging.InfoLog.Printf("New frozen edge height reached: %d, milliseconds per block: %d.", block.Height, milliseconds)
	} else {
		logging.InfoLog.Printf("New frozen edge height reached: %d.", block.Height)
	}
}

// Freeze a block.
func (s *state) freezeBlock(block *blockchain_data.Block, balanceList *blockchain_data.BalanceList) {
	if block.Height < s.frozenEdgeHeight {
		logging.ErrorLog.Print("Attempt to freeze a block below the frozen edge.")
	} else if block.Height > 0 && s.ctxt.RunMode() == interfaces.RunModeArchive && (balanceList != nil && s.chainInitialized) {
		logging.ErrorLog.Print("Attempt to bootstrap in archive mode, archive mode cannot accept a discontinuous chain.")
	} else if block.Height > 0 && (balanceList == nil && !bytes.Equal(block.PreviousBlockHash, s.frozenEdgeBlock.Hash)) {
		logging.ErrorLog.Printf("Attempt to freeze discontinuous block that is not a bootstrap block at height %d.", block.Height)
	} else {
		// commit the block
		s.ctxt.BlockHandler.CommitFrozenEdgeBlock(block, balanceList)
		// set the frozen edge
		s.frozenEdgeHeight = block.Height
		s.frozenEdgeBlock = block
		// output statistics
		if s.chainInitialized || s.ctxt.RunMode() != interfaces.RunModeArchive || block.Height%1000 == 0 {
			s.printFrozenEdgeStats(block)
		}
		// inform the other components
		message1 := messages.NewInternalMessage(messages.TypeInternalNewFrozenEdgeBlock, block, balanceList)
		router.Router.RouteInternal(message1)
	}
}

// Adds previous block hash data to all transactions in this block, see below for more explanations.
func (s *state) addPreviousBlockHashesToTransactions(block *blockchain_data.Block) {
	for _, t := range block.Transactions {
		if len(t.PreviousBlockHash) == 0 {
			t.PreviousBlockHash = s.previousBlockHashForTransaction(t.PreviousHashHeight, block.Height, block.PreviousBlockHash)
		}
	}
}

// This is an interesting case: for signing, transactions include a previous block hash each. This one is hard
// to determine above the frozen edge, "Block" does this in Java, which kinda makes zero sense as it has to reach
// all the way up to the block manager. We do it here in the block manager.
func (s *state) previousBlockHashForTransaction(hashHeight, transactionHeight int64, previousHashInChain []byte) []byte {
	// TODO: handle unfrozen state
	if hashHeight <= s.frozenEdgeHeight || !s.chainInitialized {
		block := s.ctxt.BlockHandler.GetBlock(hashHeight)
		if block != nil {
			return block.Hash
		} else {
			return make([]byte, 32, 32)
		}
	} else {
		// TODO: part above frozen edge is not done yet.
		return make([]byte, 32, 32)
	}
}

// Load and freeze the genesis block.
func (s *state) loadGenesisBlock() error {
	// We should be peachy here as we ship the genesis block with the config component and place it correctly during EnsureSetup.
	// Java tries to load the block from a trusted source, which is not a good idea IMO.
	genesisBlock := s.ctxt.BlockHandler.GetBlock(0)
	if genesisBlock != nil {
		s.genesisHash = genesisBlock.Hash
		s.freezeBlock(genesisBlock, nil)
	} else {
		return errors.New("cannot load genesis block")
	}
	return nil
}

// Send the bootstrap block request.
func (s *state) sendBootstrapBlockRequest() {
	blockRequestContent := message_content.NewBlockRequest(s.bootstrapFrozenEdgeHeight, s.bootstrapFrozenEdgeHeight, true)
	messageBlockRequest := messages.NewLocal(messages.TypeBlockRequest, blockRequestContent, s.ctxt.Identity)
	if s.ctxt.RunMode() == interfaces.RunModeSentinel || s.ctxt.RunMode() == interfaces.RunModeArchive {
		logging.InfoLog.Printf("Sending bootstrap block requests for height %d to all managed verifiers.", s.bootstrapFrozenEdgeHeight)
		for _, verifier := range s.managedVerifiers {
			go networking.FetchTcpNamed(messageBlockRequest, verifier.Host, verifier.Port)
		}
	} else {
		logging.InfoLog.Printf("Sending bootstrap block requests for height %d to all trusted entry points.", s.bootstrapFrozenEdgeHeight)
		entryPoints := s.ctxt.NodeManager.GetTrustedEntryPoints()
		for _, verifier := range entryPoints {
			go networking.FetchTcpNamed(messageBlockRequest, verifier.Host, verifier.Port)
		}
	}
}

func (s *state) setChainIsInitialized() {
	s.ctxt.BlockHandler.SetChainIsInitialized()
	s.chainInitialized = true
	message := messages.NewInternalMessage(messages.TypeInternalChainInitialized)
	router.Router.RouteInternal(message)
	logging.InfoLog.Printf("Block authority exited bootstrap phase at height: %d. Chain successfully initialized.", s.frozenEdgeHeight)
}

// Watch the chain by sending out block requests in regular intervals (Sentinel, Archive mode)
func (s *state) watchChain() {
	if len(s.managedVerifiers) > 0 {
		// If we have managed verifiers, we trust them more than anything else.
		// Together with always_track_blockchain, somebody could also use this to create a less trusting archive node.
		for i := range s.managedVerifiers {
			blockUpdateInterval := int64(blockUpdateIntervalStandard)
			if s.managedVerifierStatus[i].FastFetchMode {
				blockUpdateInterval = blockUpdateIntervalFast
			}
			timestamp := time.Now().UnixNano() / 1000000
			currentSlot := int(timestamp / blockUpdateInterval % int64(len(s.managedVerifiers)))
			if s.managedVerifierStatus[i].LastBlockRequestedTime < timestamp-blockUpdateInterval {
				if i == currentSlot || s.frozenEdgeBlock.VerificationTimestamp < timestamp-20000 {
					s.managedVerifierStatus[i].LastBlockRequestedTime = timestamp
					s.managedVerifierStatus[i].QueriedLastInterval = true
					s.getBlocksFromManagedVerifier(i)
				} else {
					s.managedVerifierStatus[i].QueriedLastInterval = false
				}
			}
		}
	}

	// Fallback: if we fall more than 35s behind the frozen edge, we request a block with votes from a random node.
	timestamp := time.Now().UnixNano() / 1000000
	if s.lastBlockRequestedTime < timestamp-blockUpdateIntervalStandard && s.frozenEdgeBlock.VerificationTimestamp < timestamp-35000 {
		logging.TraceLog.Printf("Sending block with votes request for height %d to random cycle node.", s.frozenEdgeHeight+1)
		s.lastBlockRequestedTime = timestamp
		blockRequestContent := message_content.NewBlockWithVotesRequest(s.frozenEdgeHeight + 1)
		messageBlockRequest := messages.NewLocal(messages.TypeBlockWithVotesRequest, blockRequestContent, s.ctxt.Identity)
		router.Router.RouteInternal(messages.NewInternalMessage(messages.TypeInternalSendToRandomNode, messageBlockRequest))
	}
}

// Try to get new block(s) from the managed verifier with the index i, 1 block in regular mode, 10 in fast fetch mode.
func (s *state) getBlocksFromManagedVerifier(i int) {
	if s.managedVerifierStatus[i].WaitingForAnswer {
		// previous query failed completely (nothing came back)
		s.managedVerifierStatus[i].QueryHistory[s.managedVerifierStatus[i].QueryIndex] = -1
		s.managedVerifierStatus[i].ConsecutiveSuccessfulBlockFetches = 0
		s.managedVerifierStatus[i].FastFetchMode = false
		s.managedVerifierStatus[i].QueryIndex = (s.managedVerifierStatus[i].QueryIndex + 1) % queryHistoryLength
	}
	s.managedVerifierStatus[i].WaitingForAnswer = true

	startHeight := s.frozenEdgeBlock.Height + 1
	endHeight := startHeight
	if s.managedVerifierStatus[i].FastFetchMode {
		endHeight += 9
	}
	blockRequestContent := message_content.NewBlockRequest(startHeight, endHeight, false)
	messageBlockRequest := messages.NewLocal(messages.TypeBlockRequest, blockRequestContent, s.ctxt.Identity)
	go networking.FetchTcpNamed(messageBlockRequest, s.managedVerifiers[i].Host, s.managedVerifiers[i].Port)
}

// Process a block with votes response.
func (s *state) processBlockWithVotesResponse(message *messages.Message) {
	blockResponseContent := *message.Content.(*message_content.BlockWithVotesResponse)
	if blockResponseContent.Block != nil && blockResponseContent.Votes != nil {
		voteThreshold := s.ctxt.CycleAuthority.GetCurrentCycleLength() * 3 / 4
		voteCount := 0
		blockHash := blockResponseContent.Block.Hash
		for _, vote := range blockResponseContent.Votes {
			if bytes.Equal(vote.Hash, blockHash) {
				voteMessage := messages.Message{}
				voteMessage.Timestamp = vote.Timestamp
				voteMessage.Type = messages.TypeBlockVote
				voteMessage.Content = vote
				voteMessage.SourceId = vote.SenderIdentifier
				voteMessage.Signature = vote.MessageSignature
				if voteMessage.SignatureIsValid() && s.ctxt.CycleAuthority.VerifierInCurrentCycle(vote.SenderIdentifier) {
					voteCount++
				}
			}
		}
		logging.TraceLog.Printf("Verified block with votes at height %d, vote threshold: %d, valid votes %d.", blockResponseContent.Block.Height, voteThreshold, voteCount)
		if voteCount > voteThreshold && bytes.Equal(blockResponseContent.Block.PreviousBlockHash, s.frozenEdgeBlock.Hash) {
			s.freezeBlock(blockResponseContent.Block, nil)
		}
	} else {
		logging.TraceLog.Print("Not enough info in block with votes response.")
	}
}

// Process a block response.
func (s *state) processBlockResponse(message *messages.Message) {
	blockResponseContent := *message.Content.(*message_content.BlockResponse)
	if !s.chainInitialized {
		// we are bootstrapping
		if blockResponseContent.BalanceList != nil && bytes.Equal(blockResponseContent.Blocks[0].Hash, s.bootstrapFrozenEdgeHash) && bytes.Equal(blockResponseContent.BalanceList.GetHash(), blockResponseContent.Blocks[0].BalanceListHash) {
			s.freezeBlock(blockResponseContent.Blocks[0], blockResponseContent.BalanceList)
			s.setChainIsInitialized()
		}
	} else {
		// regular operations
		for _, block := range blockResponseContent.Blocks {
			// we only accept blocks without votes from managed verifiers
			for i, verifier := range s.managedVerifiers {
				if bytes.Equal(message.SourceId, verifier.Identity.PublicKey) {
					if block.Height == s.frozenEdgeHeight+1 {
						s.freezeBlock(block, nil)
					}
					// mark successful answer
					s.managedVerifierStatus[i].WaitingForAnswer = false
					// keep a history of how many blocks we got back
					s.managedVerifierStatus[i].QueryHistory[s.managedVerifierStatus[i].QueryIndex] = len(blockResponseContent.Blocks)
					if len(blockResponseContent.Blocks) > 0 {
						// if we get a lot of blocks in a row, we are likely lagging behind the chain
						s.managedVerifierStatus[i].ConsecutiveSuccessfulBlockFetches++
						timestamp := time.Now().UnixNano() / 1000000
						if s.managedVerifierStatus[i].ConsecutiveSuccessfulBlockFetches >= 4 && s.frozenEdgeBlock.VerificationTimestamp < timestamp-140000 {
							s.managedVerifierStatus[i].FastFetchMode = true
						}
					} else {
						// deactivate fast fetch if we can't get a block twice in a row
						if s.managedVerifierStatus[i].ConsecutiveSuccessfulBlockFetches == 0 {
							s.managedVerifierStatus[i].FastFetchMode = true
						}
						s.managedVerifierStatus[i].ConsecutiveSuccessfulBlockFetches = 0
					}
					s.managedVerifierStatus[i].QueryIndex = (s.managedVerifierStatus[i].QueryIndex + 1) % queryHistoryLength
					break
				}
			}
		}
	}
}

// Historical chain loading: freeze the given historical chain block.
func (s *state) freezeHistoricalChainAt(height int64) {
	if height == 0 {
		// emit genesis block transactions
		block := s.ctxt.BlockHandler.GetBlock(0)
		_ = balance_authority.UpdateBalanceListForNextBlock(s.ctxt, nil, nil, block, true)
	}
	block := s.ctxt.BlockHandler.GetBlock(height)
	balanceList := s.ctxt.BlockHandler.GetBalanceList(height)
	if block != nil && balanceList != nil {
		logging.InfoLog.Print("Freezing historical chain in archive mode, this could take a while...")
		s.freezeBlock(block, balanceList)
		if block.CycleInformation != nil {
			logging.InfoLog.Printf("Successfully froze historical chain at height %d, reconstructed %d block's worth of chain data.", block.Height, block.CycleInformation.CycleLengths[0]+block.CycleInformation.CycleLengths[1]+block.CycleInformation.CycleLengths[2]+block.CycleInformation.CycleLengths[3])
		} else {
			logging.ErrorLog.Fatalf("Could not reconstruct historical chain data at height %d in archive mode.", block.Height)
		}
	} else {
		logging.ErrorLog.Fatal("Cannot freeze historical chain in archive mode.")
	}
}

// Load chain history from online repo if necessary.
func (s *state) loadHistoricalChain(dataStoreHeight int64) {
	defer s.ctxt.WaitGroup.Done()
	// freeze the data store height so that we can build from there
	s.freezeHistoricalChainAt(dataStoreHeight)
	// try the web repo first
	internalMessageChannel := make(chan *messages.InternalMessage, 1)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, internalMessageChannel)
	var retryDelay int64 = 1 // if we have issues with the repo, we back off exponentially
	var nextTry int64        // next try on the repo during backoff
	done := false
	for !done {
		select {
		case <-internalMessageChannel:
			done = true
		default:
			// we had an issue before, wait for the prescribed time
			if nextTry > time.Now().UnixNano() {
				// prevents busyloop, yet allows graceful exit
				time.Sleep(300 * time.Millisecond)
				continue
			}
			// try to extend historical frozen edge from online repo
			offset := (dataStoreHeight + 1) - (dataStoreHeight+1)%1000
			blocks, err := s.ctxt.BlockHandler.GetBlocks(offset, offset+999)
			if len(blocks) == 1000 {
				// reset backoff now that we got another wad of blocks
				retryDelay = 1
				nextTry = 0
			blockProcessing:
				for i := 0; i < 1000; i++ {
					// TODO: if there would be an invalid block in the store, things would crap out badly here. Maybe catch this eventuality more gracefully?
					if offset+int64(i) > dataStoreHeight && s.BlockIsValid(blocks[i]) {
						s.freezeBlock(blocks[i], nil)
						dataStoreHeight = s.frozenEdgeHeight
					}
					// allow exit during processing of a batch of blocks
					select {
					case <-internalMessageChannel:
						done = true
						break blockProcessing
					default:
						continue
					}
				}
			} else {
				retryDelay = retryDelay * 2 // 1 second, 2 seconds, 4 seconds, 8 seconds, 16 seconds, 32 seconds, 64 seconds, 128 seconds, 256 seconds
				if retryDelay > 256 || err != nil && err.Error() == "403" {
					// give up on the online repo, start regular chain loading
					done = true
				} else {
					// back off
					nextTry = time.Now().UnixNano() + retryDelay*1000000000
				}
			}
		}
	}
	s.setChainIsInitialized()
	s.chainLoaded = true
}

// Initialize chain from disk if useful. Otherwise send a bootstrap block request.
func (s *state) initializeChainAt(height int64) {
	diskHeight := block_handler.FindHighestIndividualBlockFile()
	if diskHeight > 0 && diskHeight >= height-int64(s.ctxt.CycleAuthority.GetCurrentCycleLength())*4 {
		logging.InfoLog.Print("Attempting to restart chain from disk, this could take a while...")
		block := s.ctxt.BlockHandler.GetBlock(diskHeight)
		balanceList := s.ctxt.BlockHandler.GetBalanceList(diskHeight)
		if block != nil && balanceList != nil && s.ctxt.CycleAuthority.HasCycleAt(block) {
			s.freezeBlock(block, balanceList)
			s.setChainIsInitialized()
			logging.InfoLog.Printf("Chain restarted from disk: %d.", diskHeight)
		} else {
			logging.InfoLog.Print("Could not restart chain from disk, historical data incomplete.")
		}
	}
	if !s.chainInitialized {
		s.sendBootstrapBlockRequest()
	}
	s.chainLoaded = true
}

// Main loop
func (s *state) Start() {
	defer logging.InfoLog.Print("Main loop of block authority exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	logging.InfoLog.Print("Starting main loop of block authority.")
	// keep track of managed verifier behavior during block fetching process
	s.managedVerifiers = s.ctxt.NodeManager.GetManagedVerifiers()
	s.managedVerifierStatus = make([]*networking.ManagedVerifierStatus, len(s.managedVerifiers))
	for i := range s.managedVerifiers {
		s.managedVerifierStatus[i] = new(networking.ManagedVerifierStatus)
		s.managedVerifierStatus[i].QueryHistory = make([]int, queryHistoryLength)
	}
	s.blockSpeedTracker = time.Now().UnixNano() / 1000000
	chainWatcherTicker := time.NewTicker(2 * time.Second)
	done := false
	for !done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalDataStoreHeight:
				if s.ctxt.RunMode() == interfaces.RunModeArchive {
					// "Bottom up" bootstrapping: to ensure an unbroken chain, we start from the highest block
					// in the data store. Run mode check isn't strictly necessary, but better safe than sorry.
					dataStoreHeight := m.Payload[0].(int64)
					logging.InfoLog.Printf("Bootstrapping in archive mode, starting with highest data store block: %d.", dataStoreHeight)
					s.ctxt.WaitGroup.Add(1)
					go s.loadHistoricalChain(dataStoreHeight)
				}
			case messages.TypeInternalBootstrapBlock:
				if s.ctxt.RunMode() == interfaces.RunModeSentinel {
					// "Top down" bootstrapping: we try to restart the chain from disk, or get a bootstrap block.
					// Will probably apply to other run modes too.
					s.bootstrapFrozenEdgeHeight = m.Payload[0].(int64)
					s.bootstrapFrozenEdgeHash = m.Payload[1].([]byte)
					// not protected by WaitGroup, if we exit, we exit
					go s.initializeChainAt(s.bootstrapFrozenEdgeHeight)
				}
			case messages.TypeInternalExiting:
				done = true
			}
		case m := <-s.messageChannel:
			switch m.Type {
			case messages.TypeBlockResponse:
				s.processBlockResponse(m)
			case messages.TypeBlockWithVotesResponse:
				s.processBlockWithVotesResponse(m)
			}
		case <-chainWatcherTicker.C:
			if !s.chainLoaded {
				continue
			}
			if !s.chainInitialized {
				// this can only mean that a previous bootstrap request failed
				s.sendBootstrapBlockRequest()
			} else if s.ctxt.RunMode() == interfaces.RunModeArchive || s.ctxt.RunMode() == interfaces.RunModeSentinel {
				// these modes just watch the chain
				s.watchChain()
			} else {
				// initialization done
				chainWatcherTicker.Stop()
			}
		}
	}
}

// Initialization: load the genesis block.
func (s *state) Initialize() error {
	// set message routes
	s.messageChannel = make(chan *messages.Message, 20)
	router.Router.AddRoute(messages.TypeBlockResponse, s.messageChannel)
	router.Router.AddRoute(messages.TypeBlockWithVotesResponse, s.messageChannel)
	s.internalMessageChannel = make(chan *messages.InternalMessage, 150)
	router.Router.AddInternalRoute(messages.TypeInternalDataStoreHeight, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalBootstrapBlock, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	s.fastChainInitialization = s.ctxt.Preferences.Retrieve(configuration.FastChainInitializationKey, "0") == "1"
	err := s.loadGenesisBlock()
	return err
}

// Create a block manager.
func NewBlockAuthority(ctxt *interfaces.Context) interfaces.BlockAuthorityInterface {
	s := &state{}
	s.ctxt = ctxt
	return s
}
