package services

import (
	"math"

	"zhaozhou-bridge-monitor/models"
)

func CopyMaterial(m *models.MasonryMaterial) *models.MasonryMaterial {
	return &models.MasonryMaterial{
		MaterialName:            m.MaterialName,
		Source:                  m.Source,
		Grade:                   m.Grade,
		ElasticModulus:          m.ElasticModulus,
		PoissonRatio:            m.PoissonRatio,
		Density:                 m.Density,
		CompressiveStrength:     m.CompressiveStrength,
		CompressiveStrengthCube: m.CompressiveStrengthCube,
		TensileStrength:         m.TensileStrength,
		ThermalExpansionCoeff:   m.ThermalExpansionCoeff,
		CreepCoeff:              m.CreepCoeff,
	}
}

func CopyGeometry(g *models.BridgeGeometry) *models.BridgeGeometry {
	return &models.BridgeGeometry{
		MainSpan:           g.MainSpan,
		MainRise:           g.MainRise,
		Width:              g.Width,
		SmallArchSpanLarge: g.SmallArchSpanLarge,
		SmallArchSpanSmall: g.SmallArchSpanSmall,
		SmallArchRiseLarge: g.SmallArchRiseLarge,
		SmallArchRiseSmall: g.SmallArchRiseSmall,
	}
}

func BuildComparisonCaseResult(label string, nodes []models.FEMNode, elements []models.FEMElement, stresses []models.FEMStressResult, mat *models.MasonryMaterial, geom *models.BridgeGeometry, hasOpen bool) *models.ComparisonCaseResult {
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

func ElemInZone(elem *models.FEMElement, nodes []models.FEMNode, zone string, span, rise float64) bool {
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
