/*
This is the main package for the archive node application, it sets up the archive node and starts it.
*/
package main

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo"
)

func main() {
	/*
		// Uncomment this to create a tracing profile.
		f, err := os.Create("cpu.prof")
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	*/
	archiveNode := nyzo.NewArchiveNode()
	archiveNode.Start()
}
