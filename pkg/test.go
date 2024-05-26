package pkg

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	lksdk "github.com/livekit/server-sdk-go"
)

func getEnv(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	return os.Getenv(key)
}

var apiKey = getEnv("LIVEKIT_API_KEY")
var apiSecret = getEnv("LIVEKIT_API_SECRET")
var HOST = getEnv("LIVEKIT_HOST")
var client = lksdk.NewRoomServiceClient(HOST, apiKey, apiSecret)
