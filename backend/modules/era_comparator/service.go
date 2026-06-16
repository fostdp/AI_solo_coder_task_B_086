package era_comparator

import (
	"fmt"

	"zhaozhou-bridge-monitor/models"
	"zhaozhou-bridge-monitor/services"
)

var AncientStone = &models.MasonryMaterial{
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

var ModernRC = &models.MasonryMaterial{
	MaterialName:            "C30钢筋混凝土",
	Source:                  "《混凝土结构设计规范》GB 50010-2010",
	Grade:                   "C30 (轴心抗压强度标准值fck)",
	ElasticModulus:          30e9,
	PoissonRatio:            0.2,
	Density:                 2500,
	CompressiveStrength:     20.1e6,
	CompressiveStrengthCube: 30e6,
	TensileStrength:         2.01e6,
	ThermalExpansionCoeff:   1e-5,
	CreepCoeff:              1.5,
}

var DefaultGeometry = &models.BridgeGeometry{
	MainSpan:            37.02,
	MainRise:            7.23,
	Width:               9.6,
	SmallArchSpanLarge:  3.8,
	SmallArchSpanSmall:  2.8,
	SmallArchRiseLarge:  1.5,
	SmallArchRiseSmall:  1.0,
}

func (ec *EraComparator) Compare(liveLoadPa, deltaTC float64) (*models.MaterialComparisonResult, error) {
	ancientMat := services.CopyMaterial(AncientStone)
	ancientGeom := services.CopyGeometry(DefaultGeometry)
	ancientFEM := services.NewFEMService(ancientGeom, ancientMat)
	ancientFEM.UseSubmodeling = true

	modernMat := services.CopyMaterial(ModernRC)
	modernGeom := services.CopyGeometry(DefaultGeometry)
	modernFEM := services.NewFEMService(modernGeom, modernMat)
	modernFEM.UseSubmodeling = true

	ancientCh := ancientFEM.AsyncRunFullAnalysis(liveLoadPa, deltaTC)
	modernCh := modernFEM.AsyncRunFullAnalysis(liveLoadPa, deltaTC)

	ancientResult := <-ancientCh
	if ancientResult.Error != nil {
		return nil, fmt.Errorf("ancient stone FEM failed: %w", ancientResult.Error)
	}

	modernResult := <-modernCh
	if modernResult.Error != nil {
		return nil, fmt.Errorf("modern RC FEM failed: %w", modernResult.Error)
	}

	return ec.buildResult(ancientResult, modernResult, ancientMat, modernMat, ancientGeom, modernGeom), nil
}

func (ec *EraComparator) CompareAsync(liveLoadPa, deltaTC float64) <-chan *models.MaterialComparisonResult {
	resultCh := make(chan *models.MaterialComparisonResult, 1)
	ec.WorkerPool.Submit(func() {
		defer close(resultCh)
		result, err := ec.Compare(liveLoadPa, deltaTC)
		if err != nil {
			resultCh <- &models.MaterialComparisonResult{
				Summary: &models.MaterialCompSummary{Verdict: fmt.Sprintf("计算失败: %v", err)},
			}
			return
		}
		resultCh <- result
	})
	return resultCh
}

func (ec *EraComparator) buildResult(
	ancient, modern services.AsyncFEMResult,
	ancientMat, modernMat *models.MasonryMaterial,
	ancientGeom, modernGeom *models.BridgeGeometry,
) *models.MaterialComparisonResult {
	ancientCase := services.BuildComparisonCaseResult("古石", ancient.Nodes, ancient.Elements, ancient.Stresses, ancientMat, ancientGeom, true)
	modernCase := services.BuildComparisonCaseResult("现代RC", modern.Nodes, modern.Elements, modern.Stresses, modernMat, modernGeom, true)

	stiffnessRatio := modernMat.ElasticModulus / ancientMat.ElasticModulus
	strengthRatio := modernMat.CompressiveStrength / ancientMat.CompressiveStrength

	var stressReductionPct, dispReductionPct float64
	if ancientCase.MaxVonMises > 0 {
		stressReductionPct = (ancientCase.MaxVonMises - modernCase.MaxVonMises) / ancientCase.MaxVonMises * 100
	}
	if ancientCase.MaxDisplacement > 0 {
		dispReductionPct = (ancientCase.MaxDisplacement - modernCase.MaxDisplacement) / ancientCase.MaxDisplacement * 100
	}

	loadCapacityRatio := strengthRatio

	summary := &models.MaterialCompSummary{
		StiffnessRatio:        stiffnessRatio,
		StrengthRatio:         strengthRatio,
		MaxStressReductionPct: stressReductionPct,
		MaxDispReductionPct:   dispReductionPct,
		LoadCapacityRatio:     loadCapacityRatio,
		Verdict:               fmt.Sprintf("现代RC拱桥应力降低%.1f%%，位移降低%.1f%%，但古桥千年屹立堪称奇迹", stressReductionPct, dispReductionPct),
	}

	return &models.MaterialComparisonResult{
		AncientStone: ancientCase,
		ModernRC:     modernCase,
		Summary:      summary,
	}
}
