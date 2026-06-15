window.ZhaozhouBridge3D = (function() {
    var scene, camera, renderer, controls, bridgeGroup, stressOverlayGroup;
    var crackMarkers = [], sensorMarkers = [];
    var geometryData = null, stressData = null;
    var autoRotate = false, stressVisible = false, cracksVisible = false;
    var originalPositions = null, deformed = false;
    var femNodes = [], femElements = [], femStress = [], predictions = [];
    var apiBase = (window.location.origin || 'http://localhost:8080');
    var config = null;
    var containerEl = null;
    var callbacks = {
        onStressUpdated: null,
        onPredictionsUpdated: null
    };

    async function fetchJSON(url, opts) {
        try {
            var res = await fetch(url, {
                headers: { 'Content-Type': 'application/json' },
                ...opts
            });
            if (!res.ok) {
                console.warn('[ZhaozhouBridge3D][fetchJSON] HTTP ' + res.status + ': ' + url);
                return null;
            }
            return await res.json();
        } catch (e) {
            console.warn('[ZhaozhouBridge3D][fetchJSON] Failed: ' + url, e.message);
            return null;
        }
    }

    async function loadConfig() {
        try {
            var res = await fetchJSON('config/app_config.json');
            if (res) {
                config = res;
                window.COLORS = res.front_end && res.front_end.colors ? res.front_end.colors : window.COLORS;
                if (res.front_end && res.front_end.stress_map_tier_override) {
                    var override = res.front_end.stress_map_tier_override;
                    if (override !== 'auto') {
                        var tierNum = parseInt(override, 10);
                        if (!isNaN(tierNum)) {
                            window.getGpuTier = function() { return tierNum; };
                        }
                    }
                }
            }
        } catch (e) {
            console.warn('[ZhaozhouBridge3D][loadConfig] Failed:', e.message);
        }
    }

    async function loadGeometry() {
        var geom = await fetchJSON(apiBase + '/api/bridge/geometry');
        if (geom) {
            femNodes = geom.nodes || [];
            femElements = geom.elements || [];
        }
        var predRes = await fetchJSON(apiBase + '/api/deformation/predict50', { method: 'POST' });
        if (predRes && Array.isArray(predRes)) {
            predictions = predRes.map(function(p) {
                return {
                    dx: p.predicted_dx || 0,
                    dy: p.predicted_dy || 0,
                    node_id: p.node_id
                };
            });
        }
        return { nodes: femNodes, elements: femElements };
    }

    function init(containerId) {
        containerEl = document.getElementById(containerId);
        if (!containerEl) return;

        scene = new THREE.Scene();
        scene.fog = new THREE.Fog(0x1a2530, 80, 200);

        var skyCanvas = document.createElement('canvas');
        skyCanvas.width = 512;
        skyCanvas.height = 512;
        var skyCtx = skyCanvas.getContext('2d');
        var skyGrad = skyCtx.createLinearGradient(0, 0, 0, 512);
        skyGrad.addColorStop(0, '#87CEEB');
        skyGrad.addColorStop(0.5, '#c9d8e8');
        skyGrad.addColorStop(1, '#e8dcc8');
        skyCtx.fillStyle = skyGrad;
        skyCtx.fillRect(0, 0, 512, 512);
        var skyTex = new THREE.CanvasTexture(skyCanvas);
        scene.background = skyTex;

        camera = new THREE.PerspectiveCamera(45, containerEl.clientWidth / containerEl.clientHeight, 0.1, 1000);
        camera.position.set(50, 30, 60);

        renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
        renderer.setSize(containerEl.clientWidth, containerEl.clientHeight);
        renderer.setPixelRatio(window.devicePixelRatio);
        renderer.shadowMap.enabled = true;
        renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        containerEl.appendChild(renderer.domElement);

        controls = new THREE.OrbitControls(camera, renderer.domElement);
        controls.enableDamping = true;
        controls.dampingFactor = 0.08;
        controls.target.set(18.5, 4, 0);
        controls.minDistance = 10;
        controls.maxDistance = 150;
        controls.maxPolarAngle = Math.PI / 2.1;

        var ambientLight = new THREE.AmbientLight(0xffffff, 0.55);
        scene.add(ambientLight);

        var hemiLight = new THREE.HemisphereLight(0xb4d0f0, 0x8b7355, 0.45);
        scene.add(hemiLight);

        var dirLight1 = new THREE.DirectionalLight(0xffffff, 0.85);
        dirLight1.position.set(40, 60, 30);
        dirLight1.castShadow = true;
        dirLight1.shadow.mapSize.width = 2048;
        dirLight1.shadow.mapSize.height = 2048;
        dirLight1.shadow.camera.near = 0.5;
        dirLight1.shadow.camera.far = 200;
        dirLight1.shadow.camera.left = -60;
        dirLight1.shadow.camera.right = 60;
        dirLight1.shadow.camera.top = 60;
        dirLight1.shadow.camera.bottom = -60;
        dirLight1.shadow.bias = -0.0005;
        scene.add(dirLight1);

        var dirLight2 = new THREE.DirectionalLight(0xffeedd, 0.35);
        dirLight2.position.set(-30, 40, -25);
        dirLight2.castShadow = true;
        dirLight2.shadow.mapSize.width = 1024;
        dirLight2.shadow.mapSize.height = 1024;
        dirLight2.shadow.camera.near = 0.5;
        dirLight2.shadow.camera.far = 150;
        dirLight2.shadow.camera.left = -50;
        dirLight2.shadow.camera.right = 50;
        dirLight2.shadow.camera.top = 50;
        dirLight2.shadow.camera.bottom = -50;
        scene.add(dirLight2);

        var groundGeo = new THREE.PlaneGeometry(400, 400);
        var groundCanvas = document.createElement('canvas');
        groundCanvas.width = 256;
        groundCanvas.height = 256;
        var gctx = groundCanvas.getContext('2d');
        gctx.fillStyle = '#d5cfc3';
        gctx.fillRect(0, 0, 256, 256);
        for (var i = 0; i < 2000; i++) {
            var px = Math.random() * 256;
            var py = Math.random() * 256;
            var shade = 180 + Math.floor(Math.random() * 50);
            gctx.fillStyle = 'rgb(' + shade + ',' + (shade - 10) + ',' + (shade - 30) + ')';
            gctx.fillRect(px, py, 2, 2);
        }
        var groundTex = new THREE.CanvasTexture(groundCanvas);
        groundTex.wrapS = THREE.RepeatWrapping;
        groundTex.wrapT = THREE.RepeatWrapping;
        groundTex.repeat.set(20, 20);
        var groundMat = new THREE.MeshStandardMaterial({
            map: groundTex,
            roughness: 0.95,
            metalness: 0.0,
            color: 0xcfc9bd
        });
        var ground = new THREE.Mesh(groundGeo, groundMat);
        ground.rotation.x = -Math.PI / 2;
        ground.position.y = -0.01;
        ground.receiveShadow = true;
        scene.add(ground);

        bridgeGroup = buildZhaozhouBridge();
        scene.add(bridgeGroup);

        stressOverlayGroup = new THREE.Group();
        scene.add(stressOverlayGroup);

        setupGroundAndEnvironment(bridgeGroup);

        window.addEventListener('resize', onWindowResize, false);

        bindToolbarEvents();

        var initialStress = fetchJSON(apiBase + '/api/fem/analyze', {
            method: 'POST',
            body: JSON.stringify({ live_load: 10000, delta_t: 0 })
        });
        initialStress.then(function(res) {
            if (res && Array.isArray(res)) {
                femStress = res.map(function(r) { return r.von_mises || 0; });
                var elemTriples = femElements.map(function(e) {
                    return Array.isArray(e.node_ids) ? e.node_ids : [e[0], e[1], e[2]];
                });
                updateStress(femStress, elemTriples, femNodes);
            }
        });

        animate();
    }

    function buildZhaozhouBridge() {
        var group = new THREE.Group();

        var mainSpan = 37.02;
        var mainRise = 7.23;
        var archThick = 1.0;
        var deckWidth = 9.6;
        var deckThick = 0.4;
        var abutmentHeight = 4.0;

        var parabolaCurve = new THREE.CurvePath();
        var parabolaPoints = [];
        var segments = 80;
        for (var i = 0; i <= segments; i++) {
            var t = i / segments;
            var x = t * mainSpan;
            var y = mainRise - 4 * mainRise * Math.pow(t - 0.5, 2);
            parabolaPoints.push(new THREE.Vector3(x, y + abutmentHeight, 0));
        }
        var parabolaLine = new THREE.CatmullRomCurve3(parabolaPoints);
        parabolaCurve.add(parabolaLine);

        var mainArchGeo = new THREE.TubeGeometry(parabolaLine, 80, archThick / 2, 8, false);
        var archMat = new THREE.MeshStandardMaterial({
            color: 0xc9a87c,
            roughness: 0.85,
            metalness: 0.05
        });

        var mainArch = new THREE.Mesh(mainArchGeo, archMat);
        mainArch.castShadow = true;
        mainArch.receiveShadow = true;
        group.add(mainArch);

        originalPositions = new Float32Array(mainArchGeo.attributes.position.array.length);
        originalPositions.set(mainArchGeo.attributes.position.array);

        var deckLength = mainSpan + 2;
        var deckGeo = new THREE.BoxGeometry(deckLength, deckThick, deckWidth);
        var deckMat = new THREE.MeshStandardMaterial({
            color: 0xb8a078,
            roughness: 0.9,
            metalness: 0.03
        });
        var deck = new THREE.Mesh(deckGeo, deckMat);
        deck.position.set(mainSpan / 2, abutmentHeight + mainRise + deckThick / 2, 0);
        deck.castShadow = true;
        deck.receiveShadow = true;
        group.add(deck);

        var parapetMat = new THREE.MeshStandardMaterial({
            color: 0x9a8260,
            roughness: 0.88,
            metalness: 0.02
        });
        var parapetHeight = 0.3;
        var parapetThick = 0.2;

        var parapetGeo = new THREE.BoxGeometry(deckLength, parapetHeight, parapetThick);
        var parapet1 = new THREE.Mesh(parapetGeo, parapetMat);
        parapet1.position.set(mainSpan / 2, abutmentHeight + mainRise + deckThick + parapetHeight / 2, deckWidth / 2 - parapetThick / 2);
        parapet1.castShadow = true;
        parapet1.receiveShadow = true;
        group.add(parapet1);

        var parapet2 = new THREE.Mesh(parapetGeo, parapetMat);
        parapet2.position.set(mainSpan / 2, abutmentHeight + mainRise + deckThick + parapetHeight / 2, -deckWidth / 2 + parapetThick / 2);
        parapet2.castShadow = true;
        parapet2.receiveShadow = true;
        group.add(parapet2);

        var postGeo = new THREE.CylinderGeometry(0.06, 0.06, parapetHeight, 8);
        var postSpacing = 2;
        for (var px = -1; px <= mainSpan + 1; px += postSpacing) {
            var post1 = new THREE.Mesh(postGeo, parapetMat);
            post1.position.set(px, abutmentHeight + mainRise + deckThick + parapetHeight / 2, deckWidth / 2 - parapetThick / 2);
            post1.castShadow = true;
            group.add(post1);

            var post2 = new THREE.Mesh(postGeo, parapetMat);
            post2.position.set(px, abutmentHeight + mainRise + deckThick + parapetHeight / 2, -deckWidth / 2 + parapetThick / 2);
            post2.castShadow = true;
            group.add(post2);
        }

        var spandrelMat = archMat.clone();
        var spandrelThickness = deckWidth - 0.5;

        function parabolaY(x) {
            var t = x / mainSpan;
            return mainRise - 4 * mainRise * Math.pow(t - 0.5, 2) + abutmentHeight;
        }

        var smallArches = [
            { xCenter: 5, span: 2.8, rise: 1.0, thick: 0.5 },
            { xCenter: 10, span: 3.8, rise: 1.5, thick: 0.6 },
            { xCenter: mainSpan - 10, span: 3.8, rise: 1.5, thick: 0.6 },
            { xCenter: mainSpan - 5, span: 2.8, rise: 1.0, thick: 0.5 }
        ];

        var wallSegments = [
            { xStart: 0, xEnd: smallArches[0].xCenter - smallArches[0].span / 2 },
            { xStart: smallArches[0].xCenter + smallArches[0].span / 2, xEnd: smallArches[1].xCenter - smallArches[1].span / 2 },
            { xStart: smallArches[1].xCenter + smallArches[1].span / 2, xEnd: smallArches[2].xCenter - smallArches[2].span / 2 },
            { xStart: smallArches[2].xCenter + smallArches[2].span / 2, xEnd: smallArches[3].xCenter - smallArches[3].span / 2 },
            { xStart: smallArches[3].xCenter + smallArches[3].span / 2, xEnd: mainSpan }
        ];

        wallSegments.forEach(function(seg) {
            var numSlices = Math.max(2, Math.ceil((seg.xEnd - seg.xStart) / 0.5));
            for (var s = 0; s < numSlices; s++) {
                var x0 = seg.xStart + (s / numSlices) * (seg.xEnd - seg.xStart);
                var x1 = seg.xStart + ((s + 1) / numSlices) * (seg.xEnd - seg.xStart);
                var segWidth = x1 - x0;
                var yArch0 = parabolaY(x0);
                var yArch1 = parabolaY(x1);
                var yArchAvg = (yArch0 + yArch1) / 2;
                var yDeck = abutmentHeight + mainRise + deckThick;
                var segHeight = yDeck - yArchAvg;

                if (segHeight > 0.05 && segWidth > 0.01) {
                    var segGeo = new THREE.BoxGeometry(segWidth, segHeight, spandrelThickness);
                    var segMesh = new THREE.Mesh(segGeo, spandrelMat);
                    segMesh.position.set((x0 + x1) / 2, yArchAvg + segHeight / 2, 0);
                    segMesh.castShadow = true;
                    segMesh.receiveShadow = true;
                    group.add(segMesh);
                }
            }
        });

        smallArches.forEach(function(sa) {
            var saPoints = [];
            var saSegs = 30;
            for (var si = 0; si <= saSegs; si++) {
                var st = si / saSegs;
                var sax = sa.xCenter - sa.span / 2 + st * sa.span;
                var say = sa.rise - 4 * sa.rise * Math.pow(st - 0.5, 2);
                var baseY = parabolaY(sax);
                saPoints.push(new THREE.Vector3(sax, baseY + say, 0));
            }
            var saCurve = new THREE.CatmullRomCurve3(saPoints);
            var saGeo = new THREE.TubeGeometry(saCurve, 30, sa.thick / 2, 6, false);
            var saMesh = new THREE.Mesh(saGeo, archMat);
            saMesh.castShadow = true;
            saMesh.receiveShadow = true;
            group.add(saMesh);
        });

        var abutmentWidth = 6.0;
        var abutmentThickZ = deckWidth + 1;
        var abutmentGeo = new THREE.BoxGeometry(abutmentWidth, abutmentHeight, abutmentThickZ);
        var abutmentMat = new THREE.MeshStandardMaterial({
            color: 0xb5966e,
            roughness: 0.9,
            metalness: 0.02
        });

        var abutment1 = new THREE.Mesh(abutmentGeo, abutmentMat);
        abutment1.position.set(-1 - abutmentWidth / 2 + 0.5, abutmentHeight / 2, 0);
        abutment1.castShadow = true;
        abutment1.receiveShadow = true;
        group.add(abutment1);

        var abutment2 = new THREE.Mesh(abutmentGeo, abutmentMat);
        abutment2.position.set(mainSpan + 1 + abutmentWidth / 2 - 0.5, abutmentHeight / 2, 0);
        abutment2.castShadow = true;
        abutment2.receiveShadow = true;
        group.add(abutment2);

        function addSensorMarker(position, type, label) {
            var colors = {
                ARCH: 0x00aaaa,
                PIER: 0x0066ff,
                SARCH: 0x00cc44,
                CRACK: 0xffcc00
            };
            var sensorGeo = new THREE.SphereGeometry(0.15, 16, 16);
            var sensorMat = new THREE.MeshStandardMaterial({
                color: colors[type] || 0xffffff,
                emissive: colors[type] || 0xffffff,
                emissiveIntensity: 0.6,
                roughness: 0.3,
                metalness: 0.5
            });
            var sensor = new THREE.Mesh(sensorGeo, sensorMat);
            sensor.position.copy(position);
            sensor.castShadow = true;
            sensor.userData = { type: type, label: label };
            group.add(sensor);
            sensorMarkers.push(sensor);

            var haloCanvas = document.createElement('canvas');
            haloCanvas.width = 128;
            haloCanvas.height = 128;
            var hctx = haloCanvas.getContext('2d');
            var haloGrad = hctx.createRadialGradient(64, 64, 0, 64, 64, 64);
            var hexColor = '#' + (colors[type] || 0xffffff).toString(16).padStart(6, '0');
            haloGrad.addColorStop(0, hexColor + 'cc');
            haloGrad.addColorStop(0.5, hexColor + '55');
            haloGrad.addColorStop(1, hexColor + '00');
            hctx.fillStyle = haloGrad;
            hctx.fillRect(0, 0, 128, 128);
            var haloTex = new THREE.CanvasTexture(haloCanvas);
            var haloMat = new THREE.SpriteMaterial({
                map: haloTex,
                transparent: true,
                depthWrite: false,
                blending: THREE.AdditiveBlending
            });
            var halo = new THREE.Sprite(haloMat);
            halo.scale.set(1.2, 1.2, 1.2);
            halo.position.copy(position);
            group.add(halo);
            sensor.userData.halo = halo;
        }

        addSensorMarker(new THREE.Vector3(mainSpan / 2, parabolaY(mainSpan / 2) + 0.6, 0), 'ARCH', 'M1-Crown');
        addSensorMarker(new THREE.Vector3(mainSpan * 0.25, parabolaY(mainSpan * 0.25) + 0.6, deckWidth * 0.25), 'ARCH', 'M2-QuarterL');
        addSensorMarker(new THREE.Vector3(mainSpan * 0.75, parabolaY(mainSpan * 0.75) + 0.6, -deckWidth * 0.25), 'ARCH', 'M3-QuarterR');
        addSensorMarker(new THREE.Vector3(mainSpan * 0.1, parabolaY(mainSpan * 0.1) + 0.6, 0), 'ARCH', 'M4-Left');

        addSensorMarker(new THREE.Vector3(-1 - abutmentWidth / 2 + 0.5, abutmentHeight + 0.3, 0), 'PIER', 'P1-AbutmentL');
        addSensorMarker(new THREE.Vector3(mainSpan + 1 + abutmentWidth / 2 - 0.5, abutmentHeight + 0.3, 0), 'PIER', 'P2-AbutmentR');

        smallArches.forEach(function(sa, i) {
            var sax = sa.xCenter;
            var say = parabolaY(sax) + sa.rise + 0.4;
            addSensorMarker(new THREE.Vector3(sax, say, (i % 2 === 0 ? 1 : -1) * deckWidth * 0.2), 'SARCH', 'S' + (i + 1) + '-SmallArch');
        });

        addSensorMarker(new THREE.Vector3(mainSpan * 0.35, parabolaY(mainSpan * 0.35) + 1.5, deckWidth * 0.3), 'CRACK', 'C1-SpandrelL');
        addSensorMarker(new THREE.Vector3(mainSpan * 0.65, parabolaY(mainSpan * 0.65) + 1.5, -deckWidth * 0.3), 'CRACK', 'C2-SpandrelR');

        var crackPositions = [
            { x: mainSpan * 0.15, angle: -0.3, z: 0.1 },
            { x: mainSpan * 0.85, angle: 0.3, z: -0.1 },
            { x: mainSpan * 0.3, angle: -0.15, z: deckWidth * 0.2 },
            { x: mainSpan * 0.7, angle: 0.15, z: -deckWidth * 0.2 }
        ];

        crackPositions.forEach(function(cp) {
            var crackGeo = new THREE.PlaneGeometry(1.8, 0.08);
            var crackMat = new THREE.MeshBasicMaterial({
                color: 0xff2020,
                transparent: true,
                opacity: 0.9,
                side: THREE.DoubleSide,
                depthWrite: false
            });
            var crack = new THREE.Mesh(crackGeo, crackMat);
            var cy = parabolaY(cp.x);
            crack.position.set(cp.x, cy + 0.55, cp.z);
            crack.rotation.z = cp.angle;
            crack.rotation.y = cp.z !== 0 ? (cp.z > 0 ? -0.3 : 0.3) : 0;
            crack.visible = false;
            crack.scale.set(0.01, 0.01, 0.01);
            group.add(crack);
            crackMarkers.push(crack);
        });

        return group;
    }

    function setupGroundAndEnvironment(bridgeGroup) {
        var gridHelper = new THREE.GridHelper(200, 100, 0x555555, 0x333333);
        gridHelper.position.y = 0.001;
        gridHelper.material.opacity = 0.15;
        gridHelper.material.transparent = true;
        scene.add(gridHelper);

        scene.fog.color = new THREE.Color(0x1a2530);
    }

    function updateStress(femStressResults, femElements, femNodes) {
        var startTime = performance.now();

        while (stressOverlayGroup.children.length > 0) {
            var obj = stressOverlayGroup.children[0];
            stressOverlayGroup.remove(obj);
            if (obj.geometry) obj.geometry.dispose();
            if (obj.material) {
                if (obj.material.map) obj.material.map.dispose();
                obj.material.dispose();
            }
        }

        if (!femStressResults || !femElements || !femNodes || femElements.length === 0) return;

        var getTier = window.getGpuTier || function() { return 1; };
        var tier = getTier();
        var origCount = femElements.length;

        var normFn = window.normalizeElements || function(e) { return e; };
        var normElements = normFn(femElements);
        var normNodes = femNodes;

        var stressValues = [];
        if (femStressResults && femStressResults.length > 0) {
            var first = femStressResults[0];
            if (typeof first === 'object' && first !== null) {
                for (var si = 0; si < femStressResults.length; si++) {
                    var r = femStressResults[si];
                    stressValues.push(r.von_mises !== undefined ? r.von_mises : (r.vonMises !== undefined ? r.vonMises : 0));
                }
            } else {
                for (var sj = 0; sj < femStressResults.length; sj++) {
                    stressValues.push(femStressResults[sj]);
                }
            }
        }

        var minVal = Infinity, maxVal = -Infinity;
        for (var mi = 0; mi < stressValues.length; mi++) {
            var sv = stressValues[mi];
            if (sv < minVal) minVal = sv;
            if (sv > maxVal) maxVal = sv;
        }
        if (maxVal === minVal) maxVal = minVal + 1;

        stressData = { min: minVal, max: maxVal, values: stressValues };

        var abutH = 4.0;
        var buildMerged = window.buildMergedStressGeometry;
        var buildCanvas = window.buildStressCanvasTexture;
        var decimate = window.decimateElementsForMobile;
        var computeCentroid = window.computeElementCentroid || function(el, nds) {
            var n0 = nds[el[0]] || {x:0,y:0,z:0};
            var n1 = nds[el[1]] || {x:0,y:0,z:0};
            var n2 = nds[el[2]] || {x:0,y:0,z:0};
            return { x:(n0.x+n1.x+n2.x)/3, y:(n0.y+n1.y+n2.y)/3, z:((n0.z||0)+(n1.z||0)+(n2.z||0))/3 };
        };
        var createSprite = window.createTextSprite;

        if (tier === 0) {
            var decimated = { newElems: normElements, newNodes: normNodes, newStress: stressValues };
            if (decimate && normElements.length > 60) {
                decimated = decimate(normElements, normNodes, stressValues, 60);
            }
            var workElems = decimated.newElems;
            var workNodes = decimated.newNodes;
            var workStress = decimated.newStress;

            if (buildCanvas) {
                var texResult = buildCanvas(workElems, workNodes, workStress, minVal, maxVal, 512);
                stressOverlayGroup.add(texResult.mesh);
            } else if (buildMerged) {
                var mergedGeoT0 = buildMerged(workElems, workNodes, workStress, minVal, maxVal);
                var mergedMatT0 = new THREE.MeshBasicMaterial({
                    vertexColors: true,
                    transparent: true,
                    opacity: 0.6,
                    side: THREE.DoubleSide,
                    polygonOffset: true,
                    polygonOffsetFactor: -1,
                    polygonOffsetUnits: -1,
                    depthWrite: false
                });
                var mergedMeshT0 = new THREE.Mesh(mergedGeoT0, mergedMatT0);
                mergedMeshT0.rotation.x = -Math.PI / 2;
                stressOverlayGroup.add(mergedMeshT0);
            } else {
                fallbackRender(normElements, normNodes, stressValues, minVal, maxVal, abutH);
            }
        } else if (tier === 1) {
            var labelPct = 0.1;
            if (buildMerged) {
                var mergedGeoT1 = buildMerged(normElements, normNodes, stressValues, minVal, maxVal);
                var mergedMatT1 = new THREE.MeshBasicMaterial({
                    vertexColors: true,
                    transparent: true,
                    opacity: 0.6,
                    side: THREE.DoubleSide,
                    polygonOffset: true,
                    polygonOffsetFactor: -1,
                    polygonOffsetUnits: -1,
                    depthWrite: false
                });
                var mergedMeshT1 = new THREE.Mesh(mergedGeoT1, mergedMatT1);
                mergedMeshT1.rotation.x = -Math.PI / 2;
                stressOverlayGroup.add(mergedMeshT1);
            } else {
                fallbackRender(normElements, normNodes, stressValues, minVal, maxVal, abutH);
            }

            if (createSprite) {
                var sortedIdxT1 = [];
                for (var s1 = 0; s1 < stressValues.length; s1++) {
                    sortedIdxT1.push({ idx: s1, stress: stressValues[s1] });
                }
                sortedIdxT1.sort(function(a, b) { return b.stress - a.stress; });
                var labelCountT1 = Math.min(10, Math.ceil(sortedIdxT1.length * labelPct));
                for (var l1 = 0; l1 < labelCountT1; l1++) {
                    var entry1 = sortedIdxT1[l1];
                    var el1 = normElements[entry1.idx];
                    if (!el1) continue;
                    var cent1 = computeCentroid(el1, normNodes);
                    var txt1 = (entry1.stress / 1e6).toFixed(2) + 'MPa';
                    var sp1 = createSprite(txt1, 0xffffff, 24);
                    sp1.position.set(cent1.x, cent1.y + abutH + 0.5, 0.2);
                    sp1.scale.set(2.0, 1.0, 1.0);
                    stressOverlayGroup.add(sp1);
                }
            }
        } else {
            var labelPctHi = 0.2;
            if (buildMerged) {
                var mergedGeoT2 = buildMerged(normElements, normNodes, stressValues, minVal, maxVal);
                var mergedMatT2 = new THREE.MeshBasicMaterial({
                    vertexColors: true,
                    transparent: true,
                    opacity: 0.6,
                    side: THREE.DoubleSide,
                    polygonOffset: true,
                    polygonOffsetFactor: -1,
                    polygonOffsetUnits: -1,
                    depthWrite: false
                });
                var mergedMeshT2 = new THREE.Mesh(mergedGeoT2, mergedMatT2);
                mergedMeshT2.rotation.x = -Math.PI / 2;
                stressOverlayGroup.add(mergedMeshT2);
            } else {
                fallbackRender(normElements, normNodes, stressValues, minVal, maxVal, abutH);
            }

            if (createSprite) {
                var sortedIdxT2 = [];
                for (var s2 = 0; s2 < stressValues.length; s2++) {
                    sortedIdxT2.push({ idx: s2, stress: stressValues[s2] });
                }
                sortedIdxT2.sort(function(a, b) { return b.stress - a.stress; });
                var topCountT2 = Math.ceil(sortedIdxT2.length * labelPctHi);
                for (var l2 = 0; l2 < topCountT2; l2++) {
                    var entry2 = sortedIdxT2[l2];
                    var el2 = normElements[entry2.idx];
                    if (!el2) continue;
                    var cent2 = computeCentroid(el2, normNodes);
                    var txt2 = (entry2.stress / 1e6).toFixed(2) + 'MPa';
                    var sp2 = createSprite(txt2, 0xffffff, 24);
                    sp2.position.set(cent2.x, cent2.y + abutH + 0.5, 0.2);
                    sp2.scale.set(2.0, 1.0, 1.0);
                    stressOverlayGroup.add(sp2);
                }
            }
        }

        var legendParent = document.querySelector('.monitoring-panel, .dashboard-container, body');
        if (legendParent && !document.getElementById('stress-legend')) {
            if (window.createStressLegend) {
                window.createStressLegend(legendParent);
            }
        }
        if (document.getElementById('stress-legend') && window.updateStressLegend) {
            window.updateStressLegend(minVal, maxVal);
        }

        stressOverlayGroup.visible = stressVisible;

        var elapsed = performance.now() - startTime;
        console.log('[StressMap] tier=' + tier + ', elements=' + origCount + ', draw_calls=1, ms=' + elapsed.toFixed(2));

        var groups = 8;
        var newVals = [];
        for (var gi = 0; gi < groups; gi++) {
            var start = Math.floor(gi * stressValues.length / groups);
            var end = Math.floor((gi + 1) * stressValues.length / groups);
            var sum = 0, cnt = 0;
            for (var j = start; j < end && j < stressValues.length; j++) {
                sum += (stressValues[j] || 0) / 1e6;
                cnt++;
            }
            newVals.push(cnt ? sum / cnt : 0);
        }
        if (callbacks.onStressUpdated && typeof callbacks.onStressUpdated === 'function') {
            callbacks.onStressUpdated(newVals);
        }
    }

    function fallbackRender(femElements, femNodes, stressValues, minStress, maxStress, abutH) {
        var colorFn = window.stressColorMap || function() { return new THREE.Color(0xffffff); };
        for (var ei = 0; ei < femElements.length; ei++) {
            var elem = femElements[ei];
            var stress = stressValues[ei] !== undefined ? stressValues[ei] : 0;
            var n0 = femNodes[elem[0]];
            var n1 = femNodes[elem[1]];
            var n2 = femNodes[elem[2]];
            if (!n0 || !n1 || !n2) continue;

            var shape = new THREE.Shape();
            var zOffset = 0.05;

            var sx0 = n0.x, sy0 = (n0.y || 0) + abutH;
            var sx1 = n1.x, sy1 = (n1.y || 0) + abutH;
            var sx2 = n2.x, sy2 = (n2.y || 0) + abutH;

            shape.moveTo(sx0, sy0);
            shape.lineTo(sx1, sy1);
            shape.lineTo(sx2, sy2);
            shape.lineTo(sx0, sy0);

            var shapeGeo = new THREE.ShapeGeometry(shape);
            var color = colorFn(stress, minStress, maxStress);
            var meshMat = new THREE.MeshBasicMaterial({
                color: color,
                transparent: true,
                opacity: 0.5,
                side: THREE.DoubleSide,
                depthWrite: false,
                polygonOffset: true,
                polygonOffsetFactor: -1,
                polygonOffsetUnits: -1
            });

            var posAttr = shapeGeo.attributes.position;
            for (var vi = 0; vi < posAttr.count; vi++) {
                posAttr.setZ(vi, zOffset);
            }
            posAttr.needsUpdate = true;

            var mesh = new THREE.Mesh(shapeGeo, meshMat);
            mesh.rotation.x = -Math.PI / 2;
            stressOverlayGroup.add(mesh);
        }
    }

    function setStressVisible(visible) {
        stressVisible = visible;
        if (stressOverlayGroup) {
            stressOverlayGroup.visible = visible;
        }
    }

    function setCrackVisible(visible) {
        cracksVisible = visible;
        var duration = 600;
        var startTime = performance.now();

        function animateCracks(now) {
            var elapsed = now - startTime;
            var t = Math.min(elapsed / duration, 1);
            var easeT = visible ? (1 - Math.pow(1 - t, 3)) : (1 - Math.pow(t, 3));
            var scale = visible ? easeT : (1 - easeT);

            crackMarkers.forEach(function(crack) {
                crack.visible = visible || scale > 0.01;
                crack.scale.set(visible ? Math.max(0.01, scale) : scale, visible ? Math.max(0.01, scale) : scale, Math.max(0.01, scale));
            });

            if (t < 1) {
                requestAnimationFrame(animateCracks);
            } else {
                crackMarkers.forEach(function(crack) {
                    crack.visible = visible;
                    if (visible) crack.scale.set(1, 1, 1);
                });
            }
        }
        requestAnimationFrame(animateCracks);
    }

    function predict50Years() {
        if (predictions.length === 0 || !femNodes.length) {
            var pr = fetchJSON(apiBase + '/api/deformation/predict50', { method: 'POST' });
            pr.then(function(prRes) {
                if (prRes && Array.isArray(prRes)) {
                    predictions = prRes.map(function(p) {
                        return {
                            dx: p.predicted_dx || 0,
                            dy: p.predicted_dy || 0,
                            node_id: p.node_id
                        };
                    });
                }
                animateDeformation50Years(predictions, femNodes);
                if (callbacks.onPredictionsUpdated && typeof callbacks.onPredictionsUpdated === 'function') {
                    callbacks.onPredictionsUpdated(predictions);
                }
            });
        } else {
            animateDeformation50Years(predictions, femNodes);
            if (callbacks.onPredictionsUpdated && typeof callbacks.onPredictionsUpdated === 'function') {
                callbacks.onPredictionsUpdated(predictions);
            }
        }
    }

    function animateDeformation50Years(deformationPredictions, nodes) {
        if (!bridgeGroup || !originalPositions || !nodes) return;

        var duration = 6000;
        var startTime = performance.now();
        var amplify = 500;
        var abutH = 4.0;

        var infoDiv = document.getElementById('deformation-info');
        if (!infoDiv) {
            infoDiv = document.createElement('div');
            infoDiv.id = 'deformation-info';
            infoDiv.style.cssText = 'position:absolute;top:10px;left:10px;background:rgba(0,0,0,0.75);color:#fff;padding:12px 18px;border-radius:6px;font-family:Arial;font-size:13px;z-index:20;pointer-events:none;';
            var container = containerEl || document.body;
            container.appendChild(infoDiv);
        }

        var mainArch = null;
        for (var bi = 0; bi < bridgeGroup.children.length; bi++) {
            var child = bridgeGroup.children[bi];
            if (child.geometry && child.geometry.type === 'TubeGeometry') {
                if (child.geometry.parameters && child.geometry.parameters.tubularSegments === 80) {
                    mainArch = child;
                    break;
                }
            }
        }
        if (!mainArch) {
            mainArch = bridgeGroup.children[0];
        }

        function animStep(now) {
            var elapsed = now - startTime;
            var progress = Math.min(elapsed / duration, 1);
            var easeProgress = 1 - Math.pow(1 - progress, 3);

            var geo = mainArch.geometry;
            var posAttr = geo.attributes.position;
            var positions = posAttr.array;

            for (var vi = 0; vi < posAttr.count; vi++) {
                var ox = originalPositions[vi * 3];
                var oy = originalPositions[vi * 3 + 1];
                var oz = originalPositions[vi * 3 + 2];

                var nodeY = oy - abutH;
                var interp = window.interpolateNodeDisplacements(ox, nodeY, nodes, deformationPredictions);
                var dx = interp.dx * amplify * easeProgress;
                var dy = interp.dy * amplify * easeProgress;

                positions[vi * 3] = ox + dx;
                positions[vi * 3 + 1] = oy + dy;
                positions[vi * 3 + 2] = oz;
            }
            posAttr.needsUpdate = true;
            geo.computeVertexNormals();

            var maxDisp = 0;
            if (deformationPredictions && deformationPredictions.length > 0) {
                for (var di = 0; di < deformationPredictions.length; di++) {
                    var pred = deformationPredictions[di];
                    var d = Math.sqrt(pred.dx * pred.dx + pred.dy * pred.dy);
                    if (d > maxDisp) maxDisp = d;
                }
            }
            var year = (progress * 50).toFixed(1);
            var dispMm = (maxDisp * 1000 * easeProgress).toFixed(3);
            infoDiv.innerHTML = '<strong>50-Year Deformation Simulation</strong><br>Year: ' + year + ' / 50<br>Max displacement: ' + dispMm + ' mm<br>Visual exaggeration: &times;' + amplify + '<br>Progress: ' + (progress * 100).toFixed(0) + '%';

            if (progress < 1) {
                requestAnimationFrame(animStep);
            } else {
                deformed = true;
                infoDiv.style.pointerEvents = 'auto';
                var resetBtn = document.createElement('button');
                resetBtn.textContent = 'Reset Deformation';
                resetBtn.style.cssText = 'display:block;margin-top:8px;padding:6px 12px;background:#2196F3;color:#fff;border:none;border-radius:4px;cursor:pointer;font-size:12px;';
                resetBtn.onclick = function() {
                    reset();
                    infoDiv.innerHTML = '<strong>50-Year Deformation Simulation</strong><br>Completed - View reset.';
                    setTimeout(function() { if (infoDiv.parentNode) infoDiv.parentNode.removeChild(infoDiv); }, 2500);
                };
                var existingBtn = infoDiv.querySelector('button');
                if (!existingBtn) infoDiv.appendChild(resetBtn);
            }
        }
        requestAnimationFrame(animStep);
    }

    function reset() {
        if (camera) {
            camera.position.set(50, 30, 60);
        }
        if (controls) {
            controls.target.set(18.5, 4, 0);
            controls.update();
        }

        if (deformed && originalPositions && bridgeGroup) {
            var mainArch = null;
            for (var bi = 0; bi < bridgeGroup.children.length; bi++) {
                var child = bridgeGroup.children[bi];
                if (child.geometry && child.geometry.type === 'TubeGeometry') {
                    if (child.geometry.parameters && child.geometry.parameters.tubularSegments === 80) {
                        mainArch = child;
                        break;
                    }
                }
            }
            if (!mainArch) mainArch = bridgeGroup.children[0];

            if (mainArch) {
                var geo = mainArch.geometry;
                var posAttr = geo.attributes.position;
                var positions = posAttr.array;
                for (var i = 0; i < originalPositions.length; i++) {
                    positions[i] = originalPositions[i];
                }
                posAttr.needsUpdate = true;
                geo.computeVertexNormals();
            }
        }

        deformed = false;

        var infoDiv = document.getElementById('deformation-info');
        if (infoDiv && infoDiv.parentNode) {
            infoDiv.parentNode.removeChild(infoDiv);
        }
    }

    function rotate(enabled) {
        autoRotate = enabled;
        if (controls) {
            controls.autoRotate = enabled;
            controls.autoRotateSpeed = 0.8;
        }
    }

    function animate() {
        requestAnimationFrame(animate);
        if (controls) controls.update();

        var time = Date.now() * 0.001;
        sensorMarkers.forEach(function(sensor, idx) {
            if (sensor.userData && sensor.userData.halo) {
                var pulse = 1 + 0.3 * Math.sin(time * 2 + idx * 0.7);
                sensor.userData.halo.scale.set(1.2 * pulse, 1.2 * pulse, 1.2 * pulse);
            }
        });

        if (renderer && scene && camera) {
            renderer.render(scene, camera);
        }
    }

    function onWindowResize() {
        var container = containerEl;
        if (!container || !camera || !renderer) return;
        camera.aspect = container.clientWidth / container.clientHeight;
        camera.updateProjectionMatrix();
        renderer.setSize(container.clientWidth, container.clientHeight);
    }

    function bindToolbarEvents() {
        var stressBtn = document.getElementById('btnStress') || document.getElementById('btn-stress-map');
        if (stressBtn) {
            stressBtn.addEventListener('click', async function() {
                stressBtn.classList.toggle('active');
                var active = stressBtn.classList.contains('active');
                setStressVisible(active);
                var res = await fetchJSON(apiBase + '/api/fem/analyze', {
                    method: 'POST',
                    body: JSON.stringify({ live_load: 10000, delta_t: 0 })
                });
                if (res && Array.isArray(res)) {
                    femStress = res.map(function(r) { return r.von_mises || 0; });
                    var elemTriples = femElements.map(function(e) {
                        return Array.isArray(e.node_ids) ? e.node_ids : [e[0], e[1], e[2]];
                    });
                    updateStress(femStress, elemTriples, femNodes);
                }
            });
        }

        var crackBtn = document.getElementById('btnCrack') || document.getElementById('btn-crack-markers');
        if (crackBtn) {
            crackBtn.addEventListener('click', function() {
                crackBtn.classList.toggle('active');
                var active = crackBtn.classList.contains('active');
                setCrackVisible(active);
            });
        }

        var resetBtn = document.getElementById('btnReset') || document.getElementById('btn-reset-view');
        if (resetBtn) {
            resetBtn.addEventListener('click', function() {
                reset();
            });
        }

        var rotBtn = document.getElementById('btnRotate') || document.getElementById('btn-auto-rotate');
        if (rotBtn) {
            rotBtn.addEventListener('click', function() {
                rotBtn.classList.toggle('active');
                var active = rotBtn.classList.contains('active');
                rotate(active);
            });
        }

        var predBtn = document.getElementById('btnPredict') || document.getElementById('btn-predict-50');
        if (predBtn) {
            predBtn.addEventListener('click', function() {
                predBtn.classList.toggle('active');
                predict50Years();
            });
        }
    }

    function getState() {
        return {
            stressVisible: stressVisible,
            cracksVisible: cracksVisible,
            autoRotate: autoRotate,
            deformed: deformed,
            femNodes: femNodes.slice(),
            femElements: femElements.slice(),
            femStress: femStress.slice(),
            predictions: predictions.slice(),
            stressData: stressData
        };
    }

    window.initBridgeScene = function(containerId) { return init(containerId); };
    window.updateStressMap = function(a, b, c) { return updateStress(a, b, c); };
    window.setStressMapVisible = function(v) { return setStressVisible(v); };
    window.setCrackMarkersVisible = function(v) { return setCrackVisible(v); };
    window.animateDeformation50Years = function(a, b) { return animateDeformation50Years(a, b); };
    window.resetView = function() { return reset(); };
    window.setAutoRotate = function(v) { return rotate(v); };
    window.onWindowResize = function() { return onWindowResize(); };

    return {
        init: init,
        loadConfig: loadConfig,
        loadGeometry: loadGeometry,
        updateStress: updateStress,
        setStressVisible: setStressVisible,
        setCrackVisible: setCrackVisible,
        reset: reset,
        rotate: rotate,
        predict50Years: predict50Years,
        getState: getState,
        set onStressUpdated(fn) { callbacks.onStressUpdated = fn; },
        get onStressUpdated() { return callbacks.onStressUpdated; },
        set onPredictionsUpdated(fn) { callbacks.onPredictionsUpdated = fn; },
        get onPredictionsUpdated() { return callbacks.onPredictionsUpdated; }
    };
})();
