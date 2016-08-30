package lib

import (
	"encoding/json"

	"github.com/streadway/amqp"
)

type QueueHandler struct {
	Queue   string
	Channel *amqp.Channel
	C       *Ctx
}

type FailedMsg struct {
	Queue string
	Error string
	Desc  string
	Msg   string
}

// SetupQueue creates a new channel on top of the established
// amqp connection and declares a persistent queue with the
// given name. It then returns a pointer to a QueueHandler.
func (c *Ctx) SetupQueue(queue string) (*QueueHandler, error) {
	if queue == "" {
		c.Warning.Println("Queue name is empty! A persistent and anonymous queue will be created.")
	}

	c.Debug.Println("Creating new queue handler for", queue)

	channel, err := c.AmqpConn.Channel()
	if err != nil {
		return nil, err
	}

	_, err = channel.QueueDeclare(
		queue, // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	return &QueueHandler{queue, channel, c}, nil
}

// Consume connects to a queue as a consumer, sets the QoS
// and relays all incoming messages to the supplied function.
func (c *Ctx) Consume(queue string, prefetchCount int, fn func(msg amqp.Delivery)) error {
	c.Debug.Println("Starting to consume on", queue)

	handle, err := c.SetupQueue(queue)
	if err != nil {
		return err
	}

	err = handle.Channel.Qos(
		prefetchCount, // prefetch count
		0,             // prefetch size
		false,         // global
	)
	if err != nil {
		return err
	}

	msgs, err := handle.Channel.Consume(
		handle.Queue, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	forever := make(chan bool)

	go func() {
		for m := range msgs {
			c.Debug.Println("Received a message on", queue)
			fn(m)
		}
	}()

	c.Info.Println("Consuming", queue, "...")
	<-forever

	return nil
}

// Send is used to send a message to a amqp
// queue. Channel and queue name are taken from
// the QueueHandler struct.
func (q *QueueHandler) Send(msg []byte) error {
	err := q.Channel.Publish(
		"",      // exchange
		q.Queue, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         msg,
		})
	if err != nil {
		return err
	}

	i := ""
	if len(msg) > 700 {
		i = string(msg[:700]) + " [...]"
	} else {
		i = string(msg)
	}

	q.C.Info.Println("Dispatched", i)
	return nil
}

// NackOnError accepts an error, error description, and amqp
// message. If the error is not nil a NACK is sent in reply
// to the msg. The msg will be redirected to the failed queue
// so the overseer, ehhm, "something" can handle it.
func (c *Ctx) NackOnError(err error, desc string, msg *amqp.Delivery) bool {
	if err != nil {
		c.Warning.Println("[NACK]", desc, err.Error())

		jm, err := json.Marshal(FailedMsg{
			msg.RoutingKey,
			err.Error(),
			desc,
			string(msg.Body),
		})
		if err != nil {
			c.Warning.Println(err.Error())
		}

		c.Failed.Send(jm)

		err = msg.Nack(false, false)
		if err != nil {
			c.Warning.Println("Sending NACK failed!", err.Error())
		}

		return true
	}

	return false
}
