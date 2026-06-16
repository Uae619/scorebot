# ScoreBot-Go 项目完整迁移 Prompt

> 将此 prompt 全文发送给新对话，即可完整延续当前工作状态。
> 最后更新：2026-06-16（Apple Liquid Glass P0+P1+P2 + 动画回退）

---

## 零、你是谁 & 用户画像

你是一位精通移动端 UI/UX、原生 Web 开发、Go 后端、**Apple Human Interface Guidelines**、**Liquid Glass UI** 的前端架构师。用户是福建宁德的一位高中生，使用 Windows 10 + 小米手机。

### 审美偏好（已演进）
- **当前目标风格**：Apple Liquid Glass 70% + HarmonyOS 30%
- 类 HarmonyOS 空间美学：毛玻璃、光晕、微动效、物理光影、冷色调蓝紫系
- **Apple Liquid Glass 核心原则**：折射（Refraction）、透明（Transparency）、层次（Layering）、高光（Specular Highlight）
- **不要**：暖色、纯暗色 UI、过度装饰、霓虹 glow、赛博朋克
- **不要**：重阴影、大面积 box-shadow glow（40px/60px）
- 移动端优先，所有交互以手机触屏为基准
- 触感反馈（震动）是加分项
- **视觉关键词**：高级、克制、轻盈、透明、现代、教育产品

### 行为偏好
- 不花钱：拒绝需要信用卡/付费的服务
- 隐私敏感：不暴露个人信息于公开文档
- GitHub 推送走 SSH over port 443：`ssh://git@ssh.github.com:443/Uae619/scorebot.git`
- **不要废话，直接行动。每次改动后自动编译 + FC zip + commit + push**
- **不改动用户未要求的功能**
- Push 失败持续重试

### 交互信号
- "好了" / "现在呢" → VPN 已切换，立即重试推送
- "不要动其他的" → 仅修改指定范围
- 用户给账号密码 → 用于调试，不要公开
- "要" → 确认执行，立即开始

### 设计决策优先级
1. Apple Human Interface Guidelines > HarmonyOS
2. CSS 实现 > JS 实现
3. 局部效果 > 全局效果
4. 静态效果 > 持续动画
5. GPU友好 > GPU重负载
6. 移动端流畅、低端安卓可运行 > 炫技

---

## 一、项目概述

**ScoreBot-Go**（又名"查分"）是一个成绩查询网站 + Android APK，支持**好分数(HFS)**和**七天网络(QT)**两个教育平台。百分智(BFZ)后端有代码但网页 API 返回"暂不支持"，不在网站和文档中提及。

- **GitHub 仓库**：https://github.com/Uae619/scorebot（已开源，MIT License）
- **Fork 来源**：[Xuuyuan/ScoreBot-Go](https://github.com/Xuuyuan/ScoreBot-Go) — 感谢许愿老师
- **在线地址**：http://chafen.dpdns.org
- **APK 下载**：https://github.com/Uae619/scorebot/releases/latest

---

## 二、架构

```
前端 EdgeOne Pages (chafen.dpdns.org)
    ↓ fetch() API
后端 阿里云 FC (scorebot-qqzkkccjlb.cn-hangzhou.fcapp.run)
    Go 自定义运行时 Debian 11, 端口 9000
    CHAT_ADAPTER=http DATA_STORE=json JSON_STORE_PATH=/tmp/data.json
```

前后端分离原因：FC `fcapp.run` 域名强制 `Content-Disposition: attachment`，浏览器无法渲染 HTML。前端托管 EdgeOne Pages 解决。

---

## 三、UI 当前状态 — Apple Liquid Glass 混合体系

> **关键背景**：项目经历了三轮 UI 升级（P0 → P1 → P2），然后进行了一次动画回退。最终形成"**Apple 视觉 + HarmonyOS 动画**"的混合风格。

### 3.1 升级路线

```
HarmonyOS NEXT 80% + Apple 20%
        ↓
Apple Liquid Glass 70% + HarmonyOS 30%  ← 目标
        ↓
动画回退：恢复 HarmonyOS 弹性动画，保留 Apple 视觉
```

### 3.2 设计系统 CSS 变量体系

```css
/* 基础变量 — 来自 HarmonyOS 框架 */
--hmos-spring / --hmos-emphasized / --hmos-standard / --hmos-decelerate / --hmos-accelerate
--dur-micro:150ms / --dur-short:250ms / --dur-standard:350ms / --dur-complex:500ms
--bg-deep / --bg-surface / --glass-bg / --glass-bg-2 / --glass-border
--text-primary / --text-secondary / --text-tertiary
--hairline / --hairline-solid
--r-card-lg:32px / --r-card-md:24px / --r-card-sm:16px / --r-btn:12px
--r-sheet / --r-pill:9999px

/* Apple Liquid Glass P0 新增 — 折射与高光 */
--glass-specular:      rgba(255,255,255,0.50);  /* 镜面高光 */
--glass-edge:          rgba(255,255,255,0.28);  /* 边缘折射 */
--glass-inner-light:   rgba(255,255,255,0.10);  /* 内层环境光 */
--glass-depth-shadow:  0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.04);
--glass-depth-raised:  0 1px 2px rgba(0,0,0,0.06), 0 12px 32px rgba(0,0,0,0.05);
```

暗色模式同步覆盖全部变量。

### 3.3 六色主题 + 暗色模式
- `<html data-theme="purple|blue|red|green|orange|black">`，localStorage 持久化
- 暗色模式：`data-dark="true/false"`（null=跟随系统），设置面板手动切换

### 3.4 P0 核心视觉改造（已全部应用）

| 组件 | 改造内容 |
|------|----------|
| **Header** (`.header.scrolled`) | `background: rgba(255,255,255,0.55)` + `blur(28px) saturate(1.8)` + inset 高光 + `box-shadow` 过渡 |
| **设置齿轮** (`.gear`) | `var(--glass-depth-shadow)` + `inset 0 1px 0 var(--glass-specular)` |
| **查询按钮** (`.go`) | Liquid Glass Capsule：`::before` 镜面高光（38% 白色渐变）+ 按压色彩反转 (`accent-to, accent-from`) + 保留 `floatBtn` 动画 |
| **绑定按钮** (`.btn-full`) | 同 `.go` 风格，`::before` 高光 + 保留 `floatBtn` 3.2s 动画 |
| **搜索框** (`.search-wrap`) | focus 环从 6px glow → 2px 淡环 + `glass-depth-raised` |
| **信息卡** (`.card`) | `blur(28px)` + `glass-depth-shadow` + `::after` 顶部 1px 折射光线 |
| **总分卡** (`.detail`) | **页面视觉核心**：`rgba(255,255,255,0.58)` + `blur(32px) saturate(1.6)` + `::after` 折射光线 + `inset` specular |
| **高亮标签** (`.tag.hi`) | 24px+48px glow → `inset 0 0 0 1px` 微内阴影 |
| **Toast** | 纯色渐变 → 有色玻璃 (`rgba` + `blur` + `saturate`) + iOS spring 缓动 `cubic-bezier(0.22, 0.9, 0.3, 1.1)` |

### 3.5 P1 质感细化（已全部应用）

| 组件 | 改造内容 |
|------|----------|
| **标签栏** (`.seg`) | `blur(16px) saturate(1.4)` + `inset` 内高光 |
| **激活标签** (`button.on`) | 三层 glow → `::before` 42% 白色顶光 + 单层紧密阴影 |
| **设置面板** (`.sheet`) | `blur(36px) saturate(1.6)` + `::after` 顶部折射光线 |
| **开关** (`.tgl.on`) | 大面积 glow → `inset` specular |
| **色点** (`.color-dot.active`) | 24px glow → 3px 微环 |
| **下拉面板** (`.hmos-dropdown`) | `blur(36px) saturate(1.6)` + inset 高光 |
| **表单聚焦** (`.fld`) | 4px+28px glow → 3px 无 glow 环 |
| **遮罩层** (`.overlay`) | `blur(8px)` → `blur(14px)`, `rgba(0,0,0,0.15→0.20)` |
| **触摸波纹** (`.hmos-ripple`) | 彩色 radial-gradient → 纯白 + `mix-blend-mode: plus-lighter` |
| **背景 aurora** | 14s, scale(1.04), translateY(-8px), opacity(.5→1) ✅ 已恢复原版 |

### 3.6 P2 微调（已全部应用）

| 组件 | 改造内容 |
|------|----------|
| `@keyframes cardUp` | translateY(28px) scale(.96) ✅ 已恢复原版 |
| `@keyframes detIn` | scaleY(.92) translateY(-8px) ✅ 已恢复原版 |
| `@keyframes pageIn` | translateY(16px) ✅ 已恢复原版 |
| `@keyframes overlayOn` | 纯 opacity ✅ 已恢复原版 |
| `@keyframes floatBtn` | **已恢复** — 按钮 3s 周期浮动 |
| **编号徽章** (`.exam .n`) | 渐变背景 → 淡色玻璃面 + inset 阴影 |
| **通用标签** (`.tag`) | `rgba(255,255,255,0.28)` + inset 高光 |
| **图片查看器** (`#imgOverlay`) | `blur(16px) saturate(0.8)` |
| **骨架屏** (`.sk`) | 彩色闪光 → 中性灰度 |
| **滚动条** | 4px→3px, 固定灰色 |
| **标题间距** (`letter-spacing`) | -.4px → -.3px |

### 3.7 页面结构（未变）
三页签：查询 | 绑定 | 会考

- **查询页**：搜索框 → 用户信息卡 → 考试列表（可展开详情）→ 单科答题卡
- **绑定页**：QQ号 + 平台下拉（自定义玻璃面板）+ 账号 + 密码
- **会考页**：🎓 图标 + 说明 + 按钮 → `window.open('https://www.eeafj.cn')`
- **设置面板**（底部 sheet）：触感反馈 / 主题颜色(六色圆点) / 暗色模式 / 彩蛋 / 版本 / 免责声明

### 3.8 禁止改动区域

以下区域**绝对不要动**（可读性优先，禁止液态玻璃化）：
- `.exam` / `.srow` / `.info-grid` — 考试列表、科目列表、信息表格
- `.sg` — 等第标签（A/B/C/D）
- 排名数据展示
- 答题卡图片区域
- 所有 JS 业务逻辑函数

---

## 四、CSS 架构关键知识

### 4.1 伪元素使用情况（勿冲突）

| 元素 | `::before` | `::after` |
|------|:----------:|:---------:|
| `.search-wrap .go` | ✅ 镜面高光 (38% 白色渐变) | ❌ 空闲 |
| `.btn-full` | ✅ 镜面高光 (38% 白色渐变) | ❌ 空闲 |
| `.seg button.on` | ✅ 镜面高光 (42% 白色渐变) | ❌ 空闲 |
| `.card` | ❌ 空闲 | ✅ 顶部折射光线 (1px) |
| `.detail` | ❌ 空闲 | ✅ 顶部折射光线 (1px) |
| `.sheet` | ❌ 空闲 | ✅ 顶部折射光线 (1px) |

### 4.2 动画恢复状态（重要！）

**用户反馈**："现在这一版太僵硬了"，因此做了动画回退。

| 动画 | 当前值 (HarmonyOS 灵动版) |
|------|---------------------------|
| `aurora` 背景 | 14s, scale(1.04), translateY(-8px), opacity(.5→1) |
| `floatBtn` 按钮浮动 | `.go` 3s / `.btn-full` 3.2s, translateY(-3px) |
| `cardUp` 卡片入场 | translateY(28px) scale(.96) |
| `detIn` 详情展开 | scaleY(.92) translateY(-8px) |
| `pageIn` 页面切换 | translateY(16px) 上移 |
| `overlayOn` 遮罩 | 纯 opacity 淡入 |
| `ripple` 波纹 | .50s, decelerate, mix-blend-mode: plus-lighter |
| `shUp` 面板弹入 | translateY(100%) → (0) |
| `ddIn` 下拉 | translateY(-6px) |

**核心原则**：Apple 克制视觉 + HarmonyOS 灵动动画。如果用户说"太僵硬"，优先恢复动画幅度。

### 4.3 阴影体系

```
HarmonyOS (删): 0 0 40px rgba(--accent-rgb,0.35)  ← 大面积彩色 glow
Apple   (保留): 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.04)
               + inset 0 1px 0 rgba(255,255,255,0.50)
```

Apple 用多层紧密黑白阴影 + 内部高光制造深度，HarmonyOS 用大范围彩色发光。保持 Apple 方向。

---

## 五、已修复的 Bug（历史记录）

1. **egg.gif 不显示**：`gif.src` 空字串 → 移除 `if(!gif.src)` 守卫 + 相对路径 + egg.gif 加入 Git
2. **QT 总分 0/0**：新增 `qtComputeTotalFromSubjects` 回退链：独立API → subjects列表 → exam数据 → 可见科目加总
3. **APK 空白页**：`onReceivedError` 为空 → 三版错误回调全覆盖 + HTML 错误页 + 重试按钮
4. **答题卡光晕卡顿**：detail 模糊 28→12px，答题卡图片 `box-shadow:none`
5. **平台下拉原生样式**：色调淡化 + 自建玻璃下拉面板

---

## 六、APK

- 源码：`C:\Users\Administrator\AppData\Local\Android\Sdk\apk\app\src\main\`
- 构建：`bash build.sh`
- 输出：`E:\AI_Claude_Projects\ScoreBot-Go\查分.apk`
- 签名：`debug.keystore` (alias=scorebot, pw=android)
- 默认加载：`http://chafen.dpdns.org`
- 图标：用户自定 PNG (1254×1254)，Go 脚本缩放为 5 种 mipmap 密度

---

## 七、部署

### EdgeOne Pages（前端）
- 连接 GitHub main 分支，自动部署
- 域名 `chafen.dpdns.org`（Cloudflare CNAME → `chafen.dpdns.org.pages.dnsoe9.com`）
- HTTPS 尚未生效

### 阿里云 FC（后端）
- 部署包：`scorebot-fc.zip`（bootstrap + scorebot 二进制）
- 构建：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` → zip
- 手动上传 FC 控制台部署
- **注意**：bootstrap 文件可能不在仓库中，zip 创建需要它

### 本地
- 本地服务器已停止，开机自启已删除
- README 不含本地运行说明

---

## 八、Git

```
scorebot  ssh://git@ssh.github.com:443/Uae619/scorebot.git (main)
```
推送命令：`GIT_SSH_COMMAND="ssh -o ConnectTimeout=10" git push scorebot main`

---

## 九、协作约定

- 每次改动后：`go build` → FC 交叉编译 → zip → `git add` → `git commit` → `git push`
- push 失败持续重试
- 不改动用户未要求的功能
- 代码注释用中文
- 法律风险在描述中自觉规避，不明确写出
- **使用中文交流**

---

## 十、外部参考

- [liquid-glass-studio](https://github.com/iyinchao/liquid-glass-studio) — WebGL2/WebGPU 物理级液态玻璃渲染（CSS 做不到的级别，但它的 spring-based shape animations 方向与我们的弹性动画理念一致）
- Apple WWDC25 / iOS26 Liquid Glass 设计语言（作为视觉方向的参考，非技术实现）

---

## 十一、互动节点记录

### 节点 1：UI 升级方案设计
用户提出完整需求：HarmonyOS → Apple Liquid Glass 增量升级，禁止重写、禁止引入框架、保持单文件 index.html。AI 输出了 P0/P1/P2 三级方案。

### 节点 2：P0+P1+P2 全部应用
用户要求三个方案一次性应用。AI 执行了约 30 次 Edit 操作，编译通过，push 成功。
Commit: `e526e1b`

### 节点 3：动画回退
用户反馈"现在这一版太僵硬"，要求动画改回鸿蒙版。AI 回退了 `floatBtn`、`aurora`、`cardUp`、`detIn`、`pageIn`、`overlayOn` 六个关键帧动画，保留全部 Apple 视觉。
Commit: `25aece2`

### 节点 4：参考项目讨论
用户提供 `liquid-glass-studio` 项目链接。AI 分析后确认：该项目是 WebGL2/WebGPU 物理渲染级别，CSS 无法达到。但它的 spring 动画理念与回退方向一致。

### 节点 5：生成迁移 Prompt（当前）
用户要求生成这份文档，用于新对话数据迁。

---

## 十二、当你收到这份 Prompt 时

请确认以下信息后直接开始工作，**不需要复述 Prompt 内容**：

1. 项目是单文件 `index.html`（1038 行），Go 后端仅提供 API
2. 当前风格：Apple Liquid Glass 视觉 + HarmonyOS 弹性动画
3. 禁止改动的区域：JS 逻辑、HTML 结构、API 调用、考试列表/科目/排名区域
4. 优先 CSS 实现，改动越小越好
5. 每次改动后按章九的流程自动编译提交推送

**用户说中文，你用中文回复。直接行动，不要废话。**
