window.CreepPanel = (function() {
    var apiBase = (window.location.origin || 'http://localhost:8080');
    var wsBase = apiBase.replace(/^http/, 'ws');
    var socket = null;
    var sensorRegistry = [];
    var latestData = {};
    var alerts = [];
    var sparklineCharts = {};
    var mainCharts = {};
    var wsRetryCount = 0;
    var wsReconnectTimeout = null;
    var fallbackPollerInterval = null;
    var config = null;
    var COLORS = {
        teal: '#1abc9c',
        warning: '#f39c12',
        orange: '#e67e22',
        red: '#e74c3c',
        blue: '#3498db',
        text: '#ecf0f1',
        textDim: '#95a5a6',
        grid: 'rgba(255,255,255,0.06)'
    };
    var dataCallbacks = {
        onDataUpdated: null
    };

    async function fetchJSON(url, opts) {
        try {
            var res = await fetch(url, {
                headers: { 'Content-Type': 'application/json' },
                ...opts
            });
            if (!res.ok) {
                console.warn('[CreepPanel][fetchJSON] HTTP ' + res.status + ': ' + url);
                return null;
            }
            return await res.json();
        } catch (e) {
            console.warn('[CreepPanel][fetchJSON] Failed: ' + url, e.message);
            return null;
        }
    }

    async function loadConfig() {
        try {
            var res = await fetchJSON('config/app_config.json');
            if (res) {
                config = res;
                if (res.front_end && res.front_end.colors) {
                    COLORS = res.front_end.colors;
                }
                window.COLORS = COLORS;
            }
        } catch (e) {
            console.warn('[CreepPanel][loadConfig] Failed:', e.message);
        }
        return config;
    }

    async function init() {
        var registry = await fetchJSON(apiBase + '/api/sensors');
        if (registry && Array.isArray(registry)) {
            sensorRegistry = registry;
        }

        var start = new Date(Date.now() - 24 * 3600 * 1000).toISOString();
        var alertsRes = await fetchJSON(apiBase + '/api/alerts?start=' + encodeURIComponent(start));
        if (alertsRes && Array.isArray(alertsRes)) {
            alerts = alertsRes;
        }
        renderAlerts(alerts);

        initSparklines();
        initLiveChart();
        initStressDistributionChart();
        initPredictionChart();
        setupTabHandlers();
        computeAndDisplayAgeCorrection();
    }

    function startClock() {
        var weekdays = ['日', '一', '二', '三', '四', '五', '六'];
        function tick() {
            var now = new Date();
            var t = now.getHours().toString().padStart(2, '0') + ':' +
                      now.getMinutes().toString().padStart(2, '0') + ':' +
                      now.getSeconds().toString().padStart(2, '0');
            var d = now.getFullYear() + '-' +
                      (now.getMonth() + 1).toString().padStart(2, '0') + '-' +
                      now.getDate().toString().padStart(2, '0') + ' 星期' +
                      weekdays[now.getDay()];
            var ct = document.getElementById('clockTime');
            var cd = document.getElementById('clockDate');
            if (ct) ct.textContent = t;
            if (cd) cd.textContent = d;
        }
        tick();
        setInterval(tick, 1000);
    }

    function initSparklines() {
        function makeSpark(canvasId, color, fillAlpha) {
            var ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            return new Chart(ctx, {
                type: 'line',
                data: {
                    labels: Array(30).fill(''),
                    datasets: [{
                        data: [],
                        borderColor: color,
                        backgroundColor: color + (fillAlpha || '20'),
                        fill: true,
                        tension: 0.4,
                        borderWidth: 1.5,
                        pointRadius: 0
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false }, tooltip: { enabled: false } },
                    scales: {
                        x: { display: false },
                        y: { display: false }
                    },
                    elements: {
                        line: { tension: 0.4 }
                    }
                }
            });
        }

        sparklineCharts.arch = makeSpark('sparkArch', COLORS.teal);
        sparklineCharts.pier = makeSpark('sparkPier', COLORS.warning);
        sparklineCharts.temp = makeSpark('sparkTemp', COLORS.orange);
        sparklineCharts.crack = makeSpark('sparkCrack', COLORS.red);
    }

    function pushSparkline(key, value) {
        var chart = sparklineCharts[key];
        if (!chart) return;
        chart.data.datasets[0].data.push(value);
        if (chart.data.datasets[0].data.length > 30) {
            chart.data.datasets[0].data.shift();
        }
        chart.data.labels.push('');
        if (chart.data.labels.length > 30) chart.data.labels.shift();
        chart.update('none');
    }

    function initLiveChart() {
        var baseOpts = {
            responsive: true,
            maintainAspectRatio: false,
            animation: { duration: 500 },
            plugins: {
                legend: {
                    labels: { color: COLORS.textDim, font: { size: 11 }, padding: 12, usePointStyle: true }
                },
                tooltip: {
                    backgroundColor: 'rgba(15,25,35,0.95)',
                    titleColor: COLORS.text,
                    bodyColor: COLORS.textDim,
                    borderColor: 'rgba(255,255,255,0.1)',
                    borderWidth: 1,
                    padding: 10,
                    cornerRadius: 6
                }
            }
        };

        function genLabels(n) {
            var arr = [];
            var now = new Date();
            for (var i = n - 1; i >= 0; i--) {
                var d = new Date(now - i * 30 * 60000);
                arr.push(d.getHours().toString().padStart(2, '0') + ':' +
                         d.getMinutes().toString().padStart(2, '0'));
            }
            return arr;
        }

        var liveCtx = document.getElementById('chartRealtime');
        if (liveCtx) {
            mainCharts.live = new Chart(liveCtx, {
                type: 'line',
                data: {
                    labels: genLabels(48),
                    datasets: [
                        {
                            label: '拱券应变 (με)',
                            data: Array(48).fill(null),
                            borderColor: COLORS.teal,
                            backgroundColor: 'transparent',
                            yAxisID: 'y',
                            tension: 0.35,
                            borderWidth: 2,
                            pointRadius: 1.5,
                            pointBackgroundColor: COLORS.teal
                        },
                        {
                            label: '桥墩沉降 (mm)',
                            data: Array(48).fill(null),
                            borderColor: COLORS.warning,
                            backgroundColor: 'transparent',
                            yAxisID: 'y1',
                            tension: 0.35,
                            borderWidth: 2,
                            pointRadius: 1.5,
                            pointBackgroundColor: COLORS.warning
                        },
                        {
                            label: '温度 (℃)',
                            data: Array(48).fill(null),
                            borderColor: COLORS.orange,
                            backgroundColor: 'transparent',
                            yAxisID: 'y2',
                            tension: 0.35,
                            borderWidth: 2,
                            pointRadius: 1.5,
                            pointBackgroundColor: COLORS.orange
                        },
                        {
                            label: '裂缝宽度 (mm×100)',
                            data: Array(48).fill(null),
                            borderColor: COLORS.red,
                            backgroundColor: 'transparent',
                            yAxisID: 'y3',
                            tension: 0.35,
                            borderWidth: 2,
                            pointRadius: 1.5,
                            pointBackgroundColor: COLORS.red
                        }
                    ]
                },
                options: {
                    ...baseOpts,
                    scales: {
                        x: {
                            ticks: { color: COLORS.textDim, font: { size: 10 }, maxTicksLimit: 12 },
                            grid: { color: COLORS.grid, drawBorder: false },
                            border: { display: false }
                        },
                        y: {
                            position: 'left',
                            title: { display: true, text: '应变 με / 裂缝×100', color: COLORS.textDim, font: { size: 10 } },
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { color: COLORS.grid, drawBorder: false },
                            border: { display: false }
                        },
                        y1: {
                            position: 'right',
                            title: { display: true, text: '沉降 mm', color: COLORS.textDim, font: { size: 10 } },
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { drawOnChartArea: false },
                            border: { display: false }
                        },
                        y2: {
                            position: 'right',
                            offset: true,
                            title: { display: true, text: '温度 ℃', color: COLORS.textDim, font: { size: 10 } },
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { drawOnChartArea: false },
                            border: { display: false }
                        },
                        y3: {
                            display: false,
                            position: 'right',
                            grid: { drawOnChartArea: false }
                        }
                    },
                    plugins: {
                        ...baseOpts.plugins,
                        title: { display: true, text: '传感器 24 小时趋势', color: COLORS.textDim, font: { size: 12, weight: '500' }, padding: { bottom: 16 }, align: 'start' }
                    }
                }
            });
        }
    }

    function initStressDistributionChart() {
        var baseOpts = {
            responsive: true,
            maintainAspectRatio: false,
            animation: { duration: 500 },
            plugins: {
                legend: {
                    labels: { color: COLORS.textDim, font: { size: 11 }, padding: 12, usePointStyle: true }
                },
                tooltip: {
                    backgroundColor: 'rgba(15,25,35,0.95)',
                    titleColor: COLORS.text,
                    bodyColor: COLORS.textDim,
                    borderColor: 'rgba(255,255,255,0.1)',
                    borderWidth: 1,
                    padding: 10,
                    cornerRadius: 6
                }
            }
        };

        var stressCtx = document.getElementById('chartStress');
        if (stressCtx) {
            var groups = ['拱顶 Crown', '拱肩 Shoulder L', '拱肩 Shoulder R', '拱脚 Spring L',
                            '拱脚 Spring R', '小拱 1-2', '小拱 3-4', '桥面 Deck'];
            var initStress = groups.map(function() { return Math.random() * 15; });
            var colors = initStress.map(function(v) {
                if (v > 12) return COLORS.red;
                if (v > 8) return COLORS.warning;
                return COLORS.teal;
            });

            mainCharts.stress = new Chart(stressCtx, {
                type: 'bar',
                data: {
                    labels: groups,
                    datasets: [{
                        label: 'von Mises 应力 (MPa)',
                        data: initStress,
                        backgroundColor: colors.map(function(c) { return c + '33'; }),
                        borderColor: colors,
                        borderWidth: 1.5,
                        borderRadius: 4,
                        borderSkipped: false
                    }]
                },
                options: {
                    ...baseOpts,
                    indexAxis: 'y',
                    plugins: {
                        ...baseOpts.plugins,
                        legend: { display: false },
                        title: { display: true, text: '各构件组 von Mises 等效应力分布', color: COLORS.textDim, font: { size: 12, weight: '500' }, padding: { bottom: 16 }, align: 'start' },
                        datalabels: {
                            anchor: 'end',
                            align: 'right',
                            color: COLORS.text,
                            font: { size: 10, weight: '600' },
                            formatter: function(v) { return v.toFixed(1) + ' MPa'; }
                        }
                    },
                    scales: {
                        x: {
                            title: { display: true, text: 'MPa', color: COLORS.textDim, font: { size: 10 } },
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { color: COLORS.grid, drawBorder: false },
                            border: { display: false },
                            max: 18
                        },
                        y: {
                            ticks: { color: COLORS.text, font: { size: 11 } },
                            grid: { display: false },
                            border: { display: false }
                        }
                    }
                }
            });
        }
    }

    function initPredictionChart() {
        var baseOpts = {
            responsive: true,
            maintainAspectRatio: false,
            animation: { duration: 500 },
            plugins: {
                legend: {
                    labels: { color: COLORS.textDim, font: { size: 11 }, padding: 12, usePointStyle: true }
                },
                tooltip: {
                    backgroundColor: 'rgba(15,25,35,0.95)',
                    titleColor: COLORS.text,
                    bodyColor: COLORS.textDim,
                    borderColor: 'rgba(255,255,255,0.1)',
                    borderWidth: 1,
                    padding: 10,
                    cornerRadius: 6
                }
            }
        };

        var predictCtx = document.getElementById('chartPredict');
        if (predictCtx) {
            var years = [0, 1, 5, 10, 20, 30, 50];
            var crown = years.map(function(y) { return y === 0 ? 0 : -y * 0.7 - (y > 20 ? (y - 20) * 0.05 : 0); });
            var spring = years.map(function(y) { return y === 0 ? 0 : y * 0.24; });
            var pier = years.map(function(y) { return y === 0 ? 0 : -y * 0.36; });
            var crownUp = crown.map(function(v) { return v * 1.15; });
            var crownDown = crown.map(function(v) { return v * 0.85; });

            mainCharts.predict = new Chart(predictCtx, {
                type: 'line',
                data: {
                    labels: years.map(function(y) { return y + '年'; }),
                    datasets: [
                        {
                            label: '拱顶下沉 Crown Displacement',
                            data: crown,
                            borderColor: COLORS.orange,
                            backgroundColor: COLORS.orange + '20',
                            fill: '+1',
                            borderWidth: 2.5,
                            pointRadius: 3,
                            pointBackgroundColor: COLORS.orange,
                            tension: 0.35
                        },
                        {
                            label: '置信上限',
                            data: crownUp,
                            borderColor: 'rgba(52,152,219,0.4)',
                            backgroundColor: 'transparent',
                            borderDash: [4, 4],
                            pointRadius: 0,
                            tension: 0.35
                        },
                        {
                            label: '置信下限',
                            data: crownDown,
                            borderColor: 'rgba(52,152,219,0.4)',
                            backgroundColor: COLORS.orange + '10',
                            fill: '-1',
                            borderDash: [4, 4],
                            pointRadius: 0,
                            tension: 0.35
                        },
                        {
                            label: '拱脚水平位移 Spring Horizontal',
                            data: spring,
                            borderColor: COLORS.teal,
                            backgroundColor: 'transparent',
                            borderWidth: 2,
                            pointRadius: 2,
                            tension: 0.35
                        },
                        {
                            label: '桥墩沉降 Pier Settlement',
                            data: pier,
                            borderColor: COLORS.blue,
                            backgroundColor: 'transparent',
                            borderWidth: 2,
                            pointRadius: 2,
                            tension: 0.35
                        }
                    ]
                },
                options: {
                    ...baseOpts,
                    plugins: {
                        ...baseOpts.plugins,
                        title: { display: true, text: '50年累计变形预测 (mm) — 考虑徐变、收缩与温度作用', color: COLORS.textDim, font: { size: 12, weight: '500' }, padding: { bottom: 16 }, align: 'start' }
                    },
                    scales: {
                        x: {
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { color: COLORS.grid, drawBorder: false },
                            border: { display: false }
                        },
                        y: {
                            title: { display: true, text: '累计位移 mm', color: COLORS.textDim, font: { size: 10 } },
                            ticks: { color: COLORS.textDim, font: { size: 10 } },
                            grid: { color: COLORS.grid, drawBorder: false },
                            border: { display: false }
                        }
                    }
                }
            });
        }
    }

    function updateStressBars(vals) {
        if (!mainCharts.stress) return;
        mainCharts.stress.data.datasets[0].data = vals;
        var colors = vals.map(function(v) {
            if (v > 12) return COLORS.red;
            if (v > 8) return COLORS.warning;
            return COLORS.teal;
        });
        mainCharts.stress.data.datasets[0].backgroundColor = colors.map(function(c) { return c + '33'; });
        mainCharts.stress.data.datasets[0].borderColor = colors;
        mainCharts.stress.update();
    }

    function refreshPredictions(preds) {
        if (!mainCharts.predict) return;
        mainCharts.predict.data.datasets.forEach(function(ds, i) {
            if (i === 0) {
                ds.borderWidth = 4;
                ds.pointRadius = 5;
            }
        });
        mainCharts.predict.update();
        setTimeout(function() {
            if (mainCharts.predict) {
                mainCharts.predict.data.datasets.forEach(function(ds, i) {
                    if (i === 0) {
                        ds.borderWidth = 2.5;
                        ds.pointRadius = 3;
                    }
                });
                mainCharts.predict.update();
            }
        }, 7000);
    }

    function switchTab(tabName) {
        document.querySelectorAll('.tab-btn').forEach(function(b) { b.classList.remove('active'); });
        var btn = document.querySelector('.tab-btn[data-tab="' + tabName + '"]');
        if (btn) btn.classList.add('active');

        ['panel-realtime', 'panel-stress', 'panel-predict'].forEach(function(id) {
            var el = document.getElementById(id);
            if (!el) return;
            if ((tabName === 'realtime' && id === 'panel-realtime') ||
                (tabName === 'stress' && id === 'panel-stress') ||
                (tabName === 'predict' && id === 'panel-predict')) {
                el.style.display = '';
                el.classList.add('active');
            } else {
                el.style.display = 'none';
                el.classList.remove('active');
            }
        });
    }

    function setupTabHandlers() {
        document.querySelectorAll('.tab-btn').forEach(function(btn) {
            btn.addEventListener('click', function() {
                switchTab(btn.dataset.tab);
            });
        });
    }

    function renderAlerts(list) {
        var alertList = list && list.length ? list : alerts;
        var listEl = document.getElementById('alertsList');
        if (!listEl) return;
        listEl.innerHTML = '';

        var sorted = [...alertList].sort(function(a, b) {
            var ta = a.time || a.Time;
            var tb = b.time || b.Time;
            return new Date(tb) - new Date(ta);
        });

        var sevText = { critical: '严重', warning: '警告', info: '信息' };

        if (sorted.length === 0) {
            listEl.innerHTML = '<div class="alert-empty">暂无告警</div>';
        } else {
            sorted.forEach(function(a) {
                var sev = a.severity || a.Severity || 'info';
                var msg = a.message || a.Message || '';
                var sid = a.sensor_id || a.SensorID || '';
                var tRaw = a.time || a.Time;
                var dt = new Date(tRaw);
                var tStr = isNaN(dt.getTime())
                    ? (typeof tRaw === 'string' ? tRaw.slice(11, 19) : '--:--:--')
                    : dt.getHours().toString().padStart(2, '0') + ':' +
                      dt.getMinutes().toString().padStart(2, '0') + ':' +
                      dt.getSeconds().toString().padStart(2, '0');

                var el = document.createElement('div');
                el.className = 'alert-item ' + sev + (sev === 'critical' ? ' critical-row' : '');
                el.innerHTML =
                    '<div class="alert-top">' +
                        '<span class="alert-badge badge-' + sev + '">' + (sevText[sev] || sev) + '</span>' +
                        '<span class="alert-time">' + tStr + '</span>' +
                    '</div>' +
                    '<div class="alert-meta">' +
                        '<span class="alert-sensor"><strong>' + sid + '</strong></span>' +
                    '</div>' +
                    '<div class="alert-message">' + msg + '</div>';
                listEl.appendChild(el);
            });
        }

        var counts = sorted.reduce(function(acc, a) {
            var s = (a.severity || a.Severity || 'info');
            acc[s] = (acc[s] || 0) + 1;
            return acc;
        }, { critical: 0, warning: 0, info: 0 });

        var statsEl = document.getElementById('alertStats');
        if (statsEl) {
            statsEl.innerHTML =
                '<span class="stat-critical">' + counts.critical + ' 严重</span>' +
                '<span class="stat-warning">' + counts.warning + ' 警告</span>' +
                '<span class="stat-info">' + counts.info + ' 信息</span>';
        }
    }

    function addAlert(alert) {
        alerts.unshift(alert);
        if (alerts.length > 100) alerts.pop();
        renderAlerts(alerts);
    }

    function updateLatestData(readings) {
        var now = new Date();
        var archVal = null, pierVal = null, tempVal = null, crackVal = null;
        var pierVals = [];
        var hasAlert = false;

        readings.forEach(function(r) {
            latestData[r.sensor_id || r.SensorID] = r;

            var sid = r.sensor_id || r.SensorID || '';
            var strain = r.strain_micro !== undefined ? r.strain_micro : r.StrainMicro;
            var settle = r.settlement_mm !== undefined ? r.settlement_mm : r.SettlementMM;
            var temp = r.temperature !== undefined ? r.temperature : r.Temperature;
            var crack = r.crack_width_mm !== undefined ? r.crack_width_mm : r.CrackWidthMM;

            if (sid.startsWith('ARCH')) {
                if (strain !== undefined && strain !== null) {
                    if (archVal === null || sid === 'ARCH-001') archVal = strain;
                }
            }
            if (sid.startsWith('PIER')) {
                if (settle !== undefined && settle !== null) {
                    pierVals.push(settle);
                    if (pierVal === null) pierVal = settle;
                }
            }
            if (temp !== undefined && temp !== null) {
                tempVal = tempVal === null ? temp : (tempVal + temp) / 2;
            }
            if (sid.startsWith('CRACK')) {
                if (crack !== undefined && crack !== null) {
                    if (crackVal === null || sid === 'CRACK-001') crackVal = crack;
                    var crackThreshold = (config && config.thresholds && config.thresholds.crack_width_warning_mm) || 0.3;
                    if (crack > crackThreshold) hasAlert = true;
                }
            }
        });

        if (pierVals.length > 1) {
            var rate = Math.abs(pierVals[0] - pierVals[1]);
            var settleThreshold = (config && config.thresholds && config.thresholds.settlement_rate_warning_mmpm) || 2.0;
            if (rate > settleThreshold) hasAlert = true;
        }

        if (archVal !== null) {
            var el = document.getElementById('valArch');
            if (el) el.textContent = archVal.toFixed(1);
            pushSparkline('arch', archVal);
        }
        if (pierVal !== null) {
            var el2 = document.getElementById('valPier');
            if (el2) el2.textContent = pierVal.toFixed(1);
            if (pierVals.length > 1) {
                var pier2El = document.getElementById('valPier2');
                if (pier2El) pier2El.textContent = 'PIER-002: ' + pierVals[1].toFixed(1) + ' mm';
            }
            pushSparkline('pier', pierVal);
        }
        if (tempVal !== null) {
            var el3 = document.getElementById('valTemp');
            if (el3) el3.textContent = tempVal.toFixed(1);
            pushSparkline('temp', tempVal);
        }
        if (crackVal !== null) {
            var el4 = document.getElementById('valCrack');
            if (el4) el4.textContent = crackVal.toFixed(2);
            pushSparkline('crack', crackVal);
        }

        ['metricArch', 'metricPier', 'metricTemp', 'metricCrack'].forEach(function(id) {
            var el5 = document.getElementById(id);
            if (!el5) return;
            el5.classList.remove('updated');
            void el5.offsetWidth;
            el5.classList.add('updated');
        });

        if (hasAlert) {
            ['metricArch', 'metricPier', 'metricTemp', 'metricCrack'].forEach(function(id) {
                var el6 = document.getElementById(id);
                if (el6) {
                    el6.style.borderColor = '#e74c3c';
                    el6.style.boxShadow = '0 0 14px rgba(231,76,60,0.5)';
                    setTimeout(function() {
                        if (el6) {
                            el6.style.borderColor = '';
                            el6.style.boxShadow = '';
                        }
                    }, 3000);
                }
            });
        }

        if (mainCharts.live) {
            var t = now.getHours().toString().padStart(2, '0') + ':' +
                      now.getMinutes().toString().padStart(2, '0');
            var chart = mainCharts.live;
            chart.data.labels.push(t);
            if (chart.data.labels.length > 48) chart.data.labels.shift();
            var datasets = [
                { idx: 0, val: archVal },
                { idx: 1, val: pierVal },
                { idx: 2, val: tempVal },
                { idx: 3, val: crackVal !== null ? crackVal * 100 : null }
            ];
            datasets.forEach(function(ds) {
                chart.data.datasets[ds.idx].data.push(ds.val !== null ? ds.val : null);
                if (chart.data.datasets[ds.idx].data.length > 48) {
                    chart.data.datasets[ds.idx].data.shift();
                }
            });
            chart.update('none');
        }

        var t2 = now.getHours().toString().padStart(2, '0') + ':' +
                  now.getMinutes().toString().padStart(2, '0') + ':' +
                  now.getSeconds().toString().padStart(2, '0');
        var lastUpd = document.getElementById('lastUpdate');
        if (lastUpd) lastUpd.textContent = '最后更新: ' + t2;

        if (dataCallbacks.onDataUpdated && typeof dataCallbacks.onDataUpdated === 'function') {
            dataCallbacks.onDataUpdated(readings);
        }
    }

    function connectWebSocket() {
        try {
            socket = new WebSocket(wsBase + '/ws/realtime');
        } catch (e) {
            console.warn('[CreepPanel][WS] Create failed:', e.message);
            scheduleReconnect();
            return;
        }

        socket.addEventListener('open', function() {
            wsRetryCount = 0;
            var ind = document.getElementById('wsStatus');
            var val = document.getElementById('wsStatusText');
            if (ind) ind.className = 'status-dot status-online';
            if (val) val.textContent = '已连接';
            console.log('[CreepPanel][WS] Connected');
        });

        socket.addEventListener('message', function(evt) {
            try {
                var readings = JSON.parse(evt.data);
                if (Array.isArray(readings)) {
                    updateLatestData(readings);
                }
            } catch (e) {
                console.warn('[CreepPanel][WS] Parse failed:', e.message);
            }
        });

        socket.addEventListener('close', function() {
            var ind = document.getElementById('wsStatus');
            var val = document.getElementById('wsStatusText');
            if (ind) ind.className = 'status-dot status-offline';
            if (val) val.textContent = '重连中...';
            console.log('[CreepPanel][WS] Closed');
            scheduleReconnect();
        });

        socket.addEventListener('error', function() {
            var ind = document.getElementById('wsStatus');
            var val = document.getElementById('wsStatusText');
            if (ind) ind.className = 'status-dot status-offline';
            if (val) val.textContent = '重连中...';
            console.warn('[CreepPanel][WS] Error');
        });
    }

    function scheduleReconnect() {
        if (wsReconnectTimeout) return;
        wsRetryCount = Math.min(wsRetryCount + 1, 10);
        var delay = Math.min(5 * Math.pow(1.5, wsRetryCount - 1), 30) * 1000;
        console.log('[CreepPanel][WS] Reconnect in ' + (delay / 1000).toFixed(1) + 's (attempt ' + wsRetryCount + ')');
        wsReconnectTimeout = setTimeout(function() {
            wsReconnectTimeout = null;
            connectWebSocket();
        }, delay);
    }

    function startFallbackPoller() {
        var interval = (config && config.front_end && config.front_end.poll_interval_ms) || 10000;

        async function pollLatestData() {
            if (socket && socket.readyState === WebSocket.OPEN) {
                return;
            }
            var data = await fetchJSON(apiBase + '/api/sensors/all/latest');
            if (data && Array.isArray(data)) {
                if (!(socket && socket.readyState === WebSocket.OPEN)) {
                    updateLatestData(data);
                }
            }
        }

        fallbackPollerInterval = setInterval(pollLatestData, interval);
        pollLatestData();
    }

    async function computeAndDisplayAgeCorrection() {
        try {
            var pred50 = await fetchJSON(apiBase + '/api/deformation/predict50', { method: 'POST' });
            var ageReport = pred50 && (pred50.age_report || pred50.ageReport);
            var stoneAgeYears = null;
            var phiInfNew = null;
            var phiAgeCorrection = null;

            if (ageReport) {
                stoneAgeYears = ageReport.stone_age_years;
                phiInfNew = ageReport.phi_inf_new;
                phiAgeCorrection = ageReport.phi_age_correction;
            }

            if (!stoneAgeYears && config && config.bridge && config.bridge.construction_year) {
                stoneAgeYears = 2026 - config.bridge.construction_year;
            }

            if (config && config.creep_model) {
                var cm = config.creep_model;
                if (phiInfNew === null) phiInfNew = cm.phi_inf_new_stone || 2.0;
                if (phiAgeCorrection === null && stoneAgeYears && cm.archaeological_stiffening_k) {
                    var refDays = cm.archaeological_stiffening_ref_days || 36500;
                    var stoneAgeDays = stoneAgeYears * 365.25;
                    var stiffFactor = 1 + cm.archaeological_stiffening_k * Math.log(1 + stoneAgeDays / refDays);
                    phiAgeCorrection = phiInfNew / stiffFactor;
                }
            }

            var footerInfo = document.querySelector('.footer-info');
            if (footerInfo && stoneAgeYears) {
                var ageEl = document.createElement('span');
                ageEl.id = 'ageCorrectionInfo';
                ageEl.title =
                    '石材年龄: ' + stoneAgeYears.toFixed(0) + ' 年\n' +
                    '新石徐变终值 φ∞: ' + (phiInfNew ? phiInfNew.toFixed(2) : 'N/A') + '\n' +
                    '考古龄期修正 φ: ' + (phiAgeCorrection ? phiAgeCorrection.toFixed(3) : 'N/A');
                ageEl.style.cursor = 'help';
                ageEl.style.textDecoration = 'dotted underline';
                ageEl.textContent = '桥龄: ' + stoneAgeYears.toFixed(0) + '年';
                var lastInfo = footerInfo.querySelector('#lastUpdate');
                if (lastInfo) {
                    footerInfo.insertBefore(ageEl, lastInfo);
                } else {
                    footerInfo.appendChild(ageEl);
                }
            }
        } catch (e) {
            console.warn('[CreepPanel][AgeCorrection] Failed:', e.message);
        }
    }

    return {
        init: init,
        loadConfig: loadConfig,
        startClock: startClock,
        initSparklines: initSparklines,
        initLiveChart: initLiveChart,
        initStressDistributionChart: initStressDistributionChart,
        initPredictionChart: initPredictionChart,
        renderAlerts: renderAlerts,
        addAlert: addAlert,
        updateData: updateLatestData,
        updateStressBars: updateStressBars,
        refreshPredictions: refreshPredictions,
        connectWebSocket: connectWebSocket,
        startFallbackPoller: startFallbackPoller,
        set onDataUpdated(fn) { dataCallbacks.onDataUpdated = fn; },
        get onDataUpdated() { return dataCallbacks.onDataUpdated; }
    };
})();
