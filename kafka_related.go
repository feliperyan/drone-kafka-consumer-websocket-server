package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

var (
	theBroker  string
	allBrokers []string
	theTopic   string
	usingTLS   bool
	certPEM    string
	keyPEM     string
	caPEM      string
)

func init() {
	theBroker = getOSEnvOrReplacement("KAFKA_URL", "localhost:9092")
	_, usingTLS = os.LookupEnv("KAFKA_CLIENT_CERT")
	certPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT", "")
	keyPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT_KEY", "")
	caPEM = getOSEnvOrReplacement("KAFKA_TRUSTED_CERT", "")

	theBroker = strings.ReplaceAll(theBroker, "kafka+ssl://", "")
	allBrokers = strings.Split(theBroker, ",")

	theTopic = getOSEnvOrReplacement("FRYAN_TOPIC", "drone-coordinates")
	topicPrefix := getOSEnvOrReplacement("KAFKA_PREFIX", "")
	theTopic = fmt.Sprintf("%s%s", topicPrefix, theTopic)
}

func getOSEnvOrReplacement(envVarName, valueIfNotFound string) string {
	thing, found := os.LookupEnv(envVarName)
	if found {
		return thing
	}
	return valueIfNotFound
}

func getTLSConfig() *tls.Config {
	// Define TLS configuration
	certificate, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		panic(fmt.Sprintf("X509KeyPair errored out: %s", err))
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(caPEM)); !ok {
		panic("x509.NewCertPool errored out.")

	}

	return &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
}

func initialiseKafkaReader(needsTLS bool) *kafka.Reader {

	if !needsTLS {
		rea := kafka.NewReader(kafka.ReaderConfig{
			Brokers:     allBrokers,
			Topic:       theTopic,
			GroupID:     "group-websocket-1",
			StartOffset: kafka.LastOffset,
		})
		return rea
	}

	tconf := getTLSConfig()
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       tconf,
	}

	rea := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     allBrokers,
		Topic:       theTopic,
		Dialer:      dialer,
		GroupID:     "group-websocket-1",
		StartOffset: kafka.LastOffset,
	})

	return rea
}