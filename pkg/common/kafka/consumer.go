package kafka

import (
	"sync"

	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/config"

	"github.com/Shopify/sarama"
)

type Consumer struct {
	addr          []string
	WG            sync.WaitGroup
	Topic         string
	PartitionList []int32
	Consumer      sarama.Consumer
}

func NewKafkaConsumer(addr []string, topic string) *Consumer {
	p := Consumer{}
	p.Topic = topic
	p.addr = addr
	consumerConfig := sarama.NewConfig()
	if config.Config.Kafka.SASLUserName != "" && config.Config.Kafka.SASLPassword != "" {
		consumerConfig.Net.SASL.Enable = true
		consumerConfig.Net.SASL.User = config.Config.Kafka.SASLUserName
		consumerConfig.Net.SASL.Password = config.Config.Kafka.SASLPassword
	}
	consumer, err := sarama.NewConsumer(p.addr, consumerConfig)
	if err != nil {
		panic(err.Error())
	}
	p.Consumer = consumer

	partitionList, err := consumer.Partitions(p.Topic)
	if err != nil {
		panic(err.Error())
	}
	p.PartitionList = partitionList

	return &p
}
