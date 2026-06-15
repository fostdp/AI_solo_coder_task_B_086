package services

import (
	"math"
	"testing"

	"zhaozhou-bridge-monitor/models"
)

func newTestBaseFEM() *FEMService {
	geom := &models.BridgeGeometry{
		MainSpan:           37.02,
		MainRise:           7.23,
		Width:              9.6,
		SmallArchSpanLarge: 3.8,
		SmallArchSpanSmall: 2.8,
		SmallArchRiseLarge: 1.5,
		SmallArchRiseSmall: 1.0,
	}
	mat := &models.MasonryMaterial{
		MaterialName:          "赵县青灰砂岩砌体",
		Source:                "《赵州桥结构分析与保护研究》",
		Grade:                 "MU60石材 / M10灰缝",
		ElasticModulus:        4.5e9,
		PoissonRatio:          0.18,
		Density:               2450,
		CompressiveStrength:   12e6,
		CompressiveStrengthCube: 60e6,
		TensileStrength:       1.2e6,
		ThermalExpansionCoeff: 6e-6,
		CreepCoeff:            2.2,
	}
	return NewFEMService(geom, mat)
}

func TestCompareSpandrel_NominalCase(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareSpandrel(10e3, 0)
	if err != nil {
		t.Fatalf("CompareSpandrel failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.OpenSpandrel == nil || result.SolidSpandrel == nil || result.Summary == nil {
		t.Fatal("nested result fields are nil")
	}

	if result.OpenSpandrel.Label != "敞肩拱" {
		t.Errorf("expected open label '敞肩拱', got '%s'", result.OpenSpandrel.Label)
	}
	if result.SolidSpandrel.Label != "实肩拱" {
		t.Errorf("expected solid label '实肩拱', got '%s'", result.SolidSpandrel.Label)
	}
	if !result.OpenSpandrel.HasOpenSpandrel {
		t.Error("open spandrel should have HasOpenSpandrel=true")
	}
	if result.SolidSpandrel.HasOpenSpandrel {
		t.Error("solid spandrel should have HasOpenSpandrel=false")
	}
}

func TestCompareSpandrel_MassReduction(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareSpandrel(0, 0)
	if err != nil {
		t.Fatalf("CompareSpandrel failed: %v", err)
	}

	if result.OpenSpandrel.MassKg >= result.SolidSpandrel.MassKg {
		t.Errorf("敞肩拱质量应小于实肩拱，但 open=%.2f, solid=%.2f",
			result.OpenSpandrel.MassKg, result.SolidSpandrel.MassKg)
	}

	if result.Summary.MassReductionPct <= 0 {
		t.Errorf("质量减少率应大于0，实际为%.2f%%", result.Summary.MassReductionPct)
	}
	if result.Summary.MassReductionPct >= 50 {
		t.Errorf("质量减少率异常偏高：%.2f%%", result.Summary.MassReductionPct)
	}
}

func TestCompareSpandrel_StressCharacteristics(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareSpandrel(10e3, 0)
	if err != nil {
		t.Fatalf("CompareSpandrel failed: %v", err)
	}

	openStress := result.OpenSpandrel.MaxVonMises
	solidStress := result.SolidSpandrel.MaxVonMises
	if openStress <= 0 || solidStress <= 0 {
		t.Fatalf("应力应为正值，open=%.2f, solid=%.2f", openStress, solidStress)
	}

	if result.Summary.StressRatio <= 0 {
		t.Errorf("应力比应>0，实际为%.4f", result.Summary.StressRatio)
	}

	if result.Summary.WeightAdvantage == "" {
		t.Error("WeightAdvantage 不应为空")
	}
}

func TestCompareSpandrel_Displacement(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareSpandrel(10e3, 0)
	if err != nil {
		t.Fatalf("CompareSpandrel failed: %v", err)
	}

	if result.OpenSpandrel.MaxDisplacement <= 0 {
		t.Errorf("敞肩拱位移应为正，实际为%.6f", result.OpenSpandrel.MaxDisplacement)
	}
	if result.SolidSpandrel.MaxDisplacement <= 0 {
		t.Errorf("实肩拱位移应为正，实际为%.6f", result.SolidSpandrel.MaxDisplacement)
	}
}

func TestCompareSpandrel_ZeroLiveLoad(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareSpandrel(0, 0)
	if err != nil {
		t.Fatalf("零荷载对比失败: %v", err)
	}

	if result.OpenSpandrel.MaxVonMises <= 0 {
		t.Error("零活载下仍应有自重应力")
	}
	if result.Summary.MassReductionPct <= 0 {
		t.Error("零活载下质量减少率仍应有效")
	}
}

func TestCompareSpandrel_WithThermalLoad(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	deltaT := 20.0
	result, err := cs.CompareSpandrel(10e3, deltaT)
	if err != nil {
		t.Fatalf("温度荷载对比失败: %v", err)
	}

	noThermalResult, err := cs.CompareSpandrel(10e3, 0)
	if err != nil {
		t.Fatalf("无温度荷载对比失败: %v", err)
	}

	if math.Abs(result.OpenSpandrel.MaxVonMises-noThermalResult.OpenSpandrel.MaxVonMises) < 1e-6 {
		t.Log("警告：温度荷载对应力无明显影响（可能取决于网格和约束）")
	}
}

func TestCompareMaterials_NominalCase(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.AncientStone == nil || result.ModernRC == nil || result.Summary == nil {
		t.Fatal("nested result fields are nil")
	}

	if result.AncientStone.Label != "古石" {
		t.Errorf("expected ancient label '古石', got '%s'", result.AncientStone.Label)
	}
	if result.ModernRC.Label != "现代RC" {
		t.Errorf("expected modern label '现代RC', got '%s'", result.ModernRC.Label)
	}
}

func TestCompareMaterials_StiffnessRatio(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	expectedRatio := 30e9 / 4.5e9
	if math.Abs(result.Summary.StiffnessRatio-expectedRatio) > 1e-2 {
		t.Errorf("刚度比应为%.2f，实际为%.2f", expectedRatio, result.Summary.StiffnessRatio)
	}
}

func TestCompareMaterials_StrengthRatio(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	expectedRatio := 20.1e6 / 12e6
	if math.Abs(result.Summary.StrengthRatio-expectedRatio) > 1e-2 {
		t.Errorf("强度比应为%.2f，实际为%.2f", expectedRatio, result.Summary.StrengthRatio)
	}
}

func TestCompareMaterials_DisplacementReduction(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	ancientDisp := result.AncientStone.MaxDisplacement
	modernDisp := result.ModernRC.MaxDisplacement
	if ancientDisp <= modernDisp {
		t.Errorf("古石位移应大于现代RC，ancient=%.6f, modern=%.6f", ancientDisp, modernDisp)
	}

	if result.Summary.MaxDispReductionPct <= 0 {
		t.Errorf("位移减少率应>0，实际为%.2f%%", result.Summary.MaxDispReductionPct)
	}
}

func TestCompareMaterials_StressReduction(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	if result.Summary.MaxStressReductionPct <= 0 {
		t.Errorf("应力减少率应>0，实际为%.2f%%", result.Summary.MaxStressReductionPct)
	}
}

func TestCompareMaterials_LoadCapacity(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	if result.Summary.LoadCapacityRatio <= 1 {
		t.Errorf("承载力比应>1，实际为%.2f", result.Summary.LoadCapacityRatio)
	}

	if result.Summary.Verdict == "" {
		t.Error("Verdict不应为空")
	}
}

func TestCompareMaterials_MaterialProperties(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(10e3, 0)
	if err != nil {
		t.Fatalf("CompareMaterials failed: %v", err)
	}

	if result.AncientStone.Material.ElasticModulus != 4.5e9 {
		t.Errorf("古石E应为4.5e9，实际为%.0f", result.AncientStone.Material.ElasticModulus)
	}
	if result.ModernRC.Material.ElasticModulus != 30e9 {
		t.Errorf("现代RC E应为30e9，实际为%.0f", result.ModernRC.Material.ElasticModulus)
	}
	if result.ModernRC.Material.CompressiveStrength != 20.1e6 {
		t.Errorf("现代RC抗压强度应为20.1e6，实际为%.1f", result.ModernRC.Material.CompressiveStrength)
	}
	if result.AncientStone.Material.MaterialName == "" {
		t.Error("古石材料名称不应为空")
	}
	if result.ModernRC.Material.Source == "" {
		t.Error("现代RC材料来源不应为空")
	}
}

func TestCompareMaterials_ZeroLiveLoad(t *testing.T) {
	baseFEM := newTestBaseFEM()
	cs := NewComparisonService(baseFEM)

	result, err := cs.CompareMaterials(0, 0)
	if err != nil {
		t.Fatalf("零活载材料对比失败: %v", err)
	}

	if result.AncientStone.MaxVonMises <= 0 {
		t.Error("零活载下仍应有自重应力")
	}
	if result.Summary.StiffnessRatio != 10.0 {
		t.Errorf("刚度比应恒为10，与荷载无关，实际为%.2f", result.Summary.StiffnessRatio)
	}
}

func TestBuildComparisonCaseResult_EmptyElements(t *testing.T) {
	nodes := []models.FEMNode{}
	elements := []models.FEMElement{}
	stresses := []models.FEMStressResult{}
	mat := &models.MasonryMaterial{Density: 2400}
	geom := &models.BridgeGeometry{MainSpan: 10}

	result := buildComparisonCaseResult("test", nodes, elements, stresses, mat, geom, true)

	if result.MaxVonMises != 0 {
		t.Errorf("空单元应力应为0，实际为%.2f", result.MaxVonMises)
	}
	if result.MaxDisplacement != 0 {
		t.Errorf("空节点位移应为0，实际为%.6f", result.MaxDisplacement)
	}
	if result.MassKg != 0 {
		t.Errorf("空单元质量应为0，实际为%.2f", result.MassKg)
	}
}

func TestCopyMaterial(t *testing.T) {
	original := &models.MasonryMaterial{
		MaterialName:           "测试材料",
		Source:                 "测试来源",
		Grade:                  "测试等级",
		ElasticModulus:         4.5e9,
		PoissonRatio:          0.18,
		Density:               2450,
		CompressiveStrength:    12e6,
		CompressiveStrengthCube: 60e6,
		TensileStrength:       1.2e6,
		ThermalExpansionCoeff: 6e-6,
		CreepCoeff:            2.2,
	}

	copied := copyMaterial(original)

	if copied == original {
		t.Error("copyMaterial应返回新指针，而非原指针")
	}
	if copied.ElasticModulus != original.ElasticModulus {
		t.Errorf("E拷贝错误: %.0f vs %.0f", copied.ElasticModulus, original.ElasticModulus)
	}
	if copied.PoissonRatio != original.PoissonRatio {
		t.Errorf("ν拷贝错误: %.2f vs %.2f", copied.PoissonRatio, original.PoissonRatio)
	}
	if copied.CompressiveStrength != original.CompressiveStrength {
		t.Errorf("fc拷贝错误: %.0f vs %.0f", copied.CompressiveStrength, original.CompressiveStrength)
	}
	if copied.MaterialName != original.MaterialName {
		t.Errorf("MaterialName拷贝错误: %s vs %s", copied.MaterialName, original.MaterialName)
	}
	if copied.Source != original.Source {
		t.Errorf("Source拷贝错误: %s vs %s", copied.Source, original.Source)
	}
	if copied.CompressiveStrengthCube != original.CompressiveStrengthCube {
		t.Errorf("CubeStrength拷贝错误: %.0f vs %.0f", copied.CompressiveStrengthCube, original.CompressiveStrengthCube)
	}

	copied.ElasticModulus = 999
	if original.ElasticModulus == 999 {
		t.Error("修改拷贝不应影响原始对象")
	}
}

func TestCopyGeometry(t *testing.T) {
	original := &models.BridgeGeometry{
		MainSpan:           37.02,
		MainRise:           7.23,
		Width:              9.6,
		SmallArchSpanLarge: 3.8,
		SmallArchSpanSmall: 2.8,
		SmallArchRiseLarge: 1.5,
		SmallArchRiseSmall: 1.0,
	}

	copied := copyGeometry(original)

	if copied == original {
		t.Error("copyGeometry应返回新指针")
	}
	if copied.MainSpan != original.MainSpan {
		t.Errorf("跨径拷贝错误")
	}
	if copied.MainRise != original.MainRise {
		t.Errorf("矢高拷贝错误")
	}

	copied.MainSpan = 999
	if original.MainSpan == 999 {
		t.Error("修改拷贝不应影响原始几何")
	}
}
