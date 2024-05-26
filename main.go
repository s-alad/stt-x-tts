package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/webrtc/v3"
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

type JoinRoomRequest struct {
	UserID string `json:"userid"`
}
type DeleteRoomRequest struct {
	RoomID string `json:"roomid"`
}

func trackSubscribed(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	log.Println("Track subscribed:", track.ID())
	log.Println("Publication:", publication)
	log.Println("Remote participant:", rp)
	log.Println("Remote participant identity:", rp.Identity())
	log.Println("Remote participant sid:", rp.SID())
	log.Println("Track kind:", track.Kind())
}
func dataReceived(data []byte, rp *lksdk.RemoteParticipant) {
	log.Println("Data received:", data)
	log.Println("Remote participant:", rp)
	log.Println("Remote participant identity:", rp.Identity())
	log.Println("Remote participant sid:", rp.SID())
}

func roomDisconnected() {
	log.Println("Room disconnected")
}
func participantConnected(p *lksdk.RemoteParticipant) {
	log.Println("Participant connected:", p)
}
func participantDisconnected(p *lksdk.RemoteParticipant) {
	log.Println("Participant disconnected:", p)
}

func noxJoinRoom(roomID string) {
	// send the room id to the nox worker
	identity := "nox"
	roomCB := &lksdk.RoomCallback{
		OnParticipantConnected:    participantConnected,
		OnParticipantDisconnected: participantDisconnected,
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: trackSubscribed,
			OnDataReceived:    dataReceived,
		},
		OnDisconnected: roomDisconnected,
	}
	room, err := lksdk.ConnectToRoom(HOST, lksdk.ConnectInfo{
		APIKey:              apiKey,
		APISecret:           apiSecret,
		RoomName:            roomID,
		ParticipantIdentity: identity,
	}, roomCB)
	if err != nil {
		panic(err)
	}

	log.Println("Joined room - started thread:", room)
}

func getToken(roomID, identity string, name string) (string, error) {
	canPublish := true
	canSubscribe := true

	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin:     true,
		Room:         roomID,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.AddGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(time.Hour)

	return at.ToJWT()
}

func createRoomId(userID string) string {
	return userID + "-" + time.Now().Format("20060102150405")
}

func listRooms() *livekit.ListRoomsResponse {
	log.Println("Listing rooms")
	log.Println(HOST)
	rooms, _ := client.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
	log.Println("Rooms:", rooms)
	return rooms
}

func createRoom(roomID string) *livekit.Room {
	room, _ := client.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
		Name:            roomID,
		EmptyTimeout:    10 * 60,
		MaxParticipants: 10,
	})
	log.Println("Room created:", room)
	return room
}

func deleteRoom(roomID string) {
	client.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
		Room: roomID,
	})
}

func main() {

	app := fiber.New()

	// --------------------------------------------------------------------------------------------
	// GET /
	// --------------------------------------------------------------------------------------------
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("/") })

	// --------------------------------------------------------------------------------------------
	// GET /get/token
	// 1. generate a room id
	// 2. create a room with the room id
	// 3. generate a token for the user with the room id
	// 4. send the room id to the nox worker
	// 5. send the token to the user
	//
	// Request:  {"roomid": "roomid", "userid": "userid"}
	// Response: {token": "token"}
	// --------------------------------------------------------------------------------------------
	app.Post("/get/token", func(c fiber.Ctx) error {

		// parse the request
		p := new(JoinRoomRequest)
		if err := c.Bind().JSON(p); err != nil {
			return err
		}

		// if no userid then return error
		if p.UserID == "" {
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error": "userid is required",
			})
		}

		// generate the room id and token
		UserID := p.UserID
		RoomID := createRoomId(p.UserID)

		// create the room
		room := createRoom(RoomID)

		// generate the token
		token, err := getToken(RoomID, "human", UserID)
		if err != nil {
			return err
		}

		// send the room id to the nox worker
		go noxJoinRoom(RoomID)

		// send the token to the user
		return c.JSON(fiber.Map{
			"token": token,
			"room":  room,
		})
	})

	app.Post("/create/room", func(c fiber.Ctx) error {

		p := new(JoinRoomRequest)
		if err := c.Bind().JSON(p); err != nil {
			return err
		}

		RoomID := createRoomId(p.UserID)
		UserID := p.UserID

		log.Println("RoomID:", RoomID)
		log.Println("UserID:", UserID)

		room := createRoom(RoomID)

		return c.JSON(fiber.Map{
			"room": room,
		})
	})

	app.Post("/delete/room", func(c fiber.Ctx) error {

		p := new(DeleteRoomRequest)
		if err := c.Bind().JSON(p); err != nil {
			return err
		}

		if p.RoomID == "" {
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error": "roomid is required",
			})
		}

		RoomID := p.RoomID

		log.Println("RoomID:", RoomID)

		deleteRoom(RoomID)

		return c.JSON(fiber.Map{
			"room":   RoomID,
			"status": "deleted",
		})
	})

	app.Get("/list/rooms", func(c fiber.Ctx) error {
		rooms := listRooms()
		return c.JSON(fiber.Map{
			"rooms": rooms,
		})
	})

	// Start the server
	app.Listen(":3000")
}
