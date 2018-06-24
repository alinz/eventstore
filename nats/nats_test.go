package nats_test

import (
	"math/rand"
	"sync"
	"testing"

	gonats "github.com/nats-io/go-nats"
	"github.com/nats-io/nats-streaming-server/server"

	"github.com/nulloop/eventstore"
	"github.com/nulloop/eventstore/nats"
	pb "github.com/nulloop/eventstore/proto"
)

const (
	durableName1 = "durableName1"
	durableName2 = "durableName2"
	queueName1   = "queueName1"
	queueName2   = "queueName2"

	subject1 string = "subjectName1"
	subject2 string = "subjectName2"
)

func genRandomStringer() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func runDummyServer(clusterName string) (*server.StanServer, error) {
	s, err := server.RunServer(clusterName)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func TestRunDummyServer(t *testing.T) {
	server, err := runDummyServer("dummy_server")
	if err != nil {
		t.Error(err)
	}
	server.Shutdown()
}

func TestCreateEventStore(t *testing.T) {
	server, err := runDummyServer("dummy")
	if err != nil {
		t.Error(err)
	}

	defer server.Shutdown()

	es, err := nats.New(nil, gonats.DefaultURL, "dummy", "client1")
	if err != nil {
		t.Error(err)
	}

	defer es.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	subject, err := nats.NewSubject(subject1, nats.OptMessageInstance(&pb.DummyMessage{}), nats.OptSequence(0))
	if err != nil {
		t.Error(err)
	}

	unsubscribe, err := es.Subscribe(subject, func(payload eventstore.Container) error {
		defer wg.Done()

		dummyMessage, ok := payload.Message().(*pb.DummyMessage)
		if !ok {
			t.Error("message is not DummyMessage")
		}

		if dummyMessage.Value != "this is test" {
			t.Error("message is incorrect")
		}

		return nil
	})

	if err != nil {
		t.Error(err)
	}

	defer unsubscribe()

	es.Publish(eventstore.NewPayload(subject, &pb.DummyMessage{Value: "this is test"}, "1"))

	wg.Wait()
}

// func TestAckQueueMessage(t *testing.T) {
// 	server, err := runDummyServer("dummy")
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	defer server.Shutdown()

// 	es, err := nats.New(nil, gonats.DefaultURL, "dummy", "client1")
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	defer es.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(1)

// 	unsubscribe1, err := es.Subscribe(&eventstore.Subscription{
// 		Message:     &pb.DummyMessage{},
// 		Subject:     subject1,
// 		DurableName: durableName1,
// 		QueueName:   queueName1,
// 		Timeout:     1 * time.Second,
// 		Handler: func(payload *eventstore.Payload) error {
// 			return fmt.Errorf("noop")
// 		},
// 	})

// 	if err != nil {
// 		t.Error(err)
// 	}

// 	defer unsubscribe1()

// 	unsubscribe2, err := es.Subscribe(&eventstore.Subscription{
// 		Message:     &pb.DummyMessage{},
// 		Subject:     subject1,
// 		DurableName: durableName1,
// 		QueueName:   queueName1,
// 		Timeout:     2 * time.Second,
// 		Handler: func(payload *eventstore.Payload) error {
// 			defer wg.Done()

// 			dummyMessage, ok := payload.Message.(*pb.DummyMessage)
// 			if !ok {
// 				t.Error("message is not DummyMessage")
// 			}

// 			if dummyMessage.Value != "this is test" {
// 				t.Error("message is incorrect")
// 			}

// 			return nil
// 		},
// 	})

// 	if err != nil {
// 		t.Error(err)
// 	}

// 	defer unsubscribe2()

// 	es.Publish(
// 		&eventstore.Payload{
// 			Message:       &pb.DummyMessage{Value: "this is test"},
// 			CorrelationID: genRandomStringer(),
// 			Signature:     genRandomStringer(),
// 		},
// 		subject1,
// 	)

// 	wg.Wait()
// }