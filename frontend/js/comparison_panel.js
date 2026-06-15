window.ComparisonPanel = (function() {
    var apiBase = window.location.origin || 'http://localhost:8080';
    var spandrelChart = null;
    var materialChart = null;
    var spandrelData = null;
    var materialData = null;

    function fetchJSON(url, opts) {
        return fetch(url, {
            headers: { 'Content-Type': 'application/json' },
            ...opts
        }).then(function(r) { return r.json(); });
    }

    function formatMPa(pa) {
        return (pa / 1e6).toFixed(2);
    }

    function formatMM(m) {
        return (m * 1000).toFixed(3);
    }

    function pctStr(val) {
        return val.toFixed(1) + '%';
    }

    function loadSpandrelComparison() {
        var container = document.getElementById('spandrelResult');
        if (!container) return;
        container.innerHTML = '<div class="comparison-loading">正在计算敞肩拱与实肩拱对比...</div>';

        fetchJSON(apiBase + '/api/comparison/spandrel', { method: 'POST', body: '{}' })
            .then(function(data) {
                spandrelData = data;
                renderSpandrelResult(data, container);
            })
            .catch(function(e) {
                container.innerHTML = '<div class="comparison-error">计算失败: ' + e.message + '</div>';
            });
    }

    function renderSpandrelResult(data, container) {
        var open = data.open_spandrel;
        var solid = data.solid_spandrel;
        var summary = data.summary;

        var html = '<div class="comparison-grid">';
        html += '<div class="comparison-card open">';
        html += '<h4>敞肩拱 (赵州桥原设计)</h4>';
        html += '<div class="comparison-badge">李春 · 隋代</div>';
        html += buildCaseTable(open);
        html += '</div>';

        html += '<div class="comparison-card solid">';
        html += '<h4>实肩拱 (对比方案)</h4>';
        html += '<div class="comparison-badge">传统设计</div>';
        html += buildCaseTable(solid);
        html += '</div>';
        html += '</div>';

        html += '<div class="comparison-summary">';
        html += '<h4>对比分析结论</h4>';
        html += '<div class="summary-grid">';
        html += '<div class="summary-item"><span class="summary-label">最大应力比</span><span class="summary-value">' + summary.stress_ratio.toFixed(3) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">最大位移比</span><span class="summary-value">' + summary.displacement_ratio.toFixed(3) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">自重减轻</span><span class="summary-value highlight">' + pctStr(summary.mass_reduction_pct) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">应力增加</span><span class="summary-value">' + pctStr(summary.von_mises_reduction_pct < 0 ? -summary.von_mises_reduction_pct : 0) + '</span></div>';
        html += '</div>';
        html += '<p class="summary-verdict">' + summary.weight_advantage + '</p>';
        html += '</div>';

        html += '<div class="chart-wrapper" style="height:280px;margin-top:16px;"><canvas id="spandrelBarChart"></canvas></div>';

        container.innerHTML = html;
        renderSpandrelChart(data);
    }

    function buildCaseTable(c) {
        var html = '<table class="comparison-table">';
        html += '<tr><td>最大von Mises应力</td><td>' + formatMPa(c.max_von_mises) + ' MPa</td></tr>';
        html += '<tr><td>最大位移</td><td>' + formatMM(c.max_displacement) + ' mm</td></tr>';
        html += '<tr><td>结构总质量</td><td>' + (c.mass_kg / 1000).toFixed(1) + ' t</td></tr>';
        html += '<tr><td>弹性模量</td><td>' + formatMPa(c.material.elastic_modulus) + ' MPa</td></tr>';
        html += '<tr><td>抗压强度</td><td>' + formatMPa(c.material.compressive_strength) + ' MPa</td></tr>';
        html += '</table>';
        return html;
    }

    function renderSpandrelChart(data) {
        var ctx = document.getElementById('spandrelBarChart');
        if (!ctx) return;
        if (spandrelChart) spandrelChart.destroy();

        spandrelChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['最大应力(MPa)', '最大位移(mm)', '总质量(t)'],
                datasets: [
                    {
                        label: '敞肩拱',
                        data: [
                            data.open_spandrel.max_von_mises / 1e6,
                            data.open_spandrel.max_displacement * 1000,
                            data.open_spandrel.mass_kg / 1000
                        ],
                        backgroundColor: 'rgba(52, 152, 219, 0.7)',
                        borderColor: 'rgba(52, 152, 219, 1)',
                        borderWidth: 1
                    },
                    {
                        label: '实肩拱',
                        data: [
                            data.solid_spandrel.max_von_mises / 1e6,
                            data.solid_spandrel.max_displacement * 1000,
                            data.solid_spandrel.mass_kg / 1000
                        ],
                        backgroundColor: 'rgba(231, 76, 60, 0.7)',
                        borderColor: 'rgba(231, 76, 60, 1)',
                        borderWidth: 1
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    title: { display: true, text: '敞肩拱 vs 实肩拱 性能对比', color: '#e0e0e0' },
                    legend: { labels: { color: '#ccc' } }
                },
                scales: {
                    x: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } },
                    y: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } }
                }
            }
        });
    }

    function loadMaterialComparison() {
        var container = document.getElementById('materialResult');
        if (!container) return;
        container.innerHTML = '<div class="comparison-loading">正在计算古石与现代RC对比...</div>';

        fetchJSON(apiBase + '/api/comparison/materials', { method: 'POST', body: '{}' })
            .then(function(data) {
                materialData = data;
                renderMaterialResult(data, container);
            })
            .catch(function(e) {
                container.innerHTML = '<div class="comparison-error">计算失败: ' + e.message + '</div>';
            });
    }

    function renderMaterialResult(data, container) {
        var ancient = data.ancient_stone;
        var modern = data.modern_rc;
        var summary = data.summary;

        var html = '<div class="comparison-grid">';
        html += '<div class="comparison-card ancient">';
        html += '<h4>古代青砂岩 (赵州桥)</h4>';
        html += '<div class="comparison-badge">隋代 · 公元605年</div>';
        html += buildCaseTable(ancient);
        html += '</div>';

        html += '<div class="comparison-card modern">';
        html += '<h4>现代钢筋混凝土</h4>';
        html += '<div class="comparison-badge">C35混凝土</div>';
        html += buildCaseTable(modern);
        html += '</div>';
        html += '</div>';

        html += '<div class="comparison-summary">';
        html += '<h4>跨时代材料对比</h4>';
        html += '<div class="summary-grid">';
        html += '<div class="summary-item"><span class="summary-label">刚度比(E_rc/E_stone)</span><span class="summary-value">' + summary.stiffness_ratio.toFixed(1) + 'x</span></div>';
        html += '<div class="summary-item"><span class="summary-label">强度比(f_c_rc/f_c_stone)</span><span class="summary-value">' + summary.strength_ratio.toFixed(1) + 'x</span></div>';
        html += '<div class="summary-item"><span class="summary-label">应力降低</span><span class="summary-value highlight">' + pctStr(summary.max_stress_reduction_pct) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">位移降低</span><span class="summary-value highlight">' + pctStr(summary.max_disp_reduction_pct) + '</span></div>';
        html += '</div>';
        html += '<p class="summary-verdict">' + summary.verdict + '</p>';
        html += '</div>';

        html += '<div class="chart-wrapper" style="height:280px;margin-top:16px;"><canvas id="materialBarChart"></canvas></div>';

        container.innerHTML = html;
        renderMaterialChart(data);
    }

    function renderMaterialChart(data) {
        var ctx = document.getElementById('materialBarChart');
        if (!ctx) return;
        if (materialChart) materialChart.destroy();

        materialChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['弹性模量(GPa)', '抗压强度(MPa)', '最大应力(MPa)', '最大位移(mm)'],
                datasets: [
                    {
                        label: '古代青砂岩',
                        data: [
                            data.ancient_stone.material.elastic_modulus / 1e9,
                            data.ancient_stone.material.compressive_strength / 1e6,
                            data.ancient_stone.max_von_mises / 1e6,
                            data.ancient_stone.max_displacement * 1000
                        ],
                        backgroundColor: 'rgba(230, 126, 34, 0.7)',
                        borderColor: 'rgba(230, 126, 34, 1)',
                        borderWidth: 1
                    },
                    {
                        label: '现代钢筋混凝土',
                        data: [
                            data.modern_rc.material.elastic_modulus / 1e9,
                            data.modern_rc.material.compressive_strength / 1e6,
                            data.modern_rc.max_von_mises / 1e6,
                            data.modern_rc.max_displacement * 1000
                        ],
                        backgroundColor: 'rgba(46, 204, 113, 0.7)',
                        borderColor: 'rgba(46, 204, 113, 1)',
                        borderWidth: 1
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    title: { display: true, text: '古代石拱桥 vs 现代钢筋混凝土拱桥', color: '#e0e0e0' },
                    legend: { labels: { color: '#ccc' } }
                },
                scales: {
                    x: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } },
                    y: { ticks: { color: '#aaa' }, grid: { color: 'rgba(255,255,255,0.05)' } }
                }
            }
        });
    }

    function init() {
        var btnSpandrel = document.getElementById('btnSpandrelComp');
        if (btnSpandrel) {
            btnSpandrel.addEventListener('click', loadSpandrelComparison);
        }
        var btnMaterial = document.getElementById('btnMaterialComp');
        if (btnMaterial) {
            btnMaterial.addEventListener('click', loadMaterialComparison);
        }
    }

    return {
        init: init,
        loadSpandrelComparison: loadSpandrelComparison,
        loadMaterialComparison: loadMaterialComparison
    };
})();
