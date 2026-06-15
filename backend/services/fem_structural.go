package services

import (
	"context"
	"math"
	"time"

	"zhaozhou-bridge-monitor/models"
)

type SubmodelZone struct {
	ID                      int
	Name                    string
	XMin, XMax              float64
	YMin, YMax              float64
	RefineFactor            int
	LocalNodes              []models.FEMNode
	LocalElements           []models.FEMElement
	BoundaryNodeGlobalIDs   []int
	BoundaryDisplacements   map[int][2]float64
	LocalFreeDOFs           map[int]bool
	LocalK                  [][]float64
	LocalForces             []float64
	LocalDisplacements      []float64
	StressResults           []models.FEMStressResult
}

type FEMService struct {
	Nodes         []models.FEMNode
	Elements      []models.FEMElement
	Material      *models.MasonryMaterial
	Geometry      *models.BridgeGeometry
	LoadFactor    float64
	FreeDOFs      map[int]bool
	K             [][]float64
	Forces        []float64
	Displacements []float64
	thermalDeltaT float64
	GlobalNodes   []models.FEMNode
	GlobalElements []models.FEMElement
	SubmodelZones []*SubmodelZone
	UseSubmodeling bool
}

func NewFEMService(geom *models.BridgeGeometry, mat *models.MasonryMaterial) *FEMService {
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
	return &FEMService{
		Nodes:         make([]models.FEMNode, 0),
		Elements:      make([]models.FEMElement, 0),
		Material:      mat,
		Geometry:      geom,
		LoadFactor:    1.0,
		FreeDOFs:      make(map[int]bool),
		thermalDeltaT: 0.0,
		GlobalNodes:   make([]models.FEMNode, 0),
		GlobalElements: make([]models.FEMElement, 0),
		SubmodelZones: make([]*SubmodelZone, 0),
		UseSubmodeling: true,
	}
}

func (f *FEMService) addNode(x, y float64) int {
	id := len(f.Nodes)
	f.Nodes = append(f.Nodes, models.FEMNode{
		ID: id,
		X:  x,
		Y:  y,
	})
	return id
}

func (f *FEMService) addGlobalNode(x, y float64) int {
	id := len(f.GlobalNodes)
	f.GlobalNodes = append(f.GlobalNodes, models.FEMNode{
		ID: id,
		X:  x,
		Y:  y,
	})
	return id
}

func addLocalNode(zone *SubmodelZone, x, y float64) int {
	id := len(zone.LocalNodes)
	zone.LocalNodes = append(zone.LocalNodes, models.FEMNode{
		ID: id,
		X:  x,
		Y:  y,
	})
	return id
}

func archNormalAtX(x, span, rise float64) (nx, ny float64) {
	t := x / span
	dyDx := -8 * rise * (t - 0.5) / span
	lenNorm := math.Sqrt(1 + dyDx*dyDx)
	nx = -dyDx / lenNorm
	ny = 1.0 / lenNorm
	return
}

func (f *FEMService) GenerateGlobalCoarseMesh() error {
	f.GlobalNodes = f.GlobalNodes[:0]
	f.GlobalElements = f.GlobalElements[:0]

	span := f.Geometry.MainSpan
	rise := f.Geometry.MainRise
	deckThickness := 1.0
	archThickness := 0.8

	archBottomNodes := make([]int, 0)
	archTopNodes := make([]int, 0)
	numArchNodes := 12
	for i := 0; i < numArchNodes; i++ {
		t := float64(i) / float64(numArchNodes-1)
		x := t * span
		yBottom := parabolicY(x, span, rise, 0)
		archBottomNodes = append(archBottomNodes, f.addGlobalNode(x, yBottom))

		nx, ny := archNormalAtX(x, span, rise)
		xTop := x + nx*archThickness
		yTop := yBottom + ny*archThickness
		archTopNodes = append(archTopNodes, f.addGlobalNode(xTop, yTop))
	}

	deckY := parabolicY(0, span, rise, 0) + rise + deckThickness
	deckNodes := make([]int, 0)
	numDeckNodes := 10
	for i := 0; i < numDeckNodes; i++ {
		t := float64(i) / float64(numDeckNodes-1)
		x := t * span
		deckNodes = append(deckNodes, f.addGlobalNode(x, deckY))
	}

	pierHeight := rise + 2.0
	pierBottomLeft := f.addGlobalNode(0, -pierHeight)
	pierTopLeft := archBottomNodes[0]
	pierBottomRight := f.addGlobalNode(span, -pierHeight)
	pierTopRight := archBottomNodes[len(archBottomNodes)-1]

	pierMidLeft := f.addGlobalNode(0, -pierHeight/2)
	pierMidRight := f.addGlobalNode(span, -pierHeight/2)

	spandrelConfigs := []struct {
		xStart, xEnd float64
	}{
		{2.0, 4.8},
		{5.8, 9.6},
		{27.4, 31.2},
		{32.2, 35.0},
	}

	for _, sc := range spandrelConfigs {
		xMid := (sc.xStart + sc.xEnd) / 2.0
		archIdxStart := int(math.Round(float64(numArchNodes-1) * sc.xStart / span))
		archIdxEnd := int(math.Round(float64(numArchNodes-1) * sc.xEnd / span))
		if archIdxStart < 0 {
			archIdxStart = 0
		}
		if archIdxEnd >= numArchNodes {
			archIdxEnd = numArchNodes - 1
		}
		archTopStart := archTopNodes[archIdxStart]
		archTopEnd := archTopNodes[archIdxEnd]
		archTopMid := archTopNodes[(archIdxStart+archIdxEnd)/2]

		deckIdxStart := int(math.Round(float64(numDeckNodes-1) * sc.xStart / span))
		deckIdxEnd := int(math.Round(float64(numDeckNodes-1) * sc.xEnd / span))
		if deckIdxStart < 0 {
			deckIdxStart = 0
		}
		if deckIdxEnd >= numDeckNodes {
			deckIdxEnd = numDeckNodes - 1
		}
		deckStart := deckNodes[deckIdxStart]
		deckEnd := deckNodes[deckIdxEnd]
		deckMid := deckNodes[(deckIdxStart+deckIdxEnd)/2]

		topMidID := f.addGlobalNode(xMid, deckY-0.3)

		elemID := len(f.GlobalElements)
		addGlobalTri := func(n1, n2, n3 int, thickness float64) {
			f.GlobalElements = append(f.GlobalElements, models.FEMElement{
				ID:        elemID,
				NodeIDs:   [3]int{n1, n2, n3},
				Thickness: thickness,
				Material:  f.Material,
			})
			elemID++
		}

		addGlobalTri(archTopStart, archTopMid, topMidID, 0.6)
		addGlobalTri(archTopMid, archTopEnd, topMidID, 0.6)
		addGlobalTri(archTopStart, topMidID, deckStart, 0.5)
		addGlobalTri(topMidID, deckMid, deckStart, 0.5)
		addGlobalTri(topMidID, deckEnd, deckMid, 0.5)
		addGlobalTri(archTopEnd, deckEnd, topMidID, 0.5)
	}

	elemID := len(f.GlobalElements)
	addGlobalTri := func(n1, n2, n3 int, thickness float64) {
		f.GlobalElements = append(f.GlobalElements, models.FEMElement{
			ID:        elemID,
			NodeIDs:   [3]int{n1, n2, n3},
			Thickness: thickness,
			Material:  f.Material,
		})
		elemID++
	}

	for i := 0; i < len(archBottomNodes)-1; i++ {
		addGlobalTri(archBottomNodes[i], archBottomNodes[i+1], archTopNodes[i], 0.8)
		addGlobalTri(archBottomNodes[i+1], archTopNodes[i+1], archTopNodes[i], 0.8)
	}

	nonSpandrelDeckSegments := make([][]int, 0)
	currentSegStart := 0
	spandrelXStarts := []float64{2.0, 5.8, 27.4, 32.2}
	spandrelXEnds := []float64{4.8, 9.6, 31.2, 35.0}

	for di := 0; di < len(deckNodes)-1; di++ {
		t1 := float64(di) / float64(len(deckNodes)-1)
		t2 := float64(di+1) / float64(len(deckNodes)-1)
		x1 := t1 * span
		x2 := t2 * span
		xMidSeg := (x1 + x2) / 2.0
		inSpandrel := false
		for si := 0; si < len(spandrelXStarts); si++ {
			if xMidSeg >= spandrelXStarts[si]-0.1 && xMidSeg <= spandrelXEnds[si]+0.1 {
				inSpandrel = true
				break
			}
		}
		if !inSpandrel {
			archIdx1 := int(math.Round(float64(di) * float64(len(archTopNodes)-1) / float64(len(deckNodes)-1)))
			archIdx2 := int(math.Round(float64(di+1) * float64(len(archTopNodes)-1) / float64(len(deckNodes)-1)))
			addGlobalTri(deckNodes[di], deckNodes[di+1], archTopNodes[archIdx1], 0.5)
			addGlobalTri(deckNodes[di+1], archTopNodes[archIdx2], archTopNodes[archIdx1], 0.5)
		}
	}

	addGlobalTri(pierBottomLeft, pierMidLeft, pierTopLeft, 1.0)
	addGlobalTri(pierBottomLeft, f.addGlobalNode(1.5, -pierHeight), pierMidLeft, 1.0)
	addGlobalTri(pierMidLeft, f.addGlobalNode(1.5, -pierHeight/2), pierTopLeft, 1.0)
	addGlobalTri(pierBottomRight, pierMidRight, pierTopRight, 1.0)
	addGlobalTri(pierBottomRight, f.addGlobalNode(span-1.5, -pierHeight), pierMidRight, 1.0)
	addGlobalTri(pierMidRight, f.addGlobalNode(span-1.5, -pierHeight/2), pierTopRight, 1.0)

	f.Nodes = make([]models.FEMNode, len(f.GlobalNodes))
	copy(f.Nodes, f.GlobalNodes)
	f.Elements = make([]models.FEMElement, len(f.GlobalElements))
	copy(f.Elements, f.GlobalElements)
	for i := range f.Elements {
		f.Elements[i].Material = f.Material
	}

	f.FreeDOFs = make(map[int]bool)
	totalDOFs := 2 * len(f.Nodes)
	for i := 0; i < totalDOFs; i++ {
		f.FreeDOFs[i] = true
	}
	f.FreeDOFs[2*pierBottomLeft] = false
	f.FreeDOFs[2*pierBottomLeft+1] = false
	f.FreeDOFs[2*pierBottomRight] = false
	f.FreeDOFs[2*pierBottomRight+1] = false

	f.Forces = make([]float64, totalDOFs)
	f.Displacements = make([]float64, totalDOFs)
	f.K = make([][]float64, totalDOFs)
	for i := range f.K {
		f.K[i] = make([]float64, totalDOFs)
	}
	_ = currentSegStart
	_ = nonSpandrelDeckSegments
	return nil
}

func (f *FEMService) DefineSpandrelSubmodelZones() error {
	f.SubmodelZones = make([]*SubmodelZone, 0)
	span := f.Geometry.MainSpan
	rise := f.Geometry.MainRise
	deckThickness := 1.0
	deckY := parabolicY(0, span, rise, 0) + rise + deckThickness

	zoneConfigs := []struct {
		name                   string
		xMin, xMax             float64
	}{
		{"Left-Spandrel-SA1", 2.0, 4.8},
		{"Left-Spandrel-SA2", 5.8, 9.6},
		{"Right-Spandrel-SA3", 27.4, 31.2},
		{"Right-Spandrel-SA4", 32.2, 35.0},
	}

	for idx, zc := range zoneConfigs {
		yVals := make([]float64, 5)
		for i := 0; i < 5; i++ {
			xt := zc.xMin + float64(i)*(zc.xMax-zc.xMin)/4.0
			yVals[i] = parabolicY(xt, span, rise, 0)
		}
		yMinGlobal := yVals[0]
		for _, yv := range yVals {
			if yv < yMinGlobal {
				yMinGlobal = yv
			}
		}

		zone := &SubmodelZone{
			ID:                    idx,
			Name:                  zc.name,
			XMin:                  zc.xMin,
			XMax:                  zc.xMax,
			YMin:                  yMinGlobal - 0.2,
			YMax:                  deckY + 0.2,
			RefineFactor:          3,
			LocalNodes:            make([]models.FEMNode, 0),
			LocalElements:         make([]models.FEMElement, 0),
			BoundaryNodeGlobalIDs: make([]int, 0),
			BoundaryDisplacements: make(map[int][2]float64),
			LocalFreeDOFs:         make(map[int]bool),
			StressResults:         make([]models.FEMStressResult, 0),
		}
		f.SubmodelZones = append(f.SubmodelZones, zone)
	}

	return nil
}

func parabolicY(x, span, rise, xOffset float64) float64 {
	xs := (x - xOffset) / span
	return rise - 4*rise*(xs-0.5)*(xs-0.5)
}

func (f *FEMService) GenerateMesh() error {
	if f.UseSubmodeling {
		if err := f.GenerateGlobalCoarseMesh(); err != nil {
			return err
		}
		if err := f.DefineSpandrelSubmodelZones(); err != nil {
			return err
		}
		return nil
	}

	f.Nodes = f.Nodes[:0]
	f.Elements = f.Elements[:0]

	span := f.Geometry.MainSpan
	rise := f.Geometry.MainRise
	deckThickness := 1.0
	archThickness := 0.8

	archBottomNodes := make([]int, 0)
	archTopNodes := make([]int, 0)
	numArchNodes := 20
	for i := 0; i < numArchNodes; i++ {
		t := float64(i) / float64(numArchNodes-1)
		x := t * span
		yBottom := parabolicY(x, span, rise, 0)
		archBottomNodes = append(archBottomNodes, f.addNode(x, yBottom))

		dyDx := -8 * rise * (t - 0.5) / span
		lenNorm := math.Sqrt(1 + dyDx*dyDx)
		nx := -dyDx / lenNorm
		ny := 1.0 / lenNorm
		xTop := x + nx*archThickness
		yTop := yBottom + ny*archThickness
		archTopNodes = append(archTopNodes, f.addNode(xTop, yTop))
	}

	deckY := parabolicY(0, span, rise, 0) + rise + deckThickness
	deckNodes := make([]int, 0)
	numDeckNodes := 15
	for i := 0; i < numDeckNodes; i++ {
		t := float64(i) / float64(numDeckNodes-1)
		x := t * span
		deckNodes = append(deckNodes, f.addNode(x, deckY))
	}

	pierHeight := rise + 2.0
	pierBottomLeft := f.addNode(0, -pierHeight)
	pierTopLeft := archBottomNodes[0]
	pierBottomRight := f.addNode(span, -pierHeight)
	pierTopRight := archBottomNodes[len(archBottomNodes)-1]

	pierMidLeft := f.addNode(0, -pierHeight/2)
	pierMidRight := f.addNode(span, -pierHeight/2)

	smallArches := make([][]int, 0)
	smallArchConfigs := []struct {
		span, rise, xStart float64
	}{
		{f.Geometry.SmallArchSpanSmall, f.Geometry.SmallArchRiseSmall, 2.0},
		{f.Geometry.SmallArchSpanLarge, f.Geometry.SmallArchRiseLarge, 2.0 + f.Geometry.SmallArchSpanSmall + 1.0},
		{f.Geometry.SmallArchSpanLarge, f.Geometry.SmallArchRiseLarge, span - 2.0 - f.Geometry.SmallArchSpanSmall - 1.0 - f.Geometry.SmallArchSpanLarge},
		{f.Geometry.SmallArchSpanSmall, f.Geometry.SmallArchRiseSmall, span - 2.0 - f.Geometry.SmallArchSpanSmall},
	}

	for _, cfg := range smallArchConfigs {
		saNodes := make([]int, 0)
		numSAnodes := 8
		for i := 0; i < numSAnodes; i++ {
			t := float64(i) / float64(numSAnodes-1)
			x := cfg.xStart + t*cfg.span
			yBase := parabolicY(x, span, rise, 0)
			dyDx := -8 * rise * ((x / span) - 0.5) / span
			lenNorm := math.Sqrt(1 + dyDx*dyDx)
			nx := -dyDx / lenNorm
			ny := 1.0 / lenNorm
			yArch := yBase + archThickness*ny + parabolicY(x, cfg.span, cfg.rise, cfg.xStart)
			xArch := x + archThickness*nx
			saNodes = append(saNodes, f.addNode(xArch, yArch))
		}
		smallArches = append(smallArches, saNodes)
	}

	elemID := 0
	addTri := func(n1, n2, n3 int, thickness float64) {
		f.Elements = append(f.Elements, models.FEMElement{
			ID:        elemID,
			NodeIDs:   [3]int{n1, n2, n3},
			Thickness: thickness,
			Material:  f.Material,
		})
		elemID++
	}

	for i := 0; i < len(archBottomNodes)-1; i++ {
		addTri(archBottomNodes[i], archBottomNodes[i+1], archTopNodes[i], 0.8)
		addTri(archBottomNodes[i+1], archTopNodes[i+1], archTopNodes[i], 0.8)
	}

	for i := 0; i < len(deckNodes)-1; i++ {
		archIdx := int(math.Round(float64(i) * float64(len(archTopNodes)-1) / float64(len(deckNodes)-1)))
		archIdx2 := int(math.Round(float64(i+1) * float64(len(archTopNodes)-1) / float64(len(deckNodes)-1)))
		addTri(deckNodes[i], deckNodes[i+1], archTopNodes[archIdx], 0.5)
		addTri(deckNodes[i+1], archTopNodes[archIdx2], archTopNodes[archIdx], 0.5)
	}

	addTri(pierBottomLeft, pierMidLeft, pierTopLeft, 1.0)
	addTri(pierBottomLeft, f.addNode(1.5, -pierHeight), pierMidLeft, 1.0)
	addTri(pierMidLeft, f.addNode(1.5, -pierHeight/2), pierTopLeft, 1.0)
	addTri(pierBottomRight, pierMidRight, pierTopRight, 1.0)
	addTri(pierBottomRight, f.addNode(span-1.5, -pierHeight), pierMidRight, 1.0)
	addTri(pierMidRight, f.addNode(span-1.5, -pierHeight/2), pierTopRight, 1.0)

	for _, saNodes := range smallArches {
		for i := 0; i < len(saNodes)-1; i++ {
			tBase := float64(i) / float64(len(saNodes)-1)
			tBase2 := float64(i+1) / float64(len(saNodes)-1)
			archIdx := int(math.Round(tBase * float64(len(archTopNodes)-1)))
			archIdx2 := int(math.Round(tBase2 * float64(len(archTopNodes)-1)))
			addTri(saNodes[i], saNodes[i+1], archTopNodes[archIdx], 0.6)
			addTri(saNodes[i+1], archTopNodes[archIdx2], archTopNodes[archIdx], 0.6)
		}
	}

	f.FreeDOFs = make(map[int]bool)
	totalDOFs := 2 * len(f.Nodes)
	for i := 0; i < totalDOFs; i++ {
		f.FreeDOFs[i] = true
	}
	f.FreeDOFs[2*pierBottomLeft] = false
	f.FreeDOFs[2*pierBottomLeft+1] = false
	f.FreeDOFs[2*pierBottomRight] = false
	f.FreeDOFs[2*pierBottomRight+1] = false

	f.Forces = make([]float64, totalDOFs)
	f.Displacements = make([]float64, totalDOFs)
	f.K = make([][]float64, totalDOFs)
	for i := range f.K {
		f.K[i] = make([]float64, totalDOFs)
	}

	return nil
}

func (f *FEMService) BuildSubmodelMesh(zone *SubmodelZone) error {
	zone.LocalNodes = zone.LocalNodes[:0]
	zone.LocalElements = zone.LocalElements[:0]
	zone.BoundaryNodeGlobalIDs = zone.BoundaryNodeGlobalIDs[:0]

	span := f.Geometry.MainSpan
	rise := f.Geometry.MainRise
	deckThickness := 1.0
	archThickness := 0.8
	deckY := parabolicY(0, span, rise, 0) + rise + deckThickness

	refine := zone.RefineFactor
	numFineX := 8 * refine
	numFineSABottom := 8 * refine
	numFineSATop := 8 * refine
	numDeckFine := 6 * refine

	saSpan := zone.XMax - zone.XMin
	saRise := 1.5
	if saSpan < 3.2 {
		saRise = 1.0
	}
	saXStart := zone.XMin

	archBottomRow := make([]int, 0)
	for i := 0; i <= numFineX; i++ {
		t := float64(i) / float64(numFineX)
		x := zone.XMin + t*(zone.XMax-zone.XMin)
		yBottom := parabolicY(x, span, rise, 0)
		archBottomRow = append(archBottomRow, addLocalNode(zone, x, yBottom))
	}

	archTopRow := make([]int, 0)
	for i := 0; i <= numFineSABottom; i++ {
		t := float64(i) / float64(numFineSABottom)
		x := zone.XMin + t*(zone.XMax-zone.XMin)
		yBase := parabolicY(x, span, rise, 0)
		nx, ny := archNormalAtX(x, span, rise)
		yTop := yBase + archThickness*ny
		xTop := x + archThickness*nx
		archTopRow = append(archTopRow, addLocalNode(zone, xTop, yTop))
	}

	saTopRow := make([]int, 0)
	for i := 0; i <= numFineSATop; i++ {
		t := float64(i) / float64(numFineSATop)
		x := saXStart + t*saSpan
		yBase := parabolicY(x, span, rise, 0)
		nx, ny := archNormalAtX(x, span, rise)
		ySATop := yBase + archThickness*ny + parabolicY(x, saSpan, saRise, saXStart)
		xSATop := x + archThickness*nx
		saTopRow = append(saTopRow, addLocalNode(zone, xSATop, ySATop))
	}

	deckRow := make([]int, 0)
	for i := 0; i <= numDeckFine; i++ {
		t := float64(i) / float64(numDeckFine)
		x := zone.XMin + t*(zone.XMax-zone.XMin)
		deckRow = append(deckRow, addLocalNode(zone, x, deckY))
	}

	localElemID := 0
	addLocalTri := func(n1, n2, n3 int, thickness float64) {
		zone.LocalElements = append(zone.LocalElements, models.FEMElement{
			ID:        localElemID,
			NodeIDs:   [3]int{n1, n2, n3},
			Thickness: thickness,
			Material:  f.Material,
		})
		localElemID++
	}

	numAB := len(archBottomRow)
	numAT := len(archTopRow)
	for i := 0; i < numAB-1; i++ {
		idxAT1 := int(math.Round(float64(i) * float64(numAT-1) / float64(numAB-1)))
		idxAT2 := int(math.Round(float64(i+1) * float64(numAT-1) / float64(numAB-1)))
		if idxAT2 >= numAT {
			idxAT2 = numAT - 1
		}
		addLocalTri(archBottomRow[i], archBottomRow[i+1], archTopRow[idxAT1], 0.8)
		addLocalTri(archBottomRow[i+1], archTopRow[idxAT2], archTopRow[idxAT1], 0.8)
	}

	numSAT := len(saTopRow)
	for i := 0; i < numAT-1; i++ {
		idxSAT1 := int(math.Round(float64(i) * float64(numSAT-1) / float64(numAT-1)))
		idxSAT2 := int(math.Round(float64(i+1) * float64(numSAT-1) / float64(numAT-1)))
		if idxSAT2 >= numSAT {
			idxSAT2 = numSAT - 1
		}
		addLocalTri(archTopRow[i], archTopRow[i+1], saTopRow[idxSAT1], 0.6)
		addLocalTri(archTopRow[i+1], saTopRow[idxSAT2], saTopRow[idxSAT1], 0.6)
	}

	numD := len(deckRow)
	for i := 0; i < numSAT-1; i++ {
		idxD1 := int(math.Round(float64(i) * float64(numD-1) / float64(numSAT-1)))
		idxD2 := int(math.Round(float64(i+1) * float64(numD-1) / float64(numSAT-1)))
		if idxD2 >= numD {
			idxD2 = numD - 1
		}
		addLocalTri(saTopRow[i], saTopRow[i+1], deckRow[idxD1], 0.5)
		addLocalTri(saTopRow[i+1], deckRow[idxD2], deckRow[idxD1], 0.5)
	}

	boundarySet := make(map[int]bool)
	for _, nid := range archBottomRow {
		n := zone.LocalNodes[nid]
		if math.Abs(n.X-zone.XMin) < 1e-3 || math.Abs(n.X-zone.XMax) < 1e-3 {
			boundarySet[nid] = true
		}
	}
	for _, nid := range deckRow {
		n := zone.LocalNodes[nid]
		if math.Abs(n.X-zone.XMin) < 1e-3 || math.Abs(n.X-zone.XMax) < 1e-3 {
			boundarySet[nid] = true
		}
	}

	xTol := (zone.XMax - zone.XMin) * 0.02
	yTol := (zone.YMax - zone.YMin) * 0.02
	if xTol < 0.05 {
		xTol = 0.05
	}
	if yTol < 0.05 {
		yTol = 0.05
	}

	for lnid := range boundarySet {
		ln := zone.LocalNodes[lnid]
		bestGID := -1
		bestDist := 1e20
		for _, gn := range f.GlobalNodes {
			dx := gn.X - ln.X
			dy := gn.Y - ln.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < bestDist {
				bestDist = dist
				bestGID = gn.ID
			}
		}
		if bestGID >= 0 {
			zone.BoundaryNodeGlobalIDs = append(zone.BoundaryNodeGlobalIDs, bestGID)
			zone.BoundaryDisplacements[lnid] = [2]float64{0, 0}
		}
	}
	_ = xTol
	_ = yTol

	totalDOFs := 2 * len(zone.LocalNodes)
	zone.LocalFreeDOFs = make(map[int]bool)
	for i := 0; i < totalDOFs; i++ {
		zone.LocalFreeDOFs[i] = true
	}
	for lnid := range boundarySet {
		zone.LocalFreeDOFs[2*lnid] = false
		zone.LocalFreeDOFs[2*lnid+1] = false
	}

	zone.LocalForces = make([]float64, totalDOFs)
	zone.LocalDisplacements = make([]float64, totalDOFs)
	zone.LocalK = make([][]float64, totalDOFs)
	for i := range zone.LocalK {
		zone.LocalK[i] = make([]float64, totalDOFs)
	}

	return nil
}

func (f *FEMService) InterpolateBoundaryDisplacements(zone *SubmodelZone) {
	for lnid := range zone.BoundaryDisplacements {
		ln := zone.LocalNodes[lnid]
		type neighbor struct {
			dist float64
			dx   float64
			dy   float64
		}
		neighbors := make([]neighbor, 0, 3)
		for _, gn := range f.GlobalNodes {
			dx := gn.X - ln.X
			dy := gn.Y - ln.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 1e-12 {
				dist = 1e-12
			}
			gdx := f.Displacements[2*gn.ID]
			gdy := f.Displacements[2*gn.ID+1]
			if len(neighbors) < 3 {
				neighbors = append(neighbors, neighbor{dist, gdx, gdy})
			} else {
				maxIdx := 0
				for ni := 1; ni < len(neighbors); ni++ {
					if neighbors[ni].dist > neighbors[maxIdx].dist {
						maxIdx = ni
					}
				}
				if dist < neighbors[maxIdx].dist {
					neighbors[maxIdx] = neighbor{dist, gdx, gdy}
				}
			}
		}

		weightSum := 0.0
		dxSum := 0.0
		dySum := 0.0
		for _, nb := range neighbors {
			w := 1.0 / (nb.dist * nb.dist)
			weightSum += w
			dxSum += w * nb.dx
			dySum += w * nb.dy
		}
		if weightSum > 0 {
			zone.BoundaryDisplacements[lnid] = [2]float64{dxSum / weightSum, dySum / weightSum}
		}
	}
}

func (f *FEMService) SolveSubmodel(zone *SubmodelZone, deltaT float64, gravityScale float64) error {
	numDOFs := 2 * len(zone.LocalNodes)
	for i := range zone.LocalK {
		for j := range zone.LocalK[i] {
			zone.LocalK[i][j] = 0
		}
	}
	for i := range zone.LocalForces {
		zone.LocalForces[i] = 0
	}

	E := f.Material.ElasticModulus
	nu := f.Material.PoissonRatio
	D := f.buildConstitutiveMatrix(E, nu)

	for _, elem := range zone.LocalElements {
		n1 := zone.LocalNodes[elem.NodeIDs[0]]
		n2 := zone.LocalNodes[elem.NodeIDs[1]]
		n3 := zone.LocalNodes[elem.NodeIDs[2]]

		B, area := f.buildStrainDisplacementMatrix(n1, n2, n3)
		t := elem.Thickness
		Ke := f.computeElementStiffness(B, D, t, area)

		dofMap := make([]int, 6)
		for i := 0; i < 3; i++ {
			dofMap[2*i] = 2 * elem.NodeIDs[i]
			dofMap[2*i+1] = 2*elem.NodeIDs[i] + 1
		}

		for i := 0; i < 6; i++ {
			for j := 0; j < 6; j++ {
				gi := dofMap[i]
				gj := dofMap[j]
				if gi < numDOFs && gj < numDOFs {
					zone.LocalK[gi][gj] += Ke[i][j]
				}
			}
		}
	}

	g := 9.81
	rho := f.Material.Density
	for _, elem := range zone.LocalElements {
		n1 := zone.LocalNodes[elem.NodeIDs[0]]
		n2 := zone.LocalNodes[elem.NodeIDs[1]]
		n3 := zone.LocalNodes[elem.NodeIDs[2]]
		_, area := f.buildStrainDisplacementMatrix(n1, n2, n3)
		weight := rho * g * area * elem.Thickness * gravityScale
		perNode := weight / 3.0
		for i := 0; i < 3; i++ {
			dofY := 2*elem.NodeIDs[i] + 1
			if dofY < len(zone.LocalForces) {
				zone.LocalForces[dofY] -= perNode * f.LoadFactor
			}
		}
	}

	if math.Abs(deltaT) > 1e-10 {
		alpha := f.Material.ThermalExpansionCoeff
		eps0 := [3]float64{alpha * deltaT, alpha * deltaT, 0}
		var D_eps0 [3]float64
		for i := 0; i < 3; i++ {
			D_eps0[i] = 0
			for j := 0; j < 3; j++ {
				D_eps0[i] += D[i][j] * eps0[j]
			}
		}

		for _, elem := range zone.LocalElements {
			n1 := zone.LocalNodes[elem.NodeIDs[0]]
			n2 := zone.LocalNodes[elem.NodeIDs[1]]
			n3 := zone.LocalNodes[elem.NodeIDs[2]]
			B, area := f.buildStrainDisplacementMatrix(n1, n2, n3)
			t := elem.Thickness
			var nodalForces [6]float64
			for i := 0; i < 6; i++ {
				nodalForces[i] = 0
				for k := 0; k < 3; k++ {
					nodalForces[i] += B[k][i] * D_eps0[k]
				}
				nodalForces[i] *= -t * area
			}
			for i := 0; i < 3; i++ {
				dofX := 2 * elem.NodeIDs[i]
				dofY := 2*elem.NodeIDs[i] + 1
				if dofX < len(zone.LocalForces) {
					zone.LocalForces[dofX] += nodalForces[2*i]
				}
				if dofY < len(zone.LocalForces) {
					zone.LocalForces[dofY] += nodalForces[2*i+1]
				}
			}
		}
	}

	penalty := 1e18
	for lnid, disp := range zone.BoundaryDisplacements {
		dofX := 2 * lnid
		dofY := 2*lnid + 1
		if dofX < numDOFs {
			zone.LocalK[dofX][dofX] += penalty
			zone.LocalForces[dofX] += penalty * disp[0]
		}
		if dofY < numDOFs {
			zone.LocalK[dofY][dofY] += penalty
			zone.LocalForces[dofY] += penalty * disp[1]
		}
	}

	freeList := make([]int, 0)
	for i := 0; i < numDOFs; i++ {
		if zone.LocalFreeDOFs[i] {
			freeList = append(freeList, i)
		}
	}

	for lnid, disp := range zone.BoundaryDisplacements {
		dofX := 2 * lnid
		dofY := 2*lnid + 1
		if dofX < numDOFs {
			zone.LocalDisplacements[dofX] = disp[0]
		}
		if dofY < numDOFs {
			zone.LocalDisplacements[dofY] = disp[1]
		}
	}

	nFree := len(freeList)
	if nFree > 0 {
		Kfree := make([][]float64, nFree)
		for i := range Kfree {
			Kfree[i] = make([]float64, nFree)
		}
		Ffree := make([]float64, nFree)
		for i := 0; i < nFree; i++ {
			gi := freeList[i]
			Ffree[i] = zone.LocalForces[gi]
			for j := 0; j < numDOFs; j++ {
				if !zone.LocalFreeDOFs[j] {
					Ffree[i] -= zone.LocalK[gi][j] * zone.LocalDisplacements[j]
				}
			}
			for j := 0; j < nFree; j++ {
				gj := freeList[j]
				Kfree[i][j] = zone.LocalK[gi][gj]
			}
		}

		Ufree, err := gaussianElimination(Kfree, Ffree)
		if err != nil {
			return err
		}
		for i := 0; i < nFree; i++ {
			zone.LocalDisplacements[freeList[i]] = Ufree[i]
		}
	}

	for i := range zone.LocalNodes {
		zone.LocalNodes[i].Dx = zone.LocalDisplacements[2*i]
		zone.LocalNodes[i].Dy = zone.LocalDisplacements[2*i+1]
	}

	return nil
}

func (f *FEMService) ComputeSubmodelStresses(zone *SubmodelZone, deltaT float64, baseElementIDOffset int) []models.FEMStressResult {
	results := make([]models.FEMStressResult, 0, len(zone.LocalElements))
	now := time.Now()

	E := f.Material.ElasticModulus
	nu := f.Material.PoissonRatio
	alpha := f.Material.ThermalExpansionCoeff
	D := f.buildConstitutiveMatrix(E, nu)

	for _, elem := range zone.LocalElements {
		n1 := zone.LocalNodes[elem.NodeIDs[0]]
		n2 := zone.LocalNodes[elem.NodeIDs[1]]
		n3 := zone.LocalNodes[elem.NodeIDs[2]]

		B, _ := f.buildStrainDisplacementMatrix(n1, n2, n3)

		var u [6]float64
		u[0] = n1.Dx
		u[1] = n1.Dy
		u[2] = n2.Dx
		u[3] = n2.Dy
		u[4] = n3.Dx
		u[5] = n3.Dy

		var eps [3]float64
		for i := 0; i < 3; i++ {
			eps[i] = 0
			for j := 0; j < 6; j++ {
				eps[i] += B[i][j] * u[j]
			}
		}

		eps[0] -= alpha * deltaT
		eps[1] -= alpha * deltaT

		var sig [3]float64
		for i := 0; i < 3; i++ {
			sig[i] = 0
			for j := 0; j < 3; j++ {
				sig[i] += D[i][j] * eps[j]
			}
		}

		sigx := sig[0]
		sigy := sig[1]
		tauxy := sig[2]
		vonMises := math.Sqrt(sigx*sigx - sigx*sigy + sigy*sigy + 3*tauxy*tauxy)

		nodeIDs := make([]int, 3)
		copy(nodeIDs, elem.NodeIDs[:])

		results = append(results, models.FEMStressResult{
			Time:      now,
			ElementID: elem.ID + baseElementIDOffset,
			SigmaX:    sigx,
			SigmaY:    sigy,
			TauXY:     tauxy,
			VonMises:  vonMises,
			NodeIDs:   nodeIDs,
		})
	}

	zone.StressResults = results
	return results
}

func (f *FEMService) BuildStiffnessMatrix() error {
	numDOFs := 2 * len(f.Nodes)
	for i := range f.K {
		for j := range f.K[i] {
			f.K[i][j] = 0
		}
	}

	E := f.Material.ElasticModulus
	nu := f.Material.PoissonRatio
	D := f.buildConstitutiveMatrix(E, nu)

	for _, elem := range f.Elements {
		n1 := f.Nodes[elem.NodeIDs[0]]
		n2 := f.Nodes[elem.NodeIDs[1]]
		n3 := f.Nodes[elem.NodeIDs[2]]

		B, area := f.buildStrainDisplacementMatrix(n1, n2, n3)

		t := elem.Thickness
		Ke := f.computeElementStiffness(B, D, t, area)

		dofMap := make([]int, 6)
		for i := 0; i < 3; i++ {
			dofMap[2*i] = 2 * elem.NodeIDs[i]
			dofMap[2*i+1] = 2*elem.NodeIDs[i] + 1
		}

		for i := 0; i < 6; i++ {
			for j := 0; j < 6; j++ {
				gi := dofMap[i]
				gj := dofMap[j]
				if gi < numDOFs && gj < numDOFs {
					f.K[gi][gj] += Ke[i][j]
				}
			}
		}
	}

	return nil
}

func (f *FEMService) buildConstitutiveMatrix(E, nu float64) [3][3]float64 {
	var D [3][3]float64
	factor := E / (1 - nu*nu)
	D[0][0] = factor
	D[0][1] = factor * nu
	D[0][2] = 0
	D[1][0] = factor * nu
	D[1][1] = factor
	D[1][2] = 0
	D[2][0] = 0
	D[2][1] = 0
	D[2][2] = factor * (1 - nu) / 2
	return D
}

func (f *FEMService) buildStrainDisplacementMatrix(n1, n2, n3 models.FEMNode) ([3][6]float64, float64) {
	var B [3][6]float64
	x1, y1 := n1.X, n1.Y
	x2, y2 := n2.X, n2.Y
	x3, y3 := n3.X, n3.Y

	area := 0.5 * ((x2-x1)*(y3-y1) - (x3-x1)*(y2-y1))
	if area < 0 {
		area = -area
	}

	b1 := y2 - y3
	b2 := y3 - y1
	b3 := y1 - y2

	c1 := x3 - x2
	c2 := x1 - x3
	c3 := x2 - x1

	twoA := 2 * area
	if twoA == 0 {
		twoA = 1e-10
	}

	B[0][0] = b1 / twoA
	B[0][2] = b2 / twoA
	B[0][4] = b3 / twoA

	B[1][1] = c1 / twoA
	B[1][3] = c2 / twoA
	B[1][5] = c3 / twoA

	B[2][0] = c1 / twoA
	B[2][1] = b1 / twoA
	B[2][2] = c2 / twoA
	B[2][3] = b2 / twoA
	B[2][4] = c3 / twoA
	B[2][5] = b3 / twoA

	return B, area
}

func (f *FEMService) computeElementStiffness(B [3][6]float64, D [3][3]float64, t, A float64) [6][6]float64 {
	var Ke [6][6]float64
	var DB [3][6]float64

	for i := 0; i < 3; i++ {
		for j := 0; j < 6; j++ {
			DB[i][j] = 0
			for k := 0; k < 3; k++ {
				DB[i][j] += D[i][k] * B[k][j]
			}
		}
	}

	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			Ke[i][j] = 0
			for k := 0; k < 3; k++ {
				Ke[i][j] += B[k][i] * DB[k][j]
			}
			Ke[i][j] *= t * A
		}
	}

	return Ke
}

func (f *FEMService) ApplyGravityLoad() error {
	g := 9.81
	rho := f.Material.Density

	for _, elem := range f.Elements {
		n1 := f.Nodes[elem.NodeIDs[0]]
		n2 := f.Nodes[elem.NodeIDs[1]]
		n3 := f.Nodes[elem.NodeIDs[2]]

		_, area := f.buildStrainDisplacementMatrix(n1, n2, n3)
		weight := rho * g * area * elem.Thickness
		perNode := weight / 3.0

		for i := 0; i < 3; i++ {
			dofY := 2*elem.NodeIDs[i] + 1
			if dofY < len(f.Forces) {
				f.Forces[dofY] -= perNode * f.LoadFactor
			}
		}
	}

	return nil
}

func (f *FEMService) ApplyLiveLoad(laneLoad float64) error {
	span := f.Geometry.MainSpan
	width := f.Geometry.Width
	deckNodes := make([]int, 0)

	maxY := f.Nodes[0].Y
	for _, n := range f.Nodes {
		if n.Y > maxY {
			maxY = n.Y
		}
	}
	deckY := maxY
	tol := 0.5
	for _, n := range f.Nodes {
		if math.Abs(n.Y-deckY) < tol {
			deckNodes = append(deckNodes, n.ID)
		}
	}

	sortInts(deckNodes, func(i, j int) bool {
		return f.Nodes[deckNodes[i]].X < f.Nodes[deckNodes[j]].X
	})

	for i := 0; i < len(deckNodes)-1; i++ {
		n1 := f.Nodes[deckNodes[i]]
		n2 := f.Nodes[deckNodes[i+1]]
		dx := n2.X - n1.X
		forceSegment := laneLoad * dx * width
		perNode := forceSegment / 2.0

		dof1 := 2*deckNodes[i] + 1
		dof2 := 2*deckNodes[i+1] + 1
		if dof1 < len(f.Forces) {
			f.Forces[dof1] -= perNode * f.LoadFactor
		}
		if dof2 < len(f.Forces) {
			f.Forces[dof2] -= perNode * f.LoadFactor
		}
	}

	return nil
}

func sortInts(arr []int, less func(i, j int) bool) {
	n := len(arr)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if less(j+1, j) {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}

func (f *FEMService) ApplyThermalLoad(deltaT float64) error {
	f.thermalDeltaT = deltaT
	E := f.Material.ElasticModulus
	nu := f.Material.PoissonRatio
	alpha := f.Material.ThermalExpansionCoeff
	D := f.buildConstitutiveMatrix(E, nu)

	eps0 := [3]float64{alpha * deltaT, alpha * deltaT, 0}

	var D_eps0 [3]float64
	for i := 0; i < 3; i++ {
		D_eps0[i] = 0
		for j := 0; j < 3; j++ {
			D_eps0[i] += D[i][j] * eps0[j]
		}
	}

	for _, elem := range f.Elements {
		n1 := f.Nodes[elem.NodeIDs[0]]
		n2 := f.Nodes[elem.NodeIDs[1]]
		n3 := f.Nodes[elem.NodeIDs[2]]

		B, area := f.buildStrainDisplacementMatrix(n1, n2, n3)
		t := elem.Thickness

		var nodalForces [6]float64
		for i := 0; i < 6; i++ {
			nodalForces[i] = 0
			for k := 0; k < 3; k++ {
				nodalForces[i] += B[k][i] * D_eps0[k]
			}
			nodalForces[i] *= -t * area
		}

		for i := 0; i < 3; i++ {
			dofX := 2 * elem.NodeIDs[i]
			dofY := 2*elem.NodeIDs[i] + 1
			if dofX < len(f.Forces) {
				f.Forces[dofX] += nodalForces[2*i]
			}
			if dofY < len(f.Forces) {
				f.Forces[dofY] += nodalForces[2*i+1]
			}
		}
	}

	return nil
}

func (f *FEMService) Solve() error {
	freeList := make([]int, 0)
	numDOFs := 2 * len(f.Nodes)
	for i := 0; i < numDOFs; i++ {
		if f.FreeDOFs[i] {
			freeList = append(freeList, i)
		}
	}
	nFree := len(freeList)

	Kfree := make([][]float64, nFree)
	for i := range Kfree {
		Kfree[i] = make([]float64, nFree)
	}
	Ffree := make([]float64, nFree)

	for i := 0; i < nFree; i++ {
		Ffree[i] = f.Forces[freeList[i]]
		for j := 0; j < nFree; j++ {
			Kfree[i][j] = f.K[freeList[i]][freeList[j]]
		}
	}

	Ufree, err := gaussianElimination(Kfree, Ffree)
	if err != nil {
		return err
	}

	for i := 0; i < numDOFs; i++ {
		f.Displacements[i] = 0
	}
	for i := 0; i < nFree; i++ {
		f.Displacements[freeList[i]] = Ufree[i]
	}

	for i := range f.Nodes {
		f.Nodes[i].Dx = f.Displacements[2*i]
		f.Nodes[i].Dy = f.Displacements[2*i+1]
	}

	return nil
}

func gaussianElimination(A [][]float64, b []float64) ([]float64, error) {
	n := len(b)
	Aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		Aug[i] = make([]float64, n+1)
		copy(Aug[i][:n], A[i])
		Aug[i][n] = b[i]
	}

	for col := 0; col < n; col++ {
		maxRow := col
		maxVal := math.Abs(Aug[col][col])
		for row := col + 1; row < n; row++ {
			if math.Abs(Aug[row][col]) > maxVal {
				maxVal = math.Abs(Aug[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-20 {
			for i := 0; i < n; i++ {
				if math.Abs(Aug[i][col]) > 1e-20 {
					maxRow = i
					maxVal = math.Abs(Aug[i][col])
					break
				}
			}
			if maxVal < 1e-20 {
				continue
			}
		}
		Aug[col], Aug[maxRow] = Aug[maxRow], Aug[col]

		pivot := Aug[col][col]
		if math.Abs(pivot) < 1e-15 {
			continue
		}
		for row := col + 1; row < n; row++ {
			factor := Aug[row][col] / pivot
			for j := col; j <= n; j++ {
				Aug[row][j] -= factor * Aug[col][j]
			}
		}
	}

	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := Aug[i][n]
		for j := i + 1; j < n; j++ {
			sum -= Aug[i][j] * x[j]
		}
		if math.Abs(Aug[i][i]) > 1e-15 {
			x[i] = sum / Aug[i][i]
		} else {
			x[i] = 0
		}
	}

	return x, nil
}

func (f *FEMService) ComputeElementStresses() []models.FEMStressResult {
	results := make([]models.FEMStressResult, 0, len(f.Elements))
	now := time.Now()

	E := f.Material.ElasticModulus
	nu := f.Material.PoissonRatio
	alpha := f.Material.ThermalExpansionCoeff
	D := f.buildConstitutiveMatrix(E, nu)
	deltaT := f.thermalDeltaT

	for _, elem := range f.Elements {
		n1 := f.Nodes[elem.NodeIDs[0]]
		n2 := f.Nodes[elem.NodeIDs[1]]
		n3 := f.Nodes[elem.NodeIDs[2]]

		B, _ := f.buildStrainDisplacementMatrix(n1, n2, n3)

		var u [6]float64
		u[0] = n1.Dx
		u[1] = n1.Dy
		u[2] = n2.Dx
		u[3] = n2.Dy
		u[4] = n3.Dx
		u[5] = n3.Dy

		var eps [3]float64
		for i := 0; i < 3; i++ {
			eps[i] = 0
			for j := 0; j < 6; j++ {
				eps[i] += B[i][j] * u[j]
			}
		}

		eps[0] -= alpha * deltaT
		eps[1] -= alpha * deltaT

		var sig [3]float64
		for i := 0; i < 3; i++ {
			sig[i] = 0
			for j := 0; j < 3; j++ {
				sig[i] += D[i][j] * eps[j]
			}
		}

		sigx := sig[0]
		sigy := sig[1]
		tauxy := sig[2]
		vonMises := math.Sqrt(sigx*sigx - sigx*sigy + sigy*sigy + 3*tauxy*tauxy)

		nodeIDs := make([]int, 3)
		copy(nodeIDs, elem.NodeIDs[:])

		results = append(results, models.FEMStressResult{
			Time:      now,
			ElementID: elem.ID,
			SigmaX:    sigx,
			SigmaY:    sigy,
			TauXY:     tauxy,
			VonMises:  vonMises,
			NodeIDs:   nodeIDs,
		})
	}

	return results
}

func (f *FEMService) RunFullAnalysis(liveLoad, deltaT float64) ([]models.FEMStressResult, error) {
	ctx := context.Background()
	_ = ctx

	for i := range f.Forces {
		f.Forces[i] = 0
	}
	f.thermalDeltaT = 0.0

	if !f.UseSubmodeling {
		if err := f.GenerateMesh(); err != nil {
			return nil, err
		}
		if err := f.BuildStiffnessMatrix(); err != nil {
			return nil, err
		}
		if err := f.ApplyGravityLoad(); err != nil {
			return nil, err
		}
		if liveLoad > 0 {
			if err := f.ApplyLiveLoad(liveLoad); err != nil {
				return nil, err
			}
		}
		if math.Abs(deltaT) > 1e-10 {
			if err := f.ApplyThermalLoad(deltaT); err != nil {
				return nil, err
			}
		}
		if err := f.Solve(); err != nil {
			return nil, err
		}
		return f.ComputeElementStresses(), nil
	}

	if err := f.GenerateMesh(); err != nil {
		return nil, err
	}

	if err := f.BuildStiffnessMatrix(); err != nil {
		return nil, err
	}

	if err := f.ApplyGravityLoad(); err != nil {
		return nil, err
	}

	if liveLoad > 0 {
		if err := f.ApplyLiveLoad(liveLoad); err != nil {
			return nil, err
		}
	}

	if math.Abs(deltaT) > 1e-10 {
		if err := f.ApplyThermalLoad(deltaT); err != nil {
			return nil, err
		}
		f.thermalDeltaT = deltaT
	}

	if err := f.Solve(); err != nil {
		return nil, err
	}

	globalStresses := f.ComputeElementStresses()

	submodelStressMap := make(map[int][]models.FEMStressResult)
	maxGlobalElemID := 0
	for _, gs := range globalStresses {
		if gs.ElementID > maxGlobalElemID {
			maxGlobalElemID = gs.ElementID
		}
	}

	elementIDOffset := maxGlobalElemID + 1
	for zi, zone := range f.SubmodelZones {
		if err := f.BuildSubmodelMesh(zone); err != nil {
			return nil, err
		}
		f.InterpolateBoundaryDisplacements(zone)
		if err := f.SolveSubmodel(zone, deltaT, 1.0); err != nil {
			return nil, err
		}
		zoneStresses := f.ComputeSubmodelStresses(zone, deltaT, elementIDOffset)
		submodelStressMap[zi] = zoneStresses
		elementIDOffset += len(zone.LocalElements)
	}

	allElements := make([]models.FEMElement, len(f.Elements))
	copy(allElements, f.Elements)
	zoneElemStartID := len(allElements)
	for _, zone := range f.SubmodelZones {
		for ei := range zone.LocalElements {
			zelem := zone.LocalElements[ei]
			zelem.ID = zoneElemStartID + ei
			zelem.NodeIDs[0] = len(f.Nodes) + zelem.NodeIDs[0]
			zelem.NodeIDs[1] = len(f.Nodes) + zelem.NodeIDs[1]
			zelem.NodeIDs[2] = len(f.Nodes) + zelem.NodeIDs[2]
			zelem.Material = f.Material
			allElements = append(allElements, zelem)
		}
		for _, ln := range zone.LocalNodes {
			ln.ID = len(f.Nodes)
			f.Nodes = append(f.Nodes, ln)
		}
	}
	f.Elements = allElements

	spandrelXStarts := []float64{2.0, 5.8, 27.4, 32.2}
	spandrelXEnds := []float64{4.8, 9.6, 31.2, 35.0}
	combinedResults := make([]models.FEMStressResult, 0)
	for _, gs := range globalStresses {
		if gs.ElementID >= len(f.GlobalElements) {
			combinedResults = append(combinedResults, gs)
			continue
		}
		elem := f.GlobalElements[gs.ElementID]
		n1 := f.GlobalNodes[elem.NodeIDs[0]]
		n2 := f.GlobalNodes[elem.NodeIDs[1]]
		n3 := f.GlobalNodes[elem.NodeIDs[2]]
		xCenter := (n1.X + n2.X + n3.X) / 3.0
		yCenter := (n1.Y + n2.Y + n3.Y) / 3.0
		inSpandrel := false
		for si := 0; si < len(spandrelXStarts); si++ {
			if xCenter >= spandrelXStarts[si]-0.3 && xCenter <= spandrelXEnds[si]+0.3 && yCenter >= 5.0 {
				inSpandrel = true
				break
			}
		}
		if !inSpandrel {
			combinedResults = append(combinedResults, gs)
		}
	}
	for zi := range f.SubmodelZones {
		combinedResults = append(combinedResults, submodelStressMap[zi]...)
	}

	sortStressResults(combinedResults)

	return combinedResults, nil
}

func sortStressResults(arr []models.FEMStressResult) {
	n := len(arr)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if arr[j].ElementID > arr[j+1].ElementID {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}
