/*
The block file handler does menial tasks in relation to the block storage file system.

It can retrieve blocks (with some caching) and store them, from individual block files, consolidated block files,
or from an online source provided by Nyzo.

Nyzo stores a balance list with each block. Since this is handled here, balance list updates and checking,
as well as emission of all blockchain-related events to the DB storage in archive mode are handled by the block file handler.

Implements the "BlockFileHandler" interface defined in nyzo/interfaces.
*/
package block_file_handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	blocksPerFile     = 1000 // individual blocks per block consolidation file
	filesPerDirectory = 1000 // individual consolidation files per directory
)

type state struct {
	ctxt                     *interfaces.Context
	blockCache               map[int64]*blockchain_data.Block       // keep some blocks in memory to speed up access
	blockCacheLock           sync.Mutex                             // mutex for the block cache
	balanceListCache         map[int64]*blockchain_data.BalanceList // cache for balance lists
	balanceListCacheLock     sync.Mutex                             // mutex for balance list cache
	frozenEdgeHeight         int64
	frozenEdgeTime           int64
	trailingEdgeHeight       int64
	retentionEdgeHeight      int64
	lastCycleLength          int
	chainInitialized         bool
	blockConsolidatorOption  string
	blocksSinceConsolidation int64
}

// Get an individual block.
// We just call GetBlocks with from/to as the same height.
func (s *state) GetBlock(height int64) *blockchain_data.Block {
	result, _ := s.GetBlocks(height, height)
	if result != nil && len(result) == 1 {
		return result[0]
	}
	return nil
}

// Get multiple blocks. We call a special function that tries to get each block as efficiently as possible.
// Blocks will be in ascending order, but not necessarily successive.
func (s *state) GetBlocks(heightFrom, heightTo int64) ([]*blockchain_data.Block, error) {
	if heightFrom < 0 || heightTo < heightFrom {
		return nil, errors.New("wrong height requested in GetBlocks")
	}
	var retainedError error
	result := make([]*blockchain_data.Block, 0, heightTo-heightFrom+1)
	for i := heightFrom; i <= heightTo; i++ {
		block, err := s.getBlockFromBestSource(i, heightFrom, heightTo)
		if block != nil {
			result = append(result, block)
		}
		if err != nil {
			retainedError = err
			if retainedError.Error() == "403" {
				return result, retainedError
			}
		}
	}
	return result, retainedError
}

// Get a balance list for the given height.
// Important to note: a balance list for a given height is always the list of balances AFTER the transactions
// in the block at this height have been processed. For a new block, use the balance list at its block height - 1 and
// then apply the block's transactions to it using UpdateBalanceListForNextBlock from balance_authority.
// Since the balances are an endless chain of changes, we need some "initial" truth to calculate new balances,
// e.g. we can start from the genesis block, or we get a known balance list for a certain block.
func (s *state) GetBalanceList(blockHeight int64) *blockchain_data.BalanceList {
	return s.getBalanceListFromBestSource(blockHeight)
}

// Commit a new frozen edge block. This must be considered blocking for chain advancement and can take a significant amount of time
// during startup (especially for the archive node). The rest of the system falls out of whack if we don't fully commit to
// a block before we continue adding to the chain.
// This is exclusively called by freezeBlock in the block authority.
func (s *state) CommitFrozenEdgeBlock(block *blockchain_data.Block, balanceList *blockchain_data.BalanceList) {
	s.frozenEdgeHeight = block.Height
	s.frozenEdgeTime = block.VerificationTimestamp
	// Store the block, via UpdateBalanceListForNextBlock, this will emit transaction and cycle signature events for archive mode.
	s.storeBlock(block, balanceList)
	// Cycle information.
	cycleInformation := s.ctxt.CycleAuthority.GetCycleInformationForBlock(block)
	if cycleInformation != nil {
		// set trailing + retention edge height
		s.trailingEdgeHeight = cycleInformation.CalculateTrailingEdgeHeight(block.Height)
		s.retentionEdgeHeight = s.trailingEdgeHeight - 24
		// emit cycle and block info in archive mode
		if s.ctxt.RunMode() == interfaces.RunModeArchive {
			s.emitCycleEvents(block)
			s.emitBlock(block)
		}
	} else {
		s.trailingEdgeHeight = 0
		s.retentionEdgeHeight = 0
		if s.ctxt.RunMode() == interfaces.RunModeArchive {
			logging.ErrorLog.Fatal("Could not determine cycle information for frozen edge block in archive mode.")
		}
	}
	// clean the block and balance list caches
	s.cleanCaches()
	// consolidate block files
	if s.blockConsolidatorOption != configuration.BlockFileConsolidatorOptionDisable {
		// Java does this on a timer, we do it based on actual blocks received.
		// In theory, this could lead to an issue if we restarted very frequently, never ever getting
		// a continuous flow of blocks. However, the node would be broken in this case anyway.
		s.blocksSinceConsolidation++
		if s.blocksSinceConsolidation*configuration.BlockDuration > 300000 {
			s.blocksSinceConsolidation = 0
			// Doing this asynchronously should be safu as we only touch old files.
			// We'll move the consolidated file in place atomically.
			go s.consolidateBlockFiles()
		}
	}
}

// Inform the block file handler that the chain is fully initialized. Changes some minor behavior aspects.
func (s *state) SetChainIsInitialized() {
	s.chainInitialized = true
}

// Just what it says: find highest individual block file on disk.
// Static utility.
func FindHighestIndividualBlockFile() int64 {
	var height int64
	files, err := getIndividualBlockFilesSorted()
	if err == nil {
		for _, file := range files {
			height = blockHeightForIndividualFile(file.Name())
			if height > 0 {
				break
			}
		}
	}
	return height
}

// Get block at 'height' from the best available source. If we have to user more difficult sources
// (like reading from files or even from the web), and if that helps, we'll cache any readily
// available additional blocks
func (s *state) getBlockFromBestSource(height, heightFrom, heightTo int64) (*blockchain_data.Block, error) {
	// first try: the cache
	s.blockCacheLock.Lock()
	block, ok := s.blockCache[height]
	s.blockCacheLock.Unlock()
	if ok {
		return block, nil
	}
	// second, maybe an individual block file?
	block = s.blockFromIndividualFile(height)
	if block != nil {
		s.blockCacheLock.Lock()
		s.blockCache[block.Height] = block
		s.blockCacheLock.Unlock()
		return block, nil
	}
	block = s.blockFromConsolidatedFile(height, heightFrom, heightTo)
	if block != nil {
		// consolidated already does its own caching
		return block, nil
	}
	// only go online during chain initialization
	var err error
	if !s.chainInitialized {
		block, err = s.blockFromOnlineRepository(height, heightFrom, heightTo)
	}
	return block, err
}

// Read block from an individual block file, returns nil if something went wrong.
func (s *state) blockFromIndividualFile(height int64) *blockchain_data.Block {
	fileName := individualFileForBlockHeight(height)
	if utilities.FileDoesNotExists(fileName) {
		// fail silently if the file does not exist, as this is a normal situation
		return nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		logging.ErrorLog.Printf("Unable to load block from file: %s.", err.Error())
		return nil
	}
	defer f.Close()
	// skip block count
	err = message_fields.Skip(f, 2)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	block, err := blockchain_data.ReadNewBlock(f)
	if err != nil {
		logging.ErrorLog.Print(err)
	}
	return block
}

// Read block data from consolidated file, opportunistically caching any blocks between minHeight and maxHeight.
func (s *state) blockFromConsolidatedFile(height, minHeight, maxHeight int64) *blockchain_data.Block {
	fileName := consolidatedFileForBlockHeight(height)
	if utilities.FileDoesNotExists(fileName) {
		// fail silently if the file does not exist, as this is a normal situation
		return nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		logging.ErrorLog.Printf("Unable to load blocks from consolidated file %s: %s.", fileName, err.Error())
		return nil
	}
	defer f.Close()
	var returnBlock *blockchain_data.Block
	blockCount, err := message_fields.ReadInt16(f)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	var previousBlock *blockchain_data.Block
	for i := 0; i < int(blockCount) && (previousBlock == nil || previousBlock.Height < maxHeight); i++ {
		block, err := blockchain_data.ReadNewBlock(f)
		if err != nil {
			logging.ErrorLog.Print(err)
			return nil
		}
		if previousBlock == nil || previousBlock.Height != block.Height-1 {
			_, err := blockchain_data.ReadNewBalanceList(f)
			if err != nil {
				logging.ErrorLog.Print(err)
				return nil
			}
		}
		s.blockCacheLock.Lock()
		s.blockCache[block.Height] = block
		s.blockCacheLock.Unlock()
		if block.Height == height {
			returnBlock = block
		}
		previousBlock = block
	}
	return returnBlock
}

// Read block from online block repository, this is the absolute worst case.
// Will keep block files and cache blocks opportunistically.
func (s *state) blockFromOnlineRepository(height, minHeight, maxHeight int64) (*blockchain_data.Block, error) {
	err := downloadFromOnlineRepository(height)
	return s.blockFromConsolidatedFile(height, minHeight, maxHeight), err
}

// Find the best source for the balance list that we are looking for.
// Note that these functions aren't nearly as efficient as the block retrieval functions. In normal use,
// they should not be called often, respectively all balance list calculations should happen around the (cached)
// frozen edge.
// goOnline = false prevents download of balance lists from trusted online repo.
func (s *state) getBalanceListFromBestSource(blockHeight int64) *blockchain_data.BalanceList {
	// maybe in the cache?
	s.balanceListCacheLock.Lock()
	balanceList, ok := s.balanceListCache[blockHeight]
	s.balanceListCacheLock.Unlock()
	if ok {
		return balanceList
	}
	// from an individual file
	balanceList = s.balanceListFromIndividualFile(blockHeight)
	if balanceList != nil {
		s.balanceListCacheLock.Lock()
		s.balanceListCache[blockHeight] = balanceList
		s.balanceListCacheLock.Unlock()
		return balanceList
	}
	// from a consolidated file
	balanceList = s.balanceListFromConsolidatedFile(blockHeight)
	if balanceList != nil {
		s.balanceListCacheLock.Lock()
		s.balanceListCache[blockHeight] = balanceList
		s.balanceListCacheLock.Unlock()
		return balanceList
	}
	// only go online during chain initialization
	if !s.chainInitialized {
		balanceList = s.balanceListFromOnlineRepository(blockHeight)
		if balanceList != nil {
			s.balanceListCacheLock.Lock()
			s.balanceListCache[blockHeight] = balanceList
			s.balanceListCacheLock.Unlock()
		}
	}
	return balanceList
}

// Load a balance list from an individual file.
func (s *state) balanceListFromIndividualFile(blockHeight int64) *blockchain_data.BalanceList {
	fileName := individualFileForBlockHeight(blockHeight)
	if utilities.FileDoesNotExists(fileName) {
		// fail silently if the file does not exist, as this is a normal situation
		return nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		logging.ErrorLog.Printf("Unable to load block from file: %s.", err.Error())
		return nil
	}
	defer f.Close()
	err = message_fields.Skip(f, 2)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	_, err = blockchain_data.ReadNewBlock(f)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	balanceList, err := blockchain_data.ReadNewBalanceList(f)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	return balanceList
}

// Derive a balance list for the given height from a consolidated file.
func (s *state) balanceListFromConsolidatedFile(height int64) *blockchain_data.BalanceList {
	fileName := consolidatedFileForBlockHeight(height)
	if utilities.FileDoesNotExists(fileName) {
		// fail silently if the file does not exist, as this is a normal situation
		return nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		logging.ErrorLog.Printf("Unable to read from consolidated file %s: %s.", fileName, err.Error())
		return nil
	}
	defer f.Close()
	var returnList *blockchain_data.BalanceList
	blockCount, err := message_fields.ReadInt16(f)
	if err != nil {
		logging.ErrorLog.Print(err)
		return nil
	}
	var previousBlock *blockchain_data.Block
	for i := 0; i < int(blockCount) && (previousBlock == nil || previousBlock.Height < height); i++ {
		block, err := blockchain_data.ReadNewBlock(f)
		if err != nil {
			logging.ErrorLog.Print(err)
			return nil
		}
		if previousBlock == nil || previousBlock.Height != block.Height-1 {
			returnList, err = blockchain_data.ReadNewBalanceList(f)
			if err != nil {
				logging.ErrorLog.Print(err)
				return nil
			}
		} else {
			returnList = balance_authority.UpdateBalanceListForNextBlock(s.ctxt, previousBlock.VerifierIdentifier, returnList, block, false)
		}
		// this should not happen: target block not found within this file, but better safe than sorry
		if (i+1 == int(blockCount) && block.Height != height) || block.Height > height {
			returnList = nil
		}
		previousBlock = block
	}
	return returnList
}

// Retrieve a consolidated block file from an online repo and derive the balance list for <height>.
func (s *state) balanceListFromOnlineRepository(height int64) *blockchain_data.BalanceList {
	_ = downloadFromOnlineRepository(height)
	return s.balanceListFromConsolidatedFile(height)
}

// Load and store the online block repo file that contains the block at <height>.
func downloadFromOnlineRepository(height int64) error {
	directoryIndex := height / blocksPerFile / filesPerDirectory
	fileIndex := height / blocksPerFile
	directoryName := fmt.Sprintf(configuration.DataDirectory+"/"+configuration.BlockDirectory+"/%03d", directoryIndex)
	err := os.MkdirAll(directoryName, os.ModePerm)
	if err != nil {
		return errors.New("could not create block data directory: " + err.Error())
	}
	fileName := fmt.Sprintf(configuration.DataDirectory+"/"+configuration.BlockDirectory+"/%03d/%06d.nyzoblock", directoryIndex, fileIndex)
	onlineName := fmt.Sprintf(configuration.OnlineBlockSource+"/%06d.nyzoblock", fileIndex)
	err = utilities.DownloadFile(onlineName, fileName)
	if err != nil {
		if strings.HasSuffix(err.Error(), "403") {
			return errors.New("403")
		} else {
			return errors.New("could not download block data from remote: " + err.Error())
		}
	}
	return nil
}

// Individual block file name + path for the given height.
func individualFileForBlockHeight(height int64) string {
	return fmt.Sprintf(configuration.DataDirectory+"/"+configuration.IndividualBlockDirectory+"/i_%09d.nyzoblock", height)
}

// Derive height from individual block file name, returns -1 if an error occurs.
func blockHeightForIndividualFile(fileName string) int64 {
	fileName = strings.Trim(fileName, "i_")
	fileName = strings.Trim(fileName, ".nyzoblock")
	height, err := strconv.ParseInt(fileName, 10, 64)
	if err != nil {
		height = -1
	}
	return height
}

// Consolidated block file name + path for the given height.
func consolidatedFileForBlockHeight(height int64) string {
	fileIndex := height / blocksPerFile
	return fmt.Sprintf(consolidatedPathForBlockHeight(height)+"/%06d.nyzoblock", fileIndex)
}

// Consolidated block path for the given height.
func consolidatedPathForBlockHeight(height int64) string {
	directoryIndex := height / blocksPerFile / filesPerDirectory
	return fmt.Sprintf(configuration.DataDirectory+"/"+configuration.BlockDirectory+"/%03d", directoryIndex)
}

// Get a list of individual block files, sorted in descending order (starting with the highest block)
func getIndividualBlockFilesSorted() ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(configuration.DataDirectory + "/" + configuration.IndividualBlockDirectory + "/")
	if err == nil {
		sort.SliceStable(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		return files, nil
	} else {
		return nil, err
	}
}

// Store a block in an individual file, deriving its balance list first if we don't get it passed to us during bootstrapping.
func (s *state) storeBlock(block *blockchain_data.Block, balanceList *blockchain_data.BalanceList) {
	if balanceList == nil {
		// Bootstrapping or genesis block
		previousBlock := s.GetBlock(block.Height - 1)
		if previousBlock == nil && block.Height > 0 {
			logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to get previous block.", block.Height)
			return
		}
		if block.Height > 0 {
			balanceList = s.GetBalanceList(block.Height - 1)
			if balanceList == nil {
				logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to get balance list of previous block.", block.Height)
				return
			}
		}
		if previousBlock == nil {
			balanceList = balance_authority.UpdateBalanceListForNextBlock(s.ctxt, nil, balanceList, block, s.ctxt.RunMode() == interfaces.RunModeArchive)
		} else {
			balanceList = balance_authority.UpdateBalanceListForNextBlock(s.ctxt, previousBlock.VerifierIdentifier, balanceList, block, s.ctxt.RunMode() == interfaces.RunModeArchive)
		}
	}
	if balanceList == nil {
		logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to get balance list.", block.Height)
		return
	}
	if !bytes.Equal(block.BalanceListHash, balanceList.GetHash()) {
		output, _ := json.Marshal(balanceList)
		fmt.Println(string(output))
		logging.ErrorLog.Fatalf("Cannot store block at height %d: balance list incorrect, dumped to stdout.", block.Height)
		return
	}
	// cache
	s.blockCacheLock.Lock()
	s.blockCache[block.Height] = block
	s.blockCacheLock.Unlock()
	s.balanceListCacheLock.Lock()
	s.balanceListCache[balanceList.BlockHeight] = balanceList
	s.balanceListCacheLock.Unlock()
	if !s.chainInitialized || block.Height == 0 {
		// don't overwrite genesis block, don't write individual blocks during chain initialization
		return
	}
	out, err := os.Create(individualFileForBlockHeight(block.Height))
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to create file.", block.Height)
		return
	}
	defer out.Close()

	_, err = out.Write(message_fields.SerializeInt16(int16(1)))
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to write to file.", block.Height)
		return
	}
	_, err = out.Write(block.ToBytes())
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to write to file.", block.Height)
		return
	}
	_, err = out.Write(balanceList.ToBytes())
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot store block at height %d: unable to write to file.", block.Height)
		return
	}
}

// emit block, archive mode only
func (s *state) emitBlock(block *blockchain_data.Block) {
	message := messages.NewInternalMessage(messages.TypeInternalBlock, block)
	router.Router.RouteInternal(message)
}

// Emits cycle events (nodes joining/leaving), archive mode only.
func (s *state) emitCycleEvents(block *blockchain_data.Block) {
	var newVerifier []byte               // can only be one
	var lostVerifiers [][]byte           // can be more than one
	lostVerifiers = make([][]byte, 0, 0) // normally, we don't loose any verifiers, so no need to allocate a lot of memory
	if block.CycleInformation.NewVerifier {
		newVerifier = block.VerifierIdentifier
	}
	if block.CycleInformation.CycleLengths[0] < s.lastCycleLength ||
		((block.CycleInformation.CycleLengths[0] == s.lastCycleLength) &&
			block.CycleInformation.NewVerifier) {
		var lostStartHeight, lostEndHeight int64
		if block.CycleInformation.NewVerifier {
			// the lost verifier should have verified the block before the previous block, one cycle ago, but it didn't
			lostStartHeight = block.Height - 2 - int64(block.CycleInformation.CycleLengths[0])
		} else {
			// the lost verifier should have verified the previous block, one cycle ago, but it didn't
			lostStartHeight = block.Height - 1 - int64(block.CycleInformation.CycleLengths[0])
		}
		// unless we lost more than one verifier, lostStartHeight and lostEndHeight will be the same
		lostEndHeight = lostStartHeight - int64(s.lastCycleLength-block.CycleInformation.CycleLengths[0])
		// account for the fact that a new verifier will prolong the cycle
		if !block.CycleInformation.NewVerifier {
			lostEndHeight++
		}
		// find all lost verifier IDs
		for i := lostStartHeight; i >= lostEndHeight; i-- {
			var lostVerifierBlock *blockchain_data.Block
			lostVerifierBlock = s.GetBlock(i)
			if lostVerifierBlock != nil {
				lostVerifiers = append(lostVerifiers, lostVerifierBlock.VerifierIdentifier)
			}
		}
	}
	if newVerifier != nil {
		message := messages.NewInternalMessage(messages.TypeInternalCycleEvent, block.Height, newVerifier, nil)
		router.Router.RouteInternal(message)
		logging.TraceLog.Printf("Cycle event at height %d. New verifier joined: %s.", block.Height, identity.BytesToNyzoHex(newVerifier))
	}
	for _, verifier := range lostVerifiers {
		message := messages.NewInternalMessage(messages.TypeInternalCycleEvent, block.Height, nil, verifier)
		router.Router.RouteInternal(message)
		logging.TraceLog.Printf("Cycle event at height %d. Lost verifier %s.", block.Height, identity.BytesToNyzoHex(verifier))
	}
	s.lastCycleLength = block.CycleInformation.CycleLengths[0]
}

// Clean the block and balance list cache
func (s *state) cleanCaches() {
	if s.retentionEdgeHeight >= 2 {
		s.blockCacheLock.Lock()
		for key := range s.blockCache {
			// never delete the genesis block
			if key != 0 && key < s.retentionEdgeHeight {
				delete(s.blockCache, key)
			}
		}
		s.blockCacheLock.Unlock()
	}
	s.balanceListCacheLock.Lock()
	for key := range s.balanceListCache {
		// we allow up to 6 balance lists on the frozen edge
		if key < s.frozenEdgeHeight-5 {
			delete(s.balanceListCache, key)
		}
	}
	s.balanceListCacheLock.Unlock()
}

// A stub, to satisfy the interface requirement.
func (s *state) Start() {
	s.ctxt.WaitGroup.Done()
}

// Initialization function.
func (s *state) Initialize() error {
	// Read consolidator option.
	s.blockConsolidatorOption = strings.ToLower(s.ctxt.Preferences.Retrieve(configuration.BlockFileConsolidatorKey, configuration.BlockFileConsolidatorOptionConsolidate))
	return nil
}

// Create a block file handler.
func NewBlockFileHandler(ctxt *interfaces.Context) interfaces.BlockFileHandlerInterface {
	s := &state{}
	s.ctxt = ctxt
	s.blockCache = make(map[int64]*blockchain_data.Block)
	s.balanceListCache = make(map[int64]*blockchain_data.BalanceList)
	return s
}
