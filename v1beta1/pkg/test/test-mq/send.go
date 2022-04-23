package main

import (
	"fmt"
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

	body := "Hello World!"
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	check(err)
	fmt.Println("Success!")
}
