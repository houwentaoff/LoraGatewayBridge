package main

//go:generate ./doc.sh

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brocaar/lora-gateway-bridge/backend/go-coap"
	"github.com/brocaar/lora-gateway-bridge/backend/mqttpubsub"
	"github.com/brocaar/lora-gateway-bridge/gateway"
	"github.com/brocaar/lorawan"
	"github.com/codegangsta/cli"
	log "github.com/sirupsen/logrus"
)

var version string // set by the compiler

func run(c *cli.Context) error {
	log.SetLevel(log.Level(uint8(c.Int("log-level"))))

	log.WithFields(log.Fields{
		"version": version,
		"email":   "544088192@qq.com",
	}).Info("starting LoRa Gateway Bridge")

	var pubsub *mqttpubsub.Backend
	var coappubsub *coap.Backend
	for {
		var err error
		//fmt.Println("coap-server:", c.String("coap-server"))
		if c.String("coap-server") == "" {
			pubsub, err = mqttpubsub.NewBackend(c.String("mqtt-server"), c.String("mqtt-username"), c.String("mqtt-password"), c.String("mqtt-ca-cert"), c.String("mqtt-cli-ca-cert"), c.String("mqtt-cli-key"))
			if err == nil {
				break
			}
			log.Errorf("could not setup mqtt backend, retry in 2 seconds: %s", err)
		} else {
			coappubsub, err = coap.NewBackend(c.String("coap-server"))
			if err == nil {
				break
			}
			log.Errorf("could not setup coap backend, retry in 2 seconds: %s", err)
		}

		time.Sleep(2 * time.Second)
	}
	defer func() {
		if c.String("coap-server") == "" {
			pubsub.Close()
		}
	}()

	onNew := func(mac lorawan.EUI64) error {
		if c.String("coap-server") == "" {
			return pubsub.SubscribeGatewayTX(mac)
		} else {
			return nil
		}
	}

	onDelete := func(mac lorawan.EUI64) error {
		if c.String("coap-server") == "" {
			return pubsub.UnSubscribeGatewayTX(mac)
		} else {
			return nil
		}
	}

	gw, err := gateway.NewBackend(c.String("udp-bind"), onNew, onDelete, c.Bool("skip-crc-check"))
	if err != nil {
		log.Fatalf("could not setup gateway backend: %s", err)
	}
	defer gw.Close()

	if c.String("coap-server") != "" {
		go func() {
			for {
				coappubsub.SubscribeGatewayTX([8]byte{0x0, 0x0, 0x8, 0x0, 0x27, 0x00, 0x01, 0x97})
			}
		}()
	}

	go func() {
		for rxPacket := range gw.RXPacketChan() {
			if c.String("coap-server") == "" {
				if err := pubsub.PublishGatewayRX(rxPacket.RXInfo.MAC, rxPacket); err != nil {
					log.Errorf("could not publish RXPacket: %s", err)
				}
			} else {
				fmt.Println("string ", c.String("coap-server"))
				if err := coappubsub.PublishGatewayRX(rxPacket.RXInfo.MAC, rxPacket); err != nil {
					log.Errorf("could not publish RXPacket: %s", err)
				}
			}
		}
	}()

	go func() {
		for stats := range gw.StatsChan() {
			if c.String("coap-server") == "" {
				if err := pubsub.PublishGatewayStats(stats.MAC, stats); err != nil {
					log.Errorf("could not publish GatewayStatsPacket: %s", err)
				}
			} else {
				fmt.Println("string ", c.String("coap-server"))
				if err := coappubsub.PublishGatewayStats(stats.MAC, stats); err != nil {
					log.Errorf("could not publish GatewayStatsPacket: %s", err)
				}
			}
		}
	}()

	go func() {

		if c.String("coap-server") == "" {
			for txPacket := range pubsub.TXPacketChan() {
				if err := gw.Send(txPacket); err != nil {
					log.Errorf("could not send TXPacket: %s", err)
				}
			}
		} else {
			for txPacket := range coappubsub.TXPacketChan() {
				if err := gw.SendTXPK(txPacket, [8]byte{0x0, 0x0, 0x8, 0x0, 0x27, 0x00, 0x01, 0x97}); err != nil {
					log.Errorf("could not send TXPacket: %s", err)
				}
			}
		}

	}()

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.WithField("signal", <-sigChan).Info("signal received")
	log.Warning("shutting down server")
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "lora-gateway-bridge"
	app.Usage = "abstracts the packet_forwarder protocol into JSON over MQTT or COAP"
	app.Copyright = "Joy Hou"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "udp-bind",
			Usage:  "ip:port to bind the UDP listener to",
			Value:  "0.0.0.0:1700",
			EnvVar: "UDP_BIND",
		},
		cli.StringFlag{
			Name:   "coap-server",
			Usage:  "coap server (e.g. scheme://host:port where scheme is udp (udp://127.0.0.1:5683))",
			Value:  "", //"udp://127.0.0.1:5683",
			EnvVar: "COAP_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "mqtt server (e.g. scheme://host:port where scheme is tcp, ssl or ws)",
			Value:  "tcp://127.0.0.1:1883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-username",
			Usage:  "mqtt server username (optional)",
			EnvVar: "MQTT_USERNAME",
		},
		cli.StringFlag{
			Name:   "mqtt-password",
			Usage:  "mqtt server password (optional)",
			EnvVar: "MQTT_PASSWORD",
		},
		cli.StringFlag{
			Name:   "mqtt-ca-cert",
			Usage:  "mqtt CA certificate file (optional)",
			EnvVar: "MQTT_CA_CERT",
		},
		cli.StringFlag{
			Name:   "mqtt-cli-ca-cert",
			Usage:  "mqtt cli-CA certificate file (optional)",
			EnvVar: "MQTT_CLI_CA_CERT",
		},
		cli.StringFlag{
			Name:   "mqtt-cli-key",
			Usage:  "mqtt cli-KEY pri key file (optional)",
			EnvVar: "MQTT_CLI_KEY",
		},
		cli.BoolFlag{
			Name:   "skip-crc-check",
			Usage:  "skip the CRC status-check of received packets",
			EnvVar: "SKIP_CRC_CHECK",
		},
		cli.IntFlag{
			Name:   "log-level",
			Value:  4,
			Usage:  "debug=5, info=4, warning=3, error=2, fatal=1, panic=0",
			EnvVar: "LOG_LEVEL",
		},
	}
	app.Run(os.Args)
}
