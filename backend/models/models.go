package models

import "time"

type SensorReading struct {
	Time          time.Time
	SensorID      string
	StrainMicro   float64
	SettlementMM  float64
	Temperature   float64
	CrackWidthMM  float64
}

type FEMStressResult struct {
	Time      time.Time
	ElementID int
	SigmaX    float64
	SigmaY    float64
	TauXY     float64
	VonMises  float64
	NodeIDs   []int
}

type DeformationPrediction struct {
	Time        time.Time
	NodeID      int
	PredictedDX float64
	PredictedDY float64
	TargetYear  int
}

type Alert struct {
	Time      time.Time
	AlertType string
	Severity  string
	Message   string
	SensorID  string
	Value     float64
	Threshold float64
}

type SensorInfo struct {
	SensorID      string
	SensorType    string
	LocationX     float64
	LocationY     float64
	LocationZ     float64
	InstalledDate time.Time
	Status        string
}

type FEMNode struct {
	ID int
	X  float64
	Y  float64
	Dx float64
	Dy float64
}

type FEMElement struct {
	ID        int
	NodeIDs   [3]int
	Thickness float64
	Material  *MasonryMaterial
}

type MasonryMaterial struct {
	ElasticModulus       float64
	PoissonRatio        float64
	Density              float64
	CompressiveStrength  float64
	TensileStrength      float64
	ThermalExpansionCoeff float64
	CreepCoeff           float64
}

type BridgeGeometry struct {
	MainSpan            float64
	MainRise            float64
	Width               float64
	SmallArchSpanLarge  float64
	SmallArchSpanSmall  float64
	SmallArchRiseLarge  float64
	SmallArchRiseSmall  float64
}

type HourlyAggregate struct {
	TimeBucket     time.Time
	AvgStrain      float64
	AvgSettlement  float64
	AvgTemperature float64
	AvgCrackWidth  float64
}

type ComparisonCaseResult struct {
	Label            string             `json:"label"`
	Material         *MasonryMaterial   `json:"material"`
	Geometry         *BridgeGeometry    `json:"geometry"`
	Nodes            []FEMNode          `json:"nodes"`
	Elements         []FEMElement       `json:"elements"`
	Stresses         []FEMStressResult  `json:"stresses"`
	MaxVonMises      float64            `json:"max_von_mises"`
	MaxDisplacement  float64            `json:"max_displacement"`
	MassKg           float64            `json:"mass_kg"`
	HasOpenSpandrel  bool               `json:"has_open_spandrel"`
}

type SpandrelComparisonResult struct {
	OpenSpandrel   *ComparisonCaseResult `json:"open_spandrel"`
	SolidSpandrel  *ComparisonCaseResult `json:"solid_spandrel"`
	Summary        *ComparisonSummary    `json:"summary"`
}

type ComparisonSummary struct {
	VonMisesReductionPct    float64 `json:"von_mises_reduction_pct"`
	DisplacementReductionPct float64 `json:"displacement_reduction_pct"`
	MassReductionPct        float64 `json:"mass_reduction_pct"`
	StressRatio             float64 `json:"stress_ratio"`
	DisplacementRatio       float64 `json:"displacement_ratio"`
	WeightAdvantage         string  `json:"weight_advantage"`
}

type MaterialComparisonResult struct {
	AncientStone  *ComparisonCaseResult `json:"ancient_stone"`
	ModernRC      *ComparisonCaseResult `json:"modern_rc"`
	Summary       *MaterialCompSummary  `json:"summary"`
}

type MaterialCompSummary struct {
	StiffnessRatio        float64 `json:"stiffness_ratio"`
	StrengthRatio         float64 `json:"strength_ratio"`
	MaxStressReductionPct float64 `json:"max_stress_reduction_pct"`
	MaxDispReductionPct   float64 `json:"max_disp_reduction_pct"`
	LoadCapacityRatio     float64 `json:"load_capacity_ratio"`
	Verdict               string  `json:"verdict"`
}

type ReinforcementConfig struct {
	Zone           string  `json:"zone"`
	Layers         int     `json:"layers"`
	ThicknessMM    float64 `json:"thickness_mm"`
	WidthM         float64 `json:"width_m"`
}

type ReinforcementSimulationResult struct {
	Before          *ComparisonCaseResult `json:"before"`
	After           *ComparisonCaseResult `json:"after"`
	CFRPProperties  *CFRPProperties       `json:"cfrp_properties"`
	Summary         *ReinforcementSummary `json:"summary"`
}

type CFRPProperties struct {
	ElasticModulusPa    float64 `json:"elastic_modulus_pa"`
	TensileStrengthPa   float64 `json:"tensile_strength_pa"`
	ThicknessPerLayerMM float64 `json:"thickness_per_layer_mm"`
	DensityKgM3         float64 `json:"density_kg_m3"`
}

type ReinforcementSummary struct {
	MaxStressReductionPct  float64 `json:"max_stress_reduction_pct"`
	MaxDispReductionPct    float64 `json:"max_disp_reduction_pct"`
	StiffnessIncreasePct   float64 `json:"stiffness_increase_pct"`
	CFRPVolumefraction     float64 `json:"cfrp_volume_fraction"`
	CostEstimate          string  `json:"cost_estimate"`
	SafetyFactorBefore    float64 `json:"safety_factor_before"`
	SafetyFactorAfter     float64 `json:"safety_factor_after"`
}

type VirtualBridgeDesign struct {
	SpanM          float64 `json:"span_m"`
	RiseM          float64 `json:"rise_m"`
	ArchShape      string  `json:"arch_shape"`
	NumSmallArches int     `json:"num_small_arches"`
	ArchRingThickM float64 `json:"arch_ring_thickness_m"`
	MaterialPreset string  `json:"material_preset"`
	LiveLoadKPa    float64 `json:"live_load_kpa"`
}

type VirtualBridgeResult struct {
	Design          *VirtualBridgeDesign    `json:"design"`
	Material        *MasonryMaterial        `json:"material"`
	Geometry        *BridgeGeometry         `json:"geometry"`
	Nodes           []FEMNode               `json:"nodes"`
	Elements        []FEMElement            `json:"elements"`
	Stresses        []FEMStressResult       `json:"stresses"`
	MaxVonMises     float64                 `json:"max_von_mises"`
	MaxDisplacement float64                 `json:"max_displacement"`
	SafetyFactor    float64                 `json:"safety_factor"`
	MassKg          float64                 `json:"mass_kg"`
	PassCheck       bool                    `json:"pass_check"`
	Report          *BridgeDesignReport     `json:"report"`
}

type BridgeDesignReport struct {
	StressCheck        bool    `json:"stress_check"`
	DisplacementCheck  bool    `json:"displacement_check"`
	StressUtilization  float64 `json:"stress_utilization"`
	DispSpanRatio      float64 `json:"disp_span_ratio"`
	Recommendation     string  `json:"recommendation"`
}
