package services

import (
	"math"
	"strings"
	"testing"

	"zhaozhou-bridge-monitor/models"
)

func TestDesignAndTest_ZhaozhouStyle(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          37.02,
		RiseM:          7.23,
		ArchShape:      "circular",
		NumSmallArches: 4,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("赵州桥式设计失败: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Design == nil {
		t.Fatal("Design不应为nil")
	}
	if result.Material == nil {
		t.Fatal("Material不应为nil")
	}
	if result.Report == nil {
		t.Fatal("Report不应为nil")
	}
}

func TestDesignAndTest_ModernRCBridge(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          50,
		RiseM:          10,
		ArchShape:      "parabolic",
		NumSmallArches: 0,
		ArchRingThickM: 1.5,
		MaterialPreset: "modern_rc",
		LiveLoadKPa:    20,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("现代RC拱桥设计失败: %v", err)
	}

	if result.Material.ElasticModulus != 30e9 {
		t.Errorf("现代RC E应为30e9，实际为%.0f", result.Material.ElasticModulus)
	}
}

func TestDesignAndTest_SteelArch(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          100,
		RiseM:          20,
		ArchShape:      "catenary",
		NumSmallArches: 0,
		ArchRingThickM: 0.5,
		MaterialPreset: "steel",
		LiveLoadKPa:    50,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("钢拱桥设计失败: %v", err)
	}

	if result.Material.ElasticModulus != 200e9 {
		t.Errorf("钢E应为200e9，实际为%.0f", result.Material.ElasticModulus)
	}
	if result.Material.Density != 7850 {
		t.Errorf("钢密度应为7850，实际为%.0f", result.Material.Density)
	}
}

func TestDesignAndTest_AllArchShapes(t *testing.T) {
	vbs := NewVirtualBridgeService()

	shapes := []string{"parabolic", "circular", "catenary"}
	for _, shape := range shapes {
		t.Run(shape, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      shape,
				NumSmallArches: 2,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			result, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Fatalf("%s拱型设计失败: %v", shape, err)
			}
			if result.MaxVonMises <= 0 {
				t.Errorf("%s拱型应力应为正", shape)
			}
		})
	}
}

func TestDesignAndTest_AllSmallArchCounts(t *testing.T) {
	vbs := NewVirtualBridgeService()

	counts := []int{0, 1, 2, 3, 4, 5, 6}
	for _, n := range counts {
		t.Run("small_arches_"+strings.ReplaceAll(string(rune('0'+n)), "0", ""), func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: n,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			result, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Fatalf("%d个小拱设计失败: %v", n, err)
			}
			if result.MaxDisplacement <= 0 {
				t.Errorf("%d个小拱位移应为正", n)
			}
		})
	}
}

func TestDesignAndTest_SafetyFactorCalculation(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "parabolic",
		NumSmallArches: 0,
		ArchRingThickM: 2.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    5,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	expectedSF := result.Material.CompressiveStrength / result.MaxVonMises
	if math.Abs(result.SafetyFactor-expectedSF) > 1e-6 {
		t.Errorf("安全系数计算错误，expected=%.4f, actual=%.4f", expectedSF, result.SafetyFactor)
	}
}

func TestDesignAndTest_PassCheckLogic(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          20,
		RiseM:          5,
		ArchShape:      "circular",
		NumSmallArches: 2,
		ArchRingThickM: 2.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    5,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	stressCheck := result.SafetyFactor >= 1.5
	dispCheck := result.MaxDisplacement <= design.SpanM/600.0
	expectedPass := stressCheck && dispCheck

	if result.PassCheck != expectedPass {
		t.Errorf("PassCheck不一致，expected=%v, actual=%v (stress=%v, disp=%v)",
			expectedPass, result.PassCheck, stressCheck, dispCheck)
	}

	if result.Report.StressCheck != stressCheck {
		t.Errorf("Report.StressCheck不一致")
	}
	if result.Report.DisplacementCheck != dispCheck {
		t.Errorf("Report.DisplacementCheck不一致")
	}
}

func TestDesignAndTest_MassCalculation(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "circular",
		NumSmallArches: 0,
		ArchRingThickM: 1.0,
		MaterialPreset: "modern_rc",
		LiveLoadKPa:    0,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	if result.MassKg <= 0 {
		t.Error("质量计算应为正值")
	}
}

func TestDesignAndTest_Recommendation(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "parabolic",
		NumSmallArches: 2,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	if result.Report.Recommendation == "" {
		t.Error("推荐语不应为空")
	}
}

func TestDesignAndTest_UtilizationRatio(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "circular",
		NumSmallArches: 0,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	expectedUtil := result.MaxVonMises / result.Material.CompressiveStrength
	if math.Abs(result.Report.StressUtilization-expectedUtil) > 1e-10 {
		t.Errorf("应力利用率计算错误")
	}
}

func TestDesignAndTest_DispSpanRatio(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          50,
		RiseM:          10,
		ArchShape:      "parabolic",
		NumSmallArches: 0,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	expectedRatio := result.MaxDisplacement / design.SpanM
	if math.Abs(result.Report.DispSpanRatio-expectedRatio) > 1e-12 {
		t.Errorf("跨径比计算错误")
	}
}

func TestDesignAndTest_InvalidSpan(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name  string
		span  float64
	}{
		{"过小", 4.9},
		{"过大", 200.1},
		{"负数", -10},
		{"零", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          tc.span,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: 2,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err == nil {
				t.Errorf("跨径%.1f应报错", tc.span)
			}
		})
	}
}

func TestDesignAndTest_BoundarySpan(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name string
		span float64
	}{
		{"下限5m", 5.0},
		{"上限200m", 200.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          tc.span,
				RiseM:          tc.span * 0.2,
				ArchShape:      "circular",
				NumSmallArches: 0,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Errorf("边界跨径%.1f不应报错: %v", tc.span, err)
			}
		})
	}
}

func TestDesignAndTest_InvalidRise(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name string
		rise float64
	}{
		{"过小", 0.9},
		{"过大", 50.1},
		{"负数", -5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          tc.rise,
				ArchShape:      "circular",
				NumSmallArches: 2,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err == nil {
				t.Errorf("矢高%.1f应报错", tc.rise)
			}
		})
	}
}

func TestDesignAndTest_BoundaryRise(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name string
		rise float64
	}{
		{"下限1m", 1.0},
		{"上限50m", 50.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          tc.rise,
				ArchShape:      "circular",
				NumSmallArches: 0,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Errorf("边界矢高%.1f不应报错: %v", tc.rise, err)
			}
		})
	}
}

func TestDesignAndTest_InvalidArchShape(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "triangle",
		NumSmallArches: 2,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	_, err := vbs.DesignAndTest(design)
	if err == nil {
		t.Error("无效拱型应报错")
	}
}

func TestDesignAndTest_InvalidNumSmallArches(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name string
		n    int
	}{
		{"负数", -1},
		{"过多", 7},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: tc.n,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err == nil {
				t.Errorf("小拱数量%d应报错", tc.n)
			}
		})
	}
}

func TestDesignAndTest_BoundarySmallArches(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name string
		n    int
	}{
		{"下限0个", 0},
		{"上限6个", 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: tc.n,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Errorf("小拱数量%d不应报错: %v", tc.n, err)
			}
		})
	}
}

func TestDesignAndTest_InvalidThickness(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name  string
		thick float64
	}{
		{"过薄", 0.29},
		{"过厚", 5.1},
		{"负数", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: 2,
				ArchRingThickM: tc.thick,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err == nil {
				t.Errorf("拱厚%.2f应报错", tc.thick)
			}
		})
	}
}

func TestDesignAndTest_BoundaryThickness(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name  string
		thick float64
	}{
		{"下限0.3m", 0.3},
		{"上限5m", 5.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: 0,
				ArchRingThickM: tc.thick,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    10,
			}
			_, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Errorf("拱厚%.2f不应报错: %v", tc.thick, err)
			}
		})
	}
}

func TestDesignAndTest_InvalidMaterial(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "circular",
		NumSmallArches: 2,
		ArchRingThickM: 1.0,
		MaterialPreset: "wood",
		LiveLoadKPa:    10,
	}

	_, err := vbs.DesignAndTest(design)
	if err == nil {
		t.Error("无效材料应报错")
	}
}

func TestDesignAndTest_InvalidLiveLoad(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name   string
		loadKPa float64
	}{
		{"负数", -10},
		{"过大", 500.1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: 2,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    tc.loadKPa,
			}
			_, err := vbs.DesignAndTest(design)
			if err == nil {
				t.Errorf("活载%.1f kPa应报错", tc.loadKPa)
			}
		})
	}
}

func TestDesignAndTest_BoundaryLiveLoad(t *testing.T) {
	vbs := NewVirtualBridgeService()

	testCases := []struct {
		name    string
		loadKPa float64
	}{
		{"下限0kPa", 0},
		{"上限500kPa", 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          30,
				RiseM:          6,
				ArchShape:      "circular",
				NumSmallArches: 0,
				ArchRingThickM: 1.0,
				MaterialPreset: "ancient_stone",
				LiveLoadKPa:    tc.loadKPa,
			}
			_, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Errorf("活载%.1f不应报错: %v", tc.loadKPa, err)
			}
		})
	}
}

func TestDesignAndTest_ZeroLiveLoad(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "circular",
		NumSmallArches: 2,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    0,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("零活载设计失败: %v", err)
	}

	if result.MaxVonMises <= 0 {
		t.Error("零活载下仍应有自重应力")
	}
	if result.SafetyFactor <= 1.5 {
		t.Logf("零活载安全系数: %.2f（应较高）", result.SafetyFactor)
	}
}

func TestDesignAndTest_HighLiveLoad(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "circular",
		NumSmallArches: 0,
		ArchRingThickM: 0.5,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    400,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("高活载设计失败: %v", err)
	}

	if result.SafetyFactor >= 1.5 {
		t.Logf("高活载薄拱安全系数: %.2f（预期较低）", result.SafetyFactor)
	}
}

func TestDesignAndTest_CreativeDesigns(t *testing.T) {
	vbs := NewVirtualBridgeService()

	designs := []struct {
		name   string
		design models.VirtualBridgeDesign
	}{
		{
			"扁平抛物线RC拱桥",
			models.VirtualBridgeDesign{
				SpanM:          80, RiseM: 8, ArchShape: "parabolic",
				NumSmallArches: 0, ArchRingThickM: 2.0,
				MaterialPreset: "modern_rc", LiveLoadKPa: 30,
			},
		},
		{
			"高陡钢拱桥",
			models.VirtualBridgeDesign{
				SpanM:          60, RiseM: 30, ArchShape: "catenary",
				NumSmallArches: 0, ArchRingThickM: 0.8,
				MaterialPreset: "steel", LiveLoadKPa: 100,
			},
		},
		{
			"小跨度石拱桥",
			models.VirtualBridgeDesign{
				SpanM:          10, RiseM: 3, ArchShape: "circular",
				NumSmallArches: 1, ArchRingThickM: 0.8,
				MaterialPreset: "ancient_stone", LiveLoadKPa: 20,
			},
		},
		{
			"赵州桥复刻",
			models.VirtualBridgeDesign{
				SpanM:          37.02, RiseM: 7.23, ArchShape: "circular",
				NumSmallArches: 4, ArchRingThickM: 1.03,
				MaterialPreset: "ancient_stone", LiveLoadKPa: 10,
			},
		},
	}

	for _, tc := range designs {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vbs.DesignAndTest(tc.design)
			if err != nil {
				t.Fatalf("创意设计失败: %v", err)
			}
			if result.MaxVonMises <= 0 {
				t.Error("应力应为正")
			}
			if result.MaxDisplacement <= 0 {
				t.Error("位移应为正")
			}
			if result.MassKg <= 0 {
				t.Error("质量应为正")
			}
			if result.Report.Recommendation == "" {
				t.Error("推荐语不应为空")
			}
		})
	}
}

func TestDesignAndTest_MaterialPresetConsistency(t *testing.T) {
	vbs := NewVirtualBridgeService()

	presets := map[string]float64{
		"ancient_stone": 3e9,
		"modern_rc":     30e9,
		"steel":         200e9,
	}

	for preset, expectedE := range presets {
		t.Run(preset, func(t *testing.T) {
			design := models.VirtualBridgeDesign{
				SpanM:          20, RiseM: 4, ArchShape: "circular",
				NumSmallArches: 0, ArchRingThickM: 1.0,
				MaterialPreset: preset, LiveLoadKPa: 10,
			}
			result, err := vbs.DesignAndTest(design)
			if err != nil {
				t.Fatalf("设计失败: %v", err)
			}
			if result.Material.ElasticModulus != expectedE {
				t.Errorf("%s E应为%.0f，实际为%.0f", preset, expectedE, result.Material.ElasticModulus)
			}
		})
	}
}

func TestDesignAndTest_NumNodesElements(t *testing.T) {
	vbs := NewVirtualBridgeService()

	design := models.VirtualBridgeDesign{
		SpanM:          30,
		RiseM:          6,
		ArchShape:      "parabolic",
		NumSmallArches: 2,
		ArchRingThickM: 1.0,
		MaterialPreset: "ancient_stone",
		LiveLoadKPa:    10,
	}

	result, err := vbs.DesignAndTest(design)
	if err != nil {
		t.Fatalf("设计失败: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Error("节点数不应为0")
	}
	if len(result.Elements) == 0 {
		t.Error("单元数不应为0")
	}
	if len(result.Stresses) != len(result.Elements) {
		t.Errorf("应力结果数(%d)应等于单元数(%d)", len(result.Stresses), len(result.Elements))
	}
}
