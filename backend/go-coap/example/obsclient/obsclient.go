package main

import (
	"fmt"
	"log"

	"github.com/dustin/go-coap"
)

func up() {
	s := string(`
		{
			"sites": [ 
			{ "name":"菜鸟教程" , "url":"www.runoob.com" }, 
			{ "name":"google", "url":"www.google.com" }, 
			{ "name":"微博" , "url":"www.weibo.com" },
			]
		}`)
	req := coap.Message{
		Type:      coap.NonConfirmable,
		Code:      coap.PUT,
		MessageID: 12345,
		Payload:   []byte(s),
	}

	req.AddOption(coap.Observe, 1)
	req.SetOption(coap.ContentFormat, coap.AppJSON)
	req.SetPathString("/mac/rx/")

	c, err := coap.Dial("udp", "localhost:5683")
	if err != nil {
		log.Fatalf("Error dialing: %v %v", err, c)
	}
	rv, err := c.Send(req)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}
	for err == nil {
		if rv != nil {
			if err != nil {
				log.Fatalf("Error receiving: %v", err)
			}
			log.Printf("Got %s", rv.Payload)
		}
		rv, err = c.Receive()

	}

}
func main() {
	up()
	a := string(`
	req := coap.Message{
		Type:      coap.NonConfirmable,
		Code:      coap.GET,
		MessageID: 12345,
	}

	req.AddOption(coap.Observe, 1)
	req.SetPathString("/a")

	c, err := coap.Dial("udp", "localhost:5683")
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}

	rv, err := c.Send(req)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}

	for err == nil {
		if rv != nil {
			if err != nil {
				log.Fatalf("Error receiving: %v", err)
			}
			log.Printf("Got %s", rv.Payload)
		}
		rv, err = c.Receive()

	}
	log.Printf("Done...\n")
`)
	if a == "" {
		fmt.Println("xx")
	}
}
