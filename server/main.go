package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	pb "github.com/robmonte/grpc-chat/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedChatServiceServer
	users    *pb.UserList
	messages chan *pb.ChatMessage
	streams  map[string]*pb.ChatService_OpenStreamServer
}

func (s *server) OpenStream(user *pb.User, stream pb.ChatService_OpenStreamServer) error {
	s.streams[user.Id] = &stream

	block := make(chan bool)
	<-block

	return nil
}

func (s *server) SendMessage(ctx context.Context, message *pb.ChatMessage) (*pb.Empty, error) {
	log.Printf("received message: %s", message.Message)

	for id, stream := range s.streams {
		err := (*stream).Send(&pb.ChatMessage{
			From:      message.From,
			Message:   message.Message,
			Timestamp: message.Timestamp,
		})
		if err != nil {
			stat, _ := status.FromError(err)
			if strings.HasSuffix(stat.Message(), "\"transport is closing\"") {
				delete(s.streams, id)
			} else {
				log.Printf("failed to send message %q: %v", message.Message, err)
			}
		}
	}

	return &pb.Empty{}, nil
}

func (s *server) Join(ctx context.Context, user *pb.User) (*pb.JoinResponse, error) {
	joined := fmt.Sprintf("%q joined the chat", user.Name)
	log.Println(joined)

	s.users.Users = append(s.users.Users, user)

	s.SendMessage(ctx, &pb.ChatMessage{
		From:      &pb.User{Name: "SYSTEM", Id: "SYSTEM"},
		Message:   joined,
		Timestamp: timestamppb.Now(),
	})

	return &pb.JoinResponse{Message: joined}, nil
}

func (s *server) GetUsers(ctx context.Context, empty *pb.Empty) (*pb.UserList, error) {
	return s.users, nil
}

func (s *server) Disconnect(ctx context.Context, user *pb.User) (*pb.Empty, error) {
	delete(s.streams, user.Id)

	left := fmt.Sprintf("%q left the chat", user.Name)
	log.Println(left)

	s.SendMessage(ctx, &pb.ChatMessage{
		From:      &pb.User{Name: "SYSTEM", Id: "SYSTEM"},
		Message:   left,
		Timestamp: timestamppb.Now(),
	})

	return &pb.Empty{}, nil
}

func main() {
	log.Println("starting server")
	lis, err := net.Listen("tcp", ":9876")
	if err != nil {
		log.Fatalf("failed to listen to port: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterChatServiceServer(grpcServer, &server{
		users:    &pb.UserList{},
		messages: make(chan *pb.ChatMessage),
		streams:  map[string]*pb.ChatService_OpenStreamServer{},
	})
	log.Println("server listening")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
