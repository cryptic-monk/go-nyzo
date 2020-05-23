/*
Simple networking component that listens to the Nyzo mesh.
*/
package mesh_listener

import (
	"bytes"
	"encoding/binary"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/test"
	"net"
	"strconv"
	"time"
)

type state struct {
	ctxt                   *interfaces.Context
	tcpListener            net.Listener
	udpConnection          net.PacketConn
	internalMessageChannel chan *messages.InternalMessage // channel for internal and local messages
	done                   bool                           // exit gracefully
}

// Listen to TCP connections.
func (s *state) listenTcp(tcpPort string) {
	defer s.ctxt.WaitGroup.Done()
	var err error
	s.tcpListener, err = net.Listen("tcp", ":"+tcpPort)
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot start TCP listener: %s.", err.Error())
	}
	for !s.done {
		conn, err := s.tcpListener.Accept()
		if err == nil {
			if s.ctxt.NodeManager.AcceptingMessagesFrom(conn.RemoteAddr().String()) {
				s.ctxt.WaitGroup.Add(1)
				go s.handleTcpConnection(conn)
			} else {
				// we bail right here if this node is not allowed to send us any messages
				conn.Close()
			}
		}
	}
}

// Listen to UDP connections.
func (s *state) listenUdp(udpPort string) {
	defer s.ctxt.WaitGroup.Done()
	var err error
	s.udpConnection, err = net.ListenPacket("udp", ":"+udpPort)
	if err != nil {
		logging.ErrorLog.Fatalf("Cannot start UDP listener: %s.", err.Error())
	}
	s.ctxt.WaitGroup.Add(1)
	go s.handleUdpConnection(s.udpConnection)
}

// Handle an individual incoming TCP request.
func (s *state) handleTcpConnection(conn net.Conn) {
	defer s.ctxt.WaitGroup.Done()
	defer conn.Close()
	var messageBytes []byte
	// relatively small buffer so that we don't read a lot of garbage if it's not needed
	buffer := make([]byte, 1024)
	expectedLength := 0
	totalLength := 0
	_ = conn.SetReadDeadline(time.Now().Add(messages.ReadTimeout))
	for !s.done {
		n, err := conn.Read(buffer)
		if n > 0 {
			messageBytes = append(messageBytes, buffer[:n]...)
			totalLength += n
			if totalLength >= 14 && expectedLength == 0 {
				// we have messageBytes length, timestamp and messageBytes type, let's do some checks
				expectedLength = int(binary.BigEndian.Uint32(messageBytes[:5]))
				messageType := int16(binary.BigEndian.Uint16(messageBytes[12:14]))
				if !s.ctxt.NodeManager.AcceptingMessageTypeFrom(conn.RemoteAddr().String(), messageType) {
					// no need to read more data if this client isn't allowed to send us the given messageBytes type
					return
				}
			}
		}
		if err != nil || totalLength == expectedLength {
			break
		}
	}
	if totalLength != expectedLength {
		//TODO: report infraction to node manager
		return
	}
	//TODO: testing only, comment out for production
	test.Dump_eet(messageBytes)
	message, err := messages.ReadNew(bytes.NewReader(messageBytes), conn.RemoteAddr().String())
	if err != nil {
		//TODO: report infraction to node manager
		return
	}
	message.ReplyChannel = make(chan *messages.Message)
	// route the message to whomever wants it
	router.Router.Route(message)
	// wait for a reply
	reply := <-message.ReplyChannel
	if reply != nil {
		// send message
		data := reply.SerializeForTransmission()
		_ = conn.SetWriteDeadline(time.Now().Add(messages.WriteTimeout))
		n, err := conn.Write(data)
		if err != nil || n != len(data) {
			if err != nil {
				logging.TraceLog.Printf("Error sending data to %s: %s.", conn.RemoteAddr().String(), err.Error())
			} else {
				logging.TraceLog.Printf("Error sending data to %s.", conn.RemoteAddr().String())
			}
		} else {
			// as far as I understand Go, we can just chill here for a sec, no need to do anything fancy with that
			// connection to close it a little later (allows for data to be read by the client)
			time.Sleep(1 * time.Second)
		}
	}
}

// Handle whatever comes in over UDP.
func (s *state) handleUdpConnection(conn net.PacketConn) {
	defer s.ctxt.WaitGroup.Done()
	var (
		n, expectedLength, totalLength int
		messageBytes                   []byte
		messageType                    int16
	)
	buffer := make([]byte, 4096)
	oldAddr := conn.LocalAddr()
	newAddr := conn.LocalAddr()
	for !s.done {
		var err error
		n, newAddr, err = conn.ReadFrom(buffer)
		if err != nil {
			continue
		}
		if oldAddr.String() != newAddr.String() {
			// this data comes from a new address, so it must be a new message
			messageBytes = nil
			oldAddr = newAddr
			expectedLength = 0
			totalLength = 0
		}
		if n > 0 {
			messageBytes = append(messageBytes, buffer[:n]...)
			totalLength += n
			if totalLength >= 14 && expectedLength == 0 {
				expectedLength = int(binary.BigEndian.Uint32(messageBytes[:5]))
				messageType = int16(binary.BigEndian.Uint16(messageBytes[12:14]))
			}
		}
		if totalLength > 0 && totalLength == expectedLength {
			//TODO: testing only, comment out for production
			test.Dump_eet(messageBytes)
			// we got a complete message
			if s.ctxt.NodeManager.AcceptingMessagesFrom(newAddr.String()) && s.ctxt.NodeManager.AcceptingMessageTypeFrom(newAddr.String(), messageType) {
				message, err := messages.ReadNew(bytes.NewReader(messageBytes), newAddr.String())
				if err == nil {
					message.ReplyChannel = make(chan *messages.Message)
					// route the message to whomever wants it
					router.Router.Route(message)
					// wait for a reply
					reply := <-message.ReplyChannel
					if reply != nil {
						// send message
						data := reply.SerializeForTransmission()
						_ = conn.SetWriteDeadline(time.Now().Add(messages.WriteTimeout))
						n, err := conn.WriteTo(data, newAddr)
						if err != nil || n != len(data) {
							if err != nil {
								logging.TraceLog.Printf("Error sending data to %s: %s.", newAddr.String(), err.Error())
							} else {
								logging.TraceLog.Printf("Error sending data to %s.", newAddr.String())
							}
						}
					}
				} else {
					//TODO: report infraction to node manager
				}
			}
			messageBytes = nil
			expectedLength = 0
			totalLength = 0
		}
	}
}

// Main loop
func (s *state) Start() {
	defer logging.InfoLog.Print("Mesh listener exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	logging.InfoLog.Print("Starting mesh listener.")
	s.ctxt.WaitGroup.Add(1)
	go s.listenTcp(strconv.Itoa(configuration.ListeningPortTcp))
	s.ctxt.WaitGroup.Add(1)
	go s.listenUdp(strconv.Itoa(configuration.ListeningPortUdp))
	s.done = false
	for !s.done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalExiting:
				s.done = true
				if s.tcpListener != nil {
					s.tcpListener.Close()
				}
				if s.udpConnection != nil {
					s.udpConnection.Close()
				}
			}
		}
	}
}

// Initialization function
func (s *state) Initialize() error {
	// set message routes
	s.internalMessageChannel = make(chan *messages.InternalMessage, 1)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	return nil
}

func NewMeshListener(ctxt *interfaces.Context) interfaces.MeshListenerInterface {
	s := &state{}
	s.ctxt = ctxt
	return s
}
