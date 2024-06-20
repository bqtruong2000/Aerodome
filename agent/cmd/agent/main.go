package main

import (
	"cloud.google.com/go/firestore"
	"context"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/eclipse/paho.golang/autopaho"
	paho "github.com/eclipse/paho.golang/paho"
	"github.com/spf13/viper"
	"google.golang.org/api/option"
	"log"
	"net/url"
	"runwayclub.dev/aerodome/agent/domain"
	"time"
)

func main() {
	// read agent.yaml file with viper

	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	// read the config file
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	fmt.Println(viper.GetString("serialNumber"))

	// init mqtt client
	u, err := url.Parse("mqtt://broker.emqx.io:1883")
	if err != nil {
		panic(err)
	}

	wateringEnabled := false
	fanEnabled := false

	// received data
	currentData := &domain.SensorData{
		Temperature:     0,
		Humidity:        0,
		WateringEnabled: wateringEnabled,
		FanEnabled:      fanEnabled,
	}

	cliCfg := autopaho.ClientConfig{
		ServerUrls: []*url.URL{u},
		KeepAlive:  20, // Keepalive message should be sent every 20 seconds
		// CleanStartOnInitialConnection defaults to false. Setting this to true will clear the session on the first connection.
		CleanStartOnInitialConnection: false,
		// SessionExpiryInterval - Seconds that a session will survive after disconnection.
		// It is important to set this because otherwise, any queued messages will be lost if the connection drops and
		// the server will not queue messages while it is down. The specific setting will depend upon your needs
		// (60 = 1 minute, 3600 = 1 hour, 86400 = one day, 0xFFFFFFFE = 136 years, 0xFFFFFFFF = don't expire)
		SessionExpiryInterval: 60,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			fmt.Println("mqtt connection up")
			// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if
			// the connection drops)
			if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{
					{Topic: fmt.Sprintf("AERODOME/%v/sensors", viper.GetString("serialNumber")), QoS: 0},
				},
			}); err != nil {
				fmt.Printf("failed to subscribe (%s). This is likely to mean no messages will be received.", err)
			}
			fmt.Println("mqtt subscription made")
		},
		OnConnectError: func(err error) { fmt.Printf("error whilst attempting connection: %s\n", err) },
		// eclipse/paho.golang/paho provides base mqtt functionality, the below config will be passed in for each connection
		ClientConfig: paho.ClientConfig{
			// If you are using QOS 1/2, then it's important to specify a client id (which must be unique)
			ClientID: "AERODOME",
			// OnPublishReceived is a slice of functions that will be called when a message is received.
			// You can write the function(s) yourself or use the supplied Router
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					data, err := domain.NewSensorDataFromRawData(pr.Packet.Payload)
					if err != nil {
						fmt.Printf("error parsing sensor data: %s\n", err)
						return true, nil
					}
					log.Printf("received sensor data: %+v\n", data)
					currentData = data
					return true, nil
				}},
			OnClientError: func(err error) { fmt.Printf("client error: %s\n", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					fmt.Printf("server requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					fmt.Printf("server requested disconnect; reason code: %d\n", d.ReasonCode)
				}
			},
		},
	}

	// setup firebase config
	opt := option.WithCredentialsFile("config/aerodome-agent-key.json")
	// setup firebase app
	_, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		fmt.Printf("error initializing app: %v\n", err)
		panic(err)
	} else {
		fmt.Printf("app initialized\n")
	}

	const projectID = "aerodome-agent"
	// setup firestore client & projectID
	client, err := firestore.NewClient(context.Background(), projectID, opt)
	if err != nil {
		fmt.Printf("error initializing firestore client: %v\n", err)
	} else {
		fmt.Printf("firestore client initialized\n")

	}

	ctx := context.Background()

	c, err := autopaho.NewConnection(ctx, cliCfg) // starts process; will reconnect until context cancelled
	if err != nil {
		panic(err)
	}
	// Wait for the connection to come up
	if err = c.AwaitConnection(ctx); err != nil {
		panic(err)
	}

	publishTopic := fmt.Sprintf("AERODOME/%v", viper.GetString("serialNumber"))

	// create ticker
	timer := time.NewTicker(5 * time.Second)
	deltaT := time.Now().UnixMilli()

	// create ticker for 5 minutes
	publishTicker := time.NewTicker(3 * time.Minute)
	defer publishTicker.Stop()

	onWateringMode := false
	wateringRetainTime := int64(0)

	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			deltaT = time.Now().UnixMilli() - deltaT

			if onWateringMode {
				wateringEnabled = true
				fanEnabled = true
			} else {
				wateringEnabled = false
				fanEnabled = false
			}
			if currentData.Humidity < viper.GetFloat64("thresholds.humidity.low")*100 {
				onWateringMode = true
				wateringRetainTime = 0
			}
			if currentData.Humidity > viper.GetFloat64("thresholds.humidity.high")*100 {
				wateringRetainTime += deltaT
				if wateringRetainTime > viper.GetInt64("thresholds.humidity.duration")*1000 {
					onWateringMode = false
				}
			}
			if currentData.Humidity < viper.GetFloat64("thresholds.humidity.invalid")*100 {
				onWateringMode = false

			}
			// publish data
			fanData := "00"
			if fanEnabled {
				fanData = "01"
			}
			wateringData := "10"
			if wateringEnabled {
				wateringData = "11"
			}
			if _, err = c.Publish(ctx, &paho.Publish{
				QoS:     1,
				Topic:   publishTopic,
				Payload: []byte(fanData),
			}); err != nil {
				if ctx.Err() == nil {
					panic(err) // Publish will exit when context cancelled or if something went wrong
				}
			}
			if _, err = c.Publish(ctx, &paho.Publish{
				QoS:     1,
				Topic:   publishTopic,
				Payload: []byte(wateringData),
			}); err != nil {
				if ctx.Err() == nil {
					panic(err) // Publish will exit when context cancelled or if something went wrong
				}
			}
			deltaT = time.Now().UnixMilli()
		case <-publishTicker.C:
			collectionName := time.Now().Format(time.DateOnly)
			_, _, err = client.Collection(collectionName).Add(ctx, map[string]interface{}{
				"temperature": currentData.Temperature,
				"humidity":    currentData.Humidity,
				"watering":    wateringEnabled,
				"fan":         fanEnabled,
				// timestamp is Vietnam timezone
				"timestamp": time.Now().UTC().Add(7 * time.Hour).Format(time.TimeOnly),
			})
			if err != nil {
				fmt.Printf("error adding document: %v\n", err)
			} else {
				fmt.Printf("document added\n")
			}
		case <-ctx.Done():
		}
	}

}
