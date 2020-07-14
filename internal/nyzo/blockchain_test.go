/*
Tests on blockchain level, or, more concretely: tests that would cause circular imports if done inside of one package.
*/
package nyzo

import (
	"bytes"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"io/ioutil"
	"testing"
)

var ctxt *interfaces.Context

/*
A mockup of the cycle authority, used for some tests below.
*/
type MockCycleAuthority struct {
}

func (s *MockCycleAuthority) Start() {
}

func (s *MockCycleAuthority) Initialize() error {
	return nil
}

func (s *MockCycleAuthority) GetCurrentCycleLength() int {
	return 1500
}

func (s *MockCycleAuthority) VerifierInCurrentCycle(id []byte) bool {
	return true
}

func (s *MockCycleAuthority) GetCycleInformationForBlock(block *blockchain_data.Block) *blockchain_data.CycleInformation {
	return nil
}

func (s *MockCycleAuthority) DetermineContinuityForBlock(block *blockchain_data.Block) int {
	return 1
}

func (s *MockCycleAuthority) HasCycleAt(block *blockchain_data.Block) bool {
	return true
}

func (s *MockCycleAuthority) GetTopNewVerifier() []byte {
	return make([]byte, 32, 32)
}

func (s *MockCycleAuthority) ShouldPenalizeVerifier(verifier []byte) bool {
	return false
}

func (s *MockCycleAuthority) GetMaximumTransactionsForBlockAssembly() int {
	return 10
}

/*
Test verification of the first 10K blocks.
*/
func TestVerifyBlocks(t *testing.T) {
	var previousBlock *blockchain_data.Block
	var balanceList *blockchain_data.BalanceList
	startTime := utilities.Now()
	blocks, _ := ctxt.BlockHandler.GetBlocks(0, 9999)
	for _, block := range blocks {
		// verification of the block and any transactions it contains
		if !ctxt.BlockAuthority.BlockIsValid(block) {
			t.Errorf("Block %d could not be verified.", block.Height)
		}
		// has cycle information been calculated correctly?
		if block.CycleInformation == nil {
			t.Errorf("Could not calculate cycle information for block %d.", block.Height)
		} else {
			if block.CycleInformation.NewVerifier {
				fmt.Printf("New verifier at height %d: %s, maximum cycle length %d, cycle length: %d\n", block.Height, identity.BytesToNyzoHex(block.VerifierIdentifier), block.CycleInformation.MaximumCycleLength, block.CycleInformation.CycleLengths[0])
			}
		}
		// balance lists are not technically part of a block
		if previousBlock != nil {
			balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, balanceList, block, false)
		} else {
			balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, nil, nil, block, false)
		}
		if !bytes.Equal(balanceList.GetHash(), block.BalanceListHash) {
			t.Errorf("Derived balance list hash does not match. Want: %s, got: %s", identity.BytesToNyzoHex(block.BalanceListHash), identity.BytesToNyzoHex(balanceList.GetHash()))
		}
		previousBlock = block
	}
	fmt.Printf("Took %d milliseconds to load and verify 10'000 blocks.", utilities.Now()-startTime)
}

// Test balance list normalization.
func TestNormalizeBalanceList(t *testing.T) {
	balanceList := ctxt.BlockHandler.GetBalanceList(5451011)
	fmt.Println(len(balanceList.Items))
	if len(balanceList.Items) != 3508 {
		t.Error("Balance list has unexpected length.")
	}
	identifierList := make([][]byte, 0, len(balanceList.Items))
	for _, item := range balanceList.Items {
		identifierList = append(identifierList, item.Identifier)
	}
	// append a bogus item
	balanceList.Items = append(balanceList.Items, blockchain_data.BalanceListItem{
		Identifier:     ctxt.Identity.PublicKey,
		Balance:        0,
		BlocksUntilFee: 0,
	})
	balanceList.Normalize()
	if len(balanceList.Items) != 3508 {
		t.Error("Balance list has unexpected length after normalization.")
	}
	for i, item := range balanceList.Items {
		if !bytes.Equal(item.Identifier, identifierList[i]) {
			t.Error("Order of balance list does not match.")
		}
	}
	// kill somebody's balance
	balanceList.Items[3].Balance = 0
	// duplicate
	balanceList.Items[4].Identifier = balanceList.Items[5].Identifier
	balanceList.Normalize()
	if len(balanceList.Items) != 3506 {
		t.Error("Balance list has unexpected length after bogus data injection and normalization.")
	}
}

// Test balance list calculation.
func TestBalanceAuthority(t *testing.T) {
	previousBlock := ctxt.BlockHandler.GetBlock(5451010, nil)
	previousBalanceList := ctxt.BlockHandler.GetBalanceList(5451010)
	block := ctxt.BlockHandler.GetBlock(5451011, nil)
	balanceList := balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, previousBalanceList, block, false)
	if !bytes.Equal(block.BalanceListHash, balanceList.GetHash()) {
		t.Error("Updated balance list hash does not match.")
	}
}

// Test getting a random balance list from a file.
func TestBalanceListLoading(t *testing.T) {
	balanceList := ctxt.BlockHandler.GetBalanceList(13219)
	block := ctxt.BlockHandler.GetBlock(13219, nil)
	if !bytes.Equal(balanceList.GetHash(), block.BalanceListHash) {
		t.Error("Derived balance list hash does not match.")
	}
}

// This test is here to prove the existence - or non-existence - of the so called heisennyzo, a single
// micronyzo that would appear, seemingly randomly, after roughly 5M blocks' worth of chain calculation and
// wreak havoc in every subsequent balance list update. This test will pass if the heisennyzo no longer
// appears - or will it pass when it appears?
//
// Edit: in the end, it was a trivial comparison operator bug, >100 instead of >=100, so when fees in a block would
// be exactly 100 micronyzo, the balance list calculation would diverge from Java's. The bug was hard to find because
// it was rare and we had to find the root block of the issue first.
func TestSchrodingersCat(t *testing.T) {
	balanceList := ctxt.BlockHandler.GetBalanceList(4640100)
	previousBlock := ctxt.BlockHandler.GetBlock(4640100, nil)
	// this block triggers the heisennyzo for the first time
	thisBlock := ctxt.BlockHandler.GetBlock(4640101, nil)
	balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, balanceList, thisBlock, false)
	if !bytes.Equal(balanceList.GetHash(), thisBlock.BalanceListHash) {
		t.Errorf("Derived balance list hash does not match. Want: %s, got: %s", identity.BytesToNyzoHex(thisBlock.BalanceListHash), identity.BytesToNyzoHex(balanceList.GetHash()))
	}
}

// Test transition from V0 to V1 blockchain.
func TestV0V1Transition(t *testing.T) {
	var previousBlock *blockchain_data.Block
	var balanceList *blockchain_data.BalanceList
	blocks, _ := ctxt.BlockHandler.GetBlocks(4499990, 4500300)
	balanceList = ctxt.BlockHandler.GetBalanceList(4499989)
	previousBlock = ctxt.BlockHandler.GetBlock(4499989, nil)
	for _, block := range blocks {
		// verification of the block and any transactions it contains
		if !ctxt.BlockAuthority.BlockIsValid(block) {
			t.Errorf("Block %d could not be verified.", block.Height)
		}
		// has cycle information been calculated correctly?
		if block.CycleInformation == nil {
			t.Errorf("Could not calculate cycle information for block %d.", block.Height)
		} else {
			if block.CycleInformation.NewVerifier {
				fmt.Printf("New verifier at height %d: %s, maximum cycle length %d, cycle length: %d\n", block.Height, identity.BytesToNyzoHex(block.VerifierIdentifier), block.CycleInformation.MaximumCycleLength, block.CycleInformation.CycleLengths[0])
			}
		}
		// balance lists are not technically part of a block
		balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, balanceList, block, false)
		if !bytes.Equal(balanceList.GetHash(), block.BalanceListHash) {
			t.Errorf("Derived balance list hash does not match. Want: %s, got: %s", identity.BytesToNyzoHex(block.BalanceListHash), identity.BytesToNyzoHex(balanceList.GetHash()))
		}
		previousBlock = block
	}
}

// Test handling of V2 balance list with signatures. Part 1: 1st signature ever.
func TestV2BalanceList1(t *testing.T) {
	// keep a copy of the real cycle authority in case some tests need it
	realCycleAuthority := ctxt.CycleAuthority
	// add a mock cycle authority
	ctxt.CycleAuthority = &MockCycleAuthority{}
	// test balance list calculation and serialization
	balanceList := ctxt.BlockHandler.GetBalanceList(7034301)
	previousBlock := ctxt.BlockHandler.GetBlock(7034301, nil)
	thisBlock := ctxt.BlockHandler.GetBlock(7034302, nil)
	balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, balanceList, thisBlock, false)
	ctxt.CycleAuthority = realCycleAuthority
	if !bytes.Equal(balanceList.GetHash(), thisBlock.BalanceListHash) {
		t.Errorf("Derived balance list hash does not match. Want: %s, got: %s", identity.BytesToNyzoHex(thisBlock.BalanceListHash), identity.BytesToNyzoHex(balanceList.GetHash()))
	}
}

// Test handling of V2 balance list with signatures. Part 2: 1st block with previous signatures.
func TestV2BalanceList2(t *testing.T) {
	// keep a copy of the real cycle authority in case some tests need it
	realCycleAuthority := ctxt.CycleAuthority
	// add a mock cycle authority
	ctxt.CycleAuthority = &MockCycleAuthority{}
	// test balance list calculation and serialization
	balanceList := ctxt.BlockHandler.GetBalanceList(7034302)
	previousBlock := ctxt.BlockHandler.GetBlock(7034302, nil)
	thisBlock := ctxt.BlockHandler.GetBlock(7034303, nil)
	balanceList = balance_authority.UpdateBalanceListForNextBlock(ctxt, previousBlock.VerifierIdentifier, balanceList, thisBlock, false)
	ctxt.CycleAuthority = realCycleAuthority
	if !bytes.Equal(balanceList.GetHash(), thisBlock.BalanceListHash) {
		t.Errorf("Derived balance list hash does not match. Want: %s, got: %s", identity.BytesToNyzoHex(thisBlock.BalanceListHash), identity.BytesToNyzoHex(balanceList.GetHash()))
	}
}

// Test loading a block, deriving its balance list, and storing it again.
func TestStoreBlock(t *testing.T) {
	block := ctxt.BlockHandler.GetBlock(5381633, nil)
	ctxt.BlockHandler.CommitFrozenEdgeBlock(block, nil)
	raw1, _ := ioutil.ReadFile("../../test/test_data/blocks/individual/i_005381633.nyzoblock")
	raw2, _ := ioutil.ReadFile("../../test/test_data/blocks/individual/i_005381633.nyzoblock_bak")
	if !bytes.Equal(raw1, raw2) {
		t.Error("Newly stored block file does not match.")
	}
}

// Test calculating cycle information. This will take a while.
func TestCycleInformation(t *testing.T) {
	fmt.Println("Calculating cycle information for block 7200000, this will take a while.")
	block := ctxt.BlockHandler.GetBlock(7200000, nil)
	cycleInformation := ctxt.CycleAuthority.GetCycleInformationForBlock(block)
	if cycleInformation == nil {
		t.Error("Could not calculate cycle information for block 7200000.")
	}
}

func init() {
	configuration.DataDirectory = "../../test/test_data"
	ctxt = NewDefaultContext()
	err := configuration.EnsureSetup()
	if err != nil {
		logging.ErrorLog.Fatal(err.Error())
	}
	ContextInitialize(ctxt)
}
