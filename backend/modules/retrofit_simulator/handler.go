package retrofit_simulator

import (
	"encoding/json"
	"net/http"

	"zhaozhou-bridge-monitor/models"
)

type SimulateRequest struct {
	Configs    []models.ReinforcementConfig `json:"configs"`
	LiveLoadPa float64                      `json:"live_load_pa"`
	DeltaTC    float64                      `json:"delta_t_c"`
}

func (rs *RetrofitSimulator) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.LiveLoadPa == 0 {
		req.LiveLoadPa = 4000
	}
	if len(req.Configs) == 0 {
		req.Configs = []models.ReinforcementConfig{
			{Zone: "main_arch", Layers: 2, ThicknessMM: 0.334, WidthM: 1.0, BondEfficiencyFactor: 0.75},
		}
	}

	result, err := rs.Simulate(req.Configs, req.LiveLoadPa, req.DeltaTC)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
