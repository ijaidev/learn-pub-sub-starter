package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
		handler := handlerMove(state, channel)
		queueName := routing.ArmyMovesPrefix + "." + name
		key := routing.ArmyMovesPrefix + ".*"

		err := pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, queueName, key, pubsub.Transient, handler)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		handler := handlerWar(state, channel)
		queueName := routing.WarRecognitionsPrefix
		key := routing.WarRecognitionsPrefix + ".*"

		err := pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, queueName, key, pubsub.Durable, handler)
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
				num, _ := strconv.Atoi(input[1])

				for range num {
					msg := gamelogic.GetMaliciousLog()
					pubsub.PublishGameLog(channel, name, msg)
				}

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

func handlerMove(gs *gamelogic.GameState, conn *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print(">")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutcomeMakeWar:
			{
				data := gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.GetPlayerSnap(),
				}
				key := routing.WarRecognitionsPrefix + "." + gs.GetUsername()
				err := pubsub.PublishJSON(conn, routing.ExchangePerilTopic, key, data)
				if err != nil {
					fmt.Println(err)
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			}
		case gamelogic.MoveOutComeSafe:
			{
				return pubsub.Ack
			}
		default:
			{
				return pubsub.NackDiscard
			}
		}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print(">")
		outcome, winner, loser := gs.HandleWar(war)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			{
				return pubsub.NackRequeue
			}
		case gamelogic.WarOutcomeNoUnits:
			{
				return pubsub.NackDiscard
			}
		case gamelogic.WarOutcomeOpponentWon:
			{
				err := pubsub.PublishGameLog(ch, war.Attacker.Username, fmt.Sprintf("%s won a war against %s", winner, loser))
				if err != nil {
					fmt.Println(err)
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			}
		case gamelogic.WarOutcomeYouWon:
			{
				err := pubsub.PublishGameLog(ch, war.Attacker.Username, fmt.Sprintf("%s won a war against %s", winner, loser))
				if err != nil {
					fmt.Println(err)
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			}
		case gamelogic.WarOutcomeDraw:
			{
				err := pubsub.PublishGameLog(ch, war.Attacker.Username, fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser))
				if err != nil {
					fmt.Println(err)
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			}
		default:
			{
				fmt.Println("Unknown Outcome")
				return pubsub.NackDiscard
			}
		}
	}
}
