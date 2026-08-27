const fs = require("node:fs");
const path = require("node:path");
const {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  nativeImage,
} = require("electron/main");
const { resolveAssetPath } = require("./path.cjs");

const serverUrl =
  process.env.ARCHVIZ_SERVER_URL || "http://127.0.0.1:5173/library";
const storageRoot =
  process.env.ARCHVIZ_STORAGE_ROOT || path.resolve(__dirname, "..", "storage");
const allowedOrigin = new URL(serverUrl).origin;
const dragIcon = nativeImage.createFromPath(
  path.resolve(__dirname, "..", "cmd/server/web/static/icon-192.png"),
);

function createWindow() {
  const window = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 1024,
    minHeight: 680,
    title: "建筑素材中台",
    backgroundColor: "#eef0ec",
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });

  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  window.webContents.on("will-navigate", (event, url) => {
    if (new URL(url).origin !== allowedOrigin) event.preventDefault();
  });
  window.loadURL(serverUrl);
}

ipcMain.on("archviz:start-drag", (event, asset) => {
  try {
    if (new URL(event.senderFrame.url).origin !== allowedOrigin) return;
    const file = resolveAssetPath(storageRoot, String(asset?.storageKey || ""));
    if (!fs.statSync(file).isFile()) throw new Error("素材文件不存在");
    event.sender.startDrag({ file, icon: dragIcon });
  } catch (error) {
    dialog.showMessageBox(BrowserWindow.fromWebContents(event.sender), {
      type: "error",
      title: "无法取用素材",
      message: "请确认公司素材服务器已连接。",
      detail: error instanceof Error ? error.message : String(error),
    });
  }
});

app.whenReady().then(createWindow);
app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
app.on("activate", () => {
  if (BrowserWindow.getAllWindows().length === 0) createWindow();
});
