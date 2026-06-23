# AI 图片工作台

一个基于 Go 和原生 JavaScript 构建的自托管 AI 图片生成 Web 应用。作为 OpenAI 兼容图片生成 API 的前端代理，提供完整的图片生成、编辑和管理功能。

## 功能特性

- **图片生成** - 从文本提示生成图片
- **图片编辑** - 支持局部修复（Inpainting）
- **背景移除** - 一键移除图片背景
- **用户认证** - 完整的用户系统和管理员权限控制
- **图库管理** - 浏览、分享和管理生成的图片
- **API 访问** - 支持 API Key 进行程序化访问
- **多模型支持** - 可配置多个图片生成模型
- **自包含部署** - 单一二进制文件，内置前端资源

## 技术栈

- **后端**: Go 1.21+
- **Web 框架**: Gin
- **数据库**: SQLite (GORM)
- **前端**: 原生 JavaScript + HTML/CSS
- **认证**: Session + API Key 双重认证
- **存储**: 本地文件系统

## 快速开始

### 前置要求

- Go 1.21 或更高版本
- 可选：Docker（用于容器化部署）

### 本地开发

```bash
# 1. 克隆仓库
git clone <repository-url>
cd image-generator

# 2. 下载依赖
go mod download

# 3. 复制配置文件并修改
cp config.yaml.example config.yaml
# 编辑 config.yaml，设置 OpenAI API 配置

# 4. 构建并运行
go build -o bin/server ./cmd/server
./bin/server
```

应用将在 `http://localhost:8080` 启动。

### 生产部署

```bash
# 使用 Alpine Linux 静态构建脚本
./build-alpine.sh

# 生成的二进制文件位于 dist/ig
./dist/ig
```

### 配置文件

创建 `config.yaml` 文件：

```yaml
server:
    address: :8080
    session_secret: your-secure-random-secret-here  # 生产环境必须修改
    session_name: image_workbench_session
    secure_cookies: false  # HTTPS 环境下设为 true

database:
    path: data/app.db

storage:
    original_dir: uploads/original
    generated_dir: uploads/generated
    max_upload_mb: 20

openai:
    base_url: "https://api.openai.com/v1"  # 或其他兼容 API 端点
    api_key: "sk-..."  # 你的 API Key

defaults:
    site_name: AI 图片工作台
    site_icon: AI
    allow_register: true
    available_models:
        - gpt-image-2
        - gpt-image-1
    available_sizes:
        - 1024x1024
        - 1024x1536
        - 1536x1024
```

## 项目结构

```
image-generator/
├── cmd/
│   └── server/          # 应用入口
│       ├── main.go      # 服务器初始化和路由
│       └── web/         # 前端资源（嵌入到二进制）
│           ├── *.html   # 页面
│           ├── assets/  # CSS/JS/图片
│           └── ...
├── internal/
│   ├── auth/           # 认证逻辑
│   ├── config/         # 配置管理和加密工具
│   ├── database/       # 数据库初始化
│   ├── handler/        # HTTP 处理器
│   ├── middleware/     # 中间件（认证等）
│   ├── model/          # 数据模型
│   └── service/        # 业务逻辑层
├── build-alpine.sh     # 生产构建脚本
├── config.yaml         # 配置文件（需创建）
└── go.mod
```

## API 端点

### 公开接口

- `POST /api/register` - 用户注册
- `POST /api/login` - 用户登录
- `GET /api/config` - 获取客户端配置
- `GET /api/share/:token` - 查看分享的图片

### 认证接口（需登录或 API Key）

- `POST /api/images/generate` - 生成图片
- `POST /api/images/edit` - 编辑图片
- `POST /api/images/remove-bg` - 移除背景
- `GET /api/task/:id` - 查询任务状态
- `GET /api/gallery` - 获取用户图库
- `DELETE /api/gallery/:id` - 删除图片
- `POST /api/share` - 创建分享链接

### 管理员接口（需管理员权限）

- `GET/POST /api/admin/settings` - 系统设置
- `GET /api/admin/users` - 用户列表
- `POST /api/admin/user/admin` - 授予管理员权限
- `POST /api/admin/user/ban` - 封禁用户
- `POST /api/admin/user/reset-password` - 重置密码
- `GET/POST /api/admin/models` - 模型配置
- `POST /api/admin/test-api` - 测试 API 连接
- `GET /api/admin/api-keys` - API Key 管理
- `POST /api/admin/api-keys` - 创建 API Key
- `DELETE /api/admin/api-keys` - 删除 API Key

### API Key 访问（程序化接口）

使用 `Authorization: Bearer sk-proj-...` 头部：

- `POST /api/v1/images/generate`
- `POST /api/v1/images/edit`
- `POST /api/v1/images/remove-bg`

## 认证机制

应用支持两种认证方式：

1. **Session 认证**（浏览器用户）
   - 基于 Cookie 的会话管理
   - 自动处理登录状态

2. **API Key 认证**（程序化访问）
   - OpenAI 风格的 API Key：`sk-proj-{64位十六进制}`
   - 在请求头中携带：`Authorization: Bearer <api-key>`
   - 支持备注和使用追踪

## 安全特性

- **密码哈希** - 使用 bcrypt 加密存储
- **API Key 加密** - AES-256-GCM 加密存储敏感配置
- **防暴力破解** - 登录失败次数限制
- **用户封禁** - 管理员可封禁违规用户
- **Secure Cookies** - HTTPS 环境下启用
- **会话管理** - 安全的会话密钥配置

## 开发指南

### 添加新功能

1. 在 `internal/model/` 定义数据模型
2. 在 `internal/service/` 实现业务逻辑
3. 在 `internal/handler/` 添加 HTTP 处理器
4. 在 `cmd/server/main.go` 注册路由
5. 更新前端页面和 JavaScript

### 运行测试

```bash
go test -v ./...
```

### 代码风格

- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 错误处理遵循 Go 惯例

## 部署注意事项

- **修改默认密钥** - 生产环境必须修改 `session_secret`
- **HTTPS** - 生产环境启用 `secure_cookies: true`
- **目录权限** - 确保 `data/` 和 `uploads/` 目录可写
- **备份** - 定期备份 SQLite 数据库文件
- **反向代理** - 建议使用 Nginx/Caddy 提供 HTTPS

## 常见问题

**Q: 如何更改监听端口？**  
A: 修改 `config.yaml` 中的 `server.address` 字段。

**Q: 如何添加新的图片生成模型？**  
A: 在管理员面板的"模型配置"中添加，或修改 `config.yaml` 的 `defaults.available_models`。

**Q: 数据库在哪里？**  
A: 默认位于 `data/app.db`，可在配置文件中修改路径。

**Q: 如何迁移数据？**  
A: 复制 `data/app.db` 和 `uploads/` 目录到新环境即可。

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！

