package mqttpubsub

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"sync"
	"time"

	"github.com/brocaar/lora-gateway-bridge/gateway"
	"github.com/brocaar/loraserver/api/gw"
	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/band"
	"github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

// Backend implements a MQTT pub-sub backend.
type Backend struct {
	conn         mqtt.Client
	txPacketChan chan gw.TXPacketBytes
	gateways     map[lorawan.EUI64]struct{}
	mutex        sync.RWMutex
}

// NewBackend creates a new Backend.
func NewBackend(server, username, password, cafile, clica, clikey string) (*Backend, error) {
	b := Backend{
		txPacketChan: make(chan gw.TXPacketBytes),
		gateways:     make(map[lorawan.EUI64]struct{}),
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(server)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetOnConnectHandler(b.onConnected)
	opts.SetConnectionLostHandler(b.onConnectionLost)

	if cafile != "" {
		//var tlsconfig *tls.Config
		//var err error
		tlsconfig, err := NewTLSConfig(cafile, clica, clikey)
		if err == nil {
			opts.SetTLSConfig(tlsconfig)
		}
	}

	log.WithField("server", server).Info("backend: connecting to mqtt broker")
	b.conn = mqtt.NewClient(opts)
	if token := b.conn.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &b, nil
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
	// Create tls.Config with desired tls properties
	return &tls.Config{
		// RootCAs = certs used to verify server cert.
		RootCAs: certpool,
	}, nil
}

// Close closes the backend.
func (b *Backend) Close() {
	b.conn.Disconnect(250) // wait 250 milisec to complete pending actions
}

// TXPacketChan returns the TXPacketBytes channel.
func (b *Backend) TXPacketChan() chan gw.TXPacketBytes {
	return b.txPacketChan
}

// SubscribeGatewayTX subscribes the backend to the gateway TXPacketBytes
// topic (packets the gateway needs to transmit).
func (b *Backend) SubscribeGatewayTX(mac lorawan.EUI64) error {
	defer b.mutex.Unlock()
	b.mutex.Lock()

	topic := fmt.Sprintf("gateway/%s/tx", mac.String())
	log.WithField("topic", topic).Info("backend: subscribing to topic")
	if token := b.conn.Subscribe(topic, 0, b.txPacketHandler); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	b.gateways[mac] = struct{}{}
	return nil
}

// UnSubscribeGatewayTX unsubscribes the backend from the gateway TXPacketBytes
// topic.
func (b *Backend) UnSubscribeGatewayTX(mac lorawan.EUI64) error {
	defer b.mutex.Unlock()
	b.mutex.Lock()

	topic := fmt.Sprintf("gateway/%s/tx", mac.String())
	log.WithField("topic", topic).Info("backend: unsubscribing from topic")
	if token := b.conn.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	delete(b.gateways, mac)
	return nil
}

type statsP struct {
	Stat statsPacketBytes `json:"stat"`
}
type statsPacketBytes struct {
	Time                gateway.ExpandedTime `json:"time,omitempty"`
	Latitude            *float64             `json:"lati,omitempty"`
	Longitude           *float64             `json:"long,omitempty"`
	Altitude            *float64             `json:"alti,omitempty"`
	RXPacketsReceived   int                  `json:"rxnb"`
	RXPacketsReceivedOK int                  `json:"rxok"`
	TXPacketsReceived   int                  `json:"txnb"`
	TXPacketsEmitted    int                  `json:"txPacketsEmitted"`
}

func NewstatsPacketBytes(mac lorawan.EUI64, stats gw.GatewayStatsPacket) *statsPacketBytes {
	v := new(statsPacketBytes)
	v.Time = gateway.ExpandedTime(stats.Time)
	v.Latitude = stats.Latitude
	v.Longitude = stats.Longitude
	v.Altitude = stats.Altitude
	v.RXPacketsReceived = stats.RXPacketsReceived
	v.RXPacketsReceivedOK = stats.RXPacketsReceivedOK
	v.TXPacketsReceived = stats.TXPacketsReceived
	v.TXPacketsEmitted = stats.TXPacketsEmitted
	return v
}

type rxP struct {
	RX rxPacketBytes `json:"rxpk"`
}
type rxPacketBytes struct {
	Time      gateway.ExpandedTime `json:"time,omitempty"` // receive time
	Timestamp uint32               `json:"tmst"`           // gateway internal receive timestamp with microsecond precision, will rollover every ~ 72 minutes
	Channel   int                  `json:"chan"`           // concentrator IF channel used for RX
	RFChain   int                  `json:"rfch"`           // RF chain used for RX
	Frequency float64              `json:"freq"`           // frequency in Hz
	CRCStatus int                  `json:"stat"`           // 1 = OK, -1 = fail, 0 = no CRC

	Modulation band.Modulation `json:"modu"`
	DateRate   string          `json:"datr"`
	CodeRate   string          `json:"codr"` // ECC code rate
	RSSI       int             `json:"rssi"` // RSSI in dBm
	LoRaSNR    float64         `json:"lsnr"` // LoRa signal-to-noise ratio in dB
	Size       int             `json:"size"` // packet payload size
	Data       []byte          `json:"data"`
}

func NewrxPacketBytes(mac lorawan.EUI64, rx gw.RXPacketBytes) *rxPacketBytes {
	v := new(rxPacketBytes)
	v.Time = gateway.ExpandedTime(rx.RXInfo.Time)
	v.Timestamp = rx.RXInfo.Timestamp
	v.Channel = rx.RXInfo.Channel
	v.RFChain = rx.RXInfo.RFChain
	v.Frequency = float64(rx.RXInfo.Frequency) / 1000000.0
	v.CRCStatus = rx.RXInfo.CRCStatus
	v.Modulation = rx.RXInfo.DataRate.Modulation
	s := fmt.Sprintf("SF%dBW%d", rx.RXInfo.DataRate.SpreadFactor, rx.RXInfo.DataRate.Bandwidth)
	v.DateRate = s
	v.CodeRate = rx.RXInfo.CodeRate
	v.RSSI = rx.RXInfo.RSSI
	v.LoRaSNR = rx.RXInfo.LoRaSNR
	v.Size = rx.RXInfo.Size
	v.Data = rx.PHYPayload
	return v
}

// PublishGatewayRX publishes a RX packet to the MQTT broker.
func (b *Backend) PublishGatewayRX(mac lorawan.EUI64, rxPacket gw.RXPacketBytes) error {
	topic := fmt.Sprintf("gateway/%s/rx", mac.String())
	//fmt.Println("==>joy: %s\n", rxPacket)
	rx := NewrxPacketBytes(mac, rxPacket)
	//var r rxP
	r := rxP{RX: *rx}
	//return b.publish(topic, rxPacket)
	return b.publish(topic, r)
}

// PublishGatewayStats publishes a GatewayStatsPacket to the MQTT broker.
func (b *Backend) PublishGatewayStats(mac lorawan.EUI64, stats gw.GatewayStatsPacket) error {
	topic := fmt.Sprintf("gateway/%s/stats", mac.String())
	stat := NewstatsPacketBytes(mac, stats)
	r := statsP{Stat: *stat}
	//return b.publish(topic, stats)
	return b.publish(topic, r)
}

func (b *Backend) publish(topic string, v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	log.WithField("topic", topic).Info("backend: publishing packet")
	str := string(bytes)
	fmt.Println("==>joy:", str, "\n")
	log.Debugln("connect stat:", b.conn.IsConnected())
	if token := b.conn.Publish(topic, 0, false, bytes); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

type txpk struct {
	Imme      bool    `json:"imme"`
	Timestamp uint32  `json:"tmst,omitempty"`
	Freq      float64 `json:"freq"`
	RFch      int     `json:"rfch"`
	Powe      int     `json:"powe"`
	Modu      string  `json:"modu"`
	Datr      string  `json:"datr"`
	Codr      string  `json:"codr"`
	Ipol      bool    `json:"ipol"`
	Size      int     `json:"size"`
	Data      string  `json:"data"`
}
type txP struct {
	TX txpk `json:"txpk"`
}

func getMACFromTopic(s string) string {
	r, _ := regexp.Compile("/.+/")
	match := r.FindString(s)
	del, _ := regexp.Compile("/")
	mac := del.ReplaceAllString(match, "")
	return mac
}
func str2Slice(s string) [8]byte {
	var b [8]byte
	tmpStr := ""
	for i, v := range s {
		tmpStr += string(v)
		if i%2 == 1 {
			var c byte
			fmt.Sscanf(tmpStr, "%2x", &c)
			//fmt.Printf(tmpStr, "%2x", &c)
			b[i/2] = c
			tmpStr = ""
		}
	}
	return b
}
func (b *Backend) txPacketHandler(c mqtt.Client, msg mqtt.Message) {
	log.WithField("topic", msg.Topic()).Info("backend: packet received")
	fmt.Println("==>jot txpk:", string(msg.Payload()), "\n")
	var tx txpk
	var tx1 txP
	if err := json.Unmarshal(msg.Payload(), &tx1); err != nil {
		log.Errorf("backend: decode tx packet error: %s", err)
		return
	}
	tx = tx1.TX

	fmt.Println("==>jot txpk 222:", tx, "\n")

	var txPacket gw.TXPacketBytes
	var sf int

	fmt.Sscanf(tx.Datr, "SF%d", &sf)
	var smac string = getMACFromTopic(msg.Topic())
	fmt.Println("smac ", smac)
	var mac [8]byte = str2Slice(smac)
	fmt.Println("mac ", mac)

	txPacket.TXInfo.MAC = lorawan.EUI64(mac)
	txPacket.TXInfo.Immediately = tx.Imme
	txPacket.TXInfo.Timestamp = tx.Timestamp
	txPacket.TXInfo.Frequency = int(tx.Freq * 1000000)
	txPacket.TXInfo.Power = tx.Powe
	txPacket.TXInfo.DataRate.Modulation = band.Modulation("LORA")
	txPacket.TXInfo.DataRate.SpreadFactor = sf
	txPacket.TXInfo.DataRate.Bandwidth = 125
	txPacket.TXInfo.CodeRate = tx.Codr
	txPacket.TXInfo.IPol = &tx.Ipol
	decodeBytes, err := base64.StdEncoding.DecodeString(tx.Data)
	if err != nil {
		fmt.Println("ERR:txpk base 64")
	}
	txPacket.PHYPayload = decodeBytes //[]byte(tx.Data)
	fmt.Println("==>joy:", string(txPacket.PHYPayload), ":::", tx.Data)
	//if err := json.Unmarshal(msg.Payload(), &txPacket); err != nil {
	//	log.Errorf("backend: decode tx packet error: %s", err)
	//	return
	//}
	b.txPacketChan <- txPacket
}

func (b *Backend) onConnected(c mqtt.Client) {
	defer b.mutex.RUnlock()
	b.mutex.RLock()

	log.Info("backend: connected to mqtt broker")
	if len(b.gateways) > 0 {
		for {
			log.WithField("topic_count", len(b.gateways)).Info("backend: re-registering to gateway topics")
			topics := make(map[string]byte)
			for k := range b.gateways {
				topics[fmt.Sprintf("gateway/%s/tx", k)] = 0
			}
			if token := b.conn.SubscribeMultiple(topics, b.txPacketHandler); token.Wait() && token.Error() != nil {
				log.WithField("topic_count", len(topics)).Errorf("backend: subscribe multiple failed: %s", token.Error())
				time.Sleep(time.Second)
				continue
			}
			return
		}
	}
}

func (b *Backend) onConnectionLost(c mqtt.Client, reason error) {
	log.Errorf("backend: mqtt connection error: %s", reason)
}
