package era_comparator

import (
	"encoding/json"
	"net/http"
)

type CompareRequest struct {
	LiveLoadPa float64 `json:"live_load_pa"`
	DeltaTC    float64 `json:"delta_t_c"`
}

func (ec *EraComparator) HandleCompare(w http.ResponseWriter, r *http.Request) {
	var req CompareRequest
	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if req.LiveLoadPa == 0 {
		req.LiveLoadPa = 4000
	}

	result, err := ec.Compare(req.LiveLoadPa, req.DeltaTC)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
