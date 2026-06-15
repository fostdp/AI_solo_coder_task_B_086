window.ReinforcementPanel = (function() {
    var apiBase = window.location.origin || 'http://localhost:8080';
    var resultChart = null;
    var currentResult = null;

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

    function getSelectedZones() {
        var zones = [];
        var checks = document.querySelectorAll('.reinf-zone-check:checked');
        checks.forEach(function(c) { zones.push(c.value); });
        return zones;
    }

    function getLayerCount() {
        var el = document.getElementById('reinfLayers');
        return el ? parseInt(el.value, 10) : 2;
    }

    function simulate() {
        var container = document.getElementById('reinfResult');
        if (!container) return;

        var zones = getSelectedZones();
        if (zones.length === 0) {
            container.innerHTML = '<div class="comparison-error">请至少选择一个加固区域</div>';
            return;
        }

        var layers = getLayerCount();
        var configs = zones.map(function(z) {
            return {
                zone: z,
                layers: layers,
                thickness_mm: 0.167 * layers,
                width_m: 1.0
            };
        });

        container.innerHTML = '<div class="comparison-loading">正在仿真碳纤维布加固效果...</div>';

        fetchJSON(apiBase + '/api/reinforcement/simulate', {
            method: 'POST',
            body: JSON.stringify({
                configs: configs,
                live_load_pa: 4000,
                delta_t_c: 0
            })
        }).then(function(data) {
            currentResult = data;
            renderResult(data, container);
        }).catch(function(e) {
            container.innerHTML = '<div class="comparison-error">仿真失败: ' + e.message + '</div>';
        });
    }

    function renderResult(data, container) {
        var before = data.before;
        var after = data.after;
        var cfrp = data.cfrp_properties;
        var summary = data.summary;

        var html = '<div class="reinf-config-info">';
        html += '<h4>CFRP碳纤维布参数</h4>';
        html += '<div class="summary-grid compact">';
        html += '<div class="summary-item"><span class="summary-label">弹性模量</span><span class="summary-value">' + (cfrp.elastic_modulus_pa / 1e9).toFixed(0) + ' GPa</span></div>';
        html += '<div class="summary-item"><span class="summary-label">抗拉强度</span><span class="summary-value">' + (cfrp.tensile_strength_pa / 1e6).toFixed(0) + ' MPa</span></div>';
        html += '<div class="summary-item"><span class="summary-label">单层厚度</span><span class="summary-value">' + cfrp.thickness_per_layer_mm.toFixed(3) + ' mm</span></div>';
        html += '<div class="summary-item"><span class="summary-label">密度</span><span class="summary-value">' + cfrp.density_kgm3 + ' kg/m³</span></div>';
        html += '</div></div>';

        html += '<div class="comparison-grid">';
        html += '<div class="comparison-card before">';
        html += '<h4>加固前 (原始状态)</h4>';
        html += '<table class="comparison-table">';
        html += '<tr><td>最大von Mises应力</td><td>' + formatMPa(before.max_von_mises) + ' MPa</td></tr>';
        html += '<tr><td>最大位移</td><td>' + formatMM(before.max_displacement) + ' mm</td></tr>';
        html += '<tr><td>安全系数</td><td>' + summary.safety_factor_before.toFixed(2) + '</td></tr>';
        html += '</table></div>';

        html += '<div class="comparison-card after">';
        html += '<h4>加固后 (CFRP增强)</h4>';
        html += '<table class="comparison-table">';
        html += '<tr><td>最大von Mises应力</td><td class="' + (summary.max_stress_reduction_pct > 0 ? 'value-improved' : '') + '">' + formatMPa(after.max_von_mises) + ' MPa</td></tr>';
        html += '<tr><td>最大位移</td><td class="' + (summary.max_disp_reduction_pct > 0 ? 'value-improved' : '') + '">' + formatMM(after.max_displacement) + ' mm</td></tr>';
        html += '<tr><td>安全系数</td><td class="value-improved">' + summary.safety_factor_after.toFixed(2) + '</td></tr>';
        html += '</table></div>';
        html += '</div>';

        html += '<div class="comparison-summary">';
        html += '<h4>加固效果评估</h4>';
        html += '<div class="summary-grid">';
        html += '<div class="summary-item"><span class="summary-label">应力降低</span><span class="summary-value highlight">' + pctStr(summary.max_stress_reduction_pct) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">位移降低</span><span class="summary-value highlight">' + pctStr(summary.max_disp_reduction_pct) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">刚度提升</span><span class="summary-value">' + pctStr(summary.stiffness_increase_pct) + '</span></div>';
        html += '<div class="summary-item"><span class="summary-label">CFRP体积分数</span><span class="summary-value">' + (summary.cfrp_volume_fraction * 100).toFixed(2) + '%</span></div>';
        html += '</div>';
        html += '<p class="summary-verdict">' + summary.cost_estimate + '</p>';
        html += '</div>';

        html += '<div class="chart-wrapper" style="height:260px;margin-top:16px;"><canvas id="reinfBarChart"></canvas></div>';

        container.innerHTML = html;
        renderChart(data);
    }

    function renderChart(data) {
        var ctx = document.getElementById('reinfBarChart');
        if (!ctx) return;
        if (resultChart) resultChart.destroy();

        resultChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['最大应力(MPa)', '最大位移(mm)', '安全系数'],
                datasets: [
                    {
                        label: '加固前',
                        data: [
                            data.before.max_von_mises / 1e6,
                            data.before.max_displacement * 1000,
                            data.summary.safety_factor_before
                        ],
                        backgroundColor: 'rgba(231, 76, 60, 0.7)',
                        borderColor: 'rgba(231, 76, 60, 1)',
                        borderWidth: 1
                    },
                    {
                        label: '加固后(CFRP)',
                        data: [
                            data.after.max_von_mises / 1e6,
                            data.after.max_displacement * 1000,
                            data.summary.safety_factor_after
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
                    title: { display: true, text: '碳纤维布加固前后对比', color: '#e0e0e0' },
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
        var btnSimulate = document.getElementById('btnReinfSimulate');
        if (btnSimulate) {
            btnSimulate.addEventListener('click', simulate);
        }
    }

    return {
        init: init,
        simulate: simulate
    };
})();
