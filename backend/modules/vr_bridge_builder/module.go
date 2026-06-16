package vr_bridge_builder

import (
	"github.com/gorilla/mux"

	"zhaozhou-bridge-monitor/services"
)

type VRBridgeBuilder struct {
	WorkerPool *services.FEMWorkerPool
}

func NewVRBridgeBuilder(pool *services.FEMWorkerPool) *VRBridgeBuilder {
	if pool == nil {
		pool = services.NewFEMWorkerPool(3)
	}
	return &VRBridgeBuilder{
		WorkerPool: pool,
	}
}

func (vb *VRBridgeBuilder) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/v2").Subrouter()
	api.HandleFunc("/vr-bridge/design", vb.HandleDesign).Methods("POST")
	api.HandleFunc("/vr-bridge/presets", vb.HandlePresets).Methods("GET")
}
