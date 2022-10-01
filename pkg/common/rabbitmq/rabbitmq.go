package rabbitmq

import (
	"encoding/json"
	"net"
	"os"
	"time"

	"github.com/streadway/amqp"
	"k8s.io/klog/v2"
)

const queueSize = 200

type VerbType string

const (
	VerbCreate    VerbType = "create"
	VerbConfigure VerbType = "configure"
	VerbDelete    VerbType = "delete"
)

type Msg struct {
	Verb    VerbType `bson:"verb" json:"verb"`
	JobName string   `bson:"job_name" json:"job_name"`
}

func ConnectRabbitMQ() *amqp.Connection {
	// Find service IP and port from kube-dns (CoreDNS)
	// my-svc.my-namespace.svc.cluster-domain.example
	host := "voda-scheduler-rabbitmq.voda-scheduler.svc.cluster.local"
	iprecords, err := net.LookupIP(host)
	if err != nil {
		klog.ErrorS(err, "Failed to look up rabbit-mq service host IP", "host", host)
		klog.Flush()
		os.Exit(1)
	}
	ip := iprecords[0]

	// TODO(heyfey): replace temporary url
	url := "amqp://guest:guest@" + ip.String() + ":5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		klog.ErrorS(err, "Failed to connect to rabbit-mq", "url", url)
		klog.Flush()
		os.Exit(1)
	} else {
		klog.InfoS("Connected to rabbit-mq", "url", url)
	}

	return conn
}

func PublishToQueue(conn *amqp.Connection, queueName string, msg Msg) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// TODO(heyfey)
	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(msg)
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
			Timestamp:   time.Now(),
		})
	if err != nil {
		return err
	}

	return nil
}

func ReceiveFromQueue(conn *amqp.Connection, queueName string) (<-chan Msg, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	// defer ch.Close()

	// TODO(heyfey)
	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return nil, err
	}

	// TODO(heyfey)
	msgsRaw, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, err
	}

	msgs := make(chan Msg, queueSize)
	go func() {
		for d := range msgsRaw {
			var msg Msg
			json.Unmarshal(d.Body, &msg)
			msgs <- msg
		}
	}()

	return msgs, nil
}
