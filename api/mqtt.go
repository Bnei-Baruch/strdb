package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var MQTT mqtt.Client

type MqttPayload struct {
	Action  string      `json:"action,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Source  string      `json:"src,omitempty"`
	Error   error       `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Result  string      `json:"result,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type JanusResponse struct {
	Janus       string  `json:"janus"`
	Transaction string  `json:"transaction"`
	Sessions    []int64 `json:"sessions"`
}

type PahoLogAdapter struct {
	level log.Level
}

type StrStatus struct {
	Online bool `json:"online"`
}

func NewPahoLogAdapter(level log.Level) *PahoLogAdapter {
	return &PahoLogAdapter{level: level}
}

func (a *PahoLogAdapter) Println(v ...interface{}) {
	log.Infof("MQTT: %s", fmt.Sprint(v...))
}

func (a *PahoLogAdapter) Printf(format string, v ...interface{}) {
	log.Infof("MQTT: %s", fmt.Sprintf(format, v...))
}

func InitMQTT() error {
	log.Info("[InitMQTT] Init")
	if viper.GetString("mqtt.debug") == "true" {
		mqtt.DEBUG = NewPahoLogAdapter(log.DebugLevel)
		mqtt.WARN = NewPahoLogAdapter(log.WarnLevel)
	}
	mqtt.CRITICAL = NewPahoLogAdapter(log.PanicLevel)
	mqtt.ERROR = NewPahoLogAdapter(log.ErrorLevel)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(viper.GetString("mqtt.url"))
	opts.SetClientID(viper.GetString("mqtt.client_id"))
	opts.SetUsername(viper.GetString("mqtt.user"))
	opts.SetPassword(viper.GetString("mqtt.password"))
	opts.SetAutoReconnect(true)
	opts.SetOnConnectHandler(SubMQTT)
	opts.SetConnectionLostHandler(LostMQTT)
	opts.SetBinaryWill(viper.GetString("mqtt.status_topic"), []byte("Offline"), byte(1), true)
	MQTT = mqtt.NewClient(opts)
	if token := MQTT.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	// Start Janus Admin messages sending
	go startPeriodicMessages()

	return nil
}

const maxMissedPings = 3

func startPeriodicMessages() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mutex.Lock()
			for name, server := range StrDB {
				if !server.Enable || !server.Online {
					continue
				}

				server.MissedPing++
				StrDB[name] = server

				if server.MissedPing > maxMissedPings {
					server.Online = false
					server.Sessions = 0
					StrDB[name] = server
					log.WithFields(log.Fields{
						"server":       name,
						"missed_pings": server.MissedPing,
						"last_seen":    server.LastSeen,
					}).Warn("Server marked offline: no response to admin messages")
				} else {
					topic := fmt.Sprintf("janus/%s/to-janus-admin", server.Name)
					go SendAdminMessage(topic)
				}
			}
			mutex.Unlock()
		}
	}
}

func SubMQTT(c mqtt.Client) {
	if token := MQTT.Publish(viper.GetString("mqtt.status_topic"), byte(1), true, []byte("Online")); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] notify status error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] notify status to: %s", viper.GetString("mqtt.status_topic"))
	}

	StrStatusTopic := viper.GetString("mqtt.str_status_topic")
	if token := MQTT.Subscribe(StrStatusTopic, byte(1), HandleStatusMessage); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", StrStatusTopic)
	}

	StrAdminTopic := viper.GetString("mqtt.str_admin_topic")
	if token := MQTT.Subscribe(StrAdminTopic, byte(1), HandleAdminMessage); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", StrAdminTopic)
	}
}

func LostMQTT(c mqtt.Client, err error) {
	log.Errorf("[LostMQTT] Lost connection: %s", err)
}

func SendAdminMessage(topic string) {
	message := map[string]interface{}{
		"janus":        "list_sessions",
		"transaction":  "transaction",
		"admin_secret": viper.GetString("mqtt.admin_secret"),
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		log.Errorf("[SendAdminMessage] Message parsing: %s", err)
		return
	}

	if viper.GetString("mqtt.trace") == "true" {
		log.Debugf("[SendAdminMessage] topic: %s | message: %s", topic, jsonMessage)
	}

	if token := MQTT.Publish(topic, byte(1), false, jsonMessage); token.Wait() && token.Error() != nil {
		log.Errorf("[SendAdminMessage] Pubish: %s", token.Error())
	}
}

func HandleStatusMessage(c mqtt.Client, m mqtt.Message) {
	go func() {
		s := strings.Split(m.Topic(), "/")
		if len(s) < 2 {
			log.Errorf("[HandleStatusMessage] Invalid topic format: %s", m.Topic())
			return
		}

		serverName := s[1]
		chk, _ := regexp.MatchString(`^str\d+$`, serverName)
		if !chk {
			log.WithFields(log.Fields{
				"topic":       m.Topic(),
				"server_name": serverName,
			}).Warn("[HandleStatusMessage] Server name does not match pattern")
			return
		}

		log.WithFields(log.Fields{
			"topic":   m.Topic(),
			"server":  serverName,
			"payload": string(m.Payload()),
		}).Info("[HandleStatusMessage] Received status message")

		var update StrStatus
		if err := json.Unmarshal(m.Payload(), &update); err != nil {
			log.WithFields(log.Fields{
				"server":  serverName,
				"payload": string(m.Payload()),
				"error":   err.Error(),
			}).Error("[HandleStatusMessage] Failed to unmarshal")
			return
		}

		log.WithFields(log.Fields{
			"server": serverName,
			"online": update.Online,
		}).Info("[HandleStatusMessage] Setting server status")

		SetOnline(serverName, update.Online)
	}()
}

func HandleAdminMessage(c mqtt.Client, m mqtt.Message) {
	if viper.GetString("mqtt.trace") == "true" {
		log.Debugf("[HandleAdminMessage] topic: %s | message: %s", m.Topic(), string(m.Payload()))
	}

	go func() {
		s := strings.Split(m.Topic(), "/")
		if len(s) < 2 {
			log.Errorf("[HandleAdminMessage] Invalid topic format: %s", m.Topic())
			return
		}

		serverName := s[1]
		var response JanusResponse
		if err := json.Unmarshal(m.Payload(), &response); err != nil {
			log.Errorf("[HandleAdminMessage] Failed to unmarshal: %s", err)
			return
		}

		if response.Janus == "success" {
			mutex.Lock()
			if server, ok := StrDB[serverName]; ok {
				server.Sessions = len(response.Sessions)
				server.MissedPing = 0
				server.LastSeen = time.Now().Unix()
				StrDB[serverName] = server

				log.WithFields(log.Fields{
					"server":   serverName,
					"sessions": server.Sessions,
					"online":   server.Online,
				}).Debug("[HandleAdminMessage] Updated server sessions")
			}
			mutex.Unlock()
		}
	}()
}
