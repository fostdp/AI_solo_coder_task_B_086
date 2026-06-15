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
    console.log('[Dashboard] Initialized OK');
});
