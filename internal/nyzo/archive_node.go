/*
A light node that follows the blockchain and archives it.
*/
package nyzo

import (
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/api"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/data_store"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
)

type ArchiveNodeInterface interface {
	Start()
}

type archiveNodeState struct {
	ctxt *interfaces.Context
}

func (s *archiveNodeState) Start() {
	if err := configuration.EnsureSetup(); err != nil {
		logging.ErrorLog.Fatal(err.Error())
	}
	s.ctxt.SetRunMode(interfaces.RunModeArchive)
	s.ctxt.MeshListener = nil
	s.ctxt.DataStore = data_store.NewMysqlDataStore(s.ctxt)
	apiEnabled := s.ctxt.Preferences.Retrieve(configuration.ApiEnabledKey, "0")
	if apiEnabled == "1" {
		s.ctxt.Api = api.NewApi(s.ctxt)
	}
	ContextInitialize(s.ctxt)
	ContextStart(s.ctxt)
	WaitForInterrupt()
	s.ctxt.WaitGroup.Wait()
}

// Create an archive node.
func NewArchiveNode() ArchiveNodeInterface {
	s := &archiveNodeState{}
	s.ctxt = NewDefaultContext()
	return s
}
