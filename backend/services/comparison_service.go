package services

import (
	"fmt"
	"math"

	"zhaozhou-bridge-monitor/models"
)

type ComparisonService struct {
	BaseFEM *FEMService
}

func NewComparisonService(base *FEMService) *ComparisonService {
	return &ComparisonService{BaseFEM: base}
}

func (cs *ComparisonService) CompareSpandrel(liveLoadPa, deltaTC float64) (*models.SpandrelComparisonResult, error) {
	openGeom := &models.BridgeGeometry{
		MainSpan:            cs.BaseFEM.Geometry.MainSpan,
		MainRise:            cs.BaseFEM.Geometry.MainRise,
		Width:               cs.BaseFEM.Geometry.Width,
		SmallArchSpanLarge:  cs.BaseFEM.Geometry.SmallArchSpanLarge,
		SmallArchSpanSmall:  cs.BaseFEM.Geometry.SmallArchSpanSmall,
		SmallArchRiseLarge:  cs.BaseFEM.Geometry.SmallArchRiseLarge,
		SmallArchRiseSmall:  cs.BaseFEM.Geometry.SmallArchRiseSmall,
	}
	openMat := copyMaterial(cs.BaseFEM.Material)
	openFEM := NewFEMService(openGeom, openMat)
	openStresses, err := openFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("open spandrel FEM failed: %w", err)
	}

	solidGeom := &models.BridgeGeometry{
		MainSpan:            cs.BaseFEM.Geometry.MainSpan,
		MainRise:            cs.BaseFEM.Geometry.MainRise,
		Width:               cs.BaseFEM.Geometry.Width,
		SmallArchSpanLarge:  0,
		SmallArchSpanSmall:  0,
		SmallArchRiseLarge:  0,
		SmallArchRiseSmall:  0,
	}
	solidMat := copyMaterial(cs.BaseFEM.Material)
	solidFEM := NewFEMService(solidGeom, solidMat)
	solidStresses, err := solidFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("solid spandrel FEM failed: %w", err)
	}

	openResult := buildComparisonCaseResult("敞肩拱", openFEM.Nodes, openFEM.Elements, openStresses, openMat, openGeom, true)
	solidResult := buildComparisonCaseResult("实肩拱", solidFEM.Nodes, solidFEM.Elements, solidStresses, solidMat, solidGeom, false)

	var stressRatio, dispRatio float64
	if solidResult.MaxVonMises > 0 {
		stressRatio = openResult.MaxVonMises / solidResult.MaxVonMises
	}
	if solidResult.MaxDisplacement > 0 {
		dispRatio = openResult.MaxDisplacement / solidResult.MaxDisplacement
	}
	massReductionPct := 0.0
	if solidResult.MassKg > 0 {
		massReductionPct = (solidResult.MassKg - openResult.MassKg) / solidResult.MassKg * 100
	}

	stressIncreasePct := 0.0
	if solidResult.MaxVonMises > 0 {
		stressIncreasePct = (openResult.MaxVonMises - solidResult.MaxVonMises) / solidResult.MaxVonMises * 100
	}

	summary := &models.ComparisonSummary{
		VonMisesReductionPct:     (solidResult.MaxVonMises - openResult.MaxVonMises) / solidResult.MaxVonMises * 100,
		DisplacementReductionPct: (solidResult.MaxDisplacement - openResult.MaxDisplacement) / solidResult.MaxDisplacement * 100,
		MassReductionPct:         massReductionPct,
		StressRatio:              stressRatio,
		DisplacementRatio:        dispRatio,
		WeightAdvantage:          fmt.Sprintf("敞肩拱减轻自重%.1f%%，同时应力仅增加%.1f%%，体现了李春设计的卓越智慧", massReductionPct, stressIncreasePct),
	}

	return &models.SpandrelComparisonResult{
		OpenSpandrel:  openResult,
		SolidSpandrel: solidResult,
		Summary:       summary,
	}, nil
}

func (cs *ComparisonService) CompareMaterials(liveLoadPa, deltaTC float64) (*models.MaterialComparisonResult, error) {
	ancientMat := copyMaterial(cs.BaseFEM.Material)
	ancientGeom := copyGeometry(cs.BaseFEM.Geometry)
	ancientFEM := NewFEMService(ancientGeom, ancientMat)
	ancientStresses, err := ancientFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("ancient stone FEM failed: %w", err)
	}

	modernMat := &models.MasonryMaterial{
		MaterialName:           "C30钢筋混凝土",
		Source:                 "《混凝土结构设计规范》GB 50010-2010",
		Grade:                  "C30 (轴心抗压强度标准值fck)",
		ElasticModulus:         30e9,
		PoissonRatio:          0.2,
		Density:               2500,
		CompressiveStrength:    20.1e6,
		CompressiveStrengthCube: 30e6,
		TensileStrength:       2.01e6,
		ThermalExpansionCoeff:  1e-5,
		CreepCoeff:            1.5,
	}
	modernGeom := copyGeometry(cs.BaseFEM.Geometry)
	modernFEM := NewFEMService(modernGeom, modernMat)
	modernStresses, err := modernFEM.RunFullAnalysis(liveLoadPa, deltaTC)
	if err != nil {
		return nil, fmt.Errorf("modern RC FEM failed: %w", err)
	}

	ancientResult := buildComparisonCaseResult("古石", ancientFEM.Nodes, ancientFEM.Elements, ancientStresses, ancientMat, ancientGeom, true)
	modernResult := buildComparisonCaseResult("现代RC", modernFEM.Nodes, modernFEM.Elements, modernStresses, modernMat, modernGeom, true)

	stiffnessRatio := modernMat.ElasticModulus / ancientMat.ElasticModulus
	strengthRatio := modernMat.CompressiveStrength / ancientMat.CompressiveStrength

	var stressReductionPct, dispReductionPct float64
	if ancientResult.MaxVonMises > 0 {
		stressReductionPct = (ancientResult.MaxVonMises - modernResult.MaxVonMises) / ancientResult.MaxVonMises * 100
	}
	if ancientResult.MaxDisplacement > 0 {
		dispReductionPct = (ancientResult.MaxDisplacement - modernResult.MaxDisplacement) / ancientResult.MaxDisplacement * 100
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
		AncientStone: ancientResult,
		ModernRC:     modernResult,
		Summary:      summary,
	}, nil
}

func buildComparisonCaseResult(label string, nodes []models.FEMNode, elements []models.FEMElement, stresses []models.FEMStressResult, mat *models.MasonryMaterial, geom *models.BridgeGeometry, hasOpen bool) *models.ComparisonCaseResult {
	maxVonMises := 0.0
	for _, s := range stresses {
		if s.VonMises > maxVonMises {
			maxVonMises = s.VonMises
		}
	}

	maxDisp := 0.0
	for _, n := range nodes {
		d := math.Sqrt(n.Dx*n.Dx + n.Dy*n.Dy)
		if d > maxDisp {
			maxDisp = d
		}
	}

	massKg := 0.0
	for _, elem := range elements {
		n1 := nodes[elem.NodeIDs[0]]
		n2 := nodes[elem.NodeIDs[1]]
		n3 := nodes[elem.NodeIDs[2]]
		area := 0.5 * math.Abs((n2.X-n1.X)*(n3.Y-n1.Y)-(n3.X-n1.X)*(n2.Y-n1.Y))
		massKg += area * elem.Thickness * elem.Material.Density
	}

	return &models.ComparisonCaseResult{
		Label:           label,
		Material:        mat,
		Geometry:        geom,
		Nodes:           nodes,
		Elements:        elements,
		Stresses:        stresses,
		MaxVonMises:     maxVonMises,
		MaxDisplacement: maxDisp,
		MassKg:          massKg,
		HasOpenSpandrel: hasOpen,
	}
}

func copyMaterial(m *models.MasonryMaterial) *models.MasonryMaterial {
	return &models.MasonryMaterial{
		MaterialName:           m.MaterialName,
		Source:                 m.Source,
		Grade:                  m.Grade,
		ElasticModulus:         m.ElasticModulus,
		PoissonRatio:          m.PoissonRatio,
		Density:               m.Density,
		CompressiveStrength:    m.CompressiveStrength,
		CompressiveStrengthCube: m.CompressiveStrengthCube,
		TensileStrength:       m.TensileStrength,
		ThermalExpansionCoeff: m.ThermalExpansionCoeff,
		CreepCoeff:            m.CreepCoeff,
	}
}

func copyGeometry(g *models.BridgeGeometry) *models.BridgeGeometry {
	return &models.BridgeGeometry{
		MainSpan:            g.MainSpan,
		MainRise:            g.MainRise,
		Width:               g.Width,
		SmallArchSpanLarge:  g.SmallArchSpanLarge,
		SmallArchSpanSmall:  g.SmallArchSpanSmall,
		SmallArchRiseLarge:  g.SmallArchRiseLarge,
		SmallArchRiseSmall:  g.SmallArchRiseSmall,
	}
}
