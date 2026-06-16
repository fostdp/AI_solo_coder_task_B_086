package era_comparator

import (
	"github.com/gorilla/mux"

	"zhaozhou-bridge-monitor/services"
)

type EraComparator struct {
	WorkerPool *services.FEMWorkerPool
}

func NewEraComparator(pool *services.FEMWorkerPool) *EraComparator {
	if pool == nil {
		pool = services.NewFEMWorkerPool(2)
	}
	return &EraComparator{
		WorkerPool: pool,
	}
}

func (ec *EraComparator) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/v2").Subrouter()
	api.HandleFunc("/era-comparison/materials", ec.HandleCompare).Methods("GET", "POST")
}
