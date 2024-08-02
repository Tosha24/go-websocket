package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin:     checkOrigin, 
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type Manager struct {
	// clients is a map of all connected clients to the server
	// Clientlist is a map of clients that shows if a client is connected or not
	clients ClientList
	sync.RWMutex // RWMutex is a reader/writer mutual exclusion lock. The lock can be held by an arbitrary number of readers or a single writer.

	otps RetentionMap  // RetentionMap is a map that holds the otps for the clients that are connected to the server for a certain amount of time before they are removed from the map and the client is disconnected

	handlers map[string]EventHandler // EventHandler is a function that takes an event and a client and returns an error, and handlers is a map of event types to event handlers, for example: "send-message" -> func SendMessage(Event, Client)
}

func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		clients: make(ClientList),

		handlers: make(map[string]EventHandler),
		otps:     NewRetentionMap(ctx, 5*time.Second),  // NewRetentionMap creates a new RetentionMap with a context and a duration of 5 seconds. means that the otps are stored for 5 seconds before they are removed from the map
	}
	m.setupEventHandlers() // go to the definition to understand
	return m
}

func (m *Manager) setupEventHandlers() {
	// this function sets up the event handlers for the manager instance
	// means it tells which event has what handler to call when the event is received
	// for example: when the event type is "send_message", the handler SendMessage is called
	m.handlers[EventSendMessage] = SendMessage
	// for example: when the event type is "change_room", the handler ChatRoomHandler is called
	m.handlers[EventChangeRoom] = ChatRoomHandler
}

// ChatRoomHandler() -> handles changing chat rooms
// event - the event containing the chat room change request
// c - the client that is changing the chat room, and requesting by the event
func ChatRoomHandler(event Event, c *Client) error {
	var changeRoomEvent ChangeRoomEvent

	// parses the event payload into the changeRoomEvent struct object 
	// for example: {"type": "change_room", "payload": {"name": "room1"}} -> changeRoomEvent.Name = "room1"
	if err := json.Unmarshal(event.Payload, &changeRoomEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	// changes the chatroom of the client c to the new chatroom present in the event
	c.chatroom = changeRoomEvent.Name
	return nil // no error
}

func SendMessage(event Event, c *Client) error {
	var chatevent SendMessageEvent

	// parses the event payload into the chatevent struct object
	// for example: {"type": "send_message", "payload": {"from": "percy", "message": "hello"}} -> chatevent.From = "percy", chatevent.Message = "hello"
	if err := json.Unmarshal(event.Payload, &chatevent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	// creates a new message to be broadcasted to all the clients in the chatroom
	var broadMessage NewMessageEvent
	broadMessage.Sent = time.Now()
	broadMessage.Message = chatevent.Message
	broadMessage.From = chatevent.From

	// converts the broadMessage struct object into a json object which can be sent to the clients in the chatroom
	data, err := json.Marshal(broadMessage)
	if err != nil {
		return fmt.Errorf("error marshalling event: %v", err)
	}

	// creates new event with the above data (message, from, sent) and the event type as EventNewMessage
	outgoingEvent := Event {
		Payload: data,
		Type: EventNewMessage,
	}

	// sends the event to all the clients in the chatroom 
	for client := range c.manager.clients { // iterate over all the clients in the current client's manager's client list
		if client.chatroom == c.chatroom { 
			// if the client is in the same chatroom as the current client, send the message to the 
			// client's egress channel which will be read by the client's writeMessages() method
			client.egress <- outgoingEvent
		}
	}
	return nil
}

func (m *Manager) routeEvent(event Event, c *Client) error {
	// routes the event to the correct handler based on the event type
	// if the event type is not present in the handlers map, it returns an error
	// for example: if the event type is "send_message", it calls the SendMessage handler
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("there is no such event type")
	}
}

// ServeWS() -> Handles WebSocket upgrade and client initialization
func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request) {
	// retrieves the otp from the query parameters, in frontend it was routed as "/ws?otp=otp"
	otp := r.URL.Query().Get("otp")
	if otp == "" {
		// if the otp is empty, return unauthorized
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !m.otps.VerifyOTP(otp) {
		// checks if the current otp is present in the otps map. If yes, then will remove the otp from the map else return false and unauthorized
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Println("new connection")

	// Upgrade the HTTP connection to Websocket connection with nil response header
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	fmt.Print("conn: ", conn)
	if err != nil {
		log.Println(err)
		return
	}

	// creates a new client inside the manager m with the connection and the manager instance
	client := NewClient(conn, m)

	// adds the client to the manager's client list
	m.addClient(client)

	// reads messages from the client
	go client.readMessages()
	// write messages to the client
	go client.writeMessages()
}

func (m *Manager) LoginHandler(w http.ResponseWriter, r *http.Request) {
	type userLoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req userLoginRequest

	// insert the request body into the req struct object 
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username == "percy" && req.Password == "123" {
		type response struct {
			OTP string `json:"otp"`
		}

		// creates a new otp and adds it to the otps map
		otp := m.otps.NewOTP()

		resp := response{
			OTP: otp.Key,
		}

		// marshal method converts the response struct object into a json object like {"otp": "key"}
		data, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
		// write the data to the response writer or client
		w.Write(data)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

func (m *Manager) addClient(client *Client) {
	// locks the manager instance to add the client to the client list
	// it is important to lock the manager instance because the client list is shared between multiple goroutines and we need to make sure that the client list is not modified by multiple goroutines at the same time
	m.Lock()

	defer m.Unlock()

	// adds current client to the client list
	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()

	defer m.Unlock()

	// removes the client from the client list and closes the connection of the client
	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	case "http://localhost:8080":
		return true
	default:
		return false
	}
}
