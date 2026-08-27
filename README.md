# 建筑素材中台

面向建筑设计、建筑可视化和效果图团队的内部素材库与工作中台。系统把分散在员工电脑、项目目录和共享盘中的模型、贴图、PSD、CAD、图片与灯光文件集中存放，按建筑行业结构分类，并支持从桌面端把服务器中的真实文件直接拖入 3ds Max、Photoshop 或资源管理器。

当前阶段围绕两件事展开。

- 管好素材和项目，让员工能上传、分类、搜索、预览与复用文件。
- 缩短制作软件取用素材的路径，让素材从公司服务器直接进入日常工作流。

## 核心能力

| 能力 | 当前实现 |
| --- | --- |
| 全中文工作界面 | 登录、注册、素材库、项目、筛选、设置与状态提示均已中文化 |
| 素材与项目归档 | 支持工作区、项目、文件夹、标签、自定义字段和批量操作 |
| 建筑行业分类 | 设有素材库、项目档案、参考图库、产品选型、标准规范五类档案 |
| 专业素材细分 | 覆盖 3D 模型、材质贴图、图片、HDRI、2D 配景、PSD、CAD、IES 与素材包 |
| 上传后自动归类 | 根据扩展名写入格式级分类标签，无法确认的文件进入待复核状态 |
| 搜索与筛选 | 支持关键词、标签、项目、文件夹、自定义字段、排序和视图切换 |
| 服务器统一存储 | 支持本地磁盘、SFTP 与 S3，内部部署建议使用服务器磁盘加 SMB 共享 |
| 桌面直接拖出 | Electron 桌面端把共享目录中的真实文件交给 Windows，可拖入制作软件 |
| MAX 缩略图 | 支持同名图片随 MAX 一起上传后自动设为封面，也可在素材详情中手动更换 |
| 账号与权限 | 支持注册开关、登录、工作区成员和基础权限管理 |
| 操作记录 | 提供活动记录、素材版本、衍生文件和处理状态 |

## 素材分类

系统的大类放在左侧导航中，细分条件只在当前大类下展开，避免把文件格式、建筑类型、观察视角和画面氛围混在同一层。

### 素材库

- 3D 模型
  - 家具、灯具、植物、人物、交通工具、建筑构件、景观小品、厨卫设备
  - MAX、FBX、OBJ、SKP、RVT、代理模型等格式
- 材质贴图
  - 木材、石材、混凝土、砖瓦、金属、玻璃、涂料、瓷砖、水磨石、织物、皮革、地毯
  - 地面、墙面、顶面、建筑立面、家具表面、景观铺装
  - 基础色、法线、粗糙度、金属度、置换、环境光遮蔽等通道
- 图片素材
  - 效果图、参考图、现场照片、设计过程图、分析图、平立剖、汇报排版
  - 人视、鸟瞰、仰视、平视、室内、细节特写
  - 日景、黄昏、夜景、阴天、晴天
- HDRI 环境、2D 配景、PSD 素材、CAD 图纸、IES 灯光和素材包

### 其他档案

- 项目档案按项目阶段、建筑类型和交付内容组织。
- 参考图库按专业方向、建筑类型、表现形式、观察视角、画面景别和时间氛围组织。
- 产品选型用于归档品牌、品类、规格、材质和应用部位。
- 标准规范用于归档企业标准、国家规范、制图要求和交付模板。

自动分类只处理能够从文件格式可靠判断的一级类别。沙发、石材、住宅、鸟瞰视角等内容级信息仍由上传人或资料管理员复核，避免系统给素材写入错误语义。

## 日常使用流程

1. 员工在素材库中选择项目和文件夹，拖入一批文件。
2. 服务端把原文件写入公司服务器，并根据文件格式添加一级分类。
3. 上传人补充项目、专业标签和必要的版权或版本信息。
4. 其他员工通过搜索、左侧大类和细分筛选找到素材。
5. Windows 桌面端从共享目录取出真实文件，拖入 3ds Max、Photoshop 或资源管理器。

## MAX 缩略图

`.max` 文件不能在普通服务器上稳定生成渲染预览，因此当前版本采用可靠的封面工作流，不解析模型内容。

### 同名图片自动配对

把同名 MAX 和图片放在同一次上传中。

```text
售楼处大厅.max
售楼处大厅.jpg
```

系统会把 JPG、PNG 或 WebP 作为 MAX 的封面，不再把这张图片建立为独立顶层素材。

### 手动添加封面

打开 MAX 素材详情，点击“添加缩略图”或“更换缩略图”，选择一张不超过 10 MB 的 JPG、PNG 或 WebP 图片。系统处理完成后会自动刷新素材卡片。

## 系统结构

```text
浏览器或桌面端
      │
      ▼
SvelteKit 中文界面
      │  HTTP API
      ▼
Go 服务端 ───── SQLite 数据库
      │
      ├── 本地磁盘与 SMB 共享
      ├── SFTP
      └── S3
```

- 服务端使用 Go、Fiber 和 SQLite。
- 前端使用 SvelteKit、TypeScript 和 Tailwind CSS。
- 桌面端使用 Electron，仅负责安全地把服务器文件转换为操作系统级拖拽。
- 原文件存储与业务数据库分离，数据库保存索引、项目、标签、权限和处理状态。

## 目录结构

```text
.
├── cmd/server                  Go 服务入口与前端工程
│   └── web                     SvelteKit 中文界面
├── desktop                     Windows 桌面拖拽外壳
├── internal/api                HTTP API 与路由
├── internal/service            上传、分类与业务服务
├── internal/ingress            SFTP 等导入来源
├── internal/jobs               缩略图和后台任务
├── internal/db                 SQLite 查询与迁移
├── .env.example                本地开发配置样例
├── .env.company-server.example 公司服务器配置样例
├── Dockerfile                  当前源码的生产镜像
└── docker-compose.yml          单机服务器部署配置
```

## 本地开发

### 环境要求

- Go 1.25.6 或更高版本
- Node.js 24 与 npm
- Air，用于同时开发 Go 服务和前端
- Git

图像、视频、PDF 和 Office 文件的完整预览还会用到 FFmpeg、ImageMagick、Poppler、Tesseract 和 LibreOffice。只开发分类、搜索和普通上传时可以先不安装这些可选工具。

### 启动项目

复制环境变量样例。

```bash
cp .env.example .env
```

至少修改以下配置。两个密钥应分别生成，并保持 32 个字符以上。

```dotenv
ENABLE_SIGNUP=true
JWT_SECRET=请替换为随机长密钥
APP_SECRET=请替换为另一个随机长密钥
```

安装依赖并启动前后端。

```bash
go install github.com/air-verse/air@latest
cd cmd/server/web
npm ci
cd ../../..
make dev
```

启动后访问以下地址。

- 前端开发地址 `http://127.0.0.1:5173`
- 后端接口地址 `http://127.0.0.1:8080`
- 接口文档 `http://127.0.0.1:8080/swagger/index.html`
- 健康检查 `http://127.0.0.1:8080/healthz`

首次创建管理员账号后，可以把 `ENABLE_SIGNUP` 改回 `false`，阻止未授权人员自行注册。

## 公司服务器部署

推荐在公司内网服务器上运行容器，并把持久化目录通过 SMB 只读或按部门授权给员工电脑。应用服务需要对该目录拥有读写权限，桌面客户端至少需要读取权限。

### 1. 准备配置

```bash
cp .env.company-server.example .env
```

修改服务器地址、两个随机密钥和数据目录。

```dotenv
BASE_URL=http://company-asset-server:8080
ARCHVIZ_DATA_PATH=/srv/archviz
JWT_SECRET=请替换为随机长密钥
APP_SECRET=请替换为另一个随机长密钥
```

### 2. 构建并启动

```bash
docker compose up -d --build
```

容器会从当前仓库代码构建，不会拉取其他项目的预制镜像。默认使用下面的持久化结构。

```text
/srv/archviz/
├── damask.db
└── storage/
```

### 3. 配置共享目录

把宿主机的 `/srv/archviz/storage` 通过 SMB 共享给员工。Windows 可以映射为下面的目录。

```text
\\company-asset-server\素材库\storage
```

数据库文件不要共享给普通员工，也不要放进同步盘。备份时应把数据库与 `storage` 目录放在同一个备份周期内。

### 4. 停止和查看状态

```bash
docker compose ps
docker compose logs -f
docker compose down
```

`docker compose down` 不会删除持久化目录。不要使用会删除卷或手动清空数据目录的命令。

## Windows 桌面端

网页端可以下载素材。要把服务器上的真实文件直接拖入 3ds Max 或 Photoshop，需要运行 `desktop` 目录中的桌面端，并保证这台电脑能访问同一个 SMB 共享。

PowerShell 临时运行示例。

```powershell
$env:ARCHVIZ_SERVER_URL="http://company-asset-server:8080/library"
$env:ARCHVIZ_STORAGE_ROOT="\\company-asset-server\素材库\storage"
cd desktop
npm ci
npm start
```

两个路径必须指向同一套素材数据。

| 变量 | 作用 |
| --- | --- |
| `ARCHVIZ_SERVER_URL` | 公司中台的素材库地址 |
| `ARCHVIZ_STORAGE_ROOT` | Windows 能读取的服务器素材根目录 |

桌面端会校验素材路径必须位于配置的根目录内，并阻止页面跳转到非中台域名。普通浏览器没有本机文件权限，因此只能提供下载式拖拽。

## 主要环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | Web 与 API 端口 |
| `BASE_URL` | `http://localhost:5173` | 用户访问到的外部地址 |
| `ARCHVIZ_DATA_PATH` | `./data` | 容器部署时的宿主持久化目录 |
| `DB_PATH` | `./damask.db` | SQLite 数据库位置 |
| `STORAGE` | `local` | `local`、`sftp`、`s3` 或 `memory` |
| `STORAGE_LOCAL_PATH` | `./storage` | 本地存储根目录 |
| `ENABLE_SIGNUP` | `false` | 是否开放注册入口 |
| `JWT_SECRET` | 无 | 登录令牌密钥，至少 32 个字符 |
| `APP_SECRET` | 无 | 数据加密密钥，至少 32 个字符 |
| `QUEUE_WORKERS` | `4` | 后台处理任务数量 |
| `FFMPEG_PATH` | 系统路径 | 自定义 FFmpeg 可执行文件位置 |

SFTP、S3、SMTP、Google 和 Canva 等可选配置可直接查看 `.env.example`。如果目标是从素材卡片原生拖入 3ds Max 或 Photoshop，第一阶段应优先使用服务器本地磁盘加 SMB 共享。SFTP 和 S3 文件无法直接映射为桌面端本地路径。

## 验证与测试

服务端测试。

```bash
make test
```

建筑分类与上传文件测试。

```bash
cd cmd/server/web
npm test -- src/lib/data/architectureTaxonomy.test.ts src/lib/utils/assetFiles.test.ts
```

前端类型检查和生产构建。

```bash
cd cmd/server/web
npm run check
npm run build
```

桌面路径安全测试。

```bash
cd desktop
npm test
```

## 数据与安全

- `.env`、SQLite 数据库、上传文件、构建产物和依赖目录均已加入忽略规则。
- 生产环境不要继续使用样例密钥，也不要把真实账号或服务器密码写入仓库。
- 素材上传接口只保存文件和业务索引，不会把员工本机绝对路径写入共享数据。
- 桌面端不启用页面 Node.js 权限，只开放受限的拖拽接口。
- 生产环境建议在内网反向代理后启用 HTTPS，并限制管理入口的访问范围。

## 当前边界

- 自动分类目前只判断文件格式，内容级标签需要人工确认。
- MAX 缩略图来自同名图片或手动上传，系统暂不渲染 MAX 场景。
- 原生拖入制作软件需要桌面端与可访问的共享存储。
- 浏览器版保留下载和下载式拖拽，无法绕过浏览器的本机文件权限。
- SQLite 适合当前单服务器团队使用。进入多节点或超大并发阶段后再评估数据库迁移。

## 许可证

本项目保留仓库中的 Apache License 2.0 许可文件。内部部署前请同时确认公司素材本身的版权、采购授权和项目保密要求。
