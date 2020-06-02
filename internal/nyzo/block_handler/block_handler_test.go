package block_handler

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"io"
	"os"
	"testing"
)

var ctxt interfaces.Context

func TestIndividualFileHeight(t *testing.T) {
	height := FindHighestIndividualBlockFile()
	if height != 5451012 {
		t.Error("Unexpected highest indiviual block file height.")
	}
}

func TestLoadIndividualBlock(t *testing.T) {
	block := ctxt.BlockHandler.GetBlock(5381631, nil)
	if identity.BytesToNyzoHex(block.Hash) != "b78bed638247dfa8-836c3f7dbb69f71f-4515f3341a800b51-d7f1ed0b6260a350" {
		t.Error("Block hash does not match.")
	}
	if identity.BytesToNyzoHex(block.VerifierSignature) != "8d977c9774ad297f-6efb0d97096d3000-3e869770c6a4ce91-d6264336f9490a3c-e6a7430def015b57-3f955ce6d05a9729-d4e3f34cf83b8fb4-38432aac401aa30f" {
		t.Error("Block signature does not match.")
	}
	if identity.BytesToNyzoHex(block.Transactions[0].Signature) != "d62e3106d9cbbb6b-01649398ed8e3fd9-a1b57ce36fc1eb43-c386c96c023d04aa-6f3acb69f9402388-0ea5611a9149179c-7103ca82fbee25a5-e666e11cde6f2902" {
		t.Error("Signature of 1st transaction does not match.")
	}
}

func TestLoadHistoricalBlock(t *testing.T) {
	block := ctxt.BlockHandler.GetBlock(5348050, nil)
	if identity.BytesToNyzoHex(block.VerifierSignature) != "edfd1fb015d996f9-1f2d52b905bd446a-ca6b818c778f2608-8bffb45362601194-1f39e3e3017d571d-53586799bc9e0b09-93edb58435f13c9c-6477aa1109bd790a" {
		t.Error("Block signature does not match.")
	}
}

func TestLoadIndividualBlocks(t *testing.T) {
	blocks, _ := ctxt.BlockHandler.GetBlocks(5381631, 5381633)
	if identity.BytesToNyzoHex(blocks[2].VerifierSignature) != "ad592cf2f242e83d-04bbe0fcc71bb1a3-9afd01b204d4ddca-f1bbe3dccf56aed9-3fc28d896e3216e6-c2f6bcf129bf0333-e5580e983d0b485f-02aba98dd9c5e20a" {
		t.Error("Block signature does not match.")
	}
}

func TestLoadHistoricalBlocks(t *testing.T) {
	// spanning two archive files
	blocks, _ := ctxt.BlockHandler.GetBlocks(5347980, 5348020)
	if identity.BytesToNyzoHex(blocks[29].VerifierSignature) != "a3cd5b9b69b2e47d-2e1998d06b156467-3108b83f2176c619-7b1c3566880775d9-60db36e74c226256-992d55408aba5784-d845fe36555fd9fa-6fe06d95fbf80003" {
		t.Error("Block signature does not match.")
	}
}

func TestLoadHistoricalOnlineBlocks(t *testing.T) {
	_ = os.Remove("../../test/test_data/blocks/000/000011.nyzoblock")
	_ = os.Remove("../../test/test_data/blocks/000/000012.nyzoblock")

	// spanning two archive files
	blocks, _ := ctxt.BlockHandler.GetBlocks(11990, 12010)
	if identity.BytesToNyzoHex(blocks[15].VerifierSignature) != "d9b9b6ee197d4186-b6f507688f6feb30-079f84794fa2d53e-56c9a59f03b772b9-e32e9ab0731b490c-cabb2774795961ba-cc85db307ad3d56e-50f726be4d84a609" {
		t.Error("Block signature does not match.")
	}
}

// Calculate a sha256 checksum for the given file.
func fileChecksum(fileName string) string {
	hash := sha256.New()
	f, err := os.Open(fileName)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := io.Copy(hash, f); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type mockCycleAuthorityState struct {
}

func (s *mockCycleAuthorityState) GetCurrentCycleLength() int {
	return 1800
}
func (s *mockCycleAuthorityState) VerifierInCurrentCycle(id []byte) bool {
	return true
}
func (s *mockCycleAuthorityState) GetCycleInformationForBlock(block *blockchain_data.Block) *blockchain_data.CycleInformation {
	return nil
}
func (s *mockCycleAuthorityState) DetermineContinuityForBlock(block *blockchain_data.Block) int {
	return 0
}
func (s *mockCycleAuthorityState) HasCycleAt(block *blockchain_data.Block) bool {
	return true
}
func (s *mockCycleAuthorityState) Initialize() error {
	return nil
}
func (s *mockCycleAuthorityState) Start() {

}
func (s *mockCycleAuthorityState) GetTopNewVerifier() []byte {
	return make([]byte, 32, 32)
}
func (s *mockCycleAuthorityState) ShouldPenalizeVerifier(verifier []byte) bool {
	return false
}

func TestBlockConsolidation(t *testing.T) {
	// extract a consolidated block file
	ctxt = interfaces.Context{}
	ctxt.CycleAuthority = &mockCycleAuthorityState{}
	s := &state{}
	s.ctxt = &ctxt
	s.blockCache = make(map[int64]*blockchain_data.Block)
	s.balanceListCache = make(map[int64]*blockchain_data.BalanceList)
	s.retentionEdgeHeight = 5000000
	s.chainInitialized = true
	blocks, _ := s.GetBlocks(2001000, 2001000+blocksPerFile-1)
	for _, block := range blocks {
		s.storeBlock(block, nil)
	}
	// get file checksum
	consolidatedFileName := consolidatedFileForBlockHeight(2001000)
	checksum := fileChecksum(consolidatedFileName + ".bak")
	if len(checksum) == 0 {
		t.Error("Could not calculate checksum for initial consolidated file.")
	}
	err := os.Remove(consolidatedFileName)
	if err != nil {
		t.Error("Could not delete initial consolidated block file.")
	}
	s.consolidateBlockFiles()
	checksum2 := fileChecksum(consolidatedFileName)
	if checksum != checksum2 {
		t.Error("Checksum on consolidated block file does not match.")
	}
}

func init() {
	ctxt = interfaces.Context{}
	configuration.DataDirectory = "../../../test/test_data"
	ctxt.BlockHandler = NewBlockHandler(&ctxt)
}
