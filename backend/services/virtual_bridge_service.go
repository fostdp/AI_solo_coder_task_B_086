package services

import (
	"fmt"
	"math"
	"strings"

	"zhaozhou-bridge-monitor/models"
)

type VirtualBridgeService struct{}

func NewVirtualBridgeService() *VirtualBridgeService {
	return &VirtualBridgeService{}
}

var virtualBridgeMaterialPresets = map[string]*models.MasonryMaterial{
	"ancient_stone": {
		MaterialName:           "青灰砂岩砌体",
		Source:                 "《砌体结构设计规范》GB 50003 + 石拱桥工程经验",
		Grade:                  "MU60石材 / M10灰缝",
		ElasticModulus:         4.5e9,
		PoissonRatio:          0.18,
		Density:               2450,
		CompressiveStrength:    12e6,
		CompressiveStrengthCube: 60e6,
		TensileStrength:       1.2e6,
		ThermalExpansionCoeff: 6e-6,
		CreepCoeff:            2.2,
	},
	"modern_rc": {
		MaterialName:           "C35钢筋混凝土",
		Source:                 "《混凝土结构设计规范》GB 50010-2010",
		Grade:                  "C35 (轴心抗压强度标准值fck=23.4MPa)",
		ElasticModulus:         31.5e9,
		PoissonRatio:          0.2,
		Density:               2500,
		CompressiveStrength:    23.4e6,
		CompressiveStrengthCube: 35e6,
		TensileStrength:       2.2e6,
		ThermalExpansionCoeff: 1e-5,
		CreepCoeff:            1.4,
	},
	"steel": {
		MaterialName:           "Q345结构钢",
		Source:                 "《钢结构设计标准》GB 50017-2017",
		Grade:                  "Q345 (屈服强度345MPa)",
		ElasticModulus:         206e9,
		PoissonRatio:          0.3,
		Density:               7850,
		CompressiveStrength:   345e6,
		CompressiveStrengthCube: 345e6,
		TensileStrength:       345e6,
		ThermalExpansionCoeff: 1.2e-5,
		CreepCoeff:            0.1,
	},
}

var virtualBridgeParamLimits = map[string]map[string][2]float64{
	"ancient_stone": {
		"span_m":      {5, 120},
		"rise_m":      {1, 40},
		"thickness_m": {0.4, 4.0},
		"live_load_kpa": {0, 50},
	},
	"modern_rc": {
		"span_m":      {10, 300},
		"rise_m":      {2, 80},
		"thickness_m": {0.3, 5.0},
		"live_load_kpa": {0, 100},
	},
	"steel": {
		"span_m":      {20, 500},
		"rise_m":      {3, 120},
		"thickness_m": {0.01, 1.5},
		"live_load_kpa": {0, 200},
	},
}

func (v *VirtualBridgeService) DesignAndTest(design models.VirtualBridgeDesign) (*models.VirtualBridgeResult, error) {
	mat, ok := virtualBridgeMaterialPresets[design.MaterialPreset]
	if !ok {
		return nil, fmt.Errorf("MaterialPreset must be one of ancient_stone, modern_rc, steel, got %s", design.MaterialPreset)
	}
	limits, _ := virtualBridgeParamLimits[design.MaterialPreset]

	if design.SpanM < limits["span_m"][0] || design.SpanM > limits["span_m"][1] {
		return nil, fmt.Errorf("SpanM for %s must be in [%.0f, %.0f]m, got %.2f",
			design.MaterialPreset, limits["span_m"][0], limits["span_m"][1], design.SpanM)
	}
	if design.RiseM < limits["rise_m"][0] || design.RiseM > limits["rise_m"][1] {
		return nil, fmt.Errorf("RiseM for %s must be in [%.0f, %.0f]m, got %.2f",
			design.MaterialPreset, limits["rise_m"][0], limits["rise_m"][1], design.RiseM)
	}
	if design.ArchShape != "parabolic" && design.ArchShape != "circular" && design.ArchShape != "catenary" {
		return nil, fmt.Errorf("ArchShape must be parabolic, circular, or catenary, got %s", design.ArchShape)
	}
	if design.NumSmallArches < 0 || design.NumSmallArches > 6 {
		return nil, fmt.Errorf("NumSmallArches must be in [0, 6], got %d", design.NumSmallArches)
	}
	if design.ArchRingThickM < limits["thickness_m"][0] || design.ArchRingThickM > limits["thickness_m"][1] {
		return nil, fmt.Errorf("ArchRingThickM for %s must be in [%.2f, %.2f]m, got %.2f",
			design.MaterialPreset, limits["thickness_m"][0], limits["thickness_m"][1], design.ArchRingThickM)
	}
	if design.LiveLoadKPa < limits["live_load_kpa"][0] || design.LiveLoadKPa > limits["live_load_kpa"][1] {
		return nil, fmt.Errorf("LiveLoadKPa for %s must be in [%.0f, %.0f]kPa, got %.2f",
			design.MaterialPreset, limits["live_load_kpa"][0], limits["live_load_kpa"][1], design.LiveLoadKPa)
	}

	geom := &models.BridgeGeometry{
		MainSpan: design.SpanM,
		MainRise: design.RiseM,
		Width:    9.6,
	}

	switch {
	case design.NumSmallArches == 0:
	case design.NumSmallArches == 1:
		geom.SmallArchSpanLarge = design.SpanM * 0.2
		geom.SmallArchRiseLarge = design.RiseM * 0.25
	case design.NumSmallArches == 2:
		geom.SmallArchSpanLarge = design.SpanM * 0.2
		geom.SmallArchRiseLarge = design.RiseM * 0.25
		geom.SmallArchSpanSmall = design.SpanM * 0.12
		geom.SmallArchRiseSmall = design.RiseM * 0.18
	default:
		geom.SmallArchSpanLarge = design.SpanM * 0.15
		geom.SmallArchRiseLarge = design.RiseM * 0.22
		geom.SmallArchSpanSmall = design.SpanM * 0.10
		geom.SmallArchRiseSmall = design.RiseM * 0.16
	}

	fem := NewFEMService(geom, mat)
	fem.UseSubmodeling = false

	liveLoadPa := design.LiveLoadKPa * 1000.0
	stresses, err := fem.RunFullAnalysis(liveLoadPa, 0)
	if err != nil {
		return nil, fmt.Errorf("FEM analysis failed: %v", err)
	}

	var maxVonMises float64
	for _, s := range stresses {
		if s.VonMises > maxVonMises {
			maxVonMises = s.VonMises
		}
	}

	var maxDisplacement float64
	for _, n := range fem.Nodes {
		disp := math.Sqrt(n.Dx*n.Dx + n.Dy*n.Dy)
		if disp > maxDisplacement {
			maxDisplacement = disp
		}
	}

	safetyFactor := mat.CompressiveStrength / maxVonMises

	var massKg float64
	for _, elem := range fem.Elements {
		n1 := fem.Nodes[elem.NodeIDs[0]]
		n2 := fem.Nodes[elem.NodeIDs[1]]
		n3 := fem.Nodes[elem.NodeIDs[2]]
		area := 0.5 * math.Abs(n1.X*(n2.Y-n3.Y)+n2.X*(n3.Y-n1.Y)+n3.X*(n1.Y-n2.Y))
		massKg += area * elem.Thickness * mat.Density
	}

	stressCheck := safetyFactor >= 1.5
	displacementCheck := maxDisplacement <= design.SpanM/600.0
	passCheck := stressCheck && displacementCheck

	riseSpanRatio := design.RiseM / design.SpanM
	thicknessSpanRatio := design.ArchRingThickM / design.SpanM

	riseRatioOK := riseSpanRatio >= 1.0/10.0 && riseSpanRatio <= 1.0/3.0
	thicknessRatioOK := true
	switch design.MaterialPreset {
	case "ancient_stone":
		thicknessRatioOK = thicknessSpanRatio >= 1.0/80.0 && thicknessSpanRatio <= 1.0/15.0
	case "modern_rc":
		thicknessRatioOK = thicknessSpanRatio >= 1.0/100.0 && thicknessSpanRatio <= 1.0/20.0
	case "steel":
		thicknessRatioOK = thicknessSpanRatio >= 1.0/400.0 && thicknessSpanRatio <= 1.0/50.0
	}
	rationalDesign := riseRatioOK && thicknessRatioOK

	var designNotes string
	notes := make([]string, 0)
	if riseSpanRatio < 1.0/10.0 {
		notes = append(notes, fmt.Sprintf("矢跨比%.3f偏小(建议1/10~1/3)，可能推力过大", riseSpanRatio))
	} else if riseSpanRatio > 1.0/3.0 {
		notes = append(notes, fmt.Sprintf("矢跨比%.3f偏大(建议1/10~1/3)，施工难度增加", riseSpanRatio))
	}
	if !thicknessRatioOK {
		notes = append(notes, fmt.Sprintf("厚跨比%.4f超出经验范围，需结合详细验算", thicknessSpanRatio))
	}
	if design.NumSmallArches > 0 && design.MaterialPreset == "steel" {
		notes = append(notes, "钢拱桥通常不设敞肩小拱，建议采用实腹式或桁架式")
	}
	if len(notes) == 0 {
		designNotes = "设计参数在工程经验合理范围内"
	} else {
		designNotes = strings.Join(notes, "；")
	}

	var recommendation string
	if passCheck && rationalDesign {
		recommendation = fmt.Sprintf("拱券设计合理，满足安全要求，安全系数%.2f，参数在经验范围内", safetyFactor)
	} else if passCheck && !rationalDesign {
		recommendation = fmt.Sprintf("力学验算通过(安全系数%.2f)，但设计参数超出工程经验范围，建议优化", safetyFactor)
	} else if safetyFactor < 1.5 && safetyFactor > 1.0 {
		recommendation = fmt.Sprintf("安全系数偏低(%.2f)，建议增加拱券厚度或减小跨度", safetyFactor)
	} else if safetyFactor <= 1.0 {
		recommendation = fmt.Sprintf("设计不安全！安全系数仅%.2f，必须修改设计参数", safetyFactor)
	}
	if !displacementCheck {
		ratio := int(math.Round(design.SpanM / maxDisplacement))
		recommendation = fmt.Sprintf("位移过大(跨径的%d分之一)，建议增大矢高或加厚拱券", ratio)
	}

	report := &models.BridgeDesignReport{
		StressCheck:        stressCheck,
		DisplacementCheck:  displacementCheck,
		StressUtilization:  maxVonMises / mat.CompressiveStrength,
		DispSpanRatio:      maxDisplacement / design.SpanM,
		RiseSpanRatio:      riseSpanRatio,
		ThicknessSpanRatio: thicknessSpanRatio,
		RationalDesign:     rationalDesign,
		DesignNotes:        designNotes,
		Recommendation:     recommendation,
	}

	result := &models.VirtualBridgeResult{
		Design:          &design,
		Material:        mat,
		Geometry:        geom,
		Nodes:           fem.Nodes,
		Elements:        fem.Elements,
		Stresses:        stresses,
		MaxVonMises:     maxVonMises,
		MaxDisplacement: maxDisplacement,
		SafetyFactor:    safetyFactor,
		MassKg:          massKg,
		PassCheck:       passCheck,
		Report:          report,
	}

	return result, nil
}
