package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		fmt.Println("Program is sutting down...")
		os.Exit(0)
	}()

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

	go func() {
		handler := logHandler()
		key := routing.GameLogSlug + ".*"
		err := pubsub.SubscribeGob(connection, routing.ExchangePerilTopic, routing.GameLogSlug, key, pubsub.Durable, handler)
		if err != nil {
			panic(err)
		}
	}()

	gamelogic.PrintServerHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) <= 0 {
			continue
		}

		switch input[0] {
		case "pause":
			{
				fmt.Println("Sending pause message")
				data := routing.PlayingState{
					IsPaused: true,
				}
				err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, data)
				if err != nil {
					fmt.Printf("Error sending message: %v", err)
				}
			}
		case "resume":
			{
				fmt.Println("Sending resume message")
				data := routing.PlayingState{
					IsPaused: false,
				}
				err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, data)
				if err != nil {
					fmt.Printf("Error sending message: %v", err)
				}
			}
		case "quit":
			{
				fmt.Println("Quiting...")
				os.Exit(0)
			}
		default:
			{
				fmt.Println("Unknown command!")
			}
		}
	}

}

func logHandler() func(routing.GameLog) pubsub.AckType {
	return func(gl routing.GameLog) pubsub.AckType {
		defer fmt.Print(">")
		gamelogic.WriteLog(gl)
		return pubsub.Ack
	}
}
