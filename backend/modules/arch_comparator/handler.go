package arch_comparator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CompareRequest struct {
	LiveLoadPa float64 `json:"live_load_pa"`
	DeltaTC    float64 `json:"delta_t_c"`
}

type AsyncResponse struct {
	JobID string `json:"job_id"`
	URL   string `json:"poll_url"`
}

var asyncResults = make(map[string]chan interface{})

func (ac *ArchComparator) HandleCompare(w http.ResponseWriter, r *http.Request) {
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

	result, err := ac.Compare(req.LiveLoadPa, req.DeltaTC)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (ac *ArchComparator) HandleCompareAsync(w http.ResponseWriter, r *http.Request) {
	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.LiveLoadPa == 0 {
		req.LiveLoadPa = 4000
	}

	jobID := fmt.Sprintf("arch-comp-%d", time.Now().UnixNano())
	asyncCh := ac.CompareAsync(req.LiveLoadPa, req.DeltaTC)
	asyncResults[jobID] = make(chan interface{}, 1)

	go func() {
		result := <-asyncCh
		asyncResults[jobID] <- result
		close(asyncResults[jobID])
	}()

	resp := AsyncResponse{
		JobID: jobID,
		URL:   fmt.Sprintf("/api/v2/arch-comparison/spandrel/async/%s", jobID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}
