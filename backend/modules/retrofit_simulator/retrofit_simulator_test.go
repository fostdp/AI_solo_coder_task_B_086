package retrofit_simulator

import (
	"math"
	"testing"

	"zhaozhou-bridge-monitor/models"
	"zhaozhou-bridge-monitor/services"
)

func newTestWorkerPool() *services.FEMWorkerPool {
	return services.NewFEMWorkerPool(2)
}

func TestSimulate_SingleZoneMainArch(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
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

func TestSimulate_MultiZone(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "left_spandrel", Layers: 2, WidthM: 0.3},
		{Zone: "right_spandrel", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
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

func TestSimulate_FullBridge(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "full", Layers: 5, WidthM: 1.0},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("全桥加固仿真失败: %v", err)
	}

	if result.Summary.CFRPVolumefraction <= 0 {
		t.Error("全桥加固体积分数应>0")
	}
}

func TestSimulate_SafetyFactorIncrease(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 10, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= result.Summary.SafetyFactorBefore {
		t.Errorf("加固后安全系数应提升，before=%.3f, after=%.3f",
			result.Summary.SafetyFactorBefore, result.Summary.SafetyFactorAfter)
	}
}

func TestSimulate_StressReduction(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.MaxStressReductionPct < 0 {
		t.Errorf("应力减少率不应为负，实际为%.2f%%", result.Summary.MaxStressReductionPct)
	}
}

func TestSimulate_DisplacementReduction(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.MaxDispReductionPct < 0 {
		t.Errorf("位移减少率不应为负，实际为%.2f%%", result.Summary.MaxDispReductionPct)
	}
}

func TestSimulate_StiffnessIncrease(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.StiffnessIncreasePct < 0 {
		t.Errorf("刚度提升率不应为负，实际为%.2f%%", result.Summary.StiffnessIncreasePct)
	}
}

func TestSimulate_CFRPProperties(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, WidthM: 0.1},
	}

	result, err := rs.Simulate(configs, 0, 0)
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
	if props.DefaultBondEfficiency != DefaultBondEfficiencyFactor {
		t.Errorf("默认粘结效率应为%.2f，实际为%.2f", DefaultBondEfficiencyFactor, props.DefaultBondEfficiency)
	}
	if props.InterfaceShearStrengthPa != DefaultInterfaceShearStrengthPa {
		t.Errorf("界面抗剪强度应为%.0f，实际为%.0f", DefaultInterfaceShearStrengthPa, props.InterfaceShearStrengthPa)
	}
}

func TestSimulate_CustomThickness(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	customThicknessMM := 0.5
	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, ThicknessMM: customThicknessMM, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("自定义厚度加固仿真失败: %v", err)
	}

	defaultConfigs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 1, WidthM: 0.5},
	}
	defaultResult, err := rs.Simulate(defaultConfigs, 10e3, 0)
	if err != nil {
		t.Fatalf("默认厚度加固仿真失败: %v", err)
	}

	if result.Summary.CFRPVolumefraction <= defaultResult.Summary.CFRPVolumefraction {
		t.Error("自定义更厚CFRP应有更高体积分数")
	}
}

func TestSimulate_ZeroLiveLoad(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.3},
	}

	result, err := rs.Simulate(configs, 0, 0)
	if err != nil {
		t.Fatalf("零活载加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorBefore <= 0 {
		t.Error("零活载下安全系数仍应有效")
	}
}

func TestSimulate_WithThermalLoad(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.Simulate(configs, 10e3, 15.0)
	if err != nil {
		t.Fatalf("温度荷载加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= 0 {
		t.Error("温度荷载下加固后安全系数应有效")
	}
}

func TestSimulate_InvalidZone(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "invalid_zone", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("无效zone不应直接报错，应等效于无加固: %v", err)
	}
	if result.Summary.CFRPVolumefraction != 0 {
		t.Errorf("无效zone体积分数应为0，实际为%f", result.Summary.CFRPVolumefraction)
	}
}

func TestSimulate_ZeroLayers(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 0, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("零层加固不应报错: %v", err)
	}

	if math.Abs(result.Summary.SafetyFactorBefore-result.Summary.SafetyFactorAfter) > 1e-6 {
		t.Error("零层加固前后安全系数应相等")
	}
}

func TestSimulate_ManyLayers(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 100, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("多层加固仿真失败: %v", err)
	}

	if result.Summary.SafetyFactorAfter <= result.Summary.SafetyFactorBefore {
		t.Error("大量CFRP应提升安全系数")
	}
}

func TestSimulate_ZeroWidth(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("零宽度加固不应报错: %v", err)
	}

	if result.Summary.CFRPVolumefraction != 0 {
		t.Error("零宽度体积分数应为0")
	}
}

func TestSimulate_EmptyConfigs(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("空配置列表不应报错: %v", err)
	}

	if result.Summary.CFRPVolumefraction != 0 {
		t.Error("空配置体积分数应为0")
	}
	if result.Summary.BondNote == "" {
		t.Error("空配置应有粘结说明")
	}
}

func TestSimulate_BeforeAfterConsistency(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
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

func TestSimulate_CostEstimateFormat(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 2, WidthM: 0.3},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
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

func TestSimulate_BondSafetyCheck(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 5, WidthM: 1.0, BondEfficiencyFactor: 0.75},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("加固仿真失败: %v", err)
	}

	if result.Summary.BondSafetyFactor <= 0 {
		t.Error("粘结安全系数应>0")
	}
	if result.Summary.AvgBondEfficiency <= 0 || result.Summary.AvgBondEfficiency > 1.0 {
		t.Errorf("平均粘结效率应在(0,1]范围内，实际为%.2f", result.Summary.AvgBondEfficiency)
	}
	if result.Summary.BondNote == "" {
		t.Error("粘结说明不应为空")
	}
}

func TestSimulate_CustomBondEfficiency(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	highBondConfigs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 10, WidthM: 1.0, BondEfficiencyFactor: 0.95},
	}
	highBondResult, err := rs.Simulate(highBondConfigs, 20e3, 0)
	if err != nil {
		t.Fatalf("高粘结效率加固仿真失败: %v", err)
	}

	lowBondConfigs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 10, WidthM: 1.0, BondEfficiencyFactor: 0.3},
	}
	lowBondResult, err := rs.Simulate(lowBondConfigs, 20e3, 0)
	if err != nil {
		t.Fatalf("低粘结效率加固仿真失败: %v", err)
	}

	if highBondResult.Summary.AvgBondEfficiency <= lowBondResult.Summary.AvgBondEfficiency {
		t.Errorf("高粘结效率配置应有更高的平均粘结效率，高=%.2f, 低=%.2f",
			highBondResult.Summary.AvgBondEfficiency, lowBondResult.Summary.AvgBondEfficiency)
	}
	if highBondResult.Summary.MaxStressReductionPct <= lowBondResult.Summary.MaxStressReductionPct {
		t.Errorf("高粘结效率应带来更高的应力减少率，高=%.2f%%, 低=%.2f%%",
			highBondResult.Summary.MaxStressReductionPct, lowBondResult.Summary.MaxStressReductionPct)
	}
}

func TestSimulateAsync_Basic(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5},
	}

	resultCh := rs.SimulateAsync(configs, 10e3, 0)
	result := <-resultCh

	if result == nil {
		t.Fatal("异步结果不应为nil")
	}
	if result.Summary == nil {
		t.Fatal("Summary不应为nil")
	}
	if result.Summary.BondNote != "" && result.Before == nil {
		t.Error("成功结果应有Before数据")
	}
}

func TestSimulateAsync_EmptyConfigs(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{}
	resultCh := rs.SimulateAsync(configs, 10e3, 0)
	result := <-resultCh

	if result == nil {
		t.Fatal("异步结果不应为nil")
	}
	if result.Summary == nil {
		t.Fatal("Summary不应为nil")
	}
}

func TestSimulate_DefaultMaterialProperties(t *testing.T) {
	if DefaultBridgeMaterial.ElasticModulus != 4.5e9 {
		t.Errorf("默认材料E应为4.5GPa，实际为%.0f", DefaultBridgeMaterial.ElasticModulus)
	}
	if DefaultBridgeMaterial.CompressiveStrength != 12e6 {
		t.Errorf("默认材料轴心抗压应为12MPa，实际为%.0f", DefaultBridgeMaterial.CompressiveStrength)
	}
	if DefaultBridgeMaterial.CompressiveStrengthCube != 60e6 {
		t.Errorf("默认材料立方体抗压应为60MPa，实际为%.0f", DefaultBridgeMaterial.CompressiveStrengthCube)
	}
	if DefaultBridgeMaterial.Source == "" {
		t.Error("默认材料应有文献来源")
	}
}

func TestSimulate_DefaultGeometry(t *testing.T) {
	if math.Abs(DefaultBridgeGeometry.MainSpan-37.02) > 1e-6 {
		t.Errorf("默认跨度应为37.02m，实际为%.2f", DefaultBridgeGeometry.MainSpan)
	}
	if math.Abs(DefaultBridgeGeometry.MainRise-7.23) > 1e-6 {
		t.Errorf("默认矢高应为7.23m，实际为%.2f", DefaultBridgeGeometry.MainRise)
	}
}

func TestSimulate_BondEfficiencyOutOfRange(t *testing.T) {
	rs := NewRetrofitSimulator(newTestWorkerPool())

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 3, WidthM: 0.5, BondEfficiencyFactor: 1.5},
	}

	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("超出范围的粘结效率不应报错，应使用默认值: %v", err)
	}

	if math.Abs(result.Summary.AvgBondEfficiency-DefaultBondEfficiencyFactor) > 1e-9 {
		t.Errorf("超出范围的粘结效率应回退到默认值%.2f，实际为%.2f",
			DefaultBondEfficiencyFactor, result.Summary.AvgBondEfficiency)
	}
}

func TestNewRetrofitSimulator_NilPool(t *testing.T) {
	rs := NewRetrofitSimulator(nil)
	if rs == nil {
		t.Fatal("NewRetrofitSimulator(nil)不应返回nil")
	}
	if rs.WorkerPool == nil {
		t.Error("nil pool应创建默认pool")
	}

	configs := []models.ReinforcementConfig{
		{Zone: "main_arch", Layers: 2, WidthM: 0.3},
	}
	result, err := rs.Simulate(configs, 10e3, 0)
	if err != nil {
		t.Fatalf("默认pool应能正常工作: %v", err)
	}
	if result == nil {
		t.Fatal("结果不应为nil")
	}
}
