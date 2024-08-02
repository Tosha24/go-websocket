package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/tosha24/go-websocket/actions"
)

func setupAPI() {
	log.Println("setting up api")
	ctx := context.Background()
	newManager := actions.NewManager(ctx) // newmanager creates a new manager with the context

	http.Handle("/", http.FileServer(http.Dir("../frontend")))

	http.HandleFunc("/ws", newManager.ServeWS)
	http.HandleFunc("/login", newManager.LoginHandler)
}

func main() {
	fmt.Println("starting server on port 8080")
	setupAPI()

	fmt.Println("Setting up port")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
