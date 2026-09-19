# 又一局 · One More Round

记下每一局的输赢与相聚。熟人小组的私密对局日记，Vue 3 + TypeScript + Go + MySQL，前端嵌入单个 Go 二进制。

当前已接入真实持久化与业务接口：邮箱验证码和试用邀请、小组创建/加入/切换、成员管理与玩家关联、手动游戏、三种模式记局、草稿、编辑删除、照片授权访问、时间线和筛选统计。正式入口为 `/`。

**尚未完成的外部条件：**BGG 搜索导入及关联/合并未接入，当前返回明确未授权提示，可手动添加。真实邮件服务商和部署环境尚未配置；已使用本地 SMTP 收件箱完成联调。60 秒记录目标与 2—3 个真实小组的两周试用尚未进行。

需求见 [PRD](docs/PRD.md)，字段与权限见 [接口契约](docs/API.md)，实施记录见 [实施与验收](docs/IMPLEMENTATION.md)。

## 启动

需要 Go 1.27.1、Node.js >=22.12、pnpm 11.22.0、MySQL（本地验证版本 9.0.1）及 SMTP。MySQL 数据库和专用账号需提前准备；不要使用生产数据库做联调。

1. 将 `config.example.json` 复制为 `config.json`（已忽略入库），填写 `app.mysql`、`app.smtp`、`app.origin` 和私有照片目录 `app.photo_dir`。
2. `app.origin` 必须是浏览器实际访问的源，例如 `http://127.0.0.1:8080`，不能带末尾斜杠。公网部署要求 HTTPS，Cookie 自动启用 Secure。
3. 构建并启动：

```sh
make build
./bin/one-more-round --configFile ./config.json
```

启动执行 GORM 自动迁移，按 Model 补齐缺失的表、列和索引。业务数据、配置、照片均独立于二进制；生产无需 Node、Vite 或外部 `web/dist`。

SMTP 465 使用直接 TLS，其他外部服务器要求 STARTTLS。只允许本机 SMTP 收件箱使用明文连接；发送操作有连接/读写超时与请求取消。验证码不写入应用日志。生产请填写真实 SMTP 发件地址与账号。

## 发放首次试用邀请

使用同一配置创建一次性邀请，写入仅当前用户可读的文件，7 天有效：

```sh
./bin/one-more-round --configFile ./config.json --trial-output ./trial-invitation.txt
```

命令不覆盖已有文件。把文件内链接交给试用者即可；不要提交或公开该文件。已有账号无需再用试用邀请。组主登录后可在小组页生成/撤销小组邀请，首次注册资格和入组资格分开管理。

撤销尚未使用的试用邀请：

```sh
./bin/one-more-round --configFile ./config.json --revoke-trial-file ./trial-invitation.txt
```

## 开发

```sh
# config.json 中 app.origin 设置为 http://127.0.0.1:5173
make dev-api
# 另一个终端：
make dev-web
```

Vite 将 API 代理到 `127.0.0.1:8080`。切回单二进制访问前，将 origin 改为实际服务地址。服务端使用数据库 Cookie 会话，写操作校验 Origin，不启用任意跨域。

数据库表 Model 位于 `internal/infra/mysql/models`（`models.AllModels()` 供 `AutoMigrate` 使用），业务仓储使用 `gorm.io/gen` 生成的 `internal/infra/mysql/query`，按业务上下文拆在 `internal/infra/mysql/{identity,group,game,diary,photo}.go`，由 `RepositoryFactory` 汇总；Model ↔ Domain ↔ DTO 的转换集中在 `internal/converter`。API DTO 在 `internal/app/dto`，用例在 `internal/app/services`（每个边界一个 `XxxApp`），事务/邮件/文件等端口在 `internal/app/ports`；业务错误与随机标识分别在 `internal/errcode`、`internal/pkg/secure`。修改 Model 后执行：

```sh
make generate  # 等价于 go generate ./internal/infra/mysql；无需数据库连接
make swagger   # 依据 api/ 注解重新生成 cmd/swagger/docs
```

生成器版本锁定在 `go.mod`，`query/*.gen.go` 随源码入库，不能手改。表结构由启动时的 `AutoMigrate` 按 Model 维护；业务读写使用带 Context 的 Gen Query，事务内所有 Query 绑定同一连接。

HTTP 层按资源拆分：`api/api.go` 是路由表与中间件装配，`api/router/<资源>.go` 只做协议适配（用 `ginutils.RequestHandler` 绑定与校验），`api/common` 负责会话 Cookie 与统一错误响应，`api/middleware` 负责会话校验、来源校验、日志上下文与兜底恢复。业务失败同时给出稳定业务码（`code`）和语义化 HTTP 状态，前端以 `code` 判断成败、以 HTTP 状态处理传输层失败。接口文档由注解生成到 `cmd/swagger/docs`，开发环境访问 `/swagger/`，`app.is_prod=true` 时自动隐藏。

## 前端结构

`web/src/routers/index.ts` 统一创建路由与处理登录/小组入口；每个正式业务页对应独立的 `views/**/*.vue`，不通过 URL 分支渲染多种页面。新增与编辑对局分别编排提交，共用 `RoundForm`；回顾、桌游详情和玩家详情共用时间线及统计组件。

```text
web/src/
├── api/          # request.ts 统一请求；auth/group/game/round/photo 按业务封装
├── components/   # common、layout、rounds、groups 等可复用 UI
├── routers/      # 唯一路由实例及守卫
├── types/        # 服务端 DTO、共享业务类型；无请求和存储副作用
├── utils/        # 日期、结果展示、草稿清理、错误文本
├── views/        # 登录、小组、回顾、桌游、玩家和对局独立页面
├── composables/  # 小组上下文、异步请求与分页等共享响应式逻辑
├── stores/       # 会话和当前小组状态
├── styles/       # 共用视觉规则与表单样式
├── App.vue       # 仅承载 RouterView
├── main.ts       # 注册应用与路由
└── style.css     # 全局基础规则
```

跨层导入使用 `@/`。表单草稿仍按账号、小组和记录隔离；会话凭据仍由服务端 HttpOnly Cookie 管理。

## 验证

```sh
make check        # 锁定依赖、TS、生产构建、客户端测试、Go 测试/vet、空白检查
make build        # 同一前端构建顺序 + 二进制
make integration  # 需要 mysqld/mysqladmin/python3；创建独立临时 MySQL 并运行 race 测试，结束后关闭并清理
```

没有 `OMR_TEST_MYSQL` 时，普通 Go 测试会明确跳过 MySQL 集成用例；不能把跳过视为通过。`make integration` 不访问本机已有 MySQL 服务，不修改已有业务库。已有专用测试实例也可通过 `OMR_TEST_MYSQL=127.0.0.1:端口 go test -race ./...` 验证（需要 root 空密码，仅用于隔离测试实例）。测试创建并删除随机名称的独立数据库。

已验证：并发重复提交、同键异内容冲突、旧版本编辑/删除、非成员与移除成员隔离、验证码限制、跨组资源拒绝、共同获胜与统计分母、照片权限、事务失败回滚、临时照片清理。浏览器验证了登录、建组、手动游戏/玩家、草稿刷新恢复、保存与编辑、再记一局、照片上传，以及 320/390px 与桌面布局。

## 运行边界

- 当前为单实例、本地私有照片目录方案；备份 MySQL 和照片目录，配置及密钥单独保管。
- 未关联照片保留 7 天；启动及每小时清理。需要长时间保留的草稿应及时保存。
- 对局响应分页最多 100 条；统计在服务端按本组数据计算，尚未针对大规模历史数据优化。
- `/health_check` 为进程健康，`/metrics` 提供保存/上传/外部搜索的成功失败计数与耗时；生产应限制监控入口。
- 普通 API 错误、缺失静态资源和图片不返回 SPA 首页；前端深链接可刷新。
- 尚未部署，没有连接远端生产或发送真实外部邮件。
