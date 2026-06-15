package services

import (
	"fmt"
	"math"

	"zhaozhou-bridge-monitor/models"
)

type VirtualBridgeService struct{}

func NewVirtualBridgeService() *VirtualBridgeService {
	return &VirtualBridgeService{}
}

var virtualBridgeMaterialPresets = map[string]*models.MasonryMaterial{
	"ancient_stone": {
		ElasticModulus:        3e9,
		PoissonRatio:         0.15,
		Density:              2400,
		CompressiveStrength:  25e6,
		TensileStrength:      2e6,
		ThermalExpansionCoeff: 5e-6,
		CreepCoeff:           2.0,
	},
	"modern_rc": {
		ElasticModulus:        30e9,
		PoissonRatio:         0.2,
		Density:              2500,
		CompressiveStrength:  35e6,
		TensileStrength:      3e6,
		ThermalExpansionCoeff: 1e-5,
		CreepCoeff:           1.5,
	},
	"steel": {
		ElasticModulus:        200e9,
		PoissonRatio:         0.3,
		Density:              7850,
		CompressiveStrength:  345e6,
		TensileStrength:      345e6,
		ThermalExpansionCoeff: 1.2e-5,
		CreepCoeff:           0.5,
	},
}

func (v *VirtualBridgeService) DesignAndTest(design models.VirtualBridgeDesign) (*models.VirtualBridgeResult, error) {
	if design.SpanM < 5 || design.SpanM > 200 {
		return nil, fmt.Errorf("SpanM must be in [5, 200], got %.2f", design.SpanM)
	}
	if design.RiseM < 1 || design.RiseM > 50 {
		return nil, fmt.Errorf("RiseM must be in [1, 50], got %.2f", design.RiseM)
	}
	if design.ArchShape != "parabolic" && design.ArchShape != "circular" && design.ArchShape != "catenary" {
		return nil, fmt.Errorf("ArchShape must be parabolic, circular, or catenary, got %s", design.ArchShape)
	}
	if design.NumSmallArches < 0 || design.NumSmallArches > 6 {
		return nil, fmt.Errorf("NumSmallArches must be in [0, 6], got %d", design.NumSmallArches)
	}
	if design.ArchRingThickM < 0.3 || design.ArchRingThickM > 5 {
		return nil, fmt.Errorf("ArchRingThickM must be in [0.3, 5], got %.2f", design.ArchRingThickM)
	}
	mat, ok := virtualBridgeMaterialPresets[design.MaterialPreset]
	if !ok {
		return nil, fmt.Errorf("MaterialPreset must be one of ancient_stone, modern_rc, steel, got %s", design.MaterialPreset)
	}
	if design.LiveLoadKPa < 0 || design.LiveLoadKPa > 500 {
		return nil, fmt.Errorf("LiveLoadKPa must be in [0, 500], got %.2f", design.LiveLoadKPa)
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
	fem.UseSubmodeling = true

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

	var recommendation string
	if passCheck {
		recommendation = fmt.Sprintf("拱券设计合理，满足安全要求，安全系数%.2f", safetyFactor)
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
		StressCheck:       stressCheck,
		DisplacementCheck: displacementCheck,
		StressUtilization: maxVonMises / mat.CompressiveStrength,
		DispSpanRatio:     maxDisplacement / design.SpanM,
		Recommendation:    recommendation,
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
