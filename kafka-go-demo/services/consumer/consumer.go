package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"kafkaDemo/services"
	"os/signal"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
)

//var cfg *base.MqConfig
var consumer *cluster.Consumer
var sig chan os.Signal

func init() {
	fmt.Println("init kafka consumer")

	var err error

	cfg := &configs.MqConfig{}
	configs.LoadJsonConfig(cfg, "mq.json")

	clusterCfg := cluster.NewConfig()

	clusterCfg.Net.SASL.Enable = true
	clusterCfg.Net.SASL.User = cfg.Ak
	clusterCfg.Net.SASL.Password = cfg.Password
	clusterCfg.Net.SASL.Handshake = true

	certBytes, err := ioutil.ReadFile(configs.GetFullPath(cfg.CertFile))
	clientCertPool := x509.NewCertPool()
	ok := clientCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		panic("kafka consumer failed to parse root certificate")
	}

	clusterCfg.Net.TLS.Config = &tls.Config{
		//Certificates:       []tls.Certificate{},
		RootCAs:            clientCertPool,
		InsecureSkipVerify: true,
	}

	clusterCfg.Net.TLS.Enable = true
	clusterCfg.Consumer.Return.Errors = true
	clusterCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	clusterCfg.Group.Return.Notifications = true

	clusterCfg.Version = sarama.V0_10_0_0
	if err = clusterCfg.Validate(); err != nil {
		msg := fmt.Sprintf("Kafka consumer config invalidate. config: %v. err: %v", *clusterCfg, err)
		fmt.Println(msg)
		panic(msg)
	}

	consumer, err = cluster.NewConsumer(cfg.Servers, cfg.ConsumerId, []string{cfg.Topics.Gift}, clusterCfg)
	if err != nil {
		msg := fmt.Sprintf("Create kafka consumer error: %v. config: %v", err, clusterCfg)
		fmt.Println(msg)
		panic(msg)
	}

	sig = make(chan os.Signal, 1)

}

func Start() {
	go consume()
}

func consume() {
	for {
		select {
		case msg, more := <-consumer.Messages():
			if more {

				fmt.Println("kafka consumer msg: %v", *msg)
				consumer.MarkOffset(msg, "") // mark message as processed
			}
		case err, more := <-consumer.Errors():
			if more {
				fmt.Println("Kafka consumer error: %v", err.Error())
			}
		case ntf, more := <-consumer.Notifications():
			if more {
				fmt.Println("Kafka consumer rebalance: %v", ntf)
			}
		case <-sig:
			fmt.Errorf("Stop consumer server...")
			consumer.Close()
			return
		}
	}

}

func Stop(s os.Signal) {
	fmt.Println("Recived kafka consumer stop signal...")
	sig <- s
	fmt.Println("kafka consumer stopped!!!")
}

func main() {

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	Start()

	select {
	case s := <-signals:
		Stop(s)
	}

}