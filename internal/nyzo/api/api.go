package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"net/http"
	"sync"
)

type state struct {
	ctxt                   *interfaces.Context
	internalMessageChannel chan *messages.InternalMessage // channel for internal and local messages
	server                 *http.Server
}

type Status struct {
	BlockAuthority interface{} `json:"block_authority"`
}

func (s *state) status(w http.ResponseWriter, r *http.Request) {
	status := Status{}
	status.BlockAuthority = s.ctxt.BlockAuthority.GetStatusReport()
	output, err := json.MarshalIndent(status, "", "    ")
	if err != nil {
		_, _ = fmt.Fprintf(w, "{\"error\": \"unable to generate report\"}")
	} else {
		_, _ = fmt.Fprintf(w, string(output))
	}
}

func startServer(wg *sync.WaitGroup) *http.Server {
	server := &http.Server{Addr: ":8000"}

	go func() {
		defer wg.Done()
		err := server.ListenAndServe()
		if err != http.ErrServerClosed {
			logging.ErrorLog.Fatal(err)
		}
	}()

	return server
}

func (s *state) Start() {
	defer logging.InfoLog.Print("Main loop of API exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	done := false
	for !done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalExiting:
				_ = s.server.Shutdown(context.Background())
				done = true
			}

		}
	}
}

func (s *state) Initialize() error {
	s.internalMessageChannel = make(chan *messages.InternalMessage, 2)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	http.HandleFunc("/status", s.status)
	s.ctxt.WaitGroup.Add(1)
	s.server = startServer(&s.ctxt.WaitGroup)
	return nil
}

// Create new API.
func NewApi(ctxt *interfaces.Context) interfaces.Component {
	s := &state{}
	s.ctxt = ctxt
	return s
}
