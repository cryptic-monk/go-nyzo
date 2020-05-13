/*
A simple message router with multicasting.
This way, individual entities like the node manager or the block manager can register which messages they'd like
to receive.
*/
package router

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"sync"
)

var Router *routerData

type routerData struct {
	routesLock         sync.Mutex
	routes             map[int16][]chan *messages.Message
	internalRoutesLock sync.Mutex
	internalRoutes     map[int16][]chan *messages.InternalMessage
}

func (r *routerData) AddRoute(messageType int16, channel chan *messages.Message) {
	r.routesLock.Lock()
	r.routes[messageType] = append(r.routes[messageType], channel)
	r.routesLock.Unlock()
}

func (r *routerData) AddInternalRoute(messageType int16, channel chan *messages.InternalMessage) {
	r.internalRoutesLock.Lock()
	r.internalRoutes[messageType] = append(r.internalRoutes[messageType], channel)
	r.internalRoutesLock.Unlock()
}

func (r *routerData) RemoveRoute(messageType int16, channel chan *messages.Message) {
	r.routesLock.Lock()
	allRoutes, ok := r.routes[messageType]
	if !ok {
		return
	}
	newRoutes := make([]chan *messages.Message, len(allRoutes))
	for _, route := range allRoutes {
		if route != channel {
			newRoutes = append(newRoutes, route)
		}
	}
	r.routes[messageType] = newRoutes
	r.routesLock.Unlock()
}

func (r *routerData) RemoveInternalRoute(messageType int16, channel chan *messages.InternalMessage) {
	r.internalRoutesLock.Lock()
	allRoutes, ok := r.internalRoutes[messageType]
	if !ok {
		return
	}
	newRoutes := make([]chan *messages.InternalMessage, len(allRoutes))
	for _, route := range allRoutes {
		if route != channel {
			newRoutes = append(newRoutes, route)
		}
	}
	r.internalRoutes[messageType] = newRoutes
	r.internalRoutesLock.Unlock()
}

func (r *routerData) Route(message *messages.Message) {
	r.routesLock.Lock()
	routesUnsafe, ok := r.routes[message.Type]
	routes := make([]chan *messages.Message, len(routesUnsafe))
	copy(routes, routesUnsafe)
	r.routesLock.Unlock()
	if ok {
		for _, route := range routes {
			route <- message
		}
	} else {
		// send back a nil reply if the message wants a reply but cannot be routed
		if message.ReplyChannel != nil {
			message.ReplyChannel <- nil
		}
	}
}

func (r *routerData) RouteInternal(message *messages.InternalMessage) {
	r.internalRoutesLock.Lock()
	routesUnsafe, ok := r.internalRoutes[message.Type]
	routes := make([]chan *messages.InternalMessage, len(routesUnsafe))
	copy(routes, routesUnsafe)
	r.internalRoutesLock.Unlock()
	if ok {
		for _, route := range routes {
			route <- message
		}
	} else {
		// send back a nil reply if the message wants a reply but cannot be routed
		if message.ReplyChannel != nil {
			message.ReplyChannel <- nil
		}
	}
}

// Convenience to create and route a local message and wait for a reply.
func GetInternalReply(messageType int16, a ...interface{}) *messages.InternalMessage {
	message := messages.NewInternalMessage(messageType, a...)
	message.ReplyChannel = make(chan *messages.InternalMessage, 1)
	Router.RouteInternal(message)
	return <-message.ReplyChannel
}

func init() {
	Router = &routerData{}
	Router.routes = make(map[int16][]chan *messages.Message)
	Router.internalRoutes = make(map[int16][]chan *messages.InternalMessage)
}
