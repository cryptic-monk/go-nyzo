/*
Sets up 4 loggers for messages of various priorities.
*/
package logging

import (
	"log"
	"os"
)

var (
	InfoLog    *log.Logger // general info
	WarningLog *log.Logger // something might have gone wrong
	ErrorLog   *log.Logger // definitely an error, call Fatal() on this one to terminate the program
	TraceLog   *log.Logger // debugging, messages too detailed to display them all the time
)

// Mimic the Java logging behavior by writing either to Stdout or Stderr depending on the logger used.
func init() {
	InfoLog = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	WarningLog = log.New(os.Stdout,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	ErrorLog = log.New(os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	TraceLog = log.New(os.Stdout, //ioutil.Discard if we don't want to see trace messages
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}
