package main

import (
	"log"
	"net"

	"github.com/streadway/amqp"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	// Find service IP and port from kube-dns (CoreDNS)
	// my-svc.my-namespace.svc.cluster-domain.example
	host := "voda-scheduler-rabbitmq.voda-scheduler.svc.cluster.local"
	iprecords, err := net.LookupIP(host)
	check(err)
	ip := iprecords[0]

	conn, err := amqp.Dial("amqp://guest:guest@" + ip.String() + ":5672/")
	check(err)
	defer conn.Close()

	ch, err := conn.Channel()
	check(err)
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	check(err)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	check(err)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
