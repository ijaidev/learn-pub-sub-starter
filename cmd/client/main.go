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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		fmt.Println("Program is shutting down...")
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

	name, err := gamelogic.ClientWelcome()

	if err != nil {
		panic(err)
	}

	state := gamelogic.NewGameState(name)

	go func() {
		handler := handlerPause(state)
		queueName := routing.PauseKey + "." + name

		err := pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.Transient, handler)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		handler := handlerMove(state)
		queueName := routing.ArmyMovesPrefix + "." + name
		key := routing.ArmyMovesPrefix + ".*"

		err := pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, queueName, key, pubsub.Transient, handler)
		if err != nil {
			panic(err)
		}
	}()

	gamelogic.PrintClientHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) <= 0 {
			continue
		}

		command := input[0]

		switch command {
		case "spawn":
			{
				err = state.CommandSpawn(input)
				if err != nil {
					fmt.Printf("Error: %v", err)
				} else {
					fmt.Println("Success!")
				}
			}
		case "move":
			{
				move, err := state.CommandMove(input)
				if err != nil {
					fmt.Printf("Error: %v", err)
					continue
				}
				key := routing.ArmyMovesPrefix + "." + name
				err = pubsub.PublishJSON(channel, routing.ExchangePerilTopic, key, move)
				if err != nil {
					fmt.Printf("Error: %v", err)
					continue
				}
			}
		case "status":
			{
				state.CommandStatus()
			}
		case "help":
			{
				gamelogic.PrintClientHelp()
			}
		case "spam":
			{
				fmt.Println("Spamming not allowd yet")
			}
		case "quit":
			{
				gamelogic.PrintQuit()
				os.Exit(0)
			}
		default:
			{
				fmt.Println("Unknown Command!")
			}
		}
	}

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Println(">")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print(">")
		outcome := gs.HandleMove(move)
		if outcome == gamelogic.MoveOutComeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
			return pubsub.Ack
		}
		return pubsub.NackDiscard
	}
}
