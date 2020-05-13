/*
Receives messages emitted in archive mode and writes them to a database. This is the only package that would
have to be changed to support another type of data store.
*/
package data_store

import (
	"database/sql"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/node"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	_ "github.com/go-sql-driver/mysql"
	"strings"
)

type mySqlState struct {
	ctxt                   *interfaces.Context
	internalMessageChannel chan *messages.InternalMessage // channel for internal and local messages
	db                     *sql.DB
	addTransaction         *sql.Stmt
	addCycleEvent          *sql.Stmt
	addBlock               *sql.Stmt
	addNodeStatus          *sql.Stmt
	dbHeight               int64
}

type dbTransaction struct {
	Height            int64  `db:"height"`
	Canonical         int32  `db:"canonical"`
	Timestamp         int64  `db:"timestamp"`
	Type              byte   `db:"type"`
	Sender            []byte `db:"sender"`
	Recipient         []byte `db:"recipient"`
	Data              []byte `db:"data"`
	Amount            *int64 `db:"amount"`
	AmountAfterFees   *int64 `db:"amount_after_fees"`
	SenderBalance     *int64 `db:"sender_balance"`
	RecipientBalance  *int64 `db:"recipient_balance"`
	Signature         []byte `db:"signature"`
	Vote              *byte  `db:"vote"`
	OriginalSignature []byte `db:"original_signature"`
}

// Process a transaction message: store the transaction to the database.
func (s *mySqlState) processTransactionMessage(m *messages.InternalMessage) {
	t := &dbTransaction{}
	t.Height = m.Payload[0].(int64)
	t.Canonical = m.Payload[1].(int32)
	t.Timestamp = m.Payload[2].(int64)
	t.Type = m.Payload[3].(byte)
	if m.Payload[4] != nil {
		t.Sender = m.Payload[4].([]byte)
	}
	if m.Payload[5] != nil {
		t.Recipient = m.Payload[5].([]byte)
	}
	if m.Payload[6] != nil {
		t.Data = m.Payload[6].([]byte)
	}
	if m.Payload[7] != nil {
		t.Amount = m.Payload[7].(*int64)
	}
	if m.Payload[8] != nil {
		t.AmountAfterFees = m.Payload[8].(*int64)
	}
	if m.Payload[9] != nil {
		t.SenderBalance = m.Payload[9].(*int64)
	}
	if m.Payload[10] != nil {
		t.RecipientBalance = m.Payload[10].(*int64)
	}
	if m.Payload[11] != nil {
		t.Signature = m.Payload[11].([]byte)
	}
	if m.Payload[12] != nil {
		t.Vote = m.Payload[12].(*byte)
	}
	if m.Payload[13] != nil {
		t.OriginalSignature = m.Payload[13].([]byte)
	}
	_, err := s.addTransaction.Exec(t.Height, t.Canonical, t.Timestamp, t.Type, t.Sender, t.Recipient, t.Data, t.Amount, t.AmountAfterFees, t.SenderBalance, t.RecipientBalance, t.Signature, t.Vote, t.OriginalSignature)
	if err != nil {
		logging.ErrorLog.Fatal(err)
	}

}

// Process a cycle event message: store the cycle event to the database.
func (s *mySqlState) processCycleEventMessage(m *messages.InternalMessage) {
	var joined, left []byte
	height := m.Payload[0].(int64)
	if m.Payload[1] != nil {
		joined = m.Payload[1].([]byte)
	}
	if m.Payload[2] != nil {
		left = m.Payload[2].([]byte)
	}
	var err error
	if joined != nil {
		_, err = s.addCycleEvent.Exec(height, joined, 1)
	}
	if err != nil {
		logging.ErrorLog.Fatal(err)
	}
	if left != nil {
		_, err = s.addCycleEvent.Exec(height, left, 0)
	}
	if err != nil {
		logging.ErrorLog.Fatal(err)
	}
}

// Process a block message: store the block to the database.
func (s *mySqlState) processBlockMessage(m *messages.InternalMessage) {
	block := m.Payload[0].(*blockchain_data.Block)
	// store genesis block only once
	if block.Height == 0 && s.dbHeight > 0 {
		return
	}
	// don't allow the block database to become discontinuous
	if block.Height != 0 && block.Height != s.dbHeight+1 {
		if block.Height != s.dbHeight {
			logging.ErrorLog.Fatalf("Attempt to store discontinuous block in database, height: %d.", block.Height)
		}
		return
	}
	_, err := s.addBlock.Exec(block.BlockchainVersion, block.Height, block.Hash, block.PreviousBlockHash, block.CycleInformation.CycleLengths[0], block.StartTimestamp, block.VerificationTimestamp, len(block.Transactions), block.BalanceListHash, block.VerifierIdentifier, block.VerifierSignature)
	s.dbHeight = block.Height
	if err != nil {
		logging.ErrorLog.Fatal(err)
	}
}

// Process a node status message. This has informational purposes only.
func (s *mySqlState) processNodeStatusMessage(m *messages.InternalMessage) {
	status := m.Payload[0].(*node.Status)
	_, err := s.addNodeStatus.Exec(status.Identifier, status.Timestamp, status.Ip, status.Nickname, status.Version, status.MeshTotal, status.MeshCycle, status.CycleLength, status.Transactions, status.RetentionEdge, status.TrailingEdge, status.FrozenEdge, status.OpenEdge, status.BlocksTransmitted, status.BlocksCreated, status.VoteStats, status.LastJoinHeight, status.LastRemovalHeight, status.ReceivingUDP)
	if err != nil {
		// We do not make this fatal as it is not consensus relevant.
		logging.ErrorLog.Print(err)
	}
}

// Main loop.
func (s *mySqlState) Start() {
	defer logging.InfoLog.Print("Main loop of MySql data store exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	logging.InfoLog.Print("Starting main loop of MySql data store.")
	// Emit the currently highest height in the DB, the block file handler will pick this up and boostrap from there.
	message := messages.NewInternalMessage(messages.TypeInternalDataStoreHeight, s.dbHeight)
	router.Router.RouteInternal(message)
	done := false
	for !done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalTransaction:
				s.processTransactionMessage(m)
			case messages.TypeInternalCycleEvent:
				s.processCycleEventMessage(m)
			case messages.TypeInternalBlock:
				s.processBlockMessage(m)
			case messages.TypeInternalNodeStatus:
				s.processNodeStatusMessage(m)
			case messages.TypeInternalExiting:
				_ = s.db.Close()
				done = true
			}
		}
	}

}

// Initialization function
func (s *mySqlState) Initialize() error {
	var err error
	// set message routes
	s.internalMessageChannel = make(chan *messages.InternalMessage, 5000)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalTransaction, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalCycleEvent, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalBlock, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalNodeStatus, s.internalMessageChannel)
	// connect to database
	sqlUser := s.ctxt.Preferences.Retrieve(configuration.SqlUserKey, "")
	sqlPass := s.ctxt.Preferences.Retrieve(configuration.SqlPasswordKey, "")
	sqlProtocol := s.ctxt.Preferences.Retrieve(configuration.SqlProtocolKey, "")
	sqlHost := s.ctxt.Preferences.Retrieve(configuration.SqlHostKey, "")
	sqlPort := s.ctxt.Preferences.Retrieve(configuration.SqlPortKey, "")
	sqlDbName := s.ctxt.Preferences.Retrieve(configuration.SqlDbNameKey, "")
	sqlConnection := ""
	sqlConnectionClean := ""
	if strings.TrimSpace(sqlPort) == "" {
		sqlConnection = fmt.Sprintf("%s:%s@%s(%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci", sqlUser, sqlPass, sqlProtocol, sqlHost, sqlDbName)
		sqlConnectionClean = fmt.Sprintf("%s:%s@%s(%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci", sqlUser, "<PASS>", sqlProtocol, sqlHost, sqlDbName)
	} else {
		sqlConnection = fmt.Sprintf("%s:%s@%s(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci", sqlUser, sqlPass, sqlProtocol, sqlHost, sqlPort, sqlDbName)
		sqlConnectionClean = fmt.Sprintf("%s:%s@%s(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci", sqlUser, "<PASS>", sqlProtocol, sqlHost, sqlPort, sqlDbName)
	}
	logging.InfoLog.Printf("Connecting to database: %s.", sqlConnectionClean)
	s.db, err = sql.Open("mysql", sqlConnection)
	if err != nil {
		return err
	}
	// create tables if needed
	_, err = s.db.Exec(createTransactionsTableStatement)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(createBlocksTableStatement)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(createCycleEventsTableStatement)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(createNodeStatusTableStatement)
	if err != nil {
		return err
	}
	// prepare DB statements
	s.addTransaction, err = s.db.Prepare(addTransactionStatement)
	if err != nil {
		return err
	}
	s.addBlock, err = s.db.Prepare(addBlockStatement)
	if err != nil {
		return err
	}
	s.addCycleEvent, err = s.db.Prepare(addCycleEventStatement)
	if err != nil {
		return err
	}
	s.addNodeStatus, err = s.db.Prepare(addNodeStatusStatement)
	if err != nil {
		return err
	}
	// get highest block in DB
	var height sql.NullInt64
	err = s.db.QueryRow(highestBlockStatement).Scan(&height)
	if err != nil {
		return err
	} else {
		if height.Valid {
			s.dbHeight = height.Int64
		} else {
			s.dbHeight = 0
		}
	}
	return err
}

// Create a MySql data store.
func NewMysqlDataStore(ctxt *interfaces.Context) interfaces.Component {
	s := &mySqlState{}
	s.ctxt = ctxt
	return s
}
