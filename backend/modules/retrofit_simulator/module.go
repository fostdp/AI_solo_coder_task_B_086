package retrofit_simulator

import (
	"github.com/gorilla/mux"

	"zhaozhou-bridge-monitor/services"
)

type RetrofitSimulator struct {
	WorkerPool *services.FEMWorkerPool
}

func NewRetrofitSimulator(pool *services.FEMWorkerPool) *RetrofitSimulator {
	if pool == nil {
		pool = services.NewFEMWorkerPool(2)
	}
	return &RetrofitSimulator{
		WorkerPool: pool,
	}
}

func (rs *RetrofitSimulator) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/v2").Subrouter()
	api.HandleFunc("/retrofit/simulate", rs.HandleSimulate).Methods("POST")
}
