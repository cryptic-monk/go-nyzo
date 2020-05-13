/*
This is the main package for the sentinel application, it sets up a sentinel node and starts it.
*/
package main

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo"
)

func main() {
	sentinel := nyzo.NewSentinel()
	sentinel.Start()
}
