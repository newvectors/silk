# 可选 CPU 性能采集（Windows / macOS / Linux）

仅主动设置 `SILKIDE_CPU_PROFILE` 时启用；默认采集 **30 秒**后自动停止、刷新并关闭文件。
`SILKIDE_PROFILE_SECONDS` 可设置为 5–120 秒；越界值限制到范围内，非法文本回退为 30 秒。
提前正常退出 IDE 也会刷新关闭。没有设置输出路径时，不启动采集定时器，不读取诊断内存统计。

## Windows PowerShell

在包含新版本 `silkide.exe` 的目录执行：

```powershell
$env:SILKIDE_CPU_PROFILE = Join-Path $PWD "silkide-cpu.pprof"
$env:SILKIDE_PROFILE_SECONDS = "30"
try {
    Start-Process -FilePath .\silkide.exe -Wait -RedirectStandardError .\silkide-profile.log
} finally {
    Remove-Item Env:SILKIDE_CPU_PROFILE -ErrorAction SilentlyContinue
    Remove-Item Env:SILKIDE_PROFILE_SECONDS -ErrorAction SilentlyContinue
}
```

打开后立即重现卡顿（打字、拖动、静置），等待日志出现 `CPU profile: finished`。
**30 秒后只停止采集，IDE 不会被关闭**；正常关窗后 PowerShell 返回。
输出目录必须已存在且可写；指定的旧同名 profile 文件会被覆盖，请每轮使用不同文件名。

## macOS / Linux

```sh
SILKIDE_CPU_PROFILE="$PWD/silkide-cpu.pprof" \
SILKIDE_PROFILE_SECONDS=30 ./bin/silkide 2>silkide-profile.log
```

macOS 应用包也可将可执行路径替换为 `./bin/SilkIDE.app/Contents/MacOS/silkide`。
Linux 需在实际图形桌面会话中运行。不要把 macOS/Linux 的结果当成 Windows 真机结果。

## 提供证据

记录本次操作和卡顿发生时间，提供 `.pprof`、对应日志，以及**相同构建的可执行文件/版本**。
日志只增加平台架构、IDE/Go/构建摘要、开始与结束内存统计、结果路径；不自动转储文档、环境变量或堆，也不启动 HTTP 服务。
CPU profile 可能包含函数名及源码路径，分享前先检查。

安装 Go 后可本地分析，不需要网络服务：

```sh
go tool pprof -top ./silkide.exe ./silkide-cpu.pprof
```

采集从 `main` 启动阶段开始，不包含此前的包初始化；Go CPU 样本也不能替代原生 GPU/窗口系统跟踪。
若进程被强杀或崩溃，最后尚未刷新的数据可能丢失；请优先等自动停止或正常退出。

## Windows 性能排查与本轮修复（2026-09-12）

以下是**代码路径已确认的缺陷**，不是 Windows 真机 CPU 归因结论：

- 旧 `serviceUIFrame` 在每条消息结束后尝试推进动画；有动画时令所有可见窗口全量失效。一帧超过约 16.7 ms 时，`WM_PAINT → serviceUIFrame → InvalidateRect → WM_PAINT` 可以连续运行，没有给慢帧留休息时间。
- 自定义模态窗口使用 `PeekMessage + Sleep(1)` 空轮询；现在使用阻塞 `GetMessage`。同时正确区分成功、`WM_QUIT` 和 `-1` 错误，避免错误后重派发旧消息。
- 隐藏祖先/移除整棵子树后的 Spinner、忙碌 ProgressBar 曾继续保留全局动画；现在按可见性/挂载变更停止或恢复，不逐帧扫描子树。
- 标题每 500 ms 检查修改状态，旧 `Modified → Text → strings.Join` 会反复分配全文；大纲与状态栏还在每次绘制遍历代码。现在按文本修订缓存全文、修改状态、字符数及大纲。

### 绘制策略

- **静态 0 主动帧**：不为凑 1 FPS 空画；输入、系统曝光、实际内容更新仍可触发绘制。
- 默认空闲持续动画上限 **25 FPS**，键鼠操作后 **300 ms** 内上限 **60 FPS**。这限制的是绘制，不是消息/输入处理。
- 每窗慢帧后至少休息：空闲为该帧耗时的 3 倍、交互为 1 倍，同时满足 FPS 上限。它是绘制预算，不是对整进程 CPU 百分比的保证；多窗口、业务计算、语言服务器等仍可能耗 CPU。
- 失效矩形合并后统一提交；隐藏/最小化/零尺寸窗口不主动渲染。恢复时重新提交完整画面；绘制中产生的新修改保留给下一帧。
- `WM_PAINT` 通过 `defer EndPaint` 保证错误路径也验证失效区域。`gui.Post` 合并唤醒，不丢弃任务。
- SDK 可在 UI 线程、进入事件循环前设置：

```go
gui.SetFrameRatePolicy(gui.FrameRatePolicy{
    IdleFPS: 15, InteractionFPS: 60,
    BoostDuration: 300 * time.Millisecond,
})
```

不降低分辨率、颜色精度、矢量图标或字体抗锯齿质量。默认策略同时接入 GLFW 后端，便于 SDK 保持行为一致，但 macOS/Linux 验证不代表 Windows 实测。

### Windows 验收顺序

同一 Windows、同一缩放比例、同一工程，分别测试旧版/新版：主界面静置 30 秒、设计页静置、代码连续输入、拖拽控件、菜单/模态打开静置、最小化/恢复、带 busy 控件的标签隐藏/显示。每轮分别保存 profile、日志和任务管理器 CPU 记录。

日志开始/结束摘要含累计 `Paints` 和 `PaintWall`，可用差值核对是否存在无操作持续重画。`PaintWall` 是绘制墙钟时间，不等同 CPU 时间。原生 Cairo/GDI 开销需结合 Windows 原生分析器；Go profile 只能提供部分调用栈证据。

**仍待定位**：真实 Windows 热点/旧版号、原生系统文件对话框消息泵、DPI/驱动差异；语言服务每次输入请求的去抖/取消、长日志更新和大型撤销历史轮询尚未在本轮全面改造。不要把类型交叉编译、macOS 单测或模拟帧时钟测试写成 Windows 问题已完全解决。

忙碌 Spinner/ProgressBar 的每帧更新现在只使所属控件区域失效（Win32 保持局部 Cairo 裁剪），不再因为一个旋转图标使所有窗口全量重画。没有区域声明的 SDK 动画保留全量重画兜底，避免破坏几何变换；重复 `Animation.Start()` 也不会重复注册和叠加回调。GLFW 保留其原有保守全窗裁剪语义，但仅影响拥有该控件的窗口。

### 本地验证范围

本轮使用 `GOMAXPROCS=2`、`go test -p 1` 串行执行，避免诊断期间并行编译加重宿主负载。

```sh
go test -p 1 ./gui ./ged ./cmd/silkide ./paint ./printing ./win32 -count=1
go vet -p 1 -composites=false ./gui ./ged ./cmd/silkide ./paint ./printing ./win32
```

Windows 专属消息/绘制测试使用 fake HWND 状态、可替换失效函数与模拟时钟覆盖帧策略；只有在 Windows 上运行这些测试才算执行了 Windows scheduler 测试。macOS 上 `GOOS=windows CGO_ENABLED=0 go test -tags silk_pure_go -c` 仅检查类型和链接，它使用 Cairo stub，产物**不能作为 Windows GUI 发布包**。

本轮结果：上述六包本机回归、GUI/Win32 定向 `-race` 与 `vet -composites=false` 通过；`win32/gui/ged/cmd/silkide/printing` 五包 Windows amd64 测试二进制交叉编译通过。**没有执行 Windows 原生程序，也没有测得 Windows 优化前后 CPU/FPS/输入延迟。** 本地日志与源码哈希保存在 `bin/qa-artifacts/windows-performance-review/`（Git 不收录）。
