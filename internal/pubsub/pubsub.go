package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int
type AckType int

const (
	Durable SimpleQueueType = iota
	Transient
)

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func (s SimpleQueueType) String() string {
	switch s {
	case 1:
		{
			return "durable"
		}
	default:
		{
			return "transient"
		}
	}
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonData, err := json.Marshal(val)

	if err != nil {
		return err
	}

	data := amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonData,
	}

	return ch.PublishWithContext(context.Background(), exchange, key, false, false, data)
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()

	if err != nil {
		return nil, amqp.Queue{}, err
	}

	isDurable := queueType == Durable

	table := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}

	q, err := channel.QueueDeclare(queueName, isDurable, !isDurable, !isDurable, false, table)

	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = channel.QueueBind(queueName, key, exchange, false, nil)

	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return channel, q, nil

}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	channel, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)

	if err != nil {
		return err
	}

	delChan, err := channel.Consume(queueName, "", false, false, false, false, nil)

	if err != nil {
		return err
	}

	for delivery := range delChan {
		var jsonData T
		err := json.Unmarshal(delivery.Body, &jsonData)
		if err != nil {
			return err
		}
		ackType := handler(jsonData)
		switch ackType {
		case Ack:
			{
				delivery.Ack(false)
			}
		case NackRequeue:
			{
				delivery.Nack(false, true)
			}
		case NackDiscard:
			{
				delivery.Nack(false, false)
			}
		default:
			{

				delivery.Ack(false)
			}
		}
		fmt.Printf("Processed, Sending: %v", ackType)
	}

	return nil

}
