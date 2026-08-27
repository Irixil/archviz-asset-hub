# 建筑素材中台桌面版

桌面版复用现有素材库界面，只增加操作系统级文件拖拽。公司电脑需要能够访问与服务器相同的共享存储目录。

```powershell
$env:ARCHVIZ_SERVER_URL="http://素材服务器:8080/library"
$env:ARCHVIZ_STORAGE_ROOT="\\素材服务器\素材库\storage"
npm install
npm start
```

素材从卡片拖出时，桌面版会将共享目录中的真实文件交给 Windows，可直接落到 3ds Max、Photoshop 或资源管理器。普通浏览器仍按下载链接处理拖拽。
