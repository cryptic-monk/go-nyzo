package node_manager

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"os"
)

// Load managed verifiers from configuration file.
func (s *state) loadManagedVerifiers() error {
	s.managedVerifiers = make([]*networking.ManagedVerifier, 0)
	fileName := configuration.DataDirectory + "/" + configuration.ManagedVerifiersFileName
	f, err := os.Open(fileName)
	if err != nil {
		message := fmt.Sprintf("cannot load managed verifiers: %s", err.Error())
		return errors.New(message)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		managedVerifier := networking.ManagedVerifierFromString(fmt.Sprintln(scanner.Text()))
		if managedVerifier != nil {
			s.managedVerifiers = append(s.managedVerifiers, managedVerifier)
		}
	}
	if err := scanner.Err(); err != nil {
		message := fmt.Sprintf("cannot load managed verifiers: %s", err.Error())
		return errors.New(message)
	}
	if len(s.managedVerifiers) == 0 {
		return errors.New("no managed verifiers found")
	} else {
		for _, verifier := range s.managedVerifiers {
			logging.InfoLog.Printf("Got managed verifier: %s, nick: %s.", verifier.Identity.ShortId, verifier.Identity.Nickname)
		}
	}
	return nil
}
