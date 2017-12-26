package coap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/brocaar/lora-gateway-bridge/gateway"
	"github.com/brocaar/loraserver/api/gw"
	"github.com/brocaar/lorawan"
	log "github.com/sirupsen/logrus"
)

// Backend implements a COAP pub-sub backend.
type Backend struct {
	conn *Conn
	//conn mqtt.Client
	txPacketChan chan gateway.TXPK //gw.TXPacketBytes
	gateways     map[lorawan.EUI64]struct{}
	mutex        sync.RWMutex
}

func NewBackend(server string) (*Backend, error) {
	b := Backend{
		txPacketChan: make(chan gateway.TXPK), //gw.TXPacketBytes),
		gateways:     make(map[lorawan.EUI64]struct{}),
		conn:         nil,
	}
	log.WithField("server", server).Info("backend: connecting to coap server")
	brokerURL, err := url.Parse(server)
	if err != nil {
		log.Fatalf("Error URL: %v", err)
		return nil, err
	}
	c, err := Dial(brokerURL.Scheme, brokerURL.Host)
	if err != nil {
		log.Fatalf("Error dialing: %v %v", err, c)
		return nil, err
	}
	b.conn = c
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
	topic := fmt.Sprintf("gateway/%s/tx", mac.String())
	log.WithField("topic", topic).Info("backend: subscribing to topic")

	for b.conn != nil {
		msg, err := b.conn.Receive()
		if err != nil {
			log.Fatalf("Error receiving: %v", err)
		}
		topic = msg.Path()[0]
		log.WithField("topic", topic).Info("backend: recv topic")
		fmt.Println("==>jot txpk:", string(msg.Payload), "\n")
		var tx gateway.TXPK
		if err := json.Unmarshal(msg.Payload, &tx); err != nil {
			log.Errorf("backend: decode tx packet error: %s", err)

			continue
		}
		fmt.Println("==>jot txpk 222:", tx, "\n")

		b.txPacketChan <- tx
	}
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
	req := Message{
		Type:      NonConfirmable,
		Code:      PUT,
		MessageID: 12345,
		Payload:   bytes,
	}

	req.SetOption(ContentFormat, AppJSON)
	req.SetPathString(topic)
	//c, err := Dial("udp", "127.0.0.1:5683")
	//if err != nil {
	//	log.Fatalf("Error dialing: %v %v", err, c)
	//}
	rv, err := b.conn.Send(req)
	if err != nil {
		log.Fatalf("Error sending req: %v", err)
	}
	if rv != nil {
		log.Printf("Response payload: %s", rv.Payload)
	}

	//if token := b.conn.Publish(topic, 0, false, bytes); token.Wait() && token.Error() != nil {
	//	return token.Error()
	//}
	return nil
}
