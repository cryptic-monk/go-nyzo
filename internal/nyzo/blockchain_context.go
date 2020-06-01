/*
How a Nyzo blockchain context works. See "interfaces" for a better understanding of the interfaces used.

1) Creation

Easiest way: call NewDefaultContext, the New... generators for the individual components called by this function
are not supposed to do much except allocate the data structures they need later (and saving a reference to the context).

If you are feeling fancy, you can build your own context from scratch, or you can make a default one and then change some of the
components in there, e.g. if you only need a "light" node manager, you can replace the one in the context you just created
by a "light" one. Or, maybe, for a test, you just need a context with Configuration and whichever component you want to test.

2) Initialization

Easiest way: call ContextInitialize. Initialization functions are called in sequence, starting with the Configuration component,
so they provide a deterministic way to set up basic stuff without having to worry about concurrency. Any error returned here
will terminate the program.

3) Start

Easiest way: call ContextStart. Everything is started in parallel, so from here on we have to be threadsafe. Some
components will do this via some kind of mutex locking, some will use a central messaging loop.

4) Graceful Shutdown

Whenever a goroutine is started anywhere in the blockchain context, it should be added to the context's WaitGroup
`ctxt.WaitGroup.Add(1)`, and any routine called as a goroutine should `defer ctxt.WaitGroup.Done()`. The main loop
(of the verifier, sentinel etc.) just has to `ctxt.WaitGroup.Wait()`. Wait for interrupt (here) waits for an interrupt
signal and sends a shutdown message to any long-running messaging loops.

*/
package nyzo

import (
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/block_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/block_handler"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/cycle_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/key_value_store"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/mesh_listener"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/node_manager"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/transaction_manager"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"os"
	"os/signal"
)

func NewDefaultContext() *interfaces.Context {
	ctxt := interfaces.Context{}
	ctxt.PersistentData = key_value_store.NewKeyValueStore(configuration.DataDirectory+"/"+configuration.PersistentDataFileName, ctxt.WaitGroup)
	ctxt.Preferences = key_value_store.NewKeyValueStore(configuration.DataDirectory+"/"+configuration.PreferencesFileName, ctxt.WaitGroup)
	ctxt.BlockHandler = block_handler.NewBlockHandler(&ctxt)
	ctxt.CycleAuthority = cycle_authority.NewCycleAuthority(&ctxt)
	ctxt.BlockAuthority = block_authority.NewBlockAuthority(&ctxt)
	ctxt.TransactionManager = transaction_manager.NewTransactionManager(&ctxt)
	ctxt.MeshListener = mesh_listener.NewMeshListener(&ctxt)
	ctxt.NodeManager = node_manager.NewNodeManager(&ctxt)
	return &ctxt
}

// Load the node's identity.
func loadIdentity(ctxt *interfaces.Context) error {
	id, err := identity.FromPrivateKeyFile(configuration.DataDirectory + "/" + configuration.PrivateKeyFileName)
	if err == nil {
		ctxt.Identity = id
		ctxt.Identity.LoadNicknameFromFile(configuration.DataDirectory + "/" + configuration.NicknameFileName)
	}
	return err
}

func ContextInitialize(ctxt *interfaces.Context) {
	var err error
	err = loadIdentity(ctxt)
	if err != nil {
		logging.ErrorLog.Fatal(err.Error())
	}
	if ctxt.BlockHandler != nil {
		err = ctxt.BlockHandler.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.CycleAuthority != nil {
		err = ctxt.CycleAuthority.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.BlockAuthority != nil {
		err = ctxt.BlockAuthority.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.TransactionManager != nil {
		err = ctxt.TransactionManager.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.MeshListener != nil {
		err = ctxt.MeshListener.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.NodeManager != nil {
		err = ctxt.NodeManager.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
	if ctxt.DataStore != nil {
		err = ctxt.DataStore.Initialize()
		if err != nil {
			logging.ErrorLog.Fatal(err.Error())
		}
	}
}

func ContextStart(ctxt *interfaces.Context) {
	if ctxt.BlockHandler != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.BlockHandler.Start()
	}
	if ctxt.CycleAuthority != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.CycleAuthority.Start()
	}
	if ctxt.BlockAuthority != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.BlockAuthority.Start()
	}
	if ctxt.TransactionManager != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.TransactionManager.Start()
	}
	if ctxt.MeshListener != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.MeshListener.Start()
	}
	if ctxt.NodeManager != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.NodeManager.Start()
	}
	if ctxt.DataStore != nil {
		ctxt.WaitGroup.Add(1)
		go ctxt.DataStore.Start()
	}
}

// Waits for an interrupt signal to send an exit signal to any messaging loops or other stuff that needs to know it.
func WaitForInterrupt() {
	var signalChannel chan os.Signal
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	done := false
	for !done {
		select {
		case <-signalChannel:
			done = true
			logging.InfoLog.Print("Got shutdown request.")
			message := messages.NewInternalMessage(messages.TypeInternalExiting)
			router.Router.RouteInternal(message)
		}
	}
}
