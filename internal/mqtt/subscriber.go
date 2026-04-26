package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"github.com/openmohaa/stats-api/internal/handlers"
	"github.com/openmohaa/stats-api/internal/models"
)

// MaxPayloadSize limits MQTT message payloads (1MB, matching HTTP MaxBodySize)
const MaxPayloadSize = 1048576

// Prometheus metrics for MQTT ingestion path
var (
	mqttMessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mohaa_mqtt_messages_received_total",
		Help: "Total MQTT messages received by topic type",
	}, []string{"topic_type"})

	mqttEventsIngested = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_mqtt_events_ingested_total",
		Help: "Total events ingested via MQTT",
	})

	mqttParseErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_mqtt_parse_errors_total",
		Help: "Total MQTT messages that failed to parse",
	})

	mqttPayloadDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_mqtt_payload_dropped_total",
		Help: "Total MQTT payloads dropped (too large or empty)",
	})

	mqttConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mohaa_mqtt_connected",
		Help: "Whether the MQTT subscriber is connected (1=yes, 0=no)",
	})
)

// Config holds MQTT subscriber configuration
type Config struct {
	BrokerURL    string
	ClientID     string
	TopicPrefix  string
	Username     string
	Password     string
	QoS          byte
	CleanSession bool
}

// Subscriber listens to MQTT topics and feeds events into the worker pool
type Subscriber struct {
	config Config
	client pahomqtt.Client
	pool   handlers.IngestQueue
	logger *zap.SugaredLogger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSubscriber creates a new MQTT subscriber
func NewSubscriber(cfg Config, pool handlers.IngestQueue, logger *zap.Logger) *Subscriber {
	return &Subscriber{
		config: cfg,
		pool:   pool,
		logger: logger.Sugar(),
	}
}

// Start connects to the MQTT broker and subscribes to event topics
func (s *Subscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	opts := pahomqtt.NewClientOptions().
		AddBroker(s.config.BrokerURL).
		SetClientID(s.config.ClientID).
		SetCleanSession(s.config.CleanSession).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(30 * time.Second).
		SetKeepAlive(60 * time.Second).
		SetConnectionLostHandler(s.onConnectionLost).
		SetOnConnectHandler(s.onConnect).
		SetOrderMatters(false)

	if s.config.Username != "" {
		opts.SetUsername(s.config.Username)
	}
	if s.config.Password != "" {
		opts.SetPassword(s.config.Password)
	}

	s.client = pahomqtt.NewClient(opts)

	token := s.client.Connect()
	if token.WaitTimeout(10*time.Second) && token.Error() != nil {
		return fmt.Errorf("mqtt connect failed: %w", token.Error())
	}

	s.logger.Infow("MQTT subscriber connected",
		"broker", s.config.BrokerURL,
		"clientID", s.config.ClientID,
	)
	mqttConnected.Set(1)

	return nil
}

// Stop disconnects from the MQTT broker
func (s *Subscriber) Stop() {
	s.cancel()
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect(5000) // 5s graceful disconnect
	}
	s.wg.Wait()
	mqttConnected.Set(0)
	s.logger.Info("MQTT subscriber stopped")
}

// IsConnected returns the MQTT connection status
func (s *Subscriber) IsConnected() bool {
	return s.client != nil && s.client.IsConnected()
}

// onConnect is called when the MQTT client connects/reconnects
func (s *Subscriber) onConnect(client pahomqtt.Client) {
	s.logger.Info("MQTT connected, subscribing to topics...")
	mqttConnected.Set(1)

	// Subscribe to event topics: openmohaa/events/#
	eventTopic := s.config.TopicPrefix + "/events/#"
	token := client.Subscribe(eventTopic, s.config.QoS, s.handleEventMessage)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		s.logger.Errorw("Failed to subscribe to event topic", "topic", eventTopic, "error", token.Error())
		return
	}
	s.logger.Infow("Subscribed to event topic", "topic", eventTopic)

	// Subscribe to server registration: openmohaa/servers/#
	serverTopic := s.config.TopicPrefix + "/servers/#"
	token = client.Subscribe(serverTopic, 1, s.handleServerMessage)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		s.logger.Errorw("Failed to subscribe to server topic", "topic", serverTopic, "error", token.Error())
		return
	}
	s.logger.Infow("Subscribed to server topic", "topic", serverTopic)
}

// onConnectionLost is called when the MQTT connection drops
func (s *Subscriber) onConnectionLost(_ pahomqtt.Client, err error) {
	mqttConnected.Set(0)
	s.logger.Warnw("MQTT connection lost, will auto-reconnect", "error", err)
}

// handleEventMessage processes incoming game telemetry events
func (s *Subscriber) handleEventMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	payload := msg.Payload()
	topic := msg.Topic()

	mqttMessagesReceived.WithLabelValues("events").Inc()

	// Guard against oversized payloads (matching HTTP 1MB limit)
	if len(payload) > MaxPayloadSize {
		mqttPayloadDropped.Inc()
		s.logger.Warnw("MQTT payload too large, dropping",
			"topic", topic,
			"payloadLen", len(payload),
			"maxSize", MaxPayloadSize,
		)
		return
	}

	// Extract server_id from topic: openmohaa/events/{server_id}
	serverID := extractServerID(topic, s.config.TopicPrefix+"/events/")

	s.logger.Debugw("MQTT event received",
		"topic", topic,
		"serverID", serverID,
		"payloadLen", len(payload),
	)

	events, err := parsePayload(payload)
	if err != nil {
		mqttParseErrors.Inc()
		s.logger.Warnw("Failed to parse MQTT payload",
			"topic", topic,
			"error", err,
			"preview", string(payload[:min(len(payload), 200)]),
		)
		return
	}

	// Enqueue events into the same worker pool as HTTP
	for i := range events {
		if serverID != "" && events[i].ServerID == "" {
			events[i].ServerID = serverID
		}
		if events[i].Type == "" {
			continue
		}
		handlers.NormalizeRawEventAliases(&events[i])
		s.pool.Enqueue(&events[i])
		mqttEventsIngested.Inc()
	}
}

// parsePayload deserializes a JSON payload into RawEvents.
// Supports JSON array or single JSON object formats.
func parsePayload(payload []byte) ([]models.RawEvent, error) {
	trimmed := strings.TrimSpace(string(payload))
	if len(trimmed) == 0 {
		return nil, nil
	}

	var events []models.RawEvent

	if trimmed[0] == '[' {
		if err := json.Unmarshal([]byte(trimmed), &events); err != nil {
			return nil, fmt.Errorf("invalid JSON array: %w", err)
		}
	} else if trimmed[0] == '{' {
		var event models.RawEvent
		if err := json.Unmarshal([]byte(trimmed), &event); err != nil {
			return nil, fmt.Errorf("invalid JSON object: %w", err)
		}
		events = append(events, event)
	} else {
		return nil, fmt.Errorf("unknown format: first byte %q", trimmed[0])
	}

	return events, nil
}

// handleServerMessage processes server registration/heartbeat messages
func (s *Subscriber) handleServerMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	mqttMessagesReceived.WithLabelValues("servers").Inc()
	s.logger.Debugw("MQTT server message received",
		"topic", msg.Topic(),
		"payloadLen", len(msg.Payload()),
	)
	// Server registration via MQTT can be handled here in the future.
	// For now, registration continues via HTTP POST /api/v1/servers/register.
}

// extractServerID parses the server ID from the topic path
func extractServerID(topic, prefix string) string {
	if !strings.HasPrefix(topic, prefix) {
		return ""
	}
	remainder := topic[len(prefix):]
	// Take first path segment
	if idx := strings.Index(remainder, "/"); idx >= 0 {
		return remainder[:idx]
	}
	return remainder
}
