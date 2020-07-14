package networking

import (
	"bytes"
	"encoding/binary"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/test"
	"net"
	"strconv"
	"time"
)

// Fetch over TCP from string IP or with name resolution.
// Used during startup only, so no success tracking.
func FetchTcpNamed(m *messages.Message, host string, port int32) {
	portString := strconv.Itoa(int(port))
	// establish connection
	conn, err := net.DialTimeout("tcp", host+":"+portString, messages.ConnectionTimeout)
	if err != nil {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionFailure, host, m.Type)
		router.Router.RouteInternal(message)
		logging.TraceLog.Printf("Error establishing connection to %s: %s.", host, err.Error())
		return
	} else {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionSuccess, host, m.Type)
		router.Router.RouteInternal(message)
	}
	defer conn.Close()

	fetchHandleConnection(m, conn, host)
}

// Fetch over TCP with known IP.
func FetchTcp(m *messages.Message, host []byte, port int32) {
	// we normally use bytes + int, Go uses strings
	hostString := message_fields.IP4BytesToString(host)
	portString := strconv.Itoa(int(port))
	// establish connection
	conn, err := net.DialTimeout("tcp", hostString+":"+portString, messages.ConnectionTimeout)
	if err != nil {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionFailure, hostString, m.Type)
		router.Router.RouteInternal(message)
		logging.TraceLog.Printf("Error establishing connection to %s: %s.", hostString, err.Error())
		return
	} else {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionSuccess, hostString, m.Type)
		router.Router.RouteInternal(message)
	}
	defer conn.Close()

	fetchHandleConnection(m, conn, hostString)
}

func fetchHandleConnection(m *messages.Message, conn net.Conn, hostName string) {
	// send message
	//TODO: Do not send a message that will get this IP blacklisted.
	//See: https://github.com/n-y-z-o/nyzoVerifier/blob/2ea08c5183f6cfed97984249ae78bd1be2961245/src/main/java/co/nyzo/verifier/Message.java#L178
	data := m.SerializeForTransmission()
	_ = conn.SetWriteDeadline(time.Now().Add(messages.WriteTimeout))
	n, err := conn.Write(data)
	if err != nil || n != len(data) {
		if err != nil {
			logging.TraceLog.Printf("Error sending data to %s: %s.", hostName, err.Error())
		} else {
			logging.TraceLog.Printf("Error sending data to %s.", hostName)
		}
		return
	}

	// get answer
	var answer []byte
	buffer := make([]byte, 4096)
	expectedLength := 0
	totalLength := 0
	_ = conn.SetReadDeadline(time.Now().Add(messages.ReadTimeout))
	for {
		n, err = conn.Read(buffer)
		if n > 0 {
			answer = append(answer, buffer[:n]...)
			totalLength += n
			if totalLength >= 4 && expectedLength == 0 {
				expectedLength = int(binary.BigEndian.Uint32(answer[:5]))
			}
		}
		if err != nil || totalLength == expectedLength {
			break
		}
	}
	if totalLength == 0 || totalLength != expectedLength {
		logging.TraceLog.Printf("Error reading data from %s - incomplete message.", hostName)
		return
	}
	//TODO: testing only, comment out for production
	test.Dump_eet(answer)
	message, err := messages.ReadNew(bytes.NewReader(answer), conn.RemoteAddr().String())
	if err != nil {
		//TODO: report infraction to node manager (maybe also for some of the above)
		logging.TraceLog.Printf("Could not interpret message fetched from %s, %s.", hostName, err.Error())
	} else {
		router.Router.Route(message)
	}
}

// Send an UDP message.
func SendUdp(m *messages.Message, host []byte, port int32) {
	// we normally use bytes + int, Go uses strings
	hostString := message_fields.IP4BytesToString(host)
	portString := strconv.Itoa(int(port))
	// establish connection
	conn, err := net.DialTimeout("udp", hostString+":"+portString, messages.ConnectionTimeout)
	if err != nil {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionFailure, hostString, m.Type)
		router.Router.RouteInternal(message)
		logging.TraceLog.Printf("Error establishing connection to %s: %s.", hostString, err.Error())
		return
	} else {
		message := messages.NewInternalMessage(messages.TypeInternalConnectionSuccess, hostString, m.Type)
		router.Router.RouteInternal(message)
	}
	defer conn.Close()

	sendHandleConnection(m, conn, hostString)
}

func sendHandleConnection(m *messages.Message, conn net.Conn, hostName string) {
	// send message
	//TODO: Do not send a message that will get this IP blacklisted.
	//See: https://github.com/n-y-z-o/nyzoVerifier/blob/2ea08c5183f6cfed97984249ae78bd1be2961245/src/main/java/co/nyzo/verifier/Message.java#L178
	data := m.SerializeForTransmission()
	_ = conn.SetWriteDeadline(time.Now().Add(messages.WriteTimeout))
	n, err := conn.Write(data)
	if err != nil || n != len(data) {
		if err != nil {
			logging.TraceLog.Printf("Error sending data to %s: %s.", hostName, err.Error())
		} else {
			logging.TraceLog.Printf("Error sending data to %s.", hostName)
		}
	}
}
