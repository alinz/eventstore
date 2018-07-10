package nats

import (
	"crypto/tls"
	"fmt"
	"log"
	"reflect"
	"time"

	proto "github.com/golang/protobuf/proto"
	gonats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"

	"github.com/nulloop/eventstore"
	pb "github.com/nulloop/eventstore/proto"
)

var (
	ErrSubjectType        = fmt.Errorf("subject is not nat's subject")
	ErrHandlerNil         = fmt.Errorf("handler is nil")
	ErrQueueEmpty         = fmt.Errorf("queue name is empty")
	ErrDurableEmpty       = fmt.Errorf("durable is empty")
	ErrMessageInstanceNil = fmt.Errorf("message type is nil")
)

type NatsEventstore struct {
	conn stan.Conn

	publishEnabled *AtomicBool
}

func (n *NatsEventstore) Publish(payload eventstore.Container) error {
	// PublishEnableFlag or PublishDisableFlag must be use to
	// toggle this
	if !n.publishEnabled.Value() {
		return nil
	}

	message, err := proto.Marshal(payload.Message())
	if err != nil {
		return err
	}

	data, err := proto.Marshal(&pb.Transport{
		Id:      payload.ID(),
		Payload: message,
	})
	if err != nil {
		return err
	}

	return n.conn.Publish(payload.Subject().Topic(), data)
}

func (n *NatsEventstore) Subscribe(subject eventstore.Subject, handler eventstore.Handler) (eventstore.Unsubscribe, error) {
	natsSubject, ok := subject.(*Subject)
	if !ok {
		return nil, ErrSubjectType
	}

	if reflect.ValueOf(handler).IsNil() {
		return nil, ErrHandlerNil
	}

	if natsSubject.msgInstance == nil {
		return nil, ErrMessageInstanceNil
	}

	transport := pb.Transport{}
	process := func(msg *stan.Msg) {
		err := proto.Unmarshal(msg.Data, &transport)
		if err != nil {
			log.Printf("error decoding transport, %v, %s\n", msg, err)
			return
		}

		err = proto.Unmarshal(transport.Payload, natsSubject.msgInstance)
		if err != nil {
			log.Printf("error decoding message, %v, %s\n", transport, err)
			return
		}

		err = handler(&Payload{
			id:        transport.Id,
			subject:   subject,
			message:   natsSubject.msgInstance,
			sequence:  msg.Sequence,
			timestamp: msg.Timestamp,
		})

		if err == nil {
			if err = msg.Ack(); err != nil {
				log.Printf("error ack message for %v, %s\n", msg, err)
			}
		}
	}

	var err error
	var subscriptionHandler stan.Subscription

	options := []stan.SubscriptionOption{
		stan.SetManualAckMode(),
		stan.StartAtSequence(natsSubject.sequence),
	}

	if natsSubject.durable != "" {
		stan.DurableName(natsSubject.durable)
	}

	// aw, _ := time.ParseDuration("5s")
	// if subscription.Timeout != 0 {
	// 	options = append(options, stan.AckWait(subscription.Timeout))
	// }

	if natsSubject.queue == "" {
		subscriptionHandler, err = n.conn.Subscribe(natsSubject.Topic(), process, options...)
	} else {
		subscriptionHandler, err = n.conn.QueueSubscribe(natsSubject.Topic(), natsSubject.queue, process, options...)
	}

	if err != nil {
		return nil, err
	}

	return func() error { return subscriptionHandler.Close() }, nil
}

func (n *NatsEventstore) Close() error {
	return n.conn.Close()
}

// New creates a nee eventstore
func New(tlsConfig *tls.Config, addr, clusterID, clientID string) (*NatsEventstore, error) {
	opts := make([]gonats.Option, 0)
	if tlsConfig != nil {
		opts = append(opts, gonats.Secure(tlsConfig))
	}

	nc, err := gonats.Connect(addr, opts...)
	if err != nil {
		return nil, err
	}

	var conn stan.Conn

	for {
		conn, err = stan.Connect(clusterID, clientID, stan.NatsConn(nc))
		if err != nil {
			if err == stan.ErrConnectReqTimeout {
				log.Printf("trying to connect to nats-streaming at %s\n", addr)
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, err
		}

		break
	}

	return &NatsEventstore{
		conn:           conn,
		publishEnabled: NewAtomicBool(),
	}, nil
}
