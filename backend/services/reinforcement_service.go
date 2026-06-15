package services

import (
	"fmt"
	"math"

	"zhaozhou-bridge-monitor/models"
)

const (
	DefaultCFRPElasticModulusPa     = 230e9
	DefaultCFRPTensileStrengthPa    = 3400e6
	DefaultCFRPThicknessPerLayerM   = 0.000167
	DefaultCFRPThicknessPerLayerMM  = 0.167
	DefaultCFRPDensityKgM3          = 1800
)

type ReinforcementService struct {
	BaseFEM *FEMService
}

func NewReinforcementService(base *FEMService) *ReinforcementService {
	return &ReinforcementService{BaseFEM: base}
}

func (rs *ReinforcementService) SimulateReinforcement(configs []models.ReinforcementConfig, liveLoadPa, deltaTC float64) (*models.ReinforcementSimulationResult, error) {
	beforeGeom := copyGeometry(rs.BaseFEM.Geometry)
	beforeMat := copyMaterial(rs.BaseFEM.Material)
	beforeFEM := NewFEMService(beforeGeom, beforeMat)
	beforeStresses, err := beforeFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("before reinforcement FEM failed: %w", err)
	}
	beforeResult := buildComparisonCaseResult("加固前", beforeFEM.Nodes, beforeFEM.Elements, beforeStresses, beforeMat, beforeGeom, true)

	afterGeom := copyGeometry(rs.BaseFEM.Geometry)
	afterMat := copyMaterial(rs.BaseFEM.Material)
	afterFEM := NewFEMService(afterGeom, afterMat)
	if err := afterFEM.GenerateMesh(); err != nil {
		return nil, fmt.Errorf("after reinforcement mesh generation failed: %w", err)
	}

	span := afterGeom.MainSpan
	rise := afterGeom.MainRise

	totalACFRP := 0.0
	totalAStone := 0.0
	totalCFRPAreaM2 := 0.0

	for _, cfg := range configs {
		layers := cfg.Layers
		width := cfg.WidthM
		thicknessPerLayer := DefaultCFRPThicknessPerLayerM
		if cfg.ThicknessMM > 0 {
			thicknessPerLayer = cfg.ThicknessMM / 1000.0
		}
		aCFRP := float64(layers) * thicknessPerLayer * width

		for i := range afterFEM.Elements {
			elem := &afterFEM.Elements[i]
			if !elemInZone(elem, afterFEM.Nodes, cfg.Zone, span, rise) {
				continue
			}

			aStone := elem.Thickness * 1.0
			eComp := (elem.Material.ElasticModulus*aStone + DefaultCFRPElasticModulusPa*aCFRP) / (aStone + aCFRP)
			rhoComp := (elem.Material.Density*aStone + DefaultCFRPDensityKgM3*aCFRP) / (aStone + aCFRP)

			elem.Material = &models.MasonryMaterial{
				ElasticModulus:         eComp,
				PoissonRatio:          elem.Material.PoissonRatio,
				Density:               rhoComp,
				CompressiveStrength:   elem.Material.CompressiveStrength,
				TensileStrength:       elem.Material.TensileStrength,
				ThermalExpansionCoeff: elem.Material.ThermalExpansionCoeff,
				CreepCoeff:            elem.Material.CreepCoeff,
			}

			totalACFRP += aCFRP
			totalAStone += aStone
			totalCFRPAreaM2 += float64(layers) * width * 1.0
		}
	}

	if err := afterFEM.BuildStiffnessMatrix(); err != nil {
		return nil, fmt.Errorf("after reinforcement build stiffness failed: %w", err)
	}
	if err := afterFEM.ApplyGravityLoad(); err != nil {
		return nil, fmt.Errorf("after reinforcement apply gravity failed: %w", err)
	}
	if liveLoadPa > 0 {
		if err := afterFEM.ApplyLiveLoad(liveLoadPa); err != nil {
			return nil, fmt.Errorf("after reinforcement apply live load failed: %w", err)
		}
	}
	if math.Abs(deltaTC) > 1e-10 {
		if err := afterFEM.ApplyThermalLoad(deltaTC); err != nil {
			return nil, fmt.Errorf("after reinforcement apply thermal failed: %w", err)
		}
	}
	if err := afterFEM.Solve(); err != nil {
		return nil, fmt.Errorf("after reinforcement solve failed: %w", err)
	}
	afterStresses := afterFEM.ComputeElementStresses()
	afterResult := buildComparisonCaseResult("加固后", afterFEM.Nodes, afterFEM.Elements, afterStresses, afterMat, afterGeom, true)

	var stressReductionPct, dispReductionPct, stiffnessIncreasePct float64
	if beforeResult.MaxVonMises > 0 {
		stressReductionPct = (beforeResult.MaxVonMises - afterResult.MaxVonMises) / beforeResult.MaxVonMises * 100
	}
	if beforeResult.MaxDisplacement > 0 {
		dispReductionPct = (beforeResult.MaxDisplacement - afterResult.MaxDisplacement) / beforeResult.MaxDisplacement * 100
	}
	if beforeResult.MaxDisplacement > 0 {
		stiffnessIncreasePct = (beforeResult.MaxDisplacement - afterResult.MaxDisplacement) / afterResult.MaxDisplacement * 100
	}

	var safetyFactorBefore, safetyFactorAfter float64
	if beforeResult.MaxVonMises > 0 {
		safetyFactorBefore = beforeMat.CompressiveStrength / beforeResult.MaxVonMises
	}
	if afterResult.MaxVonMises > 0 {
		safetyFactorAfter = beforeMat.CompressiveStrength / afterResult.MaxVonMises
	}

	var cfrpVolumeFraction float64
	if totalAStone+totalACFRP > 0 {
		cfrpVolumeFraction = totalACFRP / (totalAStone + totalACFRP)
	}

	costPerM2 := 800.0
	totalCostWanYuan := totalCFRPAreaM2 * costPerM2 / 10000.0
	costEstimate := fmt.Sprintf("约需CFRP布%.2f m²，估算材料费用%.2f万元", totalCFRPAreaM2, totalCostWanYuan)

	summary := &models.ReinforcementSummary{
		MaxStressReductionPct: stressReductionPct,
		MaxDispReductionPct:   dispReductionPct,
		StiffnessIncreasePct:  stiffnessIncreasePct,
		CFRPVolumefraction:    cfrpVolumeFraction,
		CostEstimate:          costEstimate,
		SafetyFactorBefore:    safetyFactorBefore,
		SafetyFactorAfter:     safetyFactorAfter,
	}

	cfrpProps := &models.CFRPProperties{
		ElasticModulusPa:    DefaultCFRPElasticModulusPa,
		TensileStrengthPa:   DefaultCFRPTensileStrengthPa,
		ThicknessPerLayerMM: DefaultCFRPThicknessPerLayerMM,
		DensityKgM3:         DefaultCFRPDensityKgM3,
	}

	return &models.ReinforcementSimulationResult{
		Before:         beforeResult,
		After:          afterResult,
		CFRPProperties: cfrpProps,
		Summary:        summary,
	}, nil
}

func elemInZone(elem *models.FEMElement, nodes []models.FEMNode, zone string, span, rise float64) bool {
	var xCenter, yCenter float64
	for _, nid := range elem.NodeIDs {
		xCenter += nodes[nid].X
		yCenter += nodes[nid].Y
	}
	xCenter /= 3.0
	yCenter /= 3.0

	switch zone {
	case "main_arch":
		return xCenter >= 0 && xCenter <= span && yCenter > rise*0.3
	case "left_spandrel":
		return xCenter >= 0 && xCenter <= span*0.35
	case "right_spandrel":
		return xCenter >= span*0.65 && xCenter <= span
	case "full":
		return true
	default:
		return false
	}
}
