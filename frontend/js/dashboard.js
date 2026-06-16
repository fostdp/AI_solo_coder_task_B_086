document.addEventListener('DOMContentLoaded', async function() {
    try {
        await window.ZhaozhouBridge3D.loadConfig();
        await window.CreepPanel.loadConfig();
    } catch(e) { console.warn('[App] Config load failed, using defaults'); }

    window.CreepPanel.startClock();
    window.CreepPanel.init();

    await window.ZhaozhouBridge3D.loadGeometry();
    window.ZhaozhouBridge3D.init('viewport3d');

    window.CreepPanel.connectWebSocket();
    window.CreepPanel.startFallbackPoller();

    ZhaozhouBridge3D.onStressUpdated = function(vals) {
        CreepPanel.updateStressBars(vals);
    };
    ZhaozhouBridge3D.onPredictionsUpdated = function(preds) {
        CreepPanel.refreshPredictions(preds);
    };
    CreepPanel.onDataUpdated = function(readings) {
    };

    window.ArchComparator.init();
    window.EraComparator.init();
    window.RetrofitSimulator.init();
    window.VRBridgeBuilder.init();

    initFeatureNav();
    initRangeDisplay();

    console.log('[Dashboard] Initialized OK');
});

function initFeatureNav() {
    var navBtns = document.querySelectorAll('.nav-btn');
    navBtns.forEach(function(btn) {
        btn.addEventListener('click', function() {
            navBtns.forEach(function(b) { b.classList.remove('active'); });
            btn.classList.add('active');

            var sectionId = btn.dataset.section;
            var sections = document.querySelectorAll('.feature-section');
            sections.forEach(function(s) { s.classList.remove('active'); });

            var target = document.getElementById(sectionId);
            if (target) target.classList.add('active');
        });
    });
}

function initRangeDisplay() {
    var rangeMap = {
        'vbSpan': 'vbSpanVal',
        'vbRise': 'vbRiseVal',
        'vbThickness': 'vbThicknessVal',
        'vbLoad': 'vbLoadVal'
    };
    Object.keys(rangeMap).forEach(function(inputId) {
        var input = document.getElementById(inputId);
        var display = document.getElementById(rangeMap[inputId]);
        if (input && display) {
            display.textContent = input.value;
            input.addEventListener('input', function() {
                display.textContent = input.value;
            });
        }
    });
}
