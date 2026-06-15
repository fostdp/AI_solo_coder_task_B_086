package config

import (
	"encoding/json"
	"fmt"
	"os"

	"zhaozhou-bridge-monitor/models"
	"zhaozhou-bridge-monitor/services"
)

type AppConfig struct {
	Bridge struct {
		MainSpan          float64 `json:"main_span_m"`
		MainRise          float64 `json:"main_rise_m"`
		Width             float64 `json:"width_m"`
		DeckThickness     float64 `json:"deck_thickness_m"`
		ArchRingThickness float64 `json:"arch_ring_thickness_m"`
		SmallArchSpanLarge float64 `json:"small_arch_span_large_m"`
		SmallArchRiseLarge float64 `json:"small_arch_rise_large_m"`
		SmallArchSpanSmall float64 `json:"small_arch_span_small_m"`
		SmallArchRiseSmall float64 `json:"small_arch_rise_small_m"`
		AbutmentHeight    float64 `json:"abutment_height_m"`
		ConstructionYearCE int     `json:"construction_year_ce"`
		SmallArchPositions []struct {
			Name string  `json:"name"`
			XMin float64 `json:"x_min"`
			XMax float64 `json:"x_max"`
			Span float64 `json:"span"`
			Rise float64 `json:"rise"`
			Size string  `json:"size"`
		} `json:"small_arch_positions_m"`
	} `json:"bridge"`
	StoneMaterial struct {
		Name                   string  `json:"name"`
		Type                   string  `json:"type"`
		ElasticModulusPa       float64 `json:"elastic_modulus_pa"`
		PoissonRatio           float64 `json:"poisson_ratio"`
		DensityKgM3            float64 `json:"density_kgm3"`
		CompressiveStrengthPa  float64 `json:"compressive_strength_pa"`
		TensileStrengthPa      float64 `json:"tensile_strength_pa"`
		ThermalExpansionCoeffPerC float64 `json:"thermal_expansion_coeff_per_c"`
		EffectiveThickness     float64 `json:"effective_thickness_m"`
	} `json:"stone_material"`
	CreepModel struct {
		PhiInfNewStone                float64 `json:"phi_inf_new_stone"`
		BetaShapeExponent             float64 `json:"beta_shape_exponent"`
		T0RefDays                     float64 `json:"t0_ref_days"`
		ReferenceLoadingAgeDays       float64 `json:"reference_loading_age_days"`
		HumidityPercent               float64 `json:"humidity_percent"`
		AlphaHMasonry                 float64 `json:"alpha_h_masonry"`
		ArchaeologicalStiffeningK     float64 `json:"archaeological_stiffening_k"`
		ArchaeologicalStiffeningRefDays float64 `json:"archaeological_stiffening_ref_days"`
		ShrinkageEpsInf               float64 `json:"shrinkage_eps_inf"`
		ShrinkageK                    float64 `json:"shrinkage_k"`
		TemperatureAmplitudeC         float64 `json:"temperature_amplitude_c"`
		TemperatureMeanC              float64 `json:"temperature_mean_c"`
	} `json:"creep_model"`
	FEMOptions struct {
		UseSubmodeling         bool    `json:"use_submodeling"`
		SubmodelRefineFactor   int     `json:"submodel_refine_factor"`
		MainArchCoarseNodes    int     `json:"main_arch_coarse_nodes"`
		DeckCoarseNodes        int     `json:"deck_coarse_nodes"`
		DefaultLiveLoadPa      float64 `json:"default_live_load_pa"`
		DefaultDeltaTC         float64 `json:"default_delta_t_c"`
	} `json:"fem_options"`
	Thresholds struct {
		StrainWarn             float64 `json:"strain_warning_micro"`
		StrainCrit             float64 `json:"strain_critical_micro"`
		SettlRateWarn          float64 `json:"settlement_rate_warning_mmpm"`
		SettlRateCrit          float64 `json:"settlement_rate_critical_mmpm"`
		SettlAbsCrit           float64 `json:"settlement_absolute_critical_mm"`
		CrackWidthWarn         float64 `json:"crack_width_warning_mm"`
		CrackWidthCrit         float64 `json:"crack_width_critical_mm"`
		CrackGrowthWarn        float64 `json:"crack_growth_warning_mmpm"`
		CrackGrowthCrit        float64 `json:"crack_growth_critical_mmpm"`
		VonMisesWarnPa         float64 `json:"von_mises_warning_pa"`
		VonMisesCritPa         float64 `json:"von_mises_critical_pa"`
	} `json:"thresholds"`
	DTUReceiver struct {
		ListenAddr      string `json:"listen_addr"`
		IngestPath      string `json:"ingest_path"`
		CooldownMs      int    `json:"reading_cooldown_ms"`
		MaxPayloadBytes int    `json:"max_payload_bytes"`
		ValidateRange   struct {
			StrainMin  float64 `json:"strain_micro_min"`
			StrainMax  float64 `json:"strain_micro_max"`
			SettlMin   float64 `json:"settlement_mm_min"`
			SettlMax   float64 `json:"settlement_mm_max"`
			TempMin    float64 `json:"temperature_c_min"`
			TempMax    float64 `json:"temperature_c_max"`
			CrackMin   float64 `json:"crack_width_mm_min"`
			CrackMax   float64 `json:"crack_width_mm_max"`
		} `json:"validate_range"`
		HistoryWindowDays int `json:"history_window_days"`
	} `json:"dtu_receiver"`
	AlarmMQTT struct {
		BrokerHostport   string  `json:"broker_hostport"`
		TopicAlerts      string  `json:"topic_alerts"`
		Qos              int     `json:"qos"`
		RetainCritical   bool    `json:"retain_critical"`
		CooldownMinutes  int     `json:"cooldown_duration_minutes"`
		MinTrendPoints   int     `json:"min_trend_points"`
		MinR2ForTrend    float64 `json:"min_r2_for_trend"`
	} `json:"alarm_mqtt"`
}

func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}
	return &cfg, nil
}

func (c *AppConfig) ToBridgeGeometry() *models.BridgeGeometry {
	return &models.BridgeGeometry{
		MainSpan:           c.Bridge.MainSpan,
		MainRise:           c.Bridge.MainRise,
		Width:              c.Bridge.Width,
		SmallArchSpanLarge: c.Bridge.SmallArchSpanLarge,
		SmallArchSpanSmall: c.Bridge.SmallArchSpanSmall,
		SmallArchRiseLarge: c.Bridge.SmallArchRiseLarge,
		SmallArchRiseSmall: c.Bridge.SmallArchRiseSmall,
	}
}

func (c *AppConfig) ToMasonryMaterial() *models.MasonryMaterial {
	return &models.MasonryMaterial{
		ElasticModulus:       c.StoneMaterial.ElasticModulusPa,
		PoissonRatio:         c.StoneMaterial.PoissonRatio,
		Density:              c.StoneMaterial.DensityKgM3,
		CompressiveStrength:  c.StoneMaterial.CompressiveStrengthPa,
		TensileStrength:      c.StoneMaterial.TensileStrengthPa,
		ThermalExpansionCoeff: c.StoneMaterial.ThermalExpansionCoeffPerC,
		CreepCoeff:           c.CreepModel.PhiInfNewStone,
	}
}

func (c *AppConfig) ToThresholdConfig() *services.ThresholdConfig {
	return &services.ThresholdConfig{
		StrainLimitMicro:     c.Thresholds.StrainCrit,
		SettlementLimitMM:    c.Thresholds.SettlRateWarn,
		SettlementAbsoluteMM: c.Thresholds.SettlAbsCrit,
		CrackWidthMM:         c.Thresholds.CrackWidthWarn,
		CrackGrowthRate:      c.Thresholds.CrackGrowthWarn,
		StressLimitPa:        c.Thresholds.VonMisesCritPa,
	}
}
