package main

import (
	"log"

	"ai-travel/internal/config"
	"ai-travel/internal/gateway"
	"ai-travel/internal/rpc"
)

func main() {
	client, err := rpc.NewTravelClient(config.Env("TRAVEL_ADDR", "127.0.0.1:8888"))
	if err != nil {
		log.Fatal("initialize travel RPC client: ", err)
	}
	gateway.New(
		config.Env("API_ADDR", "127.0.0.1:8080"),
		config.Env("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		client,
		config.Env("OPENAPI_PATH", "docs/openapi.yaml"),
	).Spin()
}
