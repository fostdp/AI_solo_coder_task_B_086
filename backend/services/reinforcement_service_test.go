package services

import (
	"math"
	"testing"

	"zhaozhou-bridge-monitor/models"
)

func TestSimulateReinforcement_SingleZoneMainArch(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("单区域加固仿真失败: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Before == nil || result.After == nil {
		t.Fatal("before/after 不应为 nil")
	}

	if result.Before.Label != "加固前" {
		t.Errorf("expected '加固前', got '%s'", result.Before.Label)
	}
	if result.After.Label != "加固后" {
		t.Errorf("expected '加固后', got '%s'", result.After.Label)
	}
}

func TestSimulateReinforcement_MultiZone(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "left_spandrel", Layers: 2, WidthM: 0.3},
		{Zone: "right_spandrel", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("多区域加固仿真失败: %v", err)
	}

	if result.Summary.CFRPVolumefraction <= 0 {
		t.Error("CFRP体积分数应>0")
	}
	if result.Summary.CostEstimate == "" {
		t.Error("成本估算不应为空")
	}
}

func TestSimulateReinforcement_FullBridge(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "full", Layers: 5, WidthM: 1.0},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("全桥加固仿真失败: %v", err)
	}

	if result.Summary.CFRPVolumefraction <= 0 {
		t.Error("全桥加固体积分数应>0")
	}
}

func TestSimulateReinforcement_SafetyFactorIncrease(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 10, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= result.Summary.SafetyFactorBefore {
		t.Errorf("加固后安全系数应提升，before=%.3f, after=%.3f",
			result.Summary.SafetyFactorBefore, result.Summary.SafetyFactorAfter)
	}
}

func TestSimulateReinforcement_StressReduction(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.MaxStressReductionPct < 0 {
		t.Errorf("应力减少率不应为负，实际为%.2f%%", result.Summary.MaxStressReductionPct)
	}
}

func TestSimulateReinforcement_DisplacementReduction(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.MaxDispReductionPct < 0 {
		t.Errorf("位移减少率不应为负，实际为%.2f%%", result.Summary.MaxDispReductionPct)
	}
}

func TestSimulateReinforcement_StiffnessIncrease(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.StiffnessIncreasePct < 0 {
		t.Errorf("刚度提升率不应为负，实际为%.2f%%", result.Summary.StiffnessIncreasePct)
	}
}

func TestSimulateReinforcement_CFRPProperties(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, WidthM: 0.1},
	}

	result, err := rs.SimulateReinforcement(configs, 0, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	props := result.CFRPProperties
	if props.ElasticModulusPa != DefaultCFRPElasticModulusPa {
		t.Errorf("CFRP E应为%.0f，实际为%.0f", DefaultCFRPElasticModulusPa, props.ElasticModulusPa)
	}
	if props.TensileStrengthPa != DefaultCFRPTensileStrengthPa {
		t.Errorf("CFRP ft应为%.0f，实际为%.0f", DefaultCFRPTensileStrengthPa, props.TensileStrengthPa)
	}
	if math.Abs(props.ThicknessPerLayerMM-DefaultCFRPThicknessPerLayerMM) > 1e-6 {
		t.Errorf("CFRP单层厚度应为%.3fmm，实际为%.3fmm", DefaultCFRPThicknessPerLayerMM, props.ThicknessPerLayerMM)
	}
	if props.DensityKgM3 != DefaultCFRPDensityKgM3 {
		t.Errorf("CFRP密度应为%.0f，实际为%.0f", DefaultCFRPDensityKgM3, props.DensityKgM3)
	}
}

func TestSimulateReinforcement_CustomThickness(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	customThicknessMM := 0.5
	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, ThicknessMM: customThicknessMM, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("自定义厚度加固仿真失败: %v", err)
	}

	defaultConfigs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, WidthM: 0.5},
	}
	defaultResult, err := rs.SimulateReinforcement(defaultConfigs, 10e3, 0)
	if err != nil {
		t.Fatalf("默认厚度加固仿真失败: %v", err)
	}

	if result.Summary.CFRPVolumefraction <= defaultResult.Summary.CFRPVolumefraction {
		t.Error("自定义更厚CFRP应有更高体积分数")
	}
}

func TestSimulateReinforcement_ZeroLiveLoad(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.3},
	}

	result, err := rs.SimulateReinforcement(configs, 0, 0)
	if err != nil {
		t.Fatalf("零活载加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorBefore <= 0 {
		t.Error("零活载下安全系数仍应有效")
	}
}

func TestSimulateReinforcement_WithThermalLoad(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 15.0)
	if err != nil {
		t.Fatalf("温度荷载加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= 0 {
		t.Error("温度荷载下加固后安全系数应有效")
	}
}

func TestSimulateReinforcement_InvalidZone(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "invalid_zone", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("无效zone不应直接报错，应等效于无加固: %v", err)
	}
	if result.Summary.CFRPVolumefraction != 0 {
		t.Errorf("无效zone体积分数应为0，实际为%f", result.Summary.CFRPVolumefraction)
	}
}

func TestSimulateReinforcement_ZeroLayers(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 0, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("零层加固不应报错: %v", err)
	}

	if math.Abs(result.Summary.SafetyFactorBefore-result.Summary.SafetyFactorAfter) > 1e-6 {
		t.Error("零层加固前后安全系数应相等")
	}
}

func TestSimulateReinforcement_ManyLayers(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 100, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("多层加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= result.Summary.SafetyFactorBefore {
		t.Error("大量CFRP应提升安全系数")
	}
}

func TestSimulateReinforcement_ZeroWidth(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("零宽度加固不应报错: %v", err)
	}

	if result.Summary.CFRPVolumefraction != 0 {
		t.Error("零宽度体积分数应为0")
	}
}

func TestSimulateReinforcement_EmptyConfigs(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("空配置列表不应报错: %v", err)
	}

	if result.Summary.CFRPVolumefraction != 0 {
		t.Error("空配置体积分数应为0")
	}
}

func TestSimulateReinforcement_NegativeLayers(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: -1, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("负层数不应直接报错: %v", err)
	}

	if result.Summary.CFRPVolumefraction >= 0 {
		t.Log("注意：负层数可能导致负的体积分数，业务层应校验")
	}
}

func TestElemInZone_MainArch(t *testing.T) {
	nodes := []models.FEMNode{
		{X: 10, Y: 5},
		{X: 12, Y: 5.5},
		{X: 11, Y: 6},
	}
	elem := &models.FEMElement{NodeIDs: []int{0, 1, 2}}

	span := 37.02
	rise := 7.23

	if !elemInZone(elem, nodes, "main_arch", span, rise) {
		t.Error("拱顶附近的单元应属于main_arch")
	}
}

func TestElemInZone_LeftSpandrel(t *testing.T) {
	nodes := []models.FEMNode{
		{X: 1, Y: 0.5},
		{X: 3, Y: 1},
		{X: 2, Y: 1.5},
	}
	elem := &models.FEMElement{NodeIDs: []int{0, 1, 2}}

	span := 37.02
	rise := 7.23

	if !elemInZone(elem, nodes, "left_spandrel", span, rise) {
		t.Error("左端单元应属于left_spandrel")
	}
	if elemInZone(elem, nodes, "right_spandrel", span, rise) {
		t.Error("左端单元不应属于right_spandrel")
	}
}

func TestElemInZone_RightSpandrel(t *testing.T) {
	nodes := []models.FEMNode{
		{X: 34, Y: 1},
		{X: 36, Y: 0.5},
		{X: 35, Y: 1.5},
	}
	elem := &models.FEMElement{NodeIDs: []int{0, 1, 2}}

	span := 37.02
	rise := 7.23

	if !elemInZone(elem, nodes, "right_spandrel", span, rise) {
		t.Error("右端单元应属于right_spandrel")
	}
}

func TestElemInZone_Full(t *testing.T) {
	nodes := []models.FEMNode{
		{X: 10, Y: 2},
		{X: 12, Y: 3},
		{X: 11, Y: 4},
	}
	elem := &models.FEMElement{NodeIDs: []int{0, 1, 2}}

	span := 37.02
	rise := 7.23

	if !elemInZone(elem, nodes, "full", span, rise) {
		t.Error("任何单元都应属于full zone")
	}
}

func TestElemInZone_InvalidZone(t *testing.T) {
	nodes := []models.FEMNode{
		{X: 10, Y: 2},
		{X: 12, Y: 3},
		{X: 11, Y: 4},
	}
	elem := &models.FEMElement{NodeIDs: []int{0, 1, 2}}

	span := 37.02
	rise := 7.23

	if elemInZone(elem, nodes, "unknown_zone", span, rise) {
		t.Error("无效zone应返回false")
	}
}

func TestSimulateReinforcement_BeforeAfterConsistency(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if len(result.Before.Nodes) != len(result.After.Nodes) {
		t.Errorf("加固前后节点数应相同，before=%d, after=%d",
			len(result.Before.Nodes), len(result.After.Nodes))
	}
	if len(result.Before.Elements) != len(result.After.Elements) {
		t.Errorf("加固前后单元数应相同，before=%d, after=%d",
			len(result.Before.Elements), len(result.After.Elements))
	}
}

func TestSimulateReinforcement_CostEstimateFormat(t *testing.T) {
	baseFEM := newTestBaseFEM()
	rs := NewReinforcementService(baseFEM)

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.SimulateReinforcement(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.CostEstimate == "" {
		t.Error("成本估算不应为空")
	}
	if len(result.Summary.CostEstimate) < 5 {
		t.Errorf("成本估算内容过短: %s", result.Summary.CostEstimate)
	}
}
