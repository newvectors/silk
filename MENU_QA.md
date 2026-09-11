# 菜单与默认对话框验收（2026-09-11）

目标是可预测的 Go 设计工作台交互，不是 Qt Creator 全功能对等声明。
测试使用 `bin/qa-workspace` / `bin/qa-home`，保留原工程和用户新增的 Rating。

## 已修复

- **点击坐标**：Cocoa 的鼠标按下/释放未必先有鼠标移动回调。GLFW 旧坐标会把
  菜单按钮点击误投到 StatusBar。现在仅在同窗口、同类型、同按钮的当前
  `NSEvent` 匹配时取 `locationInWindow`，转换为客户区逻辑坐标；保留 capture
  映射，其他平台使用原路径。实际日志从 `pos=651.21484375,682.25390625`
  错投 StatusBar，变为 `event-pos=true, pos=246,23, target=*gui.Button`。
- **菜单生命周期**：隐藏项不占位、不接收导航；禁用项不触发、不展开子菜单；
  Enter 展开子菜单，不误执行叶子回调；重复 Enter/Space 不重复执行；执行动作前
  先关闭整条 popup 链。Escape 按实际接收窗口路由，避免旧焦点截获模态取消。
- **动作统一**：菜单、工具栏、命令面板共享保存/导出/停止入口。保存只操作当前
  可见文档；全部保存有明确反馈；导出区分取消、失败、成功，显示实际文件后缀。
  最近文件显示可选择的列表，不再把“打开最近”直接等价为打开第一项。
- **属性/代码/布局**：画布“属性”确实激活属性面板，“查看代码”打开并刷新生成
  代码页；补齐 Grid/Form 菜单，根据选择、锁定和归属禁用不可用布局/对齐。
- **源码右键**：IDE 重命名/定义/引用路由到所属编辑器的 LSP 处理器，不再静默
  退化成本地字符串操作。无 LSP 明确提示，重命名不会先弹输入框或改文件。
  SDK 独立 CodeEditor 保留默认行为，并提供每实例 host overrides。
- **项目设置**：绑定当前工程根，Kit 修改真实持久化；修复模态尺寸被自然尺寸
  覆盖、标题/摘要重叠、滚动内容覆盖表头和低对比度。当前默认内容区 640×520。
- **关于**：区分 Silk IDE v0.1.3 与 Silk SDK v2.5.0，运行时/平台/后端/构建来源
  取真实值；删除旧硬编码版本、控件/代码行计数及功能对等宣传。长构建文本换行，
  小窗口可滚动，关闭按钮右对齐、至少 88×32，避免标题或正文压到按钮区。
- **无障碍树**：菜单把结果显示在 Output 而不只是 stderr；SDK 补齐类型桥接，
  解决 `Children() []IWidget` 不满足 `Children() []interface{}` 而只显示根节点。
- **运行/停止**：Stop 同时覆盖 Run 与调试。POSIX 仅停止本次命令的独立进程组；
  Windows 仅以明确 PID 的 `taskkill /T /F` 处理。独立 PTY 不受影响。异步调试
  启动使用会话所有权和取消 token，旧结果不能重新发布或覆盖新会话。

## 实际 macOS 操作结果

本轮逐项操作了主菜单 9 个入口。不是只调用回调的单测。

| 入口 | 实际结果 | 自动回归补充 |
| --- | --- | --- |
| 新建 | 显示 untitled 空画布；原 SDK 文件未变 | 工作台文档路由 |
| 打开 | 原生 Open sheet 选择 QA `main.go`，切入编辑并显示真实内容 | 文件路由/原生选择器既有测试 |
| 打开最近 | Right 展开子菜单，选择 SDK 设计并成功恢复控件 | 最近文件去歧义、命令面板复用 |
| 保存 | 设计保存成功，Output 显示相邻 `.silk.go` 已生成 | 菜单实际保存可见编辑器内容 |
| 全部保存 | 显示“所有文档均已保存” | 干净状态也反馈；与退出保护共用保存逻辑 |
| 导出 | 原生 Save sheet 写入 `menu-export.svg`，Output 显示真实路径 | XML 合法，31581 bytes，viewBox `0 0 120 90`；取消/错误分支测试 |
| 显示无障碍树 | Output 出现多层子控件，而非单个 Unknown 根 | 隐藏项过滤及嵌套 Button 回归 |
| 项目设置 | 标题/摘要/内容分隔，当前工程路径正确，右侧关闭按钮；Tab 后 Esc 关闭 | Kit 写入并重新加载验证；显式尺寸保留 |
| 关于 | 默认布局完整可读；Tab 后 Esc 关闭；同点立即重开菜单成功 | 48 组语言/主题/字号/尺寸真实 Draw；6 条关闭/焦点路径 |

另外实测：

- 编辑器右键“重命名符号”：出现“LSP 未运行”，没有输入框、没有源码修改。
- 画布选择 Button 后切到其他右侧页，再执行“属性”：切回属性页。
- 同一 Button 执行“查看代码”：底部生成代码页出现 `ui.SdkAction = gui.NewButton1(...)`。
- 单控件布局子菜单显示 VBox/HBox/Grid/Form/Break，均正确禁用；自动测试另外
  实际执行 Grid Action，验证生成包含四个子控件的 Grid，而不是只检查名称。
- F5 启动带 `MENU_STOP_READY` 标记的隔离长任务，再点工具栏 Stop：go run PID
  60949 / PGID 60949 和子进程 60955 消失，IDE 仍运行；测试后恢复原 `main.go`。
- 用户新增的第 4 个 Rating 完整保留。旧 QA 测试硬编码 3 个子项，因此首次复跑
  失败；保留旧测试/失败日志，更新为精确验证 4 个子项，并额外断言 Rating 的
  parent、局部几何、value=3、max=5、readonly=false。没有删除控件迎合旧测试。

## 回归与交付

```sh
go test -count=1 ./a11y ./gui ./ged ./cmd/silkide ./prop ./decl ./paint ./core ./graph ./internal/silkgen ./cmd/silkgen
go vet -composites=false ./a11y ./gui ./ged ./cmd/silkide ./prop ./decl ./core
git diff --check
(cd bin/qa-workspace && go test -count=1 -run TestScreenSaved -v ./ui)
```

- 上述 11 包、vet、diff 检查及 3 个屏幕保存产物运行测试通过。
- Linux：Debian 12 arm64 / Go 1.26.0，当前代码四包 `a11y/gui/ged/cmd/silkide`
  非 short 测试、vet、构建通过；UID 1000、隔离 HOME、Xvfb 原生窗口 12 秒启动
  smoke 通过。另有 Run/Stop race 回归。仅清理本轮自己创建的测试容器。
- Windows：当前代码 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`，
  `go test -tags silk_pure_go -c` 分别编译 `ged`、`cmd/silkide` 成功。
  这是类型编译，**没有执行 Windows GUI 或验证原生 Cairo 集成**。
- 最终 macOS `.app` 经 ad-hoc 签名验证，`bin/silkide` 与 app 内可执行文件相同。
  已以 `SILK_DEBUG=false` 重启，实际截图确认无调试描边，保留在 SDK 设计场景。

证据目录：`bin/qa-artifacts/menu-round/`。
主要文件：`macos-native-event-gui.log`、`macos-release-gui.log`、
`macos-final-tests.log`、`macos-final-vet.log`、`macos-final-codesign.log`、
`linux-latest-validation.txt` / `linux-latest-*.log`、`windows-latest-typecheck.log`、
`run-stop-process-check.log`、`screen-saved-generation-tests.log`、`final-sha256.txt`。
About 像素渲染样本另在 `bin/qa-artifacts/about/`，不是人工桌面截图替代品。

最终签名可执行文件 SHA-256：
`b2f8e550d96f7e4488202483d132ee0a8e1dc4143ab3f48463efd08d07a7a099`。
原始/中间二进制和设计文件分开保留；本轮未改旧备份目录权限。

## 仍需继续验收的边界

- 命令面板盘点到 66 个入口，不能由主菜单 9 项通过推导为全部通过。真实 gopls
  定义/引用/重命名、dlv 完整会话，以及 Git/差异/覆盖率等次级面板尚未逐项实测。
- Run/Build 使用磁盘代码，尚未增加运行前自动保存。Refresh 目前是重绘，不是
  重新加载文件；显示覆盖率会重新运行覆盖率任务。
- 调试取消 token 即时生效，但底层 `LaunchDebug` 编译没有 context，仍可能
  沿用原 60 秒预算，在返回时关闭迟到结果。
- Linux 验证是虚拟显示启动，不是原生桌面菜单/输入法/多屏验收；容器无 CJK
  字体。终端中文缺字、部分控件/面板本地化和次级面板配色仍有后续打磨空间。
- a11y 树诊断可遍历不等于已经实现各 OS 的完整原生无障碍 bridge。
- 没有声称 `go test ./...` 全仓通过；已有不可读 `local/` 及旧备份未修权限，
  本轮采用明确包列表。vet 仍明确关闭既有 `composites` 告警项。
