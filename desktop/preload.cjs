const { contextBridge, ipcRenderer } = require("electron/renderer");

contextBridge.exposeInMainWorld("archvizDesktop", {
  isAvailable: true,
  startDrag: (asset) => ipcRenderer.send("archviz:start-drag", asset),
});
