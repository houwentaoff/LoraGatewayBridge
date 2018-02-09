package https

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/brocaar/lora-gateway-bridge/gateway"
	"github.com/brocaar/loraserver/api/gw"
	"github.com/brocaar/lorawan"
	log "github.com/sirupsen/logrus"
)

// Backend implements a HTTPS pub-sub backend.
type Backend struct {
	conn *http.Client //net.Conn
	url  *url.URL
	//conn mqtt.Client
	txPacketChan chan gateway.TXPK //gw.TXPacketBytes
	gateways     map[lorawan.EUI64]struct{}
	mutex        sync.RWMutex
}

// NewTLSConfig returns the TLS configuration.
func NewTLSConfig(cafile, clica, clikey string) (*tls.Config, error) {
	// Import trusted certificates from CAfile.pem.

	cert, err := ioutil.ReadFile(cafile)
	if err != nil {
		log.Errorf("backend: couldn't load cafile: %s", err)
		return nil, err
	}

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cert)
	if clica != "" && clikey != "" {
		// Import client certificate/key pair
		certt, errr := tls.LoadX509KeyPair(clica, clikey)
		if errr != nil {
			panic(errr)
		}
		// Create tls.Config with desired tls properties
		return &tls.Config{
			// RootCAs = certs used to verify server cert.
			RootCAs: certpool,
			// Certificates = list of certs client sends to server.
			Certificates: []tls.Certificate{certt},
		}, nil
	}
	log.Debugln("no cli ca cli key!")
	// Create tls.Config with desired tls properties
	return &tls.Config{
		// RootCAs = certs used to verify server cert.
		RootCAs: certpool,
	}, nil
}

func NewBackend(server, cafile, clica, clikey string) (*Backend, error) {
	b := Backend{
		txPacketChan: make(chan gateway.TXPK), //gw.TXPacketBytes),
		gateways:     make(map[lorawan.EUI64]struct{}),
		conn:         nil,
	}
	log.WithField("server", server).Info("backend: connecting to https server")
	brokerURL, err := url.Parse(server)
	if err != nil {
		log.Fatalf("Error URL: %v", err)
		return nil, err
	}
	b.url = brokerURL
	tlsconfig, err := NewTLSConfig(cafile, clica, clikey)
	if err != nil {
		log.Warningln("ca file? less?")
		return nil, err
	}

	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsconfig,
		},
	}
	b.conn = cli
	log.WithField("server", server).Info("backend: connecting to https server")
	resp, err := cli.Get(fmt.Sprintf("%s://%s", brokerURL.Scheme, brokerURL.Host))
	if err != nil {
		return nil, err
	}
	log.Debugln("connected!resp:", resp)
	/*
		uri := brokerURL
		config, _ := websocket.NewConfig(uri.String(), fmt.Sprintf("https://%s", uri.Host))
		config.Protocol = []string{"test"}
		config.TlsConfig = tlsconfig
		conn, err := websocket.DialConfig(config)
		if err != nil {
			return nil, err
		}
		conn.PayloadType = websocket.BinaryFrame
	*/
	/*
		c, err := Dial(brokerURL.Scheme, brokerURL.Host)
		if err != nil {
			log.Fatalf("Error dialing: %v %v", err, c)
			return nil, err
		}
		b.conn = c
	*/
	return &b, nil
}

//gw.GatewayStatsPacket into  Semtech Stat
//gw.Stat
func newGatewayStats(mac lorawan.EUI64, gwStat gw.GatewayStatsPacket) gateway.Stat {
	stat := gateway.Stat{
		Time: gateway.ExpandedTime(gwStat.Time),
		//Lati: gwStat.Latitude,
		//Long: gwStat.Longitude,
		//Alti:
		RXNb: uint32(gwStat.RXPacketsReceived),
		RXOK: uint32(gwStat.RXPacketsReceivedOK),
		//RXFW: gwStat.
		DWNb: uint32(gwStat.TXPacketsReceived),
		TXNb: uint32(gwStat.TXPacketsEmitted),
	}
	if gwStat.Altitude != nil {
		alt := int32(*gwStat.Altitude)
		stat.Alti = &alt
	}
	if gwStat.Latitude != nil {
		lati := float64(*gwStat.Latitude)
		stat.Lati = &lati
	}
	if gwStat.Longitude != nil {
		long := float64(*gwStat.Longitude)
		stat.Long = &long
	}
	return stat
}
func newGatewayRX(mac lorawan.EUI64, rxPacket gw.RXPacketBytes) gateway.RXPK {
	rxpk := gateway.RXPK{
		Time: gateway.CompactTime(rxPacket.RXInfo.Time),
		Tmst: rxPacket.RXInfo.Timestamp,
		Freq: float64(rxPacket.RXInfo.Frequency) / 1000000.0,
		Chan: uint8(rxPacket.RXInfo.Channel),
		RFCh: uint8(rxPacket.RXInfo.RFChain),
		Stat: int8(rxPacket.RXInfo.CRCStatus),
		Modu: "LORA",
		DatR: gateway.DatR{
			LoRa: fmt.Sprintf("SF%dBW%d", rxPacket.RXInfo.DataRate.SpreadFactor,
				rxPacket.RXInfo.DataRate.Bandwidth),
		},
		CodR: rxPacket.RXInfo.CodeRate,
		RSSI: int16(rxPacket.RXInfo.RSSI),
		LSNR: rxPacket.RXInfo.LoRaSNR,
		Size: uint16(rxPacket.RXInfo.Size),
		Data: base64.StdEncoding.EncodeToString(rxPacket.PHYPayload),
	}
	return rxpk
}

// TXPacketChan returns the TXPacketBytes channel.
func (b *Backend) TXPacketChan() chan gateway.TXPK {
	return b.txPacketChan
}

// SubscribeGatewayTX subscribes the backend to the gateway TXPacketBytes
// topic (packets the gateway needs to transmit).
func (b *Backend) SubscribeGatewayTX(mac lorawan.EUI64) error {
	/*
		topic := fmt.Sprintf("gateway/%s/tx", mac.String())
		log.WithField("topic", topic).Info("backend: subscribing to topic")

		for b.conn != nil {
			msg, err := b.conn.Receive()
			if err != nil {
				//log.Fatalf("Error receiving: %v", err)
				continue
			}
			topic = msg.Path()[0]
			log.WithField("topic", topic).Info("coap backend: recv topic")
			fmt.Println("==>jot txpk:", string(msg.Payload), "\n")
			var tx gateway.TXPK
			if err := json.Unmarshal(msg.Payload, &tx); err != nil {
				log.Errorf("backend: decode tx packet error: %s", err)

				continue
			}
			fmt.Println("==>jot txpk 222:", tx, "\n")

			b.txPacketChan <- tx
		}
	*/
	return nil
}

//publishes a stats to coap server
func (b *Backend) PublishGatewayStats(mac lorawan.EUI64, stats gw.GatewayStatsPacket) error {
	topic := fmt.Sprintf("gateway/%s/stats", mac.String())
	stat := newGatewayStats(mac, stats) //NewstatsPacketBytes(mac, stats)
	//r := statsP{Stat: *stat}
	return b.publish(topic, stat)
	//return b.publish(topic, r)
}

// PublishGatewayRX publishes a RX packet to the coap.
func (b *Backend) PublishGatewayRX(mac lorawan.EUI64, rxPacket gw.RXPacketBytes) error {
	topic := fmt.Sprintf("gateway/%s/rx", mac.String())
	//fmt.Println("==>joy: %s\n", rxPacket)
	rx := newGatewayRX(mac, rxPacket) //NewrxPacketBytes(mac, rxPacket)
	//var r rxP
	//r := rxP{RX: *rx}
	//return b.publish(topic, rxPacket)
	return b.publish(topic, rx)
}
func (b *Backend) publish(topic string, v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	log.WithField("topic", topic).Info("backend: publishing packet")
	str := string(bytes)
	fmt.Println("==>joy:", str, "\n")

	resp, err := b.conn.Post(
		fmt.Sprintf("%s://%s/topic/%s",
			b.url.Scheme, b.url.Host, topic),
		"application/json",
		strings.NewReader(str))
	if err != nil {
		log.Errorln(err)
		return nil
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln(err)
		// handle error
	}
	var tx gateway.TXPK
	if err := json.Unmarshal(body, &tx); err != nil {
		log.Errorf("backend: decode tx packet error: %s", err)
		return nil
	}
	log.Debugln("==>jot txpk :", tx, "\n")

	b.txPacketChan <- tx

	//log.Debugln("recv body!:", body)
	log.Debugln("recv body!:", string(body))
	return nil
}
