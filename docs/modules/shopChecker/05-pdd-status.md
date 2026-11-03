# PDD（拼多多）价格采集进度

> 状态：暂停，等待选择稳定数据源
> 最后更新：2026-11-03

## 1. 当前结论

PDD 网页端价格采集**已能登录并首次读到真实价格**，但随后会被 PDD 的价格遮罩和风控拦截，不适合作为稳定数据源。

- 商品：`1017448128382`（滴露衣物除菌液 1500mL）
- 首次页面读取：
  - 大促价：`¥49.12`
  - 单独购买：`¥100.80`
  - 发起拼单：`¥49.12`
- 目标口径：**补贴后到手价**，该商品当前即 `¥49.12`

## 2. 登录态

- 账号：`login3rd` PDD 账号 `id=5`
- 已通过远程浏览器重新登录，Cookie 已回填：
  - `status=active`
  - `verified_status=valid`
  - 新 `PDDAccessToken` / `pdd_vds` 已保存
- 登录浏览器 profile：`%LOCALAPPDATA%\login3rd-chrome`
- 采集器 profile：`%LOCALAPPDATA%\go-backend-core\browser-profiles\pdd`
- 两者 profile 不一致，采集器即使注入 Cookie，也经常只拿到页面壳；PDD 对设备指纹/浏览器环境敏感。

## 3. 已确认的 PDD 限制

1. **价格遮罩**
   - 首次读取后，页面价格变为 `大促价 ¥4?.?2`，并显示“前往APP查看价格”。
   - 数字被替换为 `?`，不能从 DOM 还原完整价格。
2. **安全验证**
   - 分享短链 `?ps=...` 在多次访问后会 307 到 `psnl_verification.html`。
   - 说明 PDD 已对当前自动化访问触发风控。
3. **接口加密**
   - `api/oak/integration/render` 返回 `encrypt_status=3` + 100KB `encrypt_info`，页面在前端解密。
   - 直接解析原始响应拿不到明文价格。
4. **购物车不可用**
   - `cart.html` / `cart2.html` 均 302 到 `portal.html?redirect_from=/cart.html`，移动 Web 购物车不可用于采集。
   - 因此 PDD **不做购物车优先**。

## 4. 已实现的代码

- 浏览器等待价格区域渲染（最多 15 秒），避免只拿到页面壳。
- DOM 主价格解析：
  - 标签：`大促价`、`百亿补贴价`、`补贴后到手价`、`补贴后`、`补贴价`、`券后价`、`到手价`、`活动价`、`拼单价`、`发起拼单`
  - `单独购买` 作为参考原价，不作为到手价
- 遮罩检测：
  - 价格区域出现 `?` / `？` 或页面出现“前往APP查看价格”时返回错误，不写入错误价格
- 安全验证识别：
  - `psnl_verification.html` 归类为风控/限流，触发 6 小时退避
- PDD 请求串行执行，最小间隔 10 秒
- 分单位价格修正：整数字段按分转元（`99 → ¥0.99`）
- 登录失效时 `next_check_at` 至少推迟 6 小时，避免每小时空转

相关提交：

```text
dfafe1a  fix(shopchecker): PDD 登录失效退避与价格分单位修正
00164ba  feat(shopchecker): PDD 页面主价格解析与遮罩检测
e138f52  fix(shopchecker): PDD 等待价格渲染并识别安全验证
```

## 5. 当前商品状态

- 商品 `5c365313-d9b3-486f-8a7b-b18d1952e028`
- `last_price=0`
- 最新错误：`pdd: 未在页面中解析到有效价格`
- 没有把 `4?.?2` 误存成 `4.00`
- 下次检查：按 6 小时间隔

## 6. 后续建议

### 方案 A（推荐）：多多进宝/开放平台 API

- 用商品 ID 查价格/库存，不依赖网页 DOM，不受价格遮罩影响。
- 需要：多多进宝账号，或开放平台 `client_id` / `client_secret` / `access_token`。
- 待确认：接口权限、商品 ID 映射、是否支持当前商品。

### 方案 B：复用登录浏览器 profile

- 让采集器使用 `login3rd-chrome` 同一个 profile，或通过 `CHROME_REMOTE_URL` 连接登录浏览器。
- 可解决 profile 不一致导致的页面壳问题，但仍会受价格遮罩/安全验证影响。
- 适合作为临时验证，不适合长期稳定采集。

### 方案 C：App API

- 使用 `PDDAccessToken` + `anti_content` 调 App 接口。
- 需要逆向接口和风控参数，维护成本较高。

## 7. 待确认

1. 是否有多多进宝/开放平台账号或 API 凭证？
2. 是否接受 PDD 价格仅在未被遮罩时更新（可能不稳定）？
3. 是否需要继续投入 App API 逆向？
