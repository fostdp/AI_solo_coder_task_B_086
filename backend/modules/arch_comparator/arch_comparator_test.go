package arch_comparator

import (
	"testing"
	"time"

	"zhaozhou-bridge-monitor/services"
)

func TestNewArchComparator_DefaultPool(t *testing.T) {
	ac := NewArchComparator(nil)
	if ac == nil {
		t.Fatal("NewArchComparator(nil) should not return nil")
	}
	if ac.WorkerPool == nil {
		t.Error("WorkerPool should not be nil")
	}
}

func TestNewArchComparator_WithPool(t *testing.T) {
	pool := services.NewFEMWorkerPool(4)
	ac := NewArchComparator(pool)
	if ac == nil {
		t.Fatal("NewArchComparator(pool) should not return nil")
	}
	if ac.WorkerPool != pool {
		t.Error("WorkerPool should be the same as provided")
	}
}

func TestCompare_ZeroLiveLoad(t *testing.T) {
	ac := NewArchComparator(nil)

	result, err := ac.Compare(0, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.OpenSpandrel == nil || result.SolidSpandrel == nil {
		t.Fatal("OpenSpandrel and SolidSpandrel should not be nil")
	}
	if result.OpenSpandrel.Label != "敞肩拱" {
		t.Errorf("OpenSpandrel label should be '敞肩拱', got '%s'", result.OpenSpandrel.Label)
	}
	if result.SolidSpandrel.Label != "实肩拱" {
		t.Errorf("SolidSpandrel label should be '实肩拱', got '%s'", result.SolidSpandrel.Label)
	}
	if result.Summary == nil {
		t.Fatal("Summary should not be nil")
	}
}

func TestCompare_MassReduction(t *testing.T) {
	ac := NewArchComparator(nil)

	result, err := ac.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Summary.MassReductionPct <= 0 {
		t.Errorf("敞肩拱应减轻质量，实际减少%.2f%%", result.Summary.MassReductionPct)
	}
	if result.OpenSpandrel.MassKg >= result.SolidSpandrel.MassKg {
		t.Errorf("敞肩拱质量(%.2f)应小于实肩拱(%.2f)", result.OpenSpandrel.MassKg, result.SolidSpandrel.MassKg)
	}
}

func TestCompare_WithLiveLoad(t *testing.T) {
	ac := NewArchComparator(nil)

	result, err := ac.Compare(20e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.OpenSpandrel.MaxVonMises <= 0 {
		t.Error("敞肩拱应力应大于0")
	}
	if result.SolidSpandrel.MaxVonMises <= 0 {
		t.Error("实肩拱应力应大于0")
	}
	if result.OpenSpandrel.MaxDisplacement <= 0 {
		t.Error("敞肩拱位移应大于0")
	}
	if result.SolidSpandrel.MaxDisplacement <= 0 {
		t.Error("实肩拱位移应大于0")
	}
}

func TestCompare_WithThermalLoad(t *testing.T) {
	ac := NewArchComparator(nil)

	result, err := ac.Compare(10e3, 15)
	if err != nil {
		t.Fatalf("Compare with thermal load failed: %v", err)
	}

	if result.OpenSpandrel.MaxVonMises <= 0 {
		t.Error("温度荷载下敞肩拱应力应大于0")
	}
}

func TestCompareAsync_Success(t *testing.T) {
	ac := NewArchComparator(nil)

	resultCh := ac.CompareAsync(10e3, 0)

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

func TestCompare_DefaultMaterialProps(t *testing.T) {
	if DefaultZhaozhouMaterial.ElasticModulus != 4.5e9 {
		t.Errorf("DefaultZhaozhouMaterial E should be 4.5e9, got %e", DefaultZhaozhouMaterial.ElasticModulus)
	}
	if DefaultZhaozhouMaterial.CompressiveStrength != 12e6 {
		t.Errorf("DefaultZhaozhouMaterial fc should be 12e6, got %e", DefaultZhaozhouMaterial.CompressiveStrength)
	}
	if DefaultZhaozhouGeometry.MainSpan != 37.02 {
		t.Errorf("DefaultZhaozhouGeometry MainSpan should be 37.02, got %.2f", DefaultZhaozhouGeometry.MainSpan)
	}
}

func TestCompare_OpenHasSmallArches(t *testing.T) {
	ac := NewArchComparator(nil)

	result, err := ac.Compare(10e3, 0)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.OpenSpandrel.Geometry.SmallArchSpanLarge <= 0 {
		t.Error("敞肩拱应有小拱")
	}
	if result.SolidSpandrel.Geometry.SmallArchSpanLarge != 0 {
		t.Error("实肩拱不应有小拱")
	}
}
