package arch_comparator

import (
	"github.com/gorilla/mux"

	"zhaozhou-bridge-monitor/services"
)

type ArchComparator struct {
	WorkerPool *services.FEMWorkerPool
}

func NewArchComparator(pool *services.FEMWorkerPool) *ArchComparator {
	if pool == nil {
		pool = services.NewFEMWorkerPool(2)
	}
	return &ArchComparator{
		WorkerPool: pool,
	}
}

func (ac *ArchComparator) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/v2").Subrouter()
	api.HandleFunc("/arch-comparison/spandrel", ac.HandleCompare).Methods("GET", "POST")
	api.HandleFunc("/arch-comparison/spandrel/async", ac.HandleCompareAsync).Methods("POST")
}
