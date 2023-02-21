package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/robmonte/grpc-chat/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	scanner       *bufio.Scanner
	connectedName string
	client        pb.ChatServiceClient
	connectedUser *pb.User
)

type handle struct {
	stream pb.ChatService_OpenStreamClient
}

func main() {
	log.Println("Starting client")
	conn, err := grpc.Dial("localhost:9876", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	scanner = bufio.NewScanner(os.Stdin)
	connectedUser = createNewUser()

	client = pb.NewChatServiceClient(conn)
	client.Join(context.Background(), connectedUser)

	streamClient, err := client.OpenStream(context.Background(), connectedUser)
	if err != nil {
		log.Fatalf("Failed to listen on client: %v", err)
	}
	msgHandler := &handle{stream: streamClient}

	connectedName = connectedUser.Name
	fmt.Println("Welcome to the chat! To talk, just start typing!")

	go msgHandler.listen()
	go send()

	block := make(chan bool)
	<-block
}

func (h *handle) listen() {
	for {
		message, err := h.stream.Recv()
		if err != nil {
			log.Fatalf("Failed to receive message from server: %v", err)
		}

		if message != nil {
			if message.From.Name != connectedName {
				fmt.Print("\033[2K\r")
				fmt.Printf("%s> %s\n", message.From.Name, message.Message)
				fmt.Printf("%s> ", connectedName)
			}
		}
	}
}

func send() {
	for {
		fmt.Printf("%s> ", connectedName)
		_ = scanner.Scan()

		input := strings.TrimSpace(scanner.Text())
		for input == "" {
			fmt.Printf("%s> ", connectedName)
			_ = scanner.Scan()
			input = strings.TrimSpace(scanner.Text())
		}

		skip := isLocalCommand(input)

		if !skip {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			// How do you use the above cancel to break the loop? Can't defer cancel because infinite loop

			_, err := client.SendMessage(ctx, &pb.ChatMessage{
				From:      connectedUser,
				Message:   input,
				Timestamp: timestamppb.Now(),
			})
			if err != nil {
				log.Fatalf("Failed to send message: %v", err)
			}
		}
	}
}

func isLocalCommand(input string) bool {
	if strings.HasPrefix(input, "/") {
		switch input[1:] {
		case "getusers":
			list, err := client.GetUsers(context.Background(), &pb.Empty{})
			if err != nil {
				log.Fatalf("failed to get user list: %v", err)
			}

			for _, user := range list.Users {
				fmt.Printf("\t%s\n", user.GetName())
			}

			return true

		case "leave":
			client.Disconnect(context.Background(), connectedUser)
			os.Exit(0)

		default:
			return false
		}
	}

	return false
}

func createNewUser() *pb.User {
	fmt.Print("Enter your username: ")
	_ = scanner.Scan()

	return &pb.User{
		Id:   uuid.NewString(),
		Name: scanner.Text(),
	}
}
