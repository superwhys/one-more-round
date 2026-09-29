# authkit 身份认证迁移

本次使用本地 `github.com/miebyte/authkit`，通过仓库根目录 `go.work` 引用 `../../miebyte/authkit`。认证数据使用该包的 MySQL Model；邀请资格、小组成员和业务数据仍由「又一局」维护。

迁移脚本入口是 [`scripts/migrations/authkit/main.go`](../scripts/migrations/authkit/main.go)，用 `go run ./scripts/migrations/authkit` 执行。脚本直接引用 authkit 的 Model 创建新表，避免另抄一份 SQL DDL 与包定义漂移。脚本只在显式传入 `-apply` 后写入，默认只读检查。不要在应用接收请求时执行迁移。

## 数据映射

| 旧数据 | 新数据 | 处理方式 |
| --- | --- | --- |
| `omr_users.id` | `auth_accounts.id` | 原值保留；新增 `username` 为 NULL |
| `omr_users.email` | `auth_bindings(method='email')` | `identifier=email`、`account_id=id`；NULL 邮箱不建立绑定 |
| `omr_wechat_accounts` | `auth_bindings(method='wechat')` | `identifier=openid_hash`、`account_id=user_id`，包括未绑定的占位行 |
| `omr_sessions` | `auth_sessions` | `user_id→account_id`；摘要和过期时间原样复制 |
| `omr_challenges` | `auth_challenges` | 保留邮箱、验证码摘要、发送/过期时间、尝试次数、可用状态 |
| `omr_rates` | `auth_rates` | `count→hits`，保留原限流标识和窗口开始时间 |
| `omr_trials`、`omr_invites` 和其他业务表 | 保持现状 | 不复制、不删除、不修改账号引用 |

账号 ID 保持不变，因此成员、玩家关联、照片归属、评论和对局记录内的账号引用无需改写。旧程序与 authkit 都使用 32 字节随机数的十六进制会话令牌，以及 SHA-256 摘要；迁移后未到期的现有会话可继续使用，原过期时间不会延长。过期会话也按原值复制，仍然不能认证。验证码摘要和邮箱/IP 限流标识算法同样一致。

**邀请码协议变化：**新版发码只提交邮箱；新账号在登录请求中提交 `invite` 或 `group_token`，已有账号无需邀请。旧验证码本身会保留，但不再迁移或读取 `omr_challenges.invite_hash`。升级前已经收取验证码的新用户需在新版登录页面保留或重新填写邀请码；旧客户端应刷新后使用新接口。此次不创建 `omr_registration_intents`；若此前开发过程中已创建该表，新应用和迁移工具不再使用它，也不会自动删除。

**微信兼容边界：**旧表按 `(app_id, openid_hash)` 唯一，新 authkit 只按 OpenID 摘要唯一，且一个账号只能绑定一个微信身份。因此旧表存在数据时必须提供当前小程序的 `-wechat-app-id`。任何其他 AppID 的行、重复 OpenID 摘要或一账号多微信绑定都会中止，不能自动丢弃 AppID 或合并账号。若实际存在多应用数据，先明确绑定归属并制定单独迁移方案。

## 执行顺序

1. 在备份恢复出的独立数据库演练，确认本地 authkit 包、工作区配置和新版本构建可用。
2. 安排维护窗口，停止所有旧版、新版实例及其他认证写入者。对整个业务库做一致性备份，并确认能够恢复。保留旧二进制与旧配置。
3. 当前工作区迁移入口保留了本地调试连接设置；用于其他环境前，应将入口调整为 `openDatabase(os.Getenv("OMR_AUTH_MIGRATION_DSN"))`，再按以下流程使用环境变量提供 DSN；不要将数据库密码放入命令行参数、仓库文件或终端历史。以下以已有的受保护环境变量为例：

   ```sh
   export OMR_AUTH_MIGRATION_DSN="$MAINTENANCE_MYSQL_DSN"
   go run ./scripts/migrations/authkit -wechat-app-id '实际小程序 AppID'
   ```

   DSN 使用 MySQL 驱动格式，例如 `user:password@tcp(host:3306)/database`。必须明确数据库名，脚本使用 UTC。没有微信数据时可以省略 `-wechat-app-id`。预检不建表、不写数据，既有目标表必须为空。

4. 预检通过后执行同一个入口：

   ```sh
   go run ./scripts/migrations/authkit -wechat-app-id '实际小程序 AppID' -apply
   unset OMR_AUTH_MIGRATION_DSN
   ```

5. 在仍隔离公共流量的环境启动新版本，验证现有 Cookie 会话、邮箱验证码登录、微信原账号登录、邀请注册/加入小组、退出登录、成员与历史对局。确认成功后恢复流量；旧版服务不能再连接该库提供认证。

`-apply` 所需数据库权限包括读取旧表、创建/变更新认证表和插入新表。脚本不执行删除表/删除列，也不需要 DROP TABLE 权限。只读预检只需元数据与 SELECT 权限。

## 失败、重试与回滚

- 预检核对邮箱规范化、摘要格式、限流/验证码状态、微信歧义，以及认证和已存在业务表中的账号引用。引用按字节匹配，避免旧库不区分大小写而新库使用二进制排序规则时遗漏不一致的账号 ID。发现问题只报告类别和行数，不打印邮箱、摘要或业务正文。
- MySQL DDL 不能与 DML 一起回滚。脚本先预检，再根据 Model `AutoMigrate`，然后在一个 SERIALIZABLE 事务中再次预检、复制所有数据并核对行数。任何数据写入失败会回滚全部导入，可能保留已创建的空表；修复原因后可以重试。
- **成功后再次执行会明确拒绝导入。**只要任意新认证表已有数据，脚本就中止；不使用 `REPLACE`、`INSERT IGNORE` 或覆盖式 upsert。这能避免从旧表重新导入已注销会话、已消费验证码，或者覆盖新账号/新绑定。该脚本不是新旧表同步工具，也不支持把旧用户合并到一个已经有数据的 authkit 库。
- 开始恢复公共流量之前若必须回滚，应停止新版本并将整个数据库恢复到维护窗口的备份，然后运行旧版本。新版本已经接收写入后，旧表不再完整反映账号、验证码和会话状态；直接切回旧二进制会丢失这些变化或恢复已撤销权限，必须先制定数据对账与恢复方案。
- 旧表保留用于核验。确认不再回滚并满足备份保留要求后，可另外审查删除 `omr_users`、`omr_wechat_accounts`、`omr_sessions`、`omr_challenges`、`omr_rates` 的破坏性 SQL。本次脚本不包含清理动作，不能删除 `omr_trials` 或 `omr_invites`。

## 验证

迁移用例位于 `scripts/migrations/authkit/main_test.go`，通过 `OMR_TEST_MYSQL` 指向一次性 MySQL 实例，测试自行创建并销毁随机数据库，不读取业务 DSN。覆盖：只读预检无建表、账号/微信/验证码/会话/限流保留、旧会话与验证码通过真实 authkit 校验、重复执行不复活注销会话、多 AppID/孤儿引用/大小写不一致/未规范化邮箱拒绝，以及导入中途失败时全量回滚后可重试。完整宿主用例还验证旧小组与历史对局继续可读、对局 JSON 不变，以及迁移前发出的验证码在登录请求携带试用邀请后仍可完成注册。

仓库已有的一次性 MySQL 测试入口可以执行这些用例（需要本机 `mysqld`、`mysqladmin`，并先完成仓库要求的前端构建）：

```sh
./scripts/test-mysql.sh -run '^TestAuthMigration' -v
```

若已有独立临时测试实例，也可仅执行迁移包：

```sh
OMR_TEST_MYSQL='127.0.0.1:临时端口' go test -race -count=1 -v ./scripts/migrations/authkit
```
