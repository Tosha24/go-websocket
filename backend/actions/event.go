package actions

import (
	"encoding/json"
	"time"
)

type Event struct {
	// Type is the type of the event like send_message, new_message, change_room
	Type    string `json:"type"`
	// Payload is the data that is sent with the event for example: the "message" and "from" in "send_message" event
	Payload json.RawMessage `json:"payload"` 
}

type EventHandler func(event Event, c *Client) error

const (
	EventSendMessage = "send_message"
	EventNewMessage = "new_message"	
	EventChangeRoom = "change_room"
)

type SendMessageEvent struct {
	Message string `json:"message"`
	From string `json:"from"`
}

type NewMessageEvent struct {
	SendMessageEvent
	Sent time.Time `json:"sent"`
}

type ChangeRoomEvent struct {
	Name string `json:"name"`
}