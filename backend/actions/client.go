package actions

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

var (
	pongWait = 10 * time.Second
	pingInterval = (pongWait * 9)/10 // pingInterval means how often we will send ping messages to client. this has to be lower than pongWait. if pingInterval is not lower, server is automatically close waiting longer for ping messages (due to higher pongWait)
)

type ClientList map[*Client] bool

type Client struct {
	connection *websocket.Conn
	manager *Manager

	chatroom string
	egress chan Event
}

func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client {
		connection: conn,
		manager: manager,	
		egress: make(chan Event),
	}
}

func (c *Client) readMessages() {
	defer func() {
		c.manager.removeClient(c)
	}()

	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}

	c.connection.SetReadLimit(512)

	c.connection.SetPongHandler(c.pongHandler)

	for {
		_, payload, err := c.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure){
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		var request Event

		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error marshalling event: %v", err)
			break
		}

		if err := c.manager.routeEvent(request, c); err != nil {
			log.Printf("error handling event: %v", err)
		}
	}
}

func (c *Client) writeMessages() {
	defer func() {
		c.manager.removeClient(c)
	}()

	ticker := time.NewTicker(pingInterval) 

	for {
		select {
		case message, ok := <- c.egress:
			if !ok {
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Println("Connection closed: ", err)
				}
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Println(err)
				return
			}


			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Failed to send message: %v", err)
			}

			log.Println("Message sent")

		case <- ticker.C: 
			log.Println("ping")

			// send a ping to client
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte(``)); err != nil {
				log.Println("write msg err: ", err)
				return
			}
		}
	}
}

func (c *Client) pongHandler(pongMsg string) error {
	log.Println("pong")

	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}