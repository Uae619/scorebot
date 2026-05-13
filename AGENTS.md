# 准则

## 方案

Avoid degradation handling, fallback, hacks, heuristics, local stabilizations, or post-processing bandages that are not faithful general algorithms.
避免采用降级处理、兜底方案、临时补丁、启发式方法、局部稳定化手段，以及非严谨通用算法的后处理补救措施。

## 构建

最终的构建需要部署到Linux环境（阿里云函数计算FC）上，以下命令是经过验证可以跑通编译的指令，请使用其完成编译：

```bash
$env:GOOS="linux";$env:GOARCH="amd64";$env:CGO_ENABLED="0";go build -trimpath -ldflags="-s -w" -o main .
"$env:USERPROFILE\go\bin\build-fc-zip.exe" -output main.zip main
```

## 格式

无视gofmt的自动压紧格式。
