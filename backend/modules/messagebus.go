package modules

import (
	"time"

	"zhaozhou-bridge-monitor/models"
)

type BusMessage struct {
	Type          string
	Source        string
	Timestamp     time.Time
	CorrelationID string
	Payload       interface{}
}

type FEMRequestPayload struct {
	LiveLoadPa float64
	DeltaTC    float64
	RequestID  string
}

type FEMResultPayload struct {
	RequestID string
	Stresses  []models.FEMStressResult
	Elements  []models.FEMElement
	Nodes     []models.FEMNode
	ComputeMs int64
	Error     string
}

type PredictionRequestPayload struct {
	TargetYears []int
	RefTime     time.Time
}

type PredictionResultPayload struct {
	Predictions []models.DeformationPrediction
	AgeReport   map[string]float64
	ComputeMs   int64
	Error       string
}

type AlertEvaluatePayload struct {
	AllLatestReadings []models.SensorReading
	HistoryBySensor   map[string][]models.SensorReading
	FEMStresses       []models.FEMStressResult
}

type AlertResultPayload struct {
	Generated    []models.Alert
	MQTTPublished int
}

type MessageBus struct {
	SensorReadingCh chan models.SensorReading
	FEMRequestCh    chan FEMRequestPayload
	FEMResultCh     chan FEMResultPayload
	PredictionReqCh chan PredictionRequestPayload
	PredictionResCh chan PredictionResultPayload
	AlertEvalCh     chan AlertEvaluatePayload
	AlertResultCh   chan AlertResultPayload
	BroadCastCh     chan BusMessage
	ShutdownCh      chan struct{}
}

func NewMessageBus() *MessageBus {
	return &MessageBus{
		SensorReadingCh: make(chan models.SensorReading, 100),
		FEMRequestCh:    make(chan FEMRequestPayload, 10),
		FEMResultCh:     make(chan FEMResultPayload, 10),
		PredictionReqCh: make(chan PredictionRequestPayload, 10),
		PredictionResCh: make(chan PredictionResultPayload, 10),
		AlertEvalCh:     make(chan AlertEvaluatePayload, 20),
		AlertResultCh:   make(chan AlertResultPayload, 10),
		BroadCastCh:     make(chan BusMessage, 50),
		ShutdownCh:      make(chan struct{}),
	}
}

func (b *MessageBus) Close() {
	close(b.SensorReadingCh)
	close(b.FEMRequestCh)
	close(b.FEMResultCh)
	close(b.PredictionReqCh)
	close(b.PredictionResCh)
	close(b.AlertEvalCh)
	close(b.AlertResultCh)
	close(b.BroadCastCh)
	close(b.ShutdownCh)
}
