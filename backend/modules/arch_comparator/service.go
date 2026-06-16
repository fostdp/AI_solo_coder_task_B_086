package arch_comparator

import (
	"fmt"

	"zhaozhou-bridge-monitor/models"
	"zhaozhou-bridge-monitor/services"
)

var DefaultZhaozhouGeometry = &models.BridgeGeometry{
	MainSpan:            37.02,
	MainRise:            7.23,
	Width:               9.6,
	SmallArchSpanLarge:  3.8,
	SmallArchSpanSmall:  2.8,
	SmallArchRiseLarge:  1.5,
	SmallArchRiseSmall:  1.0,
}

var DefaultZhaozhouMaterial = &models.MasonryMaterial{
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

func (ac *ArchComparator) Compare(liveLoadPa, deltaTC float64) (*models.SpandrelComparisonResult, error) {
	openGeom := services.CopyGeometry(DefaultZhaozhouGeometry)
	openMat := services.CopyMaterial(DefaultZhaozhouMaterial)
	openFEM := services.NewFEMService(openGeom, openMat)
	openFEM.UseSubmodeling = true

	solidGeom := services.CopyGeometry(DefaultZhaozhouGeometry)
	solidGeom.SmallArchSpanLarge = 0
	solidGeom.SmallArchSpanSmall = 0
	solidGeom.SmallArchRiseLarge = 0
	solidGeom.SmallArchRiseSmall = 0
	solidMat := services.CopyMaterial(DefaultZhaozhouMaterial)
	solidFEM := services.NewFEMService(solidGeom, solidMat)
	solidFEM.UseSubmodeling = true

	openCh := openFEM.AsyncRunFullAnalysis(liveLoadPa, deltaTC)
	solidCh := solidFEM.AsyncRunFullAnalysis(liveLoadPa, deltaTC)

	openResult := <-openCh
	if openResult.Error != nil {
		return nil, fmt.Errorf("open spandrel FEM failed: %w", openResult.Error)
	}

	solidResult := <-solidCh
	if solidResult.Error != nil {
		return nil, fmt.Errorf("solid spandrel FEM failed: %w", solidResult.Error)
	}

	return ac.buildResult(openResult, solidResult, openMat, openGeom, solidMat, solidGeom), nil
}

func (ac *ArchComparator) CompareAsync(liveLoadPa, deltaTC float64) <-chan *models.SpandrelComparisonResult {
	resultCh := make(chan *models.SpandrelComparisonResult, 1)
	ac.WorkerPool.Submit(func() {
		defer close(resultCh)
		result, err := ac.Compare(liveLoadPa, deltaTC)
		if err != nil {
			resultCh <- &models.SpandrelComparisonResult{
				Summary: &models.ComparisonSummary{WeightAdvantage: fmt.Sprintf("计算失败: %v", err)},
			}
			return
		}
		resultCh <- result
	})
	return resultCh
}

func (ac *ArchComparator) buildResult(
	open, solid services.AsyncFEMResult,
	openMat *models.MasonryMaterial, openGeom *models.BridgeGeometry,
	solidMat *models.MasonryMaterial, solidGeom *models.BridgeGeometry,
) *models.SpandrelComparisonResult {
	openCase := services.BuildComparisonCaseResult("敞肩拱", open.Nodes, open.Elements, open.Stresses, openMat, openGeom, true)
	solidCase := services.BuildComparisonCaseResult("实肩拱", solid.Nodes, solid.Elements, solid.Stresses, solidMat, solidGeom, false)

	var stressRatio, dispRatio float64
	if solidCase.MaxVonMises > 0 {
		stressRatio = openCase.MaxVonMises / solidCase.MaxVonMises
	}
	if solidCase.MaxDisplacement > 0 {
		dispRatio = openCase.MaxDisplacement / solidCase.MaxDisplacement
	}
	massReductionPct := 0.0
	if solidCase.MassKg > 0 {
		massReductionPct = (solidCase.MassKg - openCase.MassKg) / solidCase.MassKg * 100
	}
	stressIncreasePct := 0.0
	if solidCase.MaxVonMises > 0 {
		stressIncreasePct = (openCase.MaxVonMises - solidCase.MaxVonMises) / solidCase.MaxVonMises * 100
	}

	summary := &models.ComparisonSummary{
		VonMisesReductionPct:     (solidCase.MaxVonMises - openCase.MaxVonMises) / solidCase.MaxVonMises * 100,
		DisplacementReductionPct: (solidCase.MaxDisplacement - openCase.MaxDisplacement) / solidCase.MaxDisplacement * 100,
		MassReductionPct:         massReductionPct,
		StressRatio:              stressRatio,
		DisplacementRatio:        dispRatio,
		WeightAdvantage:          fmt.Sprintf("敞肩拱减轻自重%.1f%%，同时应力仅增加%.1f%%，体现了李春设计的卓越智慧", massReductionPct, stressIncreasePct),
	}

	return &models.SpandrelComparisonResult{
		OpenSpandrel:  openCase,
		SolidSpandrel: solidCase,
		Summary:       summary,
	}
}
