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
	DefaultCFRPDensityKgM3          = 1800.0
	DefaultBondEfficiencyFactor     = 0.75
	DefaultEffectiveBondLengthMM    = 150
	DefaultInterfaceShearStrengthPa = 8e6
)

type ReinforcementService struct {
	BaseFEM *FEMService
}

func NewReinforcementService(base *FEMService) *ReinforcementService {
	return &ReinforcementService{BaseFEM: base}
}

func (rs *ReinforcementService) SimulateReinforcement(configs []models.ReinforcementConfig, liveLoadPa, deltaTC float64) (*models.ReinforcementSimulationResult, error) {
	beforeGeom := CopyGeometry(rs.BaseFEM.Geometry)
	beforeMat := CopyMaterial(rs.BaseFEM.Material)
	beforeFEM := NewFEMService(beforeGeom, beforeMat)
	beforeFEM.UseSubmodeling = false
	beforeStresses, err := beforeFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("before reinforcement FEM failed: %w", err)
	}
	beforeResult := BuildComparisonCaseResult("加固前", beforeFEM.Nodes, beforeFEM.Elements, beforeStresses, beforeMat, beforeGeom, true)

	afterGeom := CopyGeometry(rs.BaseFEM.Geometry)
	afterMat := CopyMaterial(rs.BaseFEM.Material)
	afterFEM := NewFEMService(afterGeom, afterMat)
	afterFEM.UseSubmodeling = false
	if err := afterFEM.GenerateMesh(); err != nil {
		return nil, fmt.Errorf("after reinforcement mesh generation failed: %w", err)
	}

	span := afterGeom.MainSpan
	rise := afterGeom.MainRise

	totalACFRP := 0.0
	totalAStone := 0.0
	totalCFRPAreaM2 := 0.0
	weightedBondEta := 0.0
	totalBondWeight := 0.0
	maxInterfaceShearPa := 0.0

	for _, cfg := range configs {
		layers := cfg.Layers
		if layers <= 0 {
			continue
		}
		width := cfg.WidthM
		if width <= 0 {
			continue
		}
		thicknessPerLayer := DefaultCFRPThicknessPerLayerM
		if cfg.ThicknessMM > 0 {
			thicknessPerLayer = cfg.ThicknessMM / 1000.0
		}
		bondEta := DefaultBondEfficiencyFactor
		if cfg.BondEfficiencyFactor > 0 && cfg.BondEfficiencyFactor <= 1.0 {
			bondEta = cfg.BondEfficiencyFactor
		}

		aCFRP := float64(layers) * thicknessPerLayer * width
		eCFRPEff := DefaultCFRPElasticModulusPa * bondEta

		for i := range afterFEM.Elements {
			elem := &afterFEM.Elements[i]
			if !ElemInZone(elem, afterFEM.Nodes, cfg.Zone, span, rise) {
				continue
			}

			aStone := elem.Thickness * 1.0
			eComp := (elem.Material.ElasticModulus*aStone + eCFRPEff*aCFRP) / (aStone + aCFRP)
			rhoComp := (elem.Material.Density*aStone + DefaultCFRPDensityKgM3*aCFRP) / (aStone + aCFRP)

			strainStone := 1e-6
			interfaceTau := eCFRPEff * strainStone * aCFRP / (elem.Material.ElasticModulus * aStone)
			if interfaceTau > maxInterfaceShearPa {
				maxInterfaceShearPa = interfaceTau
			}

			elem.Material = &models.MasonryMaterial{
				MaterialName:          elem.Material.MaterialName + "+CFRP加固",
				Source:                elem.Material.Source,
				Grade:                 elem.Material.Grade,
				ElasticModulus:        eComp,
				PoissonRatio:          elem.Material.PoissonRatio,
				Density:               rhoComp,
				CompressiveStrength:   elem.Material.CompressiveStrength,
				CompressiveStrengthCube: elem.Material.CompressiveStrengthCube,
				TensileStrength:       elem.Material.TensileStrength,
				ThermalExpansionCoeff: elem.Material.ThermalExpansionCoeff,
				CreepCoeff:            elem.Material.CreepCoeff,
			}

			totalACFRP += aCFRP
			totalAStone += aStone
			weightedBondEta += bondEta * aCFRP
			totalBondWeight += aCFRP
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
	afterResult := BuildComparisonCaseResult("加固后", afterFEM.Nodes, afterFEM.Elements, afterStresses, afterMat, afterGeom, true)

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

	var avgBondEta float64
	if totalBondWeight > 0 {
		avgBondEta = weightedBondEta / totalBondWeight
	} else {
		avgBondEta = DefaultBondEfficiencyFactor
	}

	var bondSafetyFactor float64
	if maxInterfaceShearPa > 0 {
		bondSafetyFactor = DefaultInterfaceShearStrengthPa / maxInterfaceShearPa
	} else {
		bondSafetyFactor = 999.0
	}
	bondCheckPass := bondSafetyFactor >= 2.0

	var bondNote string
	if len(configs) == 0 || totalACFRP <= 0 {
		bondNote = "未配置CFRP加固，无需粘结验算"
	} else if bondCheckPass {
		bondNote = fmt.Sprintf("界面粘结安全，粘结安全系数%.2f ≥ 2.0，平均粘结效率%.0f%%", bondSafetyFactor, avgBondEta*100)
	} else {
		bondNote = fmt.Sprintf("界面粘结风险较高，粘结安全系数%.2f < 2.0，建议增加锚固长度或采用机械锚固", bondSafetyFactor)
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
		BondSafetyFactor:      bondSafetyFactor,
		BondCheckPass:         bondCheckPass,
		AvgBondEfficiency:     avgBondEta,
		BondNote:              bondNote,
	}

	cfrpProps := &models.CFRPProperties{
		ElasticModulusPa:       DefaultCFRPElasticModulusPa,
		TensileStrengthPa:      DefaultCFRPTensileStrengthPa,
		ThicknessPerLayerMM:    DefaultCFRPThicknessPerLayerMM,
		DensityKgM3:            DefaultCFRPDensityKgM3,
		DefaultBondEfficiency:  DefaultBondEfficiencyFactor,
		EffectiveBondLengthMM:  DefaultEffectiveBondLengthMM,
		InterfaceShearStrengthPa: DefaultInterfaceShearStrengthPa,
	}

	return &models.ReinforcementSimulationResult{
		Before:         beforeResult,
		After:          afterResult,
		CFRPProperties: cfrpProps,
		Summary:        summary,
	}, nil
}
