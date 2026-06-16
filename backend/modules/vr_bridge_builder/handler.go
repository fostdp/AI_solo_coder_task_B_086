package vr_bridge_builder

import (
	"encoding/json"
	"net/http"

	"zhaozhou-bridge-monitor/models"
)

func (vb *VRBridgeBuilder) HandleDesign(w http.ResponseWriter, r *http.Request) {
	var design models.VirtualBridgeDesign
	if err := json.NewDecoder(r.Body).Decode(&design); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := vb.DesignAndTest(design)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type PresetResponse struct {
	Materials map[string]map[string]interface{} `json:"materials"`
	Limits    map[string]map[string][2]float64  `json:"limits"`
}

func (vb *VRBridgeBuilder) HandlePresets(w http.ResponseWriter, r *http.Request) {
	materials := make(map[string]map[string]interface{})
	for key, mat := range MaterialPresets {
		materials[key] = map[string]interface{}{
			"material_name": mat.MaterialName,
			"source":        mat.Source,
			"grade":         mat.Grade,
			"elastic_modulus_gpa": mat.ElasticModulus / 1e9,
			"compressive_strength_mpa": mat.CompressiveStrength / 1e6,
			"density":         mat.Density,
		}
	}

	resp := PresetResponse{
		Materials: materials,
		Limits:    ParamLimits,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
