# V0.1 本地试用接口契约

当前实现以手动桌游为入口，BGG 未授权时返回明确的 503；外部导入、关联 BGG 和游戏合并尚未实现。所有业务路径以 `/api/v1` 开头，成功为 `{"code":0,"data":...,"message":"success"}`，错误同时使用非 2xx HTTP 状态和非零 code。

## 身份与安全

- POST `/auth/code`：`email`，普通注册传 `invite`（试用邀请密钥）；通过小组邀请进入时传 `group_token`，先检查其有效性，无需试用邀请。验证码 10 分钟有效、60 秒重发、5 次尝试；每邮箱每小时最多 10 次、每来源 IP 每小时最多 30 次发送请求。失败邮件不返回验证码。
- POST `/auth/login`：`email`、`code`，可选 `group_token`。携带小组邀请时，在同一事务内锁定小组、校验邀请、验证邮箱、必要时注册、加入小组和创建会话；任一步失败整体回滚，错误验证码的尝试次数仍提交。小组邀请不会被消费，返回 `{id,email,group_id}`。不携带小组邀请时保留原有一次性试用注册/已有账号登录，返回 `{id,email}`。数据库只保存验证码摘要、会话摘要和邀请摘要。
- POST `/auth/group-invite`：无需登录，Body 为 `{token}`，仅返回有效邀请的小组 `{group_id,name}`。不返回组主、成员、记录或其他组内数据；令牌不放在请求 URL 中。
- GET `/me`：当前账号；POST `/auth/logout`：撤销服务端会话。
- `omr_session` Cookie 使用 HttpOnly、SameSite=Strict、生产 Secure；有效期固定 30 天，退出立即失效，不自动续期。localStorage 只保存草稿和当前小组偏好；sessionStorage 保存当前标签页的待处理邀请（最多 7 天）及邮箱/验证码发送时间（恢复期 10 分钟），不保存验证码。成功/取消清除相应邀请，退出清除邀请及登录进度。
- 所有非 GET/HEAD 业务请求的 Origin 必须与配置 `app.origin` 完全一致。命令行调用也必须带此 Header。不开放跨域访问。
- 业务错误：400 输入无效、401 未登录、403 权限/来源失败、404 不存在、409 并发/重复/关联冲突、429 请求过多、502 邮件失败、503 BGG 未接入。

## 小组与资料

- GET/POST `/groups`：列出所属小组 / 以 `name`、`player_name` 创建小组，并在同一事务创建、关联组主的玩家档案。
- POST `/join`：已登录账号凭 `token` 加入小组；重复加入幂等。未登录账号使用 `/auth/login` 的 `group_token` 完成注册/登录并加入。
- GET `/groups/:group`：小组、成员、玩家、游戏、可见的关联申请和近期地点。游戏与玩家按近期参与顺序优先。
- POST `/groups/:group/players`、`/games`：以 `name` 添加玩家或手动桌游。
- POST `/groups/:group/manage`：`action`、`target`、`value`。`claim` 申请关联 `target` 指定的已有档案；`claim-new` 以 `value` 为昵称创建新档案并同时申请关联。其余操作包括 `rename`、`alias`、`remove`、`transfer`、`approve`、`reject`、`revoke`；关联均由组主确认，服务端拒绝重复申请和重复绑定。
- GET/POST `/groups/:group/invites`：组主查看/创建邀请；7 天有效、可多人使用、可撤销，链接密钥只在创建响应中返回。链接仍为 `/join#令牌`，使用 fragment 避免进入 HTTP 访问日志。无效、过期和撤销返回不同中文提示，HTTP 400；数据库故障保留系统错误语义。
- GET `/groups/:group/export`：仅组主可下载 ZIP 备份，包含 `one-more-round.json`、`rounds.csv` 和已关联照片原图；导出包含仍在 7 天恢复期内的记录。
- 玩家名称、小组名称、游戏名称最多 255 个 Unicode 字符，与数据库字段一致。同组玩家名唯一；账号与玩家关联由数据库 `UNIQUE(group_id,account)` 约束及事务校验保证。

## 对局

- GET `/groups/:group/rounds`：`from`、`to`（包含边界的 YYYY-MM-DD）、`game`、`player`、`q`（回忆或地点）、`location`、`mode`、`outcome`、`has_photos`；`offset` 默认 0，`limit` 默认 30、最多 100。返回当前页 `items`，全筛选范围的 `total/games/players/stats/activity`。列表和统计共用一次筛选。
- GET `/groups/:group/rounds/recap?period=YYYY-MM|YYYY`：返回整月或整年的局数、游戏数、玩家数、记录时长、最常游戏/玩家与最多 12 张照片，不受列表分页影响。
- GET `/groups/:group/rounds/recycle-bin`、POST `/groups/:group/rounds/:id/restore`：查看和恢复 7 天内删除的对局；恢复请求携带当前 `version`。
- GET/POST `/groups/:group/rounds[/:id]`：详情 / 创建。
- PUT/DELETE `/groups/:group/rounds/:id`：编辑 / 移入回收站。删除体为 `{"version":1}`；删除后立即从列表和统计移除，7 天后后台永久清理。
- 创建必须带 `Idempotency-Key`（16—128 字符）；按账号和小组隔离。同键相同内容返回已有记录；同键不同内容返回 409；已删除的提交不会重新创建。
- 编辑携带读取时的 `version`；在事务中校验，成功递增。最近修改人和时间由服务端填写。

对局字段：`game_id/date/mode/outcome/players/winners/scores/teams/team_score/memory/location/minutes/photos/version`；返回另含 `id/author/updated_by/updated_at`。`updated_at` 是带时区的 RFC3339 时间戳；对局日期仅为日期字符串。

| 模式 | 结果与分数 |
| --- | --- |
| `individual` | 至少两人，`outcome=win/draw/unknown`。win 的 `winners` 至少一人，可共同获胜；scores 为玩家 ID 到分数的映射 |
| `team` | 至少两队，`outcome=win/draw/unknown`；每队 `id/name/players/score/winner`，win 可多队 winner=true；每人恰好属于一队。顶层 winners/scores 为空 |
| `coop` | 一人起，`outcome=win/loss/unknown`；填写 team_score，顶层 winners/scores/teams 为空 |

分数使用十进制字符串或 null，最多 12 位整数和 4 位小数，允许负数，超精度拒绝而不舍入。零传 `"0"`，空分数传 null。不接受指数表达式。`minutes` 是正整数或 null。回忆最多 500 个 Unicode 字符；照片 ID 不重复且最多 3 个。

每次敏感写入和成员撤销都在同一小组行锁下执行；保存对局、照片关联、幂等键同事务提交。Domain 校验模式与结果；API 不直接操作数据库。

当前查询适合邀请试用的小组数据量：后端读取本组对局后计算筛选与统计，再分页响应；未做大规模历史数据的 SQL 聚合优化，不应视为无限数据规模方案。

## 照片

- POST `/groups/:group/photos`：multipart `photo`，单张原文件不超过 2 MiB，仅 JPEG/PNG/WebP；服务端检查实际解码格式及最多 4000 万像素。
- 响应 `id`，保存对局时关联成功上传的 ID。未关联照片仅上传人可读；已关联照片仅当前小组成员可读。
- 每局最多关联 3 张照片，创建和编辑均在服务端校验。已有照片不会自动删除；超过 3 张的历史记录或草稿需先由用户移除多余照片再保存。
- GET `/groups/:group/photos/:id[?size=thumb]`：服务端鉴权后直接流式转发图片，已知大小时设置 `Content-Length`，保持 `Cache-Control: private, no-store`。不在读取链路缓存完整图片，响应结束、读取超时或请求取消时释放上游连接；已发送正文后发生错误只停止传输，不追加 JSON 错误。图片在上传时重编码去除 EXIF/GPS 元数据，长边最多 2400px，缩略图最多 480px，输出 JPEG。大图为处理后的图，不保留未经处理的原始上传。
- 失败上传可重试/移除，其余字段允许保存。运行期间每小时清理超过 7 天未关联的上传；启动时也执行一次。长期草稿中被清理的图片需要重新上传。
- 照片保存到配置的私有 OSS Bucket 与前缀。浏览器接口和照片 ID 不变，凭证及 OSS 地址不下发前端；网络/权限等 OSS 故障返回 503，只有对象确实不存在才返回 404。
- 上传前记录 `uploading` 状态，大小图均上传成功且重新验证成员身份后标记为 `ready`。失败上传补偿删除，取消请求也有独立且有界的清理期限；进程异常退出留下的上传记录由过期清理回收。
- 清理在小组事务锁内将照片标记为 `deleting`，该状态不能再读取或关联；事务外删除 OSS 对象，全部删除成功才移除元数据。失败状态保留到后续清理重试，避免网络故障遗留无法追踪的对象。照片从对局解除关联后重新计算 7 天保留期。
- 备份应同时覆盖 MySQL 元数据和 OSS 对象。不要给整个 `image/` 前缀设置 7 天过期规则，已关联的历史照片需长期保留；已有本地照片需在切换前单独迁移。

## 通知

- GET `/notifications`：返回当前账号最多 100 条新近站内通知及未读数，覆盖新成员加入、玩家关联申请/处理和 24 小时内到期的小组邀请。
- POST `/notifications/:id/read`：只能把当前账号自己的通知标为已读。

## 运维

表结构在启动时由 GORM `AutoMigrate` 按 Model 补齐缺失的表、列和索引，不使用版本化 SQL 迁移。运行时使用 goutils 的生命周期与关闭机制。

响应统一为 `{code, data, message}`：成功业务码为 `0`；失败返回稳定业务码（`1004xx`/`1005xx`），同时用 HTTP 状态表达协议语义（401 未登录、403 无权限、404 不存在、405 方法不支持、409 冲突、429 限流、502 邮件失败、503 外部依赖不可用）。业务码与 HTTP 状态是两套语义，前端以业务码判断成败、以 HTTP 状态处理传输层失败。

接口文档由 `swag init` 从 `api/` 的注解生成到 `cmd/swagger/docs`（随源码入库），开发环境挂在 `/swagger/`，`app.is_prod=true` 时返回 404。执行 `make swagger` 重新生成。
