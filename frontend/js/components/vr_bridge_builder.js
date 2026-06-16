window.VRBridgeBuilder = (function() {
    var apiBase = window.location.origin || 'http://localhost:8080';
    var designCanvas = null;
    var designCtx = null;
    var currentDesign = null;
    var resultChart = null;
    var animFrame = null;
    var testAnimProgress = 0;
    var testAnimRunning = false;

    var presets = {
        zhaozhou: { span_m: 37.02, rise_m: 7.23, arch_shape: 'parabolic', num_small_arches: 4, arch_ring_thickness_m: 1.0, material_preset: 'ancient_stone', live_load_kpa: 4 },
        modern_rc: { span_m: 50, rise_m: 10, arch_shape: 'parabolic', num_small_arches: 2, arch_ring_thickness_m: 1.5, material_preset: 'modern_rc', live_load_kpa: 10 },
        roman: { span_m: 25, rise_m: 12.5, arch_shape: 'circular', num_small_arches: 0, arch_ring_thickness_m: 1.8, material_preset: 'ancient_stone', live_load_kpa: 3 },
        steel: { span_m: 100, rise_m: 20, arch_shape: 'parabolic', num_small_arches: 3, arch_ring_thickness_m: 0.5, material_preset: 'steel', live_load_kpa: 20 }
    };

    function fetchJSON(url, opts) {
        return fetch(url, {
            headers: { 'Content-Type': 'application/json' },
            ...opts
        }).then(function(r) { return r.json(); });
    }

    function getDesignFromForm() {
        return {
            span_m: parseFloat(document.getElementById('vbSpan').value) || 37,
            rise_m: parseFloat(document.getElementById('vbRise').value) || 7,
            arch_shape: document.getElementById('vbShape').value || 'parabolic',
            num_small_arches: parseInt(document.getElementById('vbSmallArches').value, 10) || 0,
            arch_ring_thickness_m: parseFloat(document.getElementById('vbThickness').value) || 1.0,
            material_preset: document.getElementById('vbMaterial').value || 'ancient_stone',
            live_load_kpa: parseFloat(document.getElementById('vbLoad').value) || 4
        };
    }

    function applyPreset(name) {
        var p = presets[name];
        if (!p) return;
        document.getElementById('vbSpan').value = p.span_m;
        document.getElementById('vbRise').value = p.rise_m;
        document.getElementById('vbShape').value = p.arch_shape;
        document.getElementById('vbSmallArches').value = p.num_small_arches;
        document.getElementById('vbThickness').value = p.arch_ring_thickness_m;
        document.getElementById('vbMaterial').value = p.material_preset;
        document.getElementById('vbLoad').value = p.live_load_kpa;
        drawPreview();
    }

    function archY(x, span, rise, shape) {
        var t = x / span;
        if (shape === 'circular') {
            var R = (4 * rise * rise + span * span) / (8 * rise);
            var cx = span / 2;
            var cy = rise - R;
            var dx = x - cx;
            var ySq = R * R - dx * dx;
            if (ySq < 0) ySq = 0;
            return cy + Math.sqrt(ySq);
        } else if (shape === 'catenary') {
            var a = rise;
            var half = span / 2;
            return a * (Math.cosh((x - half) / a) - 1) / (Math.cosh(half / a) - 1);
        }
        return 4 * rise * t * (1 - t);
    }

    function drawPreview() {
        designCanvas = document.getElementById('vbCanvas');
        if (!designCanvas) return;
        designCtx = designCanvas.getContext('2d');

        var d = getDesignFromForm();
        currentDesign = d;

        var W = designCanvas.width;
        var H = designCanvas.height;
        var ctx = designCtx;

        ctx.clearRect(0, 0, W, H);

        var margin = 40;
        var drawW = W - 2 * margin;
        var drawH = H - 2 * margin;
        var scaleX = drawW / d.span_m;
        var maxH = d.rise_m * 1.8;
        var scaleY = drawH / maxH;

        function toScreen(x, y) {
            return [margin + x * scaleX, H - margin - y * scaleY];
        }

        ctx.strokeStyle = 'rgba(255,255,255,0.1)';
        ctx.lineWidth = 0.5;
        for (var gy = 0; gy <= maxH; gy += 5) {
            var p = toScreen(0, gy);
            ctx.beginPath();
            ctx.moveTo(margin, p[1]);
            ctx.lineTo(W - margin, p[1]);
            ctx.stroke();
        }

        ctx.strokeStyle = '#5a3a1a';
        ctx.lineWidth = 4;
        ctx.beginPath();
        var firstPt = toScreen(0, 0);
        ctx.moveTo(firstPt[0], firstPt[1]);
        for (var px = 0; px <= d.span_m; px += d.span_m / 200) {
            var y = archY(px, d.span_m, d.rise_m, d.arch_shape);
            var pt = toScreen(px, y);
            ctx.lineTo(pt[0], pt[1]);
        }
        var lastPt = toScreen(d.span_m, 0);
        ctx.lineTo(lastPt[0], lastPt[1]);
        ctx.stroke();

        var thickPx = d.arch_ring_thickness_m * scaleY;
        ctx.strokeStyle = '#8B6914';
        ctx.lineWidth = Math.max(2, thickPx);
        ctx.beginPath();
        for (var px2 = 0; px2 <= d.span_m; px2 += d.span_m / 200) {
            var y2 = archY(px2, d.span_m, d.rise_m, d.arch_shape);
            var pt2 = toScreen(px2, y2);
            if (px2 === 0) ctx.moveTo(pt2[0], pt2[1]);
            else ctx.lineTo(pt2[0], pt2[1]);
        }
        ctx.stroke();

        if (d.num_small_arches > 0) {
            var numPerSide = Math.ceil(d.num_small_arches / 2);
            for (var side = 0; side < 2; side++) {
                for (var si = 0; si < numPerSide; si++) {
                    if (side * numPerSide + si >= d.num_small_arches) break;
                    var baseX = side === 0 ? d.span_m * 0.05 + si * d.span_m * 0.2 : d.span_m * 0.95 - (si + 1) * d.span_m * 0.2;
                    var smallSpan = d.span_m * (0.15 - si * 0.03);
                    var smallRise = d.rise_m * (0.25 - si * 0.05);
                    var archBaseY = archY(baseX, d.span_m, d.rise_m, d.arch_shape) * 0.7;

                    ctx.strokeStyle = '#7a5a2a';
                    ctx.lineWidth = 2;
                    ctx.beginPath();
                    for (var sp = 0; sp <= smallSpan; sp += smallSpan / 50) {
                        var sy = archBaseY + 4 * smallRise * (sp / smallSpan) * (1 - sp / smallSpan);
                        var spt = toScreen(baseX + sp, sy);
                        if (sp === 0) ctx.moveTo(spt[0], spt[1]);
                        else ctx.lineTo(spt[0], spt[1]);
                    }
                    ctx.stroke();
                }
            }
        }

        var abutH = d.rise_m * 0.3;
        ctx.fillStyle = '#4a3a2a';
        var lb = toScreen(-1, abutH);
        var lt = toScreen(-1, 0);
        ctx.fillRect(lt[0], lb[1], margin * 0.6, lt[1] - lb[1]);
        var rb = toScreen(d.span_m + 1, abutH);
        var rt = toScreen(d.span_m + 1, 0);
        ctx.fillRect(rt[0] - margin * 0.1, rb[1], margin * 0.6, rt[1] - rb[1]);

        ctx.fillStyle = '#e0e0e0';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('跨径: ' + d.span_m + 'm', W / 2, H - 8);
        var risePt = toScreen(d.span_m / 2, d.rise_m);
        ctx.fillText('矢高: ' + d.rise_m + 'm', risePt[0], risePt[1] - 12);
        ctx.fillText('矢跨比: 1/' + (d.span_m / d.rise_m).toFixed(1), W / 2, 16);

        var matNames = { ancient_stone: '青砂岩', modern_rc: '钢筋混凝土', steel: '钢材' };
        ctx.fillText(matNames[d.material_preset] || d.material_preset, W - margin - 40, 16);
    }

    function runTest() {
        var container = document.getElementById('vbTestResult');
        if (!container) return;

        var design = getDesignFromForm();
        container.innerHTML = '<div class="comparison-loading">正在仿真拱券承重...</div>';

        fetchJSON(apiBase + '/api/v2/vr-bridge/design', {
            method: 'POST',
            body: JSON.stringify(design)
        }).then(function(data) {
            renderTestResult(data, container);
            animateLoadTest(data);
        }).catch(function(e) {
            container.innerHTML = '<div class="comparison-error">仿真失败: ' + e.message + '</div>';
        });
    }

    function loadPresets() {
        return fetchJSON(apiBase + '/api/v2/vr-bridge/presets', { method: 'GET' });
    }

    function renderTestResult(data, container) {
        var report = data.report;
        var passClass = data.pass_check ? 'test-pass' : 'test-fail';
        var passText = data.pass_check ? '✓ 通过' : '✗ 不通过';

        var html = '<div class="vb-result-header ' + passClass + '">';
        html += '<span class="vb-result-status">' + passText + '</span>';
        html += '<span class="vb-result-sf">安全系数: ' + data.safety_factor.toFixed(2) + '</span>';
        html += '</div>';

        html += '<div class="summary-grid compact">';
        html += '<div class="summary-item"><span class="summary-label">最大应力</span><span class="summary-value">' + (data.max_von_mises / 1e6).toFixed(2) + ' MPa</span></div>';
        html += '<div class="summary-item"><span class="summary-label">最大位移</span><span class="summary-value">' + (data.max_displacement * 1000).toFixed(3) + ' mm</span></div>';
        html += '<div class="summary-item"><span class="summary-label">应力利用率</span><span class="summary-value">' + (report.stress_utilization * 100).toFixed(1) + '%</span></div>';
        html += '<div class="summary-item"><span class="summary-label">位移/跨径</span><span class="summary-value">1/' + Math.round(1 / report.disp_span_ratio) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">结构质量</span><span class="summary-value">' + (data.mass_kg / 1000).toFixed(1) + ' t</span></div>';
        html += '<div class="summary-item"><span class="summary-label">抗压强度</span><span class="summary-value">' + (data.material.compressive_strength / 1e6).toFixed(0) + ' MPa</span></div>';
        html += '</div>';

        html += '<div class="vb-checks">';
        html += '<div class="check-item ' + (report.stress_check ? 'check-pass' : 'check-fail') + '">';
        html += '强度验算: σ_max/f_c = ' + report.stress_utilization.toFixed(3) + (report.stress_check ? ' ≤ 0.667 ✓' : ' > 0.667 ✗');
        html += '</div>';
        html += '<div class="check-item ' + (report.displacement_check ? 'check-pass' : 'check-fail') + '">';
        html += '刚度验算: Δ/L = 1/' + Math.round(1 / report.disp_span_ratio) + (report.displacement_check ? ' ≥ 600 ✓' : ' < 600 ✗');
        html += '</div>';
        html += '</div>';

        html += '<div class="vb-recommendation">' + report.recommendation + '</div>';

        html += '<div class="chart-wrapper" style="height:220px;margin-top:12px;"><canvas id="vbResultChart"></canvas></div>';

        container.innerHTML = html;
        renderResultChart(data);
    }

    function renderResultChart(data) {
        var ctx = document.getElementById('vbResultChart');
        if (!ctx) return;
        if (resultChart) resultChart.destroy();

        var fc = data.material.compressive_strength;
        var utilization = data.max_von_mises / fc;

        resultChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['应力利用率', '安全系数', '位移/跨径(×1000)'],
                datasets: [{
                    label: '设计值',
                    data: [
                        utilization * 100,
                        data.safety_factor,
                        data.report.disp_span_ratio * 1000
                    ],
                    backgroundColor: [
                        utilization > 0.667 ? 'rgba(231,76,60,0.7)' : 'rgba(46,204,113,0.7)',
                        data.safety_factor < 1.5 ? 'rgba(231,76,60,0.7)' : 'rgba(46,204,113,0.7)',
                        data.report.disp_span_ratio > 1/600 ? 'rgba(231,76,60,0.7)' : 'rgba(46,204,113,0.7)'
                    ],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    title: { display: true, text: '设计验算结果', color: '#e0e0e0' },
                    legend: { display: false }
                },
                scales: {
                    x: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } },
                    y: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } }
                }
            }
        });
    }

    function animateLoadTest(data) {
        if (animFrame) cancelAnimationFrame(animFrame);
        testAnimProgress = 0;
        testAnimRunning = true;

        var maxDisp = data.max_displacement;
        var amplification = 200;

        function step() {
            if (!testAnimRunning) return;
            testAnimProgress = Math.min(1, testAnimProgress + 0.015);
            var eased = 1 - Math.pow(1 - testAnimProgress, 3);

            drawPreview();

            if (designCtx && currentDesign) {
                var ctx = designCtx;
                var d = currentDesign;
                var W = designCanvas.width;
                var H = designCanvas.height;
                var margin = 40;
                var drawW = W - 2 * margin;
                var drawH = H - 2 * margin;
                var scaleX = drawW / d.span_m;
                var maxH = d.rise_m * 1.8;
                var scaleY = drawH / maxH;

                function toScreen(x, y) {
                    return [margin + x * scaleX, H - margin - y * scaleY];
                }

                var loadKPa = data.design.live_load_kpa;
                ctx.fillStyle = 'rgba(231, 76, 60, ' + (0.15 + 0.35 * eased) + ')';
                for (var lx = d.span_m * 0.1; lx < d.span_m * 0.9; lx += d.span_m / 20) {
                    var ly = archY(lx, d.span_m, d.rise_m, d.arch_shape);
                    var base = toScreen(lx, ly);
                    var top = toScreen(lx, ly + loadKPa * 0.15 * eased);
                    ctx.fillRect(base[0] - 2, top[1], 4, base[1] - top[1]);
                    ctx.beginPath();
                    ctx.moveTo(base[0] - 5, top[1] + 3);
                    ctx.lineTo(base[0], top[1]);
                    ctx.lineTo(base[0] + 5, top[1] + 3);
                    ctx.strokeStyle = 'rgba(231, 76, 60, ' + (0.5 + 0.5 * eased) + ')';
                    ctx.lineWidth = 1;
                    ctx.stroke();
                }

                ctx.strokeStyle = 'rgba(255, 200, 50, ' + (0.3 + 0.5 * eased) + ')';
                ctx.lineWidth = 3;
                ctx.setLineDash([6, 4]);
                ctx.beginPath();
                for (var dx = 0; dx <= d.span_m; dx += d.span_m / 200) {
                    var origY = archY(dx, d.span_m, d.rise_m, d.arch_shape);
                    var dispFactor = Math.sin(Math.PI * dx / d.span_m);
                    var defY = origY - maxDisp * amplification * eased * dispFactor;
                    var dpt = toScreen(dx, defY);
                    if (dx === 0) ctx.moveTo(dpt[0], dpt[1]);
                    else ctx.lineTo(dpt[0], dpt[1]);
                }
                ctx.stroke();
                ctx.setLineDash([]);

                if (eased > 0.3) {
                    var midDefY = d.rise_m - maxDisp * amplification * eased;
                    var midPt = toScreen(d.span_m / 2, midDefY);
                    ctx.fillStyle = 'rgba(255, 200, 50, 0.9)';
                    ctx.font = 'bold 12px sans-serif';
                    ctx.textAlign = 'center';
                    ctx.fillText('Δ=' + (maxDisp * 1000).toFixed(2) + 'mm', midPt[0], midPt[1] - 8);
                }
            }

            if (testAnimProgress < 1) {
                animFrame = requestAnimationFrame(step);
            } else {
                testAnimRunning = false;
            }
        }
        animFrame = requestAnimationFrame(step);
    }

    function init() {
        designCanvas = document.getElementById('vbCanvas');
        if (designCanvas) {
            drawPreview();
            var inputs = ['vbSpan', 'vbRise', 'vbShape', 'vbSmallArches', 'vbThickness', 'vbMaterial', 'vbLoad'];
            inputs.forEach(function(id) {
                var el = document.getElementById(id);
                if (el) el.addEventListener('input', drawPreview);
            });
        }

        var btnTest = document.getElementById('vbRunTest');
        if (btnTest) btnTest.addEventListener('click', runTest);

        var presetBtns = document.querySelectorAll('.vb-preset-btn');
        presetBtns.forEach(function(btn) {
            btn.addEventListener('click', function() {
                applyPreset(btn.dataset.preset);
            });
        });
    }

    return {
        init: init,
        drawPreview: drawPreview,
        runTest: runTest,
        applyPreset: applyPreset,
        loadPresets: loadPresets
    };
})();
