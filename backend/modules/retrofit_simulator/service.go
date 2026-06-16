package retrofit_simulator

import (
	"fmt"
	"math"

	"zhaozhou-bridge-monitor/models"
	"zhaozhou-bridge-monitor/services"
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

var DefaultBridgeGeometry = &models.BridgeGeometry{
	MainSpan:            37.02,
	MainRise:            7.23,
	Width:               9.6,
	SmallArchSpanLarge:  3.8,
	SmallArchSpanSmall:  2.8,
	SmallArchRiseLarge:  1.5,
	SmallArchRiseSmall:  1.0,
}

var DefaultBridgeMaterial = &models.MasonryMaterial{
	MaterialName:            "赵县青灰砂岩砌体",
	Source:                  "《赵州桥结构分析与保护研究》+《砌体结构设计规范》GB 50003",
	Grade:                   "MU60石材 / M10灰缝 (现状劣化评估)",
	ElasticModulus:          4.5e9,
	PoissonRatio:            0.18,
	Density:                 2450,
	CompressiveStrength:     12e6,
	TensileStrength:         1.2e6,
	CompressiveStrengthCube: 60e6,
	ThermalExpansionCoeff:   6e-6,
	CreepCoeff:              2.2,
}

func (rs *RetrofitSimulator) Simulate(configs []models.ReinforcementConfig, liveLoadPa, deltaTC float64) (*models.ReinforcementSimulationResult, error) {
	beforeGeom := services.CopyGeometry(DefaultBridgeGeometry)
	beforeMat := services.CopyMaterial(DefaultBridgeMaterial)
	beforeFEM := services.NewFEMService(beforeGeom, beforeMat)
	beforeFEM.UseSubmodeling = false

	beforeResult, err := rs.runFullAnalysisSync(beforeFEM, liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("before reinforcement FEM failed: %w", err)
	}
	beforeCase := services.BuildComparisonCaseResult("加固前", beforeResult.Nodes, beforeResult.Elements, beforeResult.Stresses, beforeMat, beforeGeom, true)

	afterGeom := services.CopyGeometry(DefaultBridgeGeometry)
	afterMat := services.CopyMaterial(DefaultBridgeMaterial)
	afterFEM := services.NewFEMService(afterGeom, afterMat)
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
			if !services.ElemInZone(elem, afterFEM.Nodes, cfg.Zone, span, rise) {
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
				MaterialName:            elem.Material.MaterialName + "+CFRP加固",
				Source:                  elem.Material.Source,
				Grade:                   elem.Material.Grade,
				ElasticModulus:          eComp,
				PoissonRatio:            elem.Material.PoissonRatio,
				Density:                 rhoComp,
				CompressiveStrength:     elem.Material.CompressiveStrength,
				CompressiveStrengthCube: elem.Material.CompressiveStrengthCube,
				TensileStrength:         elem.Material.TensileStrength,
				ThermalExpansionCoeff:   elem.Material.ThermalExpansionCoeff,
				CreepCoeff:              elem.Material.CreepCoeff,
			}

			totalACFRP += aCFRP
			totalAStone += aStone
			weightedBondEta += bondEta * aCFRP
			totalBondWeight += aCFRP
			totalCFRPAreaM2 += float64(layers) * width * 1.0
		}
	}

	afterResult, err := rs.runFullAnalysisManual(afterFEM, liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("after reinforcement FEM failed: %w", err)
	}
	afterCase := services.BuildComparisonCaseResult("加固后", afterResult.Nodes, afterResult.Elements, afterResult.Stresses, afterMat, afterGeom, true)

	var stressReductionPct, dispReductionPct, stiffnessIncreasePct float64
	if beforeCase.MaxVonMises > 0 {
		stressReductionPct = (beforeCase.MaxVonMises - afterCase.MaxVonMises) / beforeCase.MaxVonMises * 100
	}
	if beforeCase.MaxDisplacement > 0 {
		dispReductionPct = (beforeCase.MaxDisplacement - afterCase.MaxDisplacement) / beforeCase.MaxDisplacement * 100
	}
	if afterCase.MaxDisplacement > 0 {
		stiffnessIncreasePct = (beforeCase.MaxDisplacement - afterCase.MaxDisplacement) / afterCase.MaxDisplacement * 100
	}

	var safetyFactorBefore, safetyFactorAfter float64
	if beforeCase.MaxVonMises > 0 {
		safetyFactorBefore = beforeMat.CompressiveStrength / beforeCase.MaxVonMises
	}
	if afterCase.MaxVonMises > 0 {
		safetyFactorAfter = beforeMat.CompressiveStrength / afterCase.MaxVonMises
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
		ElasticModulusPa:        DefaultCFRPElasticModulusPa,
		TensileStrengthPa:       DefaultCFRPTensileStrengthPa,
		ThicknessPerLayerMM:     DefaultCFRPThicknessPerLayerMM,
		DensityKgM3:             DefaultCFRPDensityKgM3,
		DefaultBondEfficiency:   DefaultBondEfficiencyFactor,
		EffectiveBondLengthMM:   DefaultEffectiveBondLengthMM,
		InterfaceShearStrengthPa: DefaultInterfaceShearStrengthPa,
	}

	return &models.ReinforcementSimulationResult{
		Before:         beforeCase,
		After:          afterCase,
		CFRPProperties: cfrpProps,
		Summary:        summary,
	}, nil
}

func (rs *RetrofitSimulator) SimulateAsync(configs []models.ReinforcementConfig, liveLoadPa, deltaTC float64) <-chan *models.ReinforcementSimulationResult {
	resultCh := make(chan *models.ReinforcementSimulationResult, 1)
	rs.WorkerPool.Submit(func() {
		defer close(resultCh)
		result, err := rs.Simulate(configs, liveLoadPa, deltaTC)
		if err != nil {
			resultCh <- &models.ReinforcementSimulationResult{
				Summary: &models.ReinforcementSummary{BondNote: fmt.Sprintf("计算失败: %v", err)},
			}
			return
		}
		resultCh <- result
	})
	return resultCh
}

func (rs *RetrofitSimulator) runFullAnalysisSync(fem *services.FEMService, liveLoadPa, deltaTC float64) (services.AsyncFEMResult, error) {
	resultCh := fem.AsyncRunFullAnalysis(liveLoadPa, deltaTC)
	result := <-resultCh
	return result, result.Error
}

func (rs *RetrofitSimulator) runFullAnalysisManual(fem *services.FEMService, liveLoadPa, deltaTC float64) (services.AsyncFEMResult, error) {
	if err := fem.BuildStiffnessMatrix(); err != nil {
		return services.AsyncFEMResult{}, fmt.Errorf("build stiffness failed: %w", err)
	}
	if err := fem.ApplyGravityLoad(); err != nil {
		return services.AsyncFEMResult{}, fmt.Errorf("apply gravity failed: %w", err)
	}
	if liveLoadPa > 0 {
		if err := fem.ApplyLiveLoad(liveLoadPa); err != nil {
			return services.AsyncFEMResult{}, fmt.Errorf("apply live load failed: %w", err)
		}
	}
	if math.Abs(deltaTC) > 1e-10 {
		if err := fem.ApplyThermalLoad(deltaTC); err != nil {
			return services.AsyncFEMResult{}, fmt.Errorf("apply thermal failed: %w", err)
		}
	}
	if err := fem.Solve(); err != nil {
		return services.AsyncFEMResult{}, fmt.Errorf("solve failed: %w", err)
	}
	stresses := fem.ComputeElementStresses()

	nodesCopy := make([]models.FEMNode, len(fem.Nodes))
	elemsCopy := make([]models.FEMElement, len(fem.Elements))
	copy(nodesCopy, fem.Nodes)
	for i := range fem.Elements {
		elemsCopy[i] = fem.Elements[i]
	}

	return services.AsyncFEMResult{
		Stresses: stresses,
		Nodes:    nodesCopy,
		Elements: elemsCopy,
		Error:    nil,
	}, nil
}
