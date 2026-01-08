package rabitmq

import (
	"fmt"
	"log"
	"os"

	u "github.com/Sayan-995/dwop/internal/utils"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	RabbitMQClient        *u.RabbitMQ
	PublisherChannelCount = 10
	ConsumerChannelCount  = 10
	QueueName             = "workflow_queue"
)

func init() {
	godotenv.Load()
	err := NewRabitMQConnection()
	if err != nil {
		log.Fatalf("error initializing the rabitMQ connection: %v", err)
	}
}

func NewRabitMQConnection() error {
	conn, err := amqp.Dial(os.Getenv("RABBITMQ_CONNECTION_URL"))
	publisherPool := make(chan *amqp.Channel, PublisherChannelCount)
	if err != nil {
		return fmt.Errorf("error getting the connection for rabitMQ: %v", err)
	}
	for i := 0; i < PublisherChannelCount; i++ {
		ch, err := conn.Channel()
		if err != nil {
			return fmt.Errorf("error while creating channel for publisherPool: %v", err)
		}
		_, err = ch.QueueDeclare(
			QueueName,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("error while assigning channel to queue: %v", err)
		}
		publisherPool <- ch
	}
	RabbitMQClient = &u.RabbitMQ{
		Conn:          conn,
		PublisherPool: publisherPool,
	}
	return nil
}

func GetPublisherChan() *amqp.Channel {
	return <-RabbitMQClient.PublisherPool
}

func putPublisherChan(ch *amqp.Channel) {
	RabbitMQClient.PublisherPool <- ch
}

func PublishTask(body []byte) error {
	ch := GetPublisherChan()
	err := ch.Publish(
		"",
		QueueName,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		})
	if err != nil {
		fmt.Printf("[PublishTask] Publish error: %v, closing channel and not returning to pool\n", err)
		ch.Close()
		return fmt.Errorf("error publishing message: %v", err)
	}
	putPublisherChan(ch)
	return nil
}
