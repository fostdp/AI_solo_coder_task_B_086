package era_comparator

import (
	"testing"
	"time"

	"zhaozhou-bridge-monitor/services"
)

func TestNewEraComparator_DefaultPool(t *testing.T) {
	ec := NewEraComparator(nil)
	if ec == nil {
		t.Fatal("NewEraComparator(nil) should not return nil")
	}
	if ec.WorkerPool == nil {
		t.Error("WorkerPool should not be nil")
	}
}

func TestNewEraComparator_WithPool(t *testing.T) {
	pool := services.NewFEMWorkerPool(4)
	ec := NewEraComparator(pool)
	if ec == nil {
		t.Fatal("NewEraComparator(pool) should not return nil")
	}
	if ec.WorkerPool != pool {
		t.Error("WorkerPool should be the same as provided")
	}
}

func TestCompare_ZeroLiveLoad(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(0, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.AncientStone == nil || result.ModernRC == nil {
		t.Fatal("AncientStone and ModernRC should not be nil")
	}
	if result.AncientStone.Label != "古石" {
		t.Errorf("AncientStone label should be '古石', got '%s'", result.AncientStone.Label)
	}
	if result.ModernRC.Label != "现代RC" {
		t.Errorf("ModernRC label should be '现代RC', got '%s'", result.ModernRC.Label)
	}
	if result.Summary == nil {
		t.Fatal("Summary should not be nil")
	}
}

func TestCompare_MaterialProperties(t *testing.T) {
	if AncientStone.ElasticModulus != 4.5e9 {
		t.Errorf("AncientStone E should be 4.5e9, got %e", AncientStone.ElasticModulus)
	}
	if AncientStone.CompressiveStrength != 12e6 {
		t.Errorf("AncientStone fc should be 12e6, got %e", AncientStone.CompressiveStrength)
	}
	if ModernRC.ElasticModulus != 30e9 {
		t.Errorf("ModernRC E should be 30e9, got %e", ModernRC.ElasticModulus)
	}
	if ModernRC.CompressiveStrength != 20.1e6 {
		t.Errorf("ModernRC fc should be 20.1e6, got %e", ModernRC.CompressiveStrength)
	}
}

func TestCompare_StiffnessRatio(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	expectedRatio := ModernRC.ElasticModulus / AncientStone.ElasticModulus
	if result.Summary.StiffnessRatio != expectedRatio {
		t.Errorf("StiffnessRatio should be %.2f, got %.2f", expectedRatio, result.Summary.StiffnessRatio)
	}
	if result.Summary.StiffnessRatio < 6.0 || result.Summary.StiffnessRatio > 7.0 {
		t.Errorf("StiffnessRatio should be around 6.67, got %.2f", result.Summary.StiffnessRatio)
	}
}

func TestCompare_StrengthRatio(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	expectedRatio := ModernRC.CompressiveStrength / AncientStone.CompressiveStrength
	if result.Summary.StrengthRatio != expectedRatio {
		t.Errorf("StrengthRatio should be %.2f, got %.2f", expectedRatio, result.Summary.StrengthRatio)
	}
}

func TestCompare_ModernLowerStress(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(20e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.ModernRC.MaxVonMises >= result.AncientStone.MaxVonMises {
		t.Errorf("现代RC应力(%.2e)应小于古石应力(%.2e)", result.ModernRC.MaxVonMises, result.AncientStone.MaxVonMises)
	}
	if result.Summary.MaxStressReductionPct <= 0 {
		t.Errorf("应力应减少，实际减少%.2f%%", result.Summary.MaxStressReductionPct)
	}
}

func TestCompare_ModernLowerDisplacement(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(20e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.ModernRC.MaxDisplacement >= result.AncientStone.MaxDisplacement {
		t.Errorf("现代RC位移(%.6f)应小于古石位移(%.6f)", result.ModernRC.MaxDisplacement, result.AncientStone.MaxDisplacement)
	}
	if result.Summary.MaxDispReductionPct <= 0 {
		t.Errorf("位移应减少，实际减少%.2f%%", result.Summary.MaxDispReductionPct)
	}
}

func TestCompare_WithThermalLoad(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(10e3, 15)
	if err != nil {
		t.Fatalf("Compare with thermal load failed: %v", err)
	}

	if result.AncientStone.MaxVonMises <= 0 {
		t.Error("温度荷载下古石应力应大于0")
	}
	if result.ModernRC.MaxVonMises <= 0 {
		t.Error("温度荷载下现代RC应力应大于0")
	}
}

func TestCompareAsync_Success(t *testing.T) {
	ec := NewEraComparator(nil)

	resultCh := ec.CompareAsync(10e3, 0)

	select {
	case result := <-resultCh:
		if result == nil {
			t.Fatal("async result should not be nil")
		}
		if result.Summary == nil {
			t.Fatal("Summary should not be nil")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("async compare timed out after 30s")
	}
}

func TestCompare_SameGeometry(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.AncientStone.Geometry.MainSpan != result.ModernRC.Geometry.MainSpan {
		t.Error("古今对比应使用相同的几何")
	}
	if result.AncientStone.Geometry.MainRise != result.ModernRC.Geometry.MainRise {
		t.Error("古今对比应使用相同的矢高")
	}
}

func TestCompare_LoadCapacityRatio(t *testing.T) {
	ec := NewEraComparator(nil)

	result, err := ec.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Summary.LoadCapacityRatio != result.Summary.StrengthRatio {
		t.Error("LoadCapacityRatio应等于StrengthRatio")
	}
	if result.Summary.LoadCapacityRatio <= 1.0 {
		t.Error("现代RC的承载力应高于古石")
	}
}
