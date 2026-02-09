package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	const connectionString = "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(connectionString)

	if err != nil {
		panic(err)
	}

	defer connection.Close()
	fmt.Println("Rabbit connection successful!")

	channel, err := connection.Channel()

	if err != nil {
		panic(err)
	}

	jsonData := routing.PlayingState{
		IsPaused: true,
	}

	err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, jsonData)

	if err != nil {
		panic(err)
	}

	<-ctx.Done()

	fmt.Println("Program is sutting down...")
}
