/*
This is the main package for the verifier application, it sets up the verifier and starts it.
*/
package main

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo"
)

func main() {
	verifier := nyzo.NewVerifier()
	verifier.Start()
}
