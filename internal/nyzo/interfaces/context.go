/*
Context summarizes the various components of a node.

The struct presented here is extended with a number of simple utility functions for the RunMode. Placing the
RunMode here is more convenient than handling it like the more complex components.
*/
package interfaces

import (
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"sync"
	"sync/atomic"
)

// The blockchain context. See Readme.
type Context struct {
	runMode            int32
	WaitGroup          sync.WaitGroup
	Identity           *identity.Identity
	PersistentData     KeyValueStoreInterface
	Preferences        KeyValueStoreInterface
	BlockHandler       BlockHandlerInterface
	CycleAuthority     CycleAuthorityInterface
	BlockAuthority     BlockAuthorityInterface
	TransactionManager TransactionManagerInterface
	MeshListener       MeshListenerInterface
	NodeManager        NodeManagerInterface
	DataStore          Component
	Api                Component
}

const (
	// Nyzo style Verifier.
	RunModeVerifier = 0
	// Nyzo style Sentinel.
	RunModeSentinel = 1
	// Nyzo style Client.
	RunModeClient = 2
	// Nyzo style Micropay Client.
	RunModeMicropayClient = 3
	// Nyzo style Micropay Server.
	RunModeMicropayServer = 4
	// Nyzo style Documentation Server.
	RunModeDocumentationServer = 5
	// Go style Archive Node.
	RunModeArchive = 100
)

// This can be called whenever needed.
func (ctxt *Context) RunMode() int32 {
	return atomic.LoadInt32(&ctxt.runMode)
}

// Run mode should only be set during startup. Default: verifier.
func (ctxt *Context) SetRunMode(runMode int32) {
	logging.InfoLog.Printf("*** setting run mode %s for version %s ***", getRunModeName(int(runMode)), configuration.Version)
	atomic.StoreInt32(&ctxt.runMode, runMode)
}

// Nyzo style ID (aka "override suffix") for run modes.
func GetRunModeId(runMode int32) string {
	switch runMode {
	case RunModeVerifier:
		return "verifier"
	case RunModeSentinel:
		return "sentinel"
	case RunModeClient:
		return "client"
	case RunModeMicropayClient:
		return "micropay_client"
	case RunModeMicropayServer:
		return "micropay_server"
	case RunModeDocumentationServer:
		return "documentation_server"
	case RunModeArchive:
		return "archive"
	default:
		return "unknown"
	}
}

// Get a name for the given run mode.
func getRunModeName(runMode int) string {
	switch runMode {
	case RunModeVerifier:
		return "Verifier"
	case RunModeSentinel:
		return "Sentinel"
	case RunModeClient:
		return "Client"
	case RunModeMicropayClient:
		return "Micropay Client"
	case RunModeMicropayServer:
		return "Micropay Server"
	case RunModeDocumentationServer:
		return "Documentation Server"
	case RunModeArchive:
		return "Archive"
	default:
		return "Unknown"
	}
}
