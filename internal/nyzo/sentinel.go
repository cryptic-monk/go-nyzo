/*
A node that protects in-cycle verifiers and cycle joins.
*/
package nyzo

import (
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/api"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
)

type SentinelInterface interface {
	Start()
}

type sentinelState struct {
	ctxt *interfaces.Context
}

func (s *sentinelState) Start() {
	if err := configuration.EnsureSetup(); err != nil {
		logging.ErrorLog.Fatal(err.Error())
	}
	s.ctxt.SetRunMode(interfaces.RunModeSentinel)
	s.ctxt.MeshListener = nil
	apiEnabled := s.ctxt.Preferences.Retrieve(configuration.ApiEnabledKey, "0")
	if apiEnabled == "1" {
		s.ctxt.Api = api.NewApi(s.ctxt)
	}
	ContextInitialize(s.ctxt)
	ContextStart(s.ctxt)
	WaitForInterrupt()
	s.ctxt.WaitGroup.Wait()
}

// Create a sentinel.
func NewSentinel() SentinelInterface {
	s := &sentinelState{}
	s.ctxt = NewDefaultContext()
	return s
}
