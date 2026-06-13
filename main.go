package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aliyun/fc-runtime-go-sdk/fc"
)

func main() {
	configureRuntimeFromEnv()
	switch selectedChatAdapter() {
	case "http":
		addr := strings.TrimSpace(os.Getenv("API_LISTEN"))
		if addr == "" {
			port := strings.TrimSpace(os.Getenv("PORT"))
			if port == "" {
				port = strings.TrimSpace(os.Getenv("FC_SERVER_PORT"))
				if port == "" {
					port = "8080"
				}
			}
			addr = "0.0.0.0:" + port
		}
		if err := StartAPIServer(addr); err != nil {
			log.Fatal(err)
		}
		return
	case "fc":
		fc.Start(HandleRequest)
		return
	default:
		if err := runCLIChat(context.Background()); err != nil {
			log.Fatal(err)
		}
	}
}

func selectedChatAdapter() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("CHAT_ADAPTER")))
}

func configureRuntimeFromEnv() {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DATA_STORE"))) {
	case "memory":
		dataStore = NewMemoryStore()
	case "json", "":
		dataStore = NewJSONStore()
	case "sqlite":
		dataStore = NewSQLiteStore()
	case "mysql":
		dataStore = MySQLStore{}
	default:
		log.Printf("unknown DATA_STORE=%q, using JSONStore", os.Getenv("DATA_STORE"))
		dataStore = NewJSONStore()
	}
}
