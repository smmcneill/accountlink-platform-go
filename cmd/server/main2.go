package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var errConsumerClosed = errors.New("consumer closed")

type Message struct {
	Value string
}

type Consumer interface {
	ReadMessage(context.Context) (Message, error)
	Close() error
}

// fakeConsumer simulates a Kafka consumer.
type fakeConsumer struct {
	msgs   chan Message
	closed chan struct{}
	once   sync.Once
}

func newFakeConsumer() *fakeConsumer {
	c := &fakeConsumer{
		msgs:   make(chan Message),
		closed: make(chan struct{}),
	}

	go func() {
		defer close(c.msgs)

		for i := 1; i <= 10; i++ {
			select {
			case <-c.closed:
				log.Println(">>>> closed signal")
				return
			case <-time.After(500 * time.Millisecond):
				c.msgs <- Message{Value: fmt.Sprintf("message-%d", i)}
			}
		}
	}()

	return c
}

func (c *fakeConsumer) ReadMessage(ctx context.Context) (Message, error) {
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-c.closed:
		return Message{}, errConsumerClosed
	case msg, ok := <-c.msgs:
		if !ok {
			return Message{}, errConsumerClosed
		}
		return msg, nil
	}
}

func (c *fakeConsumer) Close() error {
	c.once.Do(func() {
		close(c.closed)
	})
	return nil
}

func main2() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	consumer := newFakeConsumer()
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("consumer close: %v", err)
		}
	}()

	log.Println("consumer started")

	for {
		msg, err := consumer.ReadMessage(ctx)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				log.Println("shutdown signal received")
				log.Println("consumer stopped cleanly")
				return
			case errors.Is(err, errConsumerClosed):
				log.Println("consumer closed")
				return
			default:
				log.Printf("read message: %v", err)
				continue
			}
		}

		// Finish the message already admitted to this process.
		if err := handleMessage(ctx, msg); err != nil {
			log.Printf("handle message %q: %v", msg.Value, err)
		}
	}
}

func handleMessage(ctx context.Context, msg Message) error {
	log.Printf("processing %s", msg.Value)

	select {
	case <-ctx.Done():
		// In a real system, choose deliberately:
		// - return ctx.Err() to abort quickly, or
		// - ignore ctx here if you want to finish in-flight work.
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}

	log.Printf("finished %s", msg.Value)
	return nil
}
