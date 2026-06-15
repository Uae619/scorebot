# 查分 · ScoreBot

**一个简洁的成绩查询网页应用，支持好分数、七天网络、百分智平台。**

> 基于 [Xuuyuan/ScoreBot-Go](https://github.com/Xuuyuan/ScoreBot-Go) 二次开发，感谢原作者的优秀工作。

---

## 在线地址

🔗 **[chafen.dpdns.org](http://chafen.dpdns.org)**（移动端优先）

## Android 应用

📱 [下载 APK](https://github.com/Uae619/scorebot/releases/latest) — 一键安装，即开即用

---

## 功能

- 支持好分数（学生版/家长版）、七天网络、百分智三个平台
- 考试列表、单科成绩、总分排名、等第预估
- 答题卡原图查看（支持双指缩放）
- PWA 支持，可添加到手机主屏幕
- 触感反馈（需设备支持）
- 界面适配浅色/暗色模式，六色主题切换

---

## 本地运行

```bash
# 启动服务
CHAT_ADAPTER=http DATA_STORE=sqlite SQLITE_STORE_PATH=data.sqlite API_LISTEN=0.0.0.0:8080 ./scorebot
```

浏览器打开 `http://localhost:8080`

---

## 部署

- **前端**：EdgeOne Pages（静态托管）
- **后端**：阿里云函数计算 FC（Go 自定义运行时）
- **域名**：chafen.dpdns.org（Cloudflare DNS）

---

## 技术栈

Go · SQLite · 原生 HTML/CSS/JS · WebView APK

---

## 致谢

本项目基于 [Xuuyuan/ScoreBot-Go](https://github.com/Xuuyuan/ScoreBot-Go)（MIT License）修改而来。原项目为 QQ Bot 场景设计，本 fork 将其改造为移动端网页应用并持续维护。

---

## 声明

本工具仅供个人学习与研究使用。使用者应在查询前自行获取对应账号持有人的明确同意，并自行了解相关平台的使用条款。开发者不存储任何用户数据于远端服务器，所有账号信息仅缓存在用户本地或临时会话中。
