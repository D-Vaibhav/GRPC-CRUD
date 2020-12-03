package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/vaibhav/grpc_crud/server/handlers"
	blog_proto "github.com/vaibhav/grpc_crud/server/protos/blog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

const (
	listenerPort string = ":4040"
	dbClientURI  string = "mongodb://localhost:2700"
)

var (
	mongoClient *mongo.Client
	blogDBC     *mongo.Collection
	mongoCtx    context.Context
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("Starting server on port:", listenerPort)

	listener, err := net.Listen("tcp", listenerPort)
	if err != nil {
		log.Fatalf("Unable to listen on port = %s: %v", listenerPort, err)
	}

	// initializing grpc Server for RPC req-res
	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...) //returing pointer to the server

	// service server
	blogServiceServer := handlers.BlogServiceServer{}

	// registering service server with the GRPC server
	blog_proto.RegisterBlogServiceServer(grpcServer, &blogServiceServer)

	// ---------------------------------- DATABASE ---------------------------------
	fmt.Println("connecting to mongoDB...")

	// assigning mongoCtx
	mongoCtx = context.Background() // non-nil empty context

	// connecting to mongo
	dbClient, err := mongo.Connect(mongoCtx, options.Client().ApplyURI(dbClientURI))
	if err != nil {
		log.Fatal(err)
	}

	// Check whether the connection was succesful by pinging the MongoDB server
	err = dbClient.Ping(mongoCtx, nil)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v\n", err)
	} else {
		fmt.Println("Connected to MongoDB")
	}

	// assigning blogDBC
	blogDBC = dbClient.Database("mydb").Collection("blog")

	// ---------------------------------- LISTENING TO SERVER ----------------------------
	// starting server in child goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()
	fmt.Printf("Server succesfully started on port : %s", listenerPort)

	// ----------------------------------- SHUT-DOWN -------------------------------------
	c := make(chan os.Signal) // channel to recieve an os signal

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-c

	// After receiving CTRL+C Properly stop the server
	fmt.Println("\nStopping the server...")
	grpcServer.Stop()

	listener.Close()

	fmt.Println("Closing MongoDB connection")
	dbClient.Disconnect(mongoCtx)

	fmt.Println("Done.")
}
