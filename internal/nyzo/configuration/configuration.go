/*
This package will eventually consolidate all data relevant to the genesis of one particular Nyzo chain:
The amount of coins in the system, block time, the genesis block, the trusted entry points etc.
The idea is, that to start a new chain with Nyzo characteristics, the contents of this package are the only things
you'd have to change.
*/
package configuration

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"os"
	"path"
	"runtime"
	"strings"
)

const (
	// Software version
	Version = "go-587"
	// Comment from original Java version regarding the monetary system:
	// We want this to be a functioning monetary system. The maximum number of coins is 100 million. The fraction used
	// for dividing coins is 1 million (all transactions must be a whole-number multiple of 1/1000000 coins).

	// If we have a coin value of $1 = ∩1, then the transaction increment is one-ten-thousandth of a cent, and the
	// market cap is $100 million. If we have a coin value of $100,000, then the transaction increment is $0.10,
	// and the market cap is $10 trillion.
	NyzosInSystem                 = 100000000
	MicronyzoMultiplierRatio      = 1000000
	MicronyzosInSystem            = NyzosInSystem * MicronyzoMultiplierRatio
	MaximumCycleTransactionAmount = 100000 * MicronyzoMultiplierRatio
	// Charge an account maintenance fee every ... blocks
	BlocksBetweenFee = 500
	// Spam prevention, transactions must not create accounts with less than x micronyzo.
	MinimumPreferredBalance = 10 * MicronyzoMultiplierRatio
	// Transaction rate limiting. The baseline transaction rate is 10 transactions per second, which is 70 transactions per block.
	BaselineTransactionRate      = 10
	BaselineTransactionsPerBlock = BlockDuration * BaselineTransactionRate / 1000

	MinimumVerificationInterval = 1500
	MinimumBlockchainVersion    = 0
	MaximumBlockchainVersion    = 2

	// v2 cycle transaction caps
	ApprovedCycleTransactionRetentionInterval = 10000
	MaximumCycleTransactionSumPerInterval     = 100000 * MicronyzoMultiplierRatio

	// Target block duration in milliseconds
	BlockDuration = 7000

	// Sentinel automatic whitelisting interval, in milliseconds (10 minutes)
	DynamicWhitelistInterval = 1000 * 60 * 10

	// Data storage location and file names (this is assigned to a var below, so that we can change it for testing.
	//dataDirectory          = "test/test_data_directory"
	dataDirectory            = "/var/lib/nyzo/production"
	PrivateKeyFileName       = "verifier_private_seed"
	VerifierInfoFileName     = "verifier_info"
	NicknameFileName         = "nickname"
	EntryPointFileName       = "trusted_entry_points"
	ManagedVerifiersFileName = "managed_verifiers"
	NodesFileName            = "nodes"
	PersistentDataFileName   = "persistent_data"
	PreferencesFileName      = "preferences"
	BlockDirectory           = "blocks"
	IndividualBlockDirectory = "blocks/individual"
	GenesisBlockFileName     = "i_000000000.nyzoblock"
	LockedAccountsFileName   = "locked_accounts"
	SeedTransactionDirectory = "seed_transactions"
	SeedTransactionSource    = "https://seed.nyzo.co/seedTransactions"

	// The public id of the genesis block verifier
	GenesisVerifierNyzoHex = "64afc20a4a4097e8-494239f2e7d1b1db-de59a9b157453138-f4716b72a0424fef"
	// Not sure what this account is/was used for, but we have to reproduce it to be compatible with the Java version
	TransferAccountNyzoHex = "0000000000000000-0000000000000000-0000000000000000-0000000000000001"
	// Imaginary account (no known private key) for cycle funding
	CycleAccountNyzoHex = "0000000000000000-0000000000000000-0000000000000000-0000000000000002"
	// Seed funding account id.
	SeedAccountNyzoHex = "12d454a69523f739-eb5eb71c7deb8701-1804df336ae0e2c1-9e0b24a636683e31"

	// How long a new verifier has to wait until it enters the lottery, in milliseconds (30 days)
	LotteryWaitTime = 1000 * 60 * 60 * 24 * 30

	// Online block source
	OnlineBlockSource = "https://nyzo-blocks.nyc3.digitaloceanspaces.com"

	// Listening ports
	ListeningPortTcp = 9444
	ListeningPortUdp = 9446

	// Keys for persistent data key value store
	HaveNodeHistoryKey             = "have_node_history"
	WinningIdentifierKey           = "winning_identifier"
	LastVerifierJoinHeightKey      = "last_verifier_join_height"
	LastVerifierRemovalHeightKey   = "last_verifier_removal_height"
	LastBlockTransmissionHeightKey = "sentinel_last_block_transmission_height"
	LastBlockTransmissionInfoKey   = "sentinel_last_block_transmitted"
	LastBlockTransmissionResultKey = "sentinel_last_block_transmission_results"
	ApiEnabledKey                  = "api_enabled"

	// Keys for user preferences
	SqlProtocolKey                         = "sql_protocol"
	SqlHostKey                             = "sql_host"
	SqlPortKey                             = "sql_port"
	SqlDbNameKey                           = "sql_db_name"
	SqlUserKey                             = "sql_user"
	SqlPasswordKey                         = "sql_password"
	BlockFileConsolidatorKey               = "block_file_consolidator"
	BlockFileConsolidatorOptionConsolidate = "consolidate"
	BlockFileConsolidatorOptionDelete      = "delete"
	BlockFileConsolidatorOptionDisable     = "disable"
	FastChainInitializationKey             = "fast_chain_initialization"
)

var DataDirectory string
var LockedAccounts map[string]struct{}
var GenesisVerifier []byte
var TransferAccount []byte
var CycleAccount []byte
var SeedAccount []byte

// Make sure that our environment is set up correctly and that we have all the info needed to start a node.
func EnsureSetup() error {
	_, filename, _, ok := runtime.Caller(0)
	var myDir string
	if ok {
		myDir = path.Dir(filename)
	} else {
		return errors.New("could not determine config package location")
	}
	// Create data directory
	err := os.MkdirAll(DataDirectory, os.ModePerm)
	if err != nil {
		return errors.New("could not create data directory: " + err.Error())
	}
	// Ensure we have a node identity
	if utilities.FileDoesNotExists(DataDirectory + "/" + PrivateKeyFileName) {
		id, err := identity.New(DataDirectory+"/"+PrivateKeyFileName, DataDirectory+"/"+VerifierInfoFileName)
		if err != nil {
			return err
		}
		// Even write a nickname file (not really necessary tbh)
		if utilities.FileDoesNotExists(DataDirectory + "/" + NicknameFileName) {
			err = utilities.StringToFile(id.ShortId, DataDirectory+"/"+NicknameFileName)
		}
		if err != nil {
			return errors.New("could not create nickname file: " + err.Error())
		}
	}
	// Copy trusted entry points file if needed
	if utilities.FileDoesNotExists(DataDirectory + "/" + EntryPointFileName) {
		err = utilities.CopyFile(myDir+"/"+EntryPointFileName, DataDirectory+"/"+EntryPointFileName)
	}
	if err != nil {
		return errors.New("could not copy trusted entry points data: " + err.Error())
	}
	// Create block directory
	err = os.MkdirAll(DataDirectory+"/"+BlockDirectory, os.ModePerm)
	if err != nil {
		return errors.New("could not create block data directory: " + err.Error())
	}
	// Create individual block directory
	err = os.MkdirAll(DataDirectory+"/"+IndividualBlockDirectory, os.ModePerm)
	if err != nil {
		return errors.New("could not create block data directory: " + err.Error())
	}
	// Copy genesis block if needed
	if utilities.FileDoesNotExists(DataDirectory + "/" + IndividualBlockDirectory + "/" + GenesisBlockFileName) {
		err = utilities.CopyFile(myDir+"/"+GenesisBlockFileName, DataDirectory+"/"+IndividualBlockDirectory+"/"+GenesisBlockFileName)
	}
	if err != nil {
		return errors.New("could not copy genesis block: " + err.Error())
	}
	return nil
}

// Returns true if id is in the locked accounts list
func IsLockedAccount(id []byte) bool {
	s := identity.BytesToNyzoHex(id)
	_, ok := LockedAccounts[s]
	return ok
}

// Load trusted entry points from configuration file
func loadLockedAccounts() {
	LockedAccounts = make(map[string]struct{})
	_, filename, _, ok := runtime.Caller(0)
	var myDir string
	if ok {
		myDir = path.Dir(filename)
	} else {
		return
	}
	fileName := myDir + "/" + LockedAccountsFileName
	f, err := os.Open(fileName)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := fmt.Sprintln(scanner.Text())
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			LockedAccounts[line] = struct{}{}
		}
	}
}

func ActivationHeight(version int16) int64 {
	activationHeights := []int64{0, 4500000, 7000000}
	if version < 0 || version >= int16(len(activationHeights)) {
		return 9223372036854775807
	} else {
		return activationHeights[version]
	}
}

// This is only used to lightly penalize blocks that are not upgrades when an upgrade is allowed. This gives
// preference to the upgrade block, if provided.
func IsMissedUpgradeOpportunity(blockHeight int64, blockVersion int16, previousBlockVersion int16) bool {
	if blockHeight >= ActivationHeight(blockVersion+1) &&
		blockVersion == previousBlockVersion &&
		blockVersion < MaximumBlockchainVersion &&
		blockHeight%50 == 0 {
		return true
	}
	return false
}

// This is used to penalize upgrade blocks that are submitted at non-preferred times (too early, or too late).
func IsImproperlyTimedUpgrade(blockHeight int64, blockVersion int16, previousBlockVersion int16) bool {
	if blockVersion > previousBlockVersion && (blockHeight < ActivationHeight(blockVersion) || blockHeight%50 == 0) {
		return true
	}
	return false
}

func init() {
	DataDirectory = dataDirectory
	GenesisVerifier, _ = identity.NyzoHexToBytes([]byte(GenesisVerifierNyzoHex), 32)
	TransferAccount, _ = identity.NyzoHexToBytes([]byte(TransferAccountNyzoHex), 32)
	CycleAccount, _ = identity.NyzoHexToBytes([]byte(CycleAccountNyzoHex), 32)
	SeedAccount, _ = identity.NyzoHexToBytes([]byte(SeedAccountNyzoHex), 32)
	loadLockedAccounts()
}
