package services

import (
	"context"
	"math"
	"time"

	"zhaozhou-bridge-monitor/models"
)

type DeformationPredictor struct {
	Material                 *models.MasonryMaterial
	Geometry                 *models.BridgeGeometry
	FEMService               *FEMService
	AnnualTempCycleAmplitude float64
	AnnualTempMean           float64
	CreepPhiInf              float64
	CreepBeta                float64
	StoneAgeDays             float64
	ConstructionYear         int
	AgeCorrectionFactor      float64
	HumidityPercent          float64
	StoneEffectiveThickness  float64
	CreepLoadingAgeDays      float64
}

func NewDeformationPredictor(fem *FEMService) *DeformationPredictor {
	mat := fem.Material
	if mat == nil {
		mat = &models.MasonryMaterial{
			ElasticModulus:        3e9,
			PoissonRatio:          0.15,
			Density:               2400,
			CompressiveStrength:   25e6,
			TensileStrength:       2e6,
			ThermalExpansionCoeff: 5e-6,
			CreepCoeff:            2.0,
		}
	}
	geom := fem.Geometry
	if geom == nil {
		geom = &models.BridgeGeometry{
			MainSpan:           37.02,
			MainRise:           7.23,
			Width:              9.6,
			SmallArchSpanLarge: 3.8,
			SmallArchSpanSmall: 2.8,
			SmallArchRiseLarge: 1.5,
			SmallArchRiseSmall: 1.0,
		}
	}
	p := &DeformationPredictor{
		Material:                 mat,
		Geometry:                 geom,
		FEMService:               fem,
		AnnualTempCycleAmplitude: 25.0,
		AnnualTempMean:           12.0,
		CreepPhiInf:              2.0,
		CreepBeta:                0.3,
		ConstructionYear:         605,
		HumidityPercent:          65.0,
		StoneEffectiveThickness:  0.6,
		CreepLoadingAgeDays:      28.0,
	}
	p.StoneAgeDays = float64(2026-605) * 365.25
	p.AgeCorrectionFactor = p.computeAgeCorrectionFactor()
	return p
}

func (p *DeformationPredictor) TemperatureAtTime(t time.Time) float64 {
	dayOfYear := float64(t.YearDay())
	return p.AnnualTempMean + p.AnnualTempCycleAmplitude*math.Sin(2*math.Pi*(dayOfYear-80)/365)
}

func (p *DeformationPredictor) betaH() float64 {
	h := p.HumidityPercent
	if h >= 90.0 {
		return 1.0
	}
	alphaH := 0.8
	return 1.0 + alphaH*(1.0-h/100.0)
}

func (p *DeformationPredictor) betaT0(t0 float64) float64 {
	return 1.0 / (0.1 + math.Pow(math.Max(t0, 1.0), 0.2))
}

func (p *DeformationPredictor) computeAgeCorrectionFactor() float64 {
	phiBaseInf := 2.0
	betaHVal := p.betaH()
	fcmMPa := p.Material.CompressiveStrength / 1e6
	if fcmMPa < 1.0 {
		fcmMPa = 25.0
	}
	betaFcm := 0.7 + 1.0/math.Sqrt(fcmMPa/10.0)
	t0Ancient := p.StoneAgeDays
	betaT0Ancient := p.betaT0(t0Ancient)
	tRef := 36500.0
	kStiffening := 0.25
	betaAgeArch := math.Exp(-kStiffening * math.Log(p.StoneAgeDays/tRef))
	phiInfAncient := phiBaseInf * betaHVal * betaFcm * betaT0Ancient * betaAgeArch
	t0New := 28.0
	betaT0New := p.betaT0(t0New)
	betaAgeArchNew := 1.0
	phiInfNew := phiBaseInf * betaHVal * betaFcm * betaT0New * betaAgeArchNew
	if phiInfNew < 1e-20 {
		return 1.0
	}
	return phiInfAncient / phiInfNew
}

func (p *DeformationPredictor) CreepFactor(tDays, loadingAgeDays float64) float64 {
	t0 := 365.0
	ratio := tDays / t0
	if ratio < 0 {
		ratio = 0
	}
	betaT0Factor := 1.0 / (0.1 + math.Pow(math.Max(loadingAgeDays, 1.0), 0.2))
	return p.CreepPhiInf * (1 - math.Exp(-math.Pow(ratio, p.CreepBeta))) * betaT0Factor * p.AgeCorrectionFactor
}

func (p *DeformationPredictor) CreepFactorAged(tDaysSinceNow float64) float64 {
	phiHistorical := p.AgeCorrectionFactor * p.originalCreepFactor(p.StoneAgeDays)
	phiTotal := p.AgeCorrectionFactor * p.originalCreepFactor(p.StoneAgeDays+tDaysSinceNow)
	return phiTotal - phiHistorical
}

func (p *DeformationPredictor) originalCreepFactor(tDays float64) float64 {
	t0 := 365.0
	ratio := tDays / t0
	if ratio < 0 {
		ratio = 0
	}
	return p.CreepPhiInf * (1 - math.Exp(-math.Pow(ratio, p.CreepBeta)))
}

func (p *DeformationPredictor) ShrinkageStrain(tDays float64) float64 {
	epsShInf := 3e-4
	kSh := 0.01
	if tDays < 0 {
		tDays = 0
	}
	return epsShInf * (1 - math.Exp(-kSh*math.Sqrt(tDays)))
}

func (p *DeformationPredictor) ShrinkageStrainAged(tDaysSinceNow float64) float64 {
	if p.StoneAgeDays > 36500.0 {
		return 0.0
	}
	return p.ShrinkageStrain(p.StoneAgeDays+tDaysSinceNow) - p.ShrinkageStrain(p.StoneAgeDays)
}

func (p *DeformationPredictor) ComputeThermalDeformation(targetYear int, refTime time.Time) ([]models.DeformationPrediction, error) {
	ctx := context.Background()
	_ = ctx

	tMax := time.Date(targetYear, 7, 15, 0, 0, 0, 0, refTime.Location())
	tMin := time.Date(targetYear, 1, 15, 0, 0, 0, 0, refTime.Location())

	tempMax := p.TemperatureAtTime(tMax)
	tempMin := p.TemperatureAtTime(tMin)

	deltaTMax := tempMax - p.AnnualTempMean
	deltaTMin := tempMin - p.AnnualTempMean

	now := time.Now()
	predictions := make([]models.DeformationPrediction, 0)

	fem := p.FEMService

	for i := range fem.Forces {
		fem.Forces[i] = 0
	}
	fem.thermalDeltaT = 0.0

	if err := fem.GenerateMesh(); err != nil {
		return nil, err
	}
	if err := fem.BuildStiffnessMatrix(); err != nil {
		return nil, err
	}
	if err := fem.ApplyThermalLoad(deltaTMax); err != nil {
		return nil, err
	}
	if err := fem.Solve(); err != nil {
		return nil, err
	}

	maxNodes := make([]models.FEMNode, len(fem.Nodes))
	copy(maxNodes, fem.Nodes)

	for i := range fem.Forces {
		fem.Forces[i] = 0
	}
	fem.thermalDeltaT = 0.0

	if err := fem.BuildStiffnessMatrix(); err != nil {
		return nil, err
	}
	if err := fem.ApplyThermalLoad(deltaTMin); err != nil {
		return nil, err
	}
	if err := fem.Solve(); err != nil {
		return nil, err
	}

	minNodes := make([]models.FEMNode, len(fem.Nodes))
	copy(minNodes, fem.Nodes)

	for _, n := range maxNodes {
		predictions = append(predictions, models.DeformationPrediction{
			Time:        now,
			NodeID:      n.ID,
			PredictedDX: n.Dx,
			PredictedDY: n.Dy,
			TargetYear:  targetYear,
		})
	}

	for _, n := range minNodes {
		predictions = append(predictions, models.DeformationPrediction{
			Time:        now,
			NodeID:      n.ID,
			PredictedDX: n.Dx,
			PredictedDY: n.Dy,
			TargetYear:  targetYear,
		})
	}

	return predictions, nil
}

func (p *DeformationPredictor) ComputeCreepDeformation(targetYear int, refTime time.Time, baseDisplacements []models.FEMNode) ([]models.DeformationPrediction, error) {
	offsetYears := float64(targetYear - refTime.Year())
	phi := p.CreepFactorAged(offsetYears * 365.25)
	factor := 1.0 + phi

	now := time.Now()
	predictions := make([]models.DeformationPrediction, 0, len(baseDisplacements))

	for _, n := range baseDisplacements {
		predictions = append(predictions, models.DeformationPrediction{
			Time:        now,
			NodeID:      n.ID,
			PredictedDX: n.Dx * factor,
			PredictedDY: n.Dy * factor,
			TargetYear:  targetYear,
		})
	}

	return predictions, nil
}

func (p *DeformationPredictor) ComputeShrinkageDeformation(targetYear int, refTime time.Time) ([]models.DeformationPrediction, error) {
	ctx := context.Background()
	_ = ctx

	offsetYears := float64(targetYear - refTime.Year())
	epsSh := p.ShrinkageStrainAged(offsetYears * 365.25)

	alpha := p.Material.ThermalExpansionCoeff
	if alpha < 1e-20 {
		alpha = 5e-6
	}
	deltaTEquiv := -epsSh / alpha

	fem := p.FEMService

	for i := range fem.Forces {
		fem.Forces[i] = 0
	}
	fem.thermalDeltaT = 0.0

	if err := fem.GenerateMesh(); err != nil {
		return nil, err
	}
	if err := fem.BuildStiffnessMatrix(); err != nil {
		return nil, err
	}
	if err := fem.ApplyThermalLoad(deltaTEquiv); err != nil {
		return nil, err
	}
	if err := fem.Solve(); err != nil {
		return nil, err
	}

	now := time.Now()
	predictions := make([]models.DeformationPrediction, 0, len(fem.Nodes))

	for _, n := range fem.Nodes {
		predictions = append(predictions, models.DeformationPrediction{
			Time:        now,
			NodeID:      n.ID,
			PredictedDX: n.Dx,
			PredictedDY: n.Dy,
			TargetYear:  targetYear,
		})
	}

	return predictions, nil
}

func (p *DeformationPredictor) Predict50YearDeformation(refTime time.Time) ([]models.DeformationPrediction, error) {
	ctx := context.Background()
	_ = ctx

	targetYears := []int{1, 5, 10, 20, 30, 50}
	allPredictions := make([]models.DeformationPrediction, 0)
	now := time.Now()

	fem := p.FEMService

	for _, offsetYear := range targetYears {
		targetYear := refTime.Year() + offsetYear

		for i := range fem.Forces {
			fem.Forces[i] = 0
		}
		fem.thermalDeltaT = 0.0

		if err := fem.GenerateMesh(); err != nil {
			return nil, err
		}
		if err := fem.BuildStiffnessMatrix(); err != nil {
			return nil, err
		}
		if err := fem.ApplyGravityLoad(); err != nil {
			return nil, err
		}
		if err := fem.Solve(); err != nil {
			return nil, err
		}

		elasticNodes := make([]models.FEMNode, len(fem.Nodes))
		for i, n := range fem.Nodes {
			elasticNodes[i] = models.FEMNode{
				ID: n.ID,
				X:  n.X,
				Y:  n.Y,
				Dx: n.Dx,
				Dy: n.Dy,
			}
		}

		tDays := float64(offsetYear) * 365.25
		phi := p.CreepFactorAged(tDays)
		creepFactor := 1.0 + phi

		epsSh := p.ShrinkageStrainAged(tDays)
		alpha := p.Material.ThermalExpansionCoeff
		if alpha < 1e-20 {
			alpha = 5e-6
		}
		deltaTEquivSh := -epsSh / alpha

		for i := range fem.Forces {
			fem.Forces[i] = 0
		}
		fem.thermalDeltaT = 0.0

		if err := fem.BuildStiffnessMatrix(); err != nil {
			return nil, err
		}
		if err := fem.ApplyThermalLoad(deltaTEquivSh); err != nil {
			return nil, err
		}
		if err := fem.Solve(); err != nil {
			return nil, err
		}

		shrinkageNodes := make([]models.FEMNode, len(fem.Nodes))
		for i, n := range fem.Nodes {
			shrinkageNodes[i] = models.FEMNode{
				ID: n.ID,
				X:  n.X,
				Y:  n.Y,
				Dx: n.Dx,
				Dy: n.Dy,
			}
		}

		deltaTExtreme := p.AnnualTempCycleAmplitude

		for i := range fem.Forces {
			fem.Forces[i] = 0
		}
		fem.thermalDeltaT = 0.0

		if err := fem.BuildStiffnessMatrix(); err != nil {
			return nil, err
		}
		if err := fem.ApplyThermalLoad(deltaTExtreme); err != nil {
			return nil, err
		}
		if err := fem.Solve(); err != nil {
			return nil, err
		}

		thermalNodes := make([]models.FEMNode, len(fem.Nodes))
		for i, n := range fem.Nodes {
			thermalNodes[i] = models.FEMNode{
				ID: n.ID,
				X:  n.X,
				Y:  n.Y,
				Dx: n.Dx,
				Dy: n.Dy,
			}
		}

		for i, el := range elasticNodes {
			sh := shrinkageNodes[i]
			th := thermalNodes[i]

			totalDx := el.Dx*creepFactor + sh.Dx + th.Dx
			totalDy := el.Dy*creepFactor + sh.Dy + th.Dy

			allPredictions = append(allPredictions, models.DeformationPrediction{
				Time:        now,
				NodeID:      el.ID,
				PredictedDX: totalDx,
				PredictedDY: totalDy,
				TargetYear:  targetYear,
			})
		}
	}

	return allPredictions, nil
}

func (p *DeformationPredictor) GetAgeCorrectionReport() map[string]float64 {
	return map[string]float64{
		"stone_age_days":        p.StoneAgeDays,
		"stone_age_years":       p.StoneAgeDays / 365.25,
		"age_correction_factor": p.AgeCorrectionFactor,
		"humidity_factor_beta_H": p.betaH(),
		"loading_age_beta_t0":   p.betaT0(p.CreepLoadingAgeDays),
		"historical_creep":      p.CreepFactor(p.StoneAgeDays, 28.0),
		"future_creep_50y":      p.CreepFactorAged(50 * 365.25),
		"historical_shrinkage":  p.ShrinkageStrain(p.StoneAgeDays),
	}
}
