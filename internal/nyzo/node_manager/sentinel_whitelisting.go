package node_manager

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

// Send ip requests to all managed verifiers. 1st step to get whitelisted.
func (s *state) startWhitelistUpdate() {
	logging.InfoLog.Print("Starting whitelisting process for all managed verifiers.")
	s.lastWhitelistUpdate = utilities.Now()
	for _, verifier := range s.managedVerifiers {
		messageIpRequest := messages.NewLocal(messages.TypeIpAddressRequest, nil, verifier.Identity)
		go networking.FetchTcpNamed(messageIpRequest, verifier.Host, verifier.Port)
	}
}

// Process an IP address response. 2nd whitelisting step.
func (s *state) processIpAddressResponse(message *messages.Message) {
	ipResponseContent := *message.Content.(*message_content.IpAddress)
	for _, verifier := range s.managedVerifiers {
		// we only accept responses from known verifiers
		if bytes.Equal(message.SourceId, verifier.Identity.PublicKey) {
			messageWhitelistRequest := messages.NewLocal(messages.TypeWhitelistRequest, &ipResponseContent, verifier.Identity)
			go networking.FetchTcpNamed(messageWhitelistRequest, verifier.Host, verifier.Port)
		}
	}
}

// Process a whitelisting response. 3rd whitelisting step.
func (s *state) processWhitelistResponse(message *messages.Message) {
	whitelistResponseContent := *message.Content.(*message_content.BooleanResponse)
	for _, verifier := range s.managedVerifiers {
		// we only accept responses from known verifiers
		if bytes.Equal(message.SourceId, verifier.Identity.PublicKey) {
			logging.TraceLog.Printf("Whitelist response from %s is %t, message: %s.", verifier.Identity.ShortId, whitelistResponseContent.Success, whitelistResponseContent.Message)
		}
	}
}
