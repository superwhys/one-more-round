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

## 尚未验证

- 60 秒录入目标：需要真实用户参与，不能用编译通过或自动化操作代替。
- 2—3 个真实小组的两周试用：尚未进行。
- BGG 外部检索：授权未获批，未接入。
