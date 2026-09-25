# 测试与验证

本文汇总仓库的测试入口、环境变量与已验证范围。除特别说明外，命令均在仓库根目录执行。

## 常规检查

```sh
make check        # 锁定依赖、Prettier 检查、TS 检查、生产构建、前端测试、Go 测试/vet、git diff --check
make build        # 同一前端构建顺序 + 二进制
make integration  # 创建独立临时 MySQL，运行 race 测试，结束后关闭并清理
```

`make check` 与 `make build` 都会先构建前端，因为部分 Go 测试依赖嵌入的 `web/dist`。

## MySQL 集成测试

普通 `go test ./...` 在没有 `OMR_TEST_MYSQL` 时会**明确跳过** MySQL 集成用例——**跳过不等于通过**，需要跑集成测试时不要只看常规测试的绿灯。

| 变量 | 作用 |
| --- | --- |
| `OMR_TEST_MYSQL` | 形如 `127.0.0.1:端口`，指向可用的隔离 MySQL 实例 |

`make integration` 会通过 `scripts/test-mysql.sh` 用 `mysqld` 与 `mysqladmin` 启动一个临时实例（需要 `mysqld`、`mysqladmin`、`python3`），随机端口、临时数据目录，结束后关闭并删除。它**不访问本机已运行的 MySQL 服务，也不修改已有业务库**，因此可以放心执行。

已有专用测试实例时，也可以直接指定：

```sh
OMR_TEST_MYSQL=127.0.0.1:端口 go test -race ./...
```

该方式需要实例的 root 账号为空密码，仅适用于隔离的测试实例。测试会创建并删除随机名称的独立数据库。

## 浏览器回归测试

需要先安装 Playwright 与 Chrome。以下测试默认跳过，设置对应变量为 `1` 后才运行。

| 变量 | 覆盖内容 |
| --- | --- |
| `OMR_BROWSER_TEST=1` | 小组邀请流程：新邮箱注册入组、刷新恢复、已有成员进入、加入第二个组、注销清理及邀请失效提示 |
| `OMR_PHOTO_BROWSER_TEST=1` | 照片限制与上传：每局最多 3 张、单张原始文件最多 2 MiB；覆盖前端 1600px 预压缩、顺序上传、繁忙重试、损坏图片、统一图片 URL、批量选择、编辑替换、旧草稿、直接 API 拦截，以及 320/390px 与桌面布局 |
| `OMR_SHARE_BROWSER_TEST=1` | 对局公开分享：已登录的分享控件与匿名响应式分享页 |

前两项通过 `make integration` 调用：

```sh
OMR_BROWSER_TEST=1 make integration
OMR_PHOTO_BROWSER_TEST=1 make integration
OMR_SHARE_BROWSER_TEST=1 make integration
```

小组邀请回归还需要已构建的二进制，用于验证前端资源嵌入：

| 变量 | 作用 |
| --- | --- |
| `OMR_TEST_BINARY` | 新二进制的绝对路径。测试会把它复制到**不含 `web/dist` 的临时目录**，用隔离配置与数据库启动，检查首页、邀请/登录深链接、嵌入资源，以及 API 错误与缺失资源是否正确分流 |

```sh
make build
OMR_BROWSER_TEST=1 OMR_TEST_BINARY="$PWD/bin/one-more-round" make integration
```

以下变量对上述浏览器测试通用：

| 变量 | 作用 |
| --- | --- |
| `OMR_PLAYWRIGHT_MODULE` | Playwright 不在默认模块路径时，用 `file:///绝对路径/playwright/index.mjs` 指向现有安装 |
| `OMR_BROWSER_ARTIFACTS` | 指定一个**已存在**的目录，测试会把截图写入其中 |

浏览器测试使用隔离数据库和仅在测试服务器中存在的内存收件箱，**不向外部邮箱发信**，照片写入隔离的临时目录，**不访问 OSS**。

## 照片内存回归

该用例单独运行，**不要加 `-race`**：

```sh
OMR_PHOTO_MEMORY_TEST=1 go test -run '^TestDisplayPhotoMemoryBudget$' -count=1 -v ./internal/infra/photos
```

它连续处理三张 1600×1600、16 位、2 MiB 的 PNG，采样额外 Go 堆峰值并要求不超过 100 MiB。这是该处理路径的堆内存预算，**不是整个进程 RSS 或服务器总内存的保证**。

比较不同格式的处理分配量：

```sh
go test -run '^$' -bench BenchmarkDisplayPhotoMemory -benchtime=1x -benchmem ./internal/infra/photos
```

## OSS 实连测试

需要真实凭证才能运行，默认不访问 OSS。

| 变量 | 作用 |
| --- | --- |
| `OMR_TEST_OSS_CONFIG` | 指向含 `app.oss` 的配置文件路径 |

```sh
OMR_TEST_OSS_CONFIG="$PWD/config.json" go test -count=1 -run '^TestOSSLive$' -v ./internal/infra/photos
```

测试会在配置的前缀下上传两张随机命名的图片，读取校验后删除，**不修改业务数据库**。首次接入 OSS 时必须以真实凭证跑通，不能用编译通过代替。

## 已验证范围

以下行为已有测试或浏览器操作覆盖：

- **一致性**：并发重复提交、同键异内容冲突、旧版本编辑与删除、事务失败回滚。
- **权限**：非成员与已移除成员隔离、跨组资源拒绝、照片访问权限。
- **身份**：验证码限制、邀请流程。
- **统计**：共同获胜、统计分母。
- **清理**：临时照片清理。
- **浏览器流程**：登录、建组、手动游戏与玩家、草稿刷新恢复、保存与编辑、再记一局、照片上传，以及 320/390px 与桌面布局。
- **本轮新增**：隔离 MySQL 竞态测试覆盖想玩清单的权限与跨组资源、BGG 合并后的活跃和回收站对局、统计与想玩状态，以及事务失败回滚；临时 mock 浏览器流程覆盖想玩清单、聚会相册和合并确认的页面操作与窄屏布局。

## 尚未验证

- 60 秒录入目标：需要真实用户参与，不能用编译通过或自动化操作代替。
- 2—3 个真实小组的两周试用：尚未进行。
- BGG 外部检索：配置 `app.bgg.token` 后由服务端搜索并导入；未配置时有效查询返回 503。2026-09-25 使用本地配置令牌只读请求官方搜索与详情接口，分别返回 HTTP 200；新页面的生产环境端到端流程尚未验收。手动条目关联和组主确认合并已通过本地单元及隔离 MySQL 测试。

## 微信小程序认证

`go test ./internal/infra/wechat ./api/middleware` 验证 code2Session 成功/无效凭证/超时/重定向/异常响应、配置校验、敏感错误脱敏，以及小程序 Bearer 与浏览器 Cookie 的 Origin 边界。

`make integration` 额外运行 `TestWechat*` 真实 MySQL 用例：首次复用已有邮箱账号、双向唯一绑定、邀请约束、并发注册仅一账号、邀请入组事务回滚、独立账号不合并、补绑新邮箱/会话轮换、验证码重复消费和错误尝试上限、小程序 HTTP 登录与 Bearer 写入/注销。测试微信 provider 使用确定性替身，不调用真实微信，不发送外部邮件。

真实 `wx.login`、生产 AppID/Secret、微信合法域名和真机页面需要在微信开发者工具及真机另行联调；上述测试不能替代微信平台实连验证。

### 开发者工具交互用 API fixture

前端构建完成后运行：

```sh
./scripts/mini-fixture.sh
```

服务固定监听 `http://127.0.0.1:8080`，端口被占用时直接失败，不会终止已有服务。脚本创建独立临时 MySQL，照片保存在测试临时目录；不读取项目私有配置，不连接生产 SMTP、OSS 或微信。账号、小组、对局、统计、绑定与授权均调用真实应用服务，只有外部微信 code2Session 和邮件投递使用测试替身。每次 `wx.login` 返回的非空 code 都映射到相同测试微信身份。

将小程序开发环境 API 地址设为该地址，并在开发者工具中关闭合法域名校验。可选两条测试路径：首次登录验证 `mini-existing@example.test`，使用已有的「开发者工具测试小组」（两位玩家、一个手动桌游）；或填写试用邀请码 `mini-ui-trial` 创建独立无邮箱微信账号，再创建小组、记局和补绑一个新邮箱。第二种路径之后绑定已有邮箱应明确失败，因为独立账号不合并。

下列入口**仅存在于显式启用的 `_test.go` fixture，生产二进制不包含**：

- GET `/__test__/fixture`：当前试用邀请码、已有邮箱和小组邀请。
- GET `/__test__/code?email=目标邮箱`：在小程序发送验证码后读取测试收件箱。
- POST `/__test__/stop`：正常停止 HTTP 服务并清理测试数据库、照片；重新执行脚本可获得全新数据。也可 Ctrl+C 停止。

测试进程最长运行 45 分钟。微信平台真正的 code2Session 授权、合法域名与真机能力仍需单独实连验证，fixture 不代表这些能力已验证。

启动 fixture 后，开启开发者工具服务端口，以 `cli auto --project "$PWD/mini" --auto-port 9420` 连接，再运行 `node mini/scripts/smoke.mjs`。脚本会检查 fixture 标识，避免误用其他环境。2026-09-25 本机已验证首次邮箱绑定、三种对局模式、负分/零分、原生图片处理上传/下载、评论、编辑、公开分享投影及撤销、回收站恢复、桌游架、相册和回顾卡片生成；未调用真实 code2Session，未向真实微信联系人发送分享。
