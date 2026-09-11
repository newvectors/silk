# Silk IDE 工作台验收记录

本轮目标：向 Qt Creator 风格的 Go 可视化设计工作台推进，优先修复真实桌面流程与图标精度。不是完整 Qt Creator 功能对等验收。

## 本次改动

- 欢迎 / 编辑 / 设计模式；保存、撤销、重做按当前可见文档分派；保存全部。
- 文件脏标记、真实编辑标签、退出保护；恢复标签顺序、活动文件、光标、滚动及工作模式。
- 独立预览入口；拖放与缩放后刷新属性值；保留用户调整的面板显隐。
- GLFW 事件坐标与修饰键修复；原生窗口销毁延后至事件分发结束；关闭预览不再退出主程序。
- 原生文件选择器打开期间继续处理绘制和排队任务，失败 / 取消不误保存。
- 内置矢量图标与精确语义色板；旧 PNG 不再覆盖 IDE 的同名内置图标。未知自定义资源保持原样。
- `.silkui` 的 `form / children / props` 格式正确生成 Go 控件树，保留嵌套、几何、空文字、标量与 RGBA。生成代码按属性分行并经 `gofmt`。
- 表单位于 `ui/` 等子目录时，构建 / 运行寻找最近的 `go.mod`；显式工程和运行目录配置优先。

## 图标与色彩

软件色值为 **8-bit sRGB、非预乘 RGBA**；不宣称显示器校准、HDR 或 16-bit 色深。

| 语义 | 深色主题 | 浅色主题 |
| --- | --- | --- |
| 普通线条 | `#CBD5E1` | `#475569` |
| 保存 / 预览 / 选中 | `#60A5FA` | `#2563EB` |
| 运行 | `#4ADE80` | `#15803D` |
| 构建 / 警告 | `#FBBF24` | `#A16207` |
| 停止 / 危险 | `#F87171` | `#B91C1C` |
| 调试 | `#C4B5FD` | `#7C3AED` |
| 禁用 | `#64748B` | `#94A3B8` |

像素测试覆盖色值、透明角像素、禁用状态、主题切换、已缓存资源升级、绘图路径保留和 1×～8× 渲染一致性；活动语义色与默认背景的对比度至少 3:1。

本地色板预览：`bin/qa-artifacts/icons-2x.png`。原始 `gen_image/` 未改动。

## 实际桌面验证（2026-09-11，macOS arm64）

测试使用隔离工程 `bin/qa-workspace` 和隔离设置 `bin/qa-home`。

1. 文件树打开 `main.go` / `go.mod`，编辑、全选、撤销、重做、保存；实际文件内容与脏标记一致。
2. 从控件面板拖入 Button，缩放至 35 × 10 mm；属性面板同步显示宽高。
3. 保存 `ui/design.silkui`，屏幕 Output 显示相邻 `.silk.go` 已写入。
4. 对**这份屏幕保存的文件**执行生成函数并构造真实 GUI 对象：
   `gui.Form → gui.Button "Button" bounds=[94 57 132 38]`。
   单位转换为固定 96 DPI 逻辑像素；不是仅检查语法或编译。
5. 独立预览显示 Button；关闭预览后主设计窗口仍在，属性和选择保持。
6. 正常退出返回 0；重启恢复设计模式、测试设计及编辑标签 / 光标状态。
7. 修复前，从 `ui/` 打开的表单点击运行错误报告无 main 包；修复后同一界面执行 `go run .`，工作目录为 QA 工程根，输出 `Designer screen test`。随后点击构建，Output 显示根目录的 `go build ./...` 与 `build ok`。
8. IDE 和命令行 `silkgen` 为同一设计生成相同的 `BuildDesign` 函数。

新增集成回归还实际编译并执行生成代码，验证嵌套容器、禁用按钮、含引号 / 换行 / `@ref:` 的文字、空标签、数值上限以及 `#13A6E380` 精确半透明颜色。设计数据不作为代码执行。

## 复验命令

```sh
make test-ide

go vet -composites=false ./cmd/silkide ./decl ./gui ./ged ./prop ./paint ./graph

git diff --check

# 屏幕保存产物的运行时复验（隔离 QA 工程已用 replace 指向本仓库）
cd bin/qa-workspace
go test -count=1 -run TestScreenSavedDesignBuildsRealLayout -v ./ui
```

本轮共验证 42 个包：40 个通过测试，`cmd/silkgen` 和 `data` 无测试文件。最后的代码修改再次通过所有受影响包测试。`go build` 成功，`.app` 和 `bin/silkide` 使用同一二进制。

## 已知边界 / 未完成项

- 未声称 `go test ./...` 全仓通过：本机已有不可读取的 `local/` 目录，未修改其权限；采用显式包列表。
- 默认 `go vet` 仍有既有跨包未命名结构体字面量告警；上面的检查**明确关闭 composites 项**。本轮额外修复了不可达语句和测试中复制互斥锁的问题。
- 本次 QA PATH 未找到 `gopls`；真实 LSP 与 `dlv` 调试器未端到端验收。没有以假服务单测冒充真实服务可用。
- Windows / Linux 尚未本机实测；CI 配置不是平台验收结果。
- 声明式生成负责控件结构与属性，事件处理器仍需宿主通过 `BuildWithIndex` 连接，或使用命令式导出。
- 非标量复杂属性尚未全覆盖；不宣称所有控件、插件和所有输入组合均已验证。
- 自动化剪贴板粘贴曾报告超时但文字实际已写入，并观察到短暂 UI watchdog 告警；不能据此宣称粘贴延迟已解决。外部原生文件选择器由实际返回路径确认成功，未声称自动完成了选择器内部所有步骤。
- 终端欢迎语存在中文缺字显示，部分面板仍有混合语言；字体回退与完整本地化待后续处理。
- 测试中曾出现两个 QA 实例，旧实例已关闭；不要同时启动多个使用同一 QA 设置目录的实例。首次原生 Save 返回的 Downloads 测试文件保留，后续均使用工作区内副本。

## 继续开发

启动：`make run-ide`。当前桌面留有隔离 QA 实例。
下一阶段建议聚焦真实 gopls / dlv、事件编辑与生成、工程模板及跨平台字体 / 输入回归，而非继续堆积未验证的面板。


## 设计器细节复验（同日追加）

差距表、操作说明和仍未覆盖项见 [DESIGNER_GAPS.md](DESIGNER_GAPS.md)。

### 已在实际桌面验证

- 从 Button 控件箱拖到表单右下边缘：新增对象立即选中，属性为 X=75、Y=93、宽25、高7 mm，完整位于100×100 mm表单内。
- 拖到灰色表单外：显示红色 ×“无法投放：请移入表单范围”；取消后一次撤销移除上一有效投放，没有额外无效投放步骤。
- 四控件框选后分别应用 Grid、Form、HBox、VBox；显示真实网格、标签/字段对、单行、单列，而非仅改变容器类型。
- Form 预览实际显示四控件；关闭预览主窗口仍在。解除 VBox 后四个子控件保留。
- 自动布局子控件自由拖动被拒绝，显示原因；随后 Undo 直接撤销布局，证明拒绝手势没有污染历史。
- macOS Cmd+A 确实选中四个控件而不是表单；从表单内部空白开始框选也成功。
- 深色禁用按钮不再发白；切换编辑模式，第二设计操作栏隐藏并归还空间；缩窄窗口后工具栏与 Dock 保持独立区域。
- 自动恢复后标题为 `design.silkui *`，不再是 `.autosave`；显式保存写回原设计文件，恢复副本与修改前基线分别保留。
- Save Form → Undo → HBox 的不同历史分支正确保持 dirty。自身保存事件后 Undo 仍有效；不是只有单测检查。

### 自动验证与屏幕产物

- 42 个显式包回归：40 个有测试的包通过，2 个无测试文件。后续修改另跑受影响包无缓存回归。
- 新增真实生成程序执行：Grid/Form 布局、resize、嵌套结构，不只做 parser 或编译检查。
- `bin/qa-artifacts/details-screen-runtime.log` 执行屏幕 Save 生成的 `BuildDesign()`，得到四个真实 Button，并在 HBox 改为300×120逻辑像素后验证非重叠与边界。
- `bin/qa-artifacts/details-baseline` 保留原设计、旧 autosave、恢复后原件、四控件基线、Form布局副本、先前运行时测试及隔离设置。
- `bin/qa-artifacts/designer-panel-icons-2x.png` 为新增图标的双主题图集；覆盖16～64 px与精确语义色。

桌面自动化曾在恢复弹窗关闭后返回 `noWindowsAvailable`；正常退出并无弹窗重启后，拖放与点击复验恢复。该工具状态问题未冒充产品拖放测试通过；上述通过项来自后续真实成功操作。溢出菜单状态与极窄尺寸由自动回归确认，未声称其每项均经桌面逐个点击验收。


### 子控件悬空：实际桌面与产物闭环

使用 `ui/floating.silkui`：外层自由 VBox + Button + 内层 VBox + Edit 孙控件。

- 拖外层左上尺寸手柄：外层 `[10,10,65,60] → [20,20,55,50]` mm。
- 子 Button `[15,15] → [25,25]`、内层 VBox `[15,32] → [25,42]`、孙 Edit `[20,36] → [30,46]`，父子相对偏移保持。
- 单步 Cmd+Z 全部回到原位；Cmd+Shift+Z 全部重做。没有漂浮在原位置或被错误裁掉。
- 屏幕 Save 的 `floating.silkui` / `floating.silk.go` 已留存，`floating-screen-runtime.log` 实际构造该 Go 产物并验证父对象身份、子孙局部坐标；再次 Layout／resize 后自由布局位置不变。
- 实际预览最终完整显示 `Child button` 和 `Nested child`。修复前 IDE 的预览宿主在 AddView 后读取已被覆盖的尺寸，窗口落入320×240下限，导致本例看起来空白；现在在附加之前保存尺寸并为窗口框架预留空间。
- 原始与最终场景分开保留：`details-baseline/floating-before.silkui`、`floating-after.silkui`、`floating-after.silk.go`。

```sh
cd bin/qa-workspace
go test -count=1 -run 'TestScreenSaved' -v ./ui
```

最终受影响的 `cmd/silkide`、`ged`、`graph`、`gui`、`decl`、`paint`、`prop` 均通过无缓存回归；限定 vet 和 diff check 通过。最新 `.app` 与 `bin/silkide` 内容相同，SHA-256：
`b0c121312e9e2608e82bc40f7abd1e25fcffcd224ed7c49ba19b5dc893af5035`。

仍需区分：HBox 的自然尺寸控件贴左、余下空白是尚未补齐的 sizePolicy/stretch 编辑能力，不是对象脱离；本次不宣称填满空间或全部 Qt 布局能力已经完成。基础属性 Edit 没有新增字段本地 Undo 历史，Cmd+Z 沿用现有设计命令历史。

## 2026-09-11：纠正“悬空”定位，统一修复浮层与事件底层

用户截图的绿色 `已保存 floating.silkui` 位于主窗口上方，本节处理的是通知、菜单、下拉框的窗口归属；与上一节画布子孙控件几何问题不同。完整边界见 `POPUP_AUDIT.md`。

### 回放与运行时证据

隔离环境仍使用 `bin/qa-home`，工作目录 `bin/qa-workspace`。追加独立测试文档 `ui/popups.silkui`，不覆盖 `ui/floating.silkui`。验收期间使用 `SILK_DEBUG=true` 记录类型、owner 和几何；最终运行关闭调试描边。

- 保存通知：在主窗口内部、工具栏下方显示中性主题卡片；移动主窗口仍在内部，点关闭立即消失。真实 DrawWidgetAll 测试验证覆盖顺序、精确像素、不进入 Children/布局/序列化树、多个 owner 隔离。
- 主菜单：实际点击菜单后用 End/Return 打开“关于”，关闭模态后主窗保持可用。发现同点重开无响应，补上按下时的实时命中/hover刷新和销毁时捕获清理；在同一 `[246,55]` 连续开关三次，日志出现三次 show、两次 hide。
- 右键菜单：设计画布右键成功建立 `*gui.Frame` owner。主菜单展开后执行窗口 zoom / minimize，日志各出现 owner `dismiss` 和对应 `hide`，还原窗口可继续使用。
- Cocoa 最终绑定：菜单在 `07:42:25` / `07:42:29` 记录 `native-owner: bound=true`，移动 owner 后收起；不是只保存一个 Go 指针。
- 日期预览：最终深色背景与日期文字可读；`07:43:35` 日历绑定成功，ownerRect=`574,312,907x710`，popup=`650,509,232x200`；缩放预览于 `07:43:36` 自动收起。
- ComboBox：预览右侧箭头已可见；`07:44:28` 组合框绑定成功，popup=`650,697,454x58`，位于同一 owner 内；关闭预览于 `07:44:40` 触发 dismiss/hide，主窗口继续运行。

证据日志：`bin/qa-artifacts/popup-debug-gui.log`、`popup-dropdown-gui.log`、`native-owner-gui.log`。原生独立 popup 不包含在工具的主窗截图中，因此不把主窗截图“没看到菜单”当作菜单消失的证据；使用真实交互配合原生 show/hide 和 owner 检查。Cocoa 截图工具的短暂 thumbnail / SCStreamError 不计作应用崩溃。

### 最终回归

```sh
go test ./gui ./cmd/silkide ./ged ./graph ./prop ./paint ./decl -count=1
go vet -composites=false ./cmd/silkide ./gui ./ged ./graph ./prop ./paint ./decl
go build -o bin/qa-artifacts/silkide-overlay ./cmd/silkide
git diff --check
```

以上通过；日志 `/tmp/dc-native-owner-tests.log`、`/tmp/dc-native-owner-vet.log`。`.app` 与 `bin/silkide` 的 SHA-256 均为 `cd34ef723a0d1b03e3984c90973498ca8d4c84500ff2bebc78d120925cca9ecf`。

未冒充已完成：真实双屏/混合 DPI、Linux 原生 transient-for、Windows 完整 GUI、所有 tooltip 的逐项屏幕验收、外部文件选择器 owner。多屏坐标与深浅主题目前另有 headless/像素回归，不能代替这些目标环境验证。

收尾：以 `SILK_DEBUG=false` 重启相同 SHA 二进制，主窗截图确认无调试描边、日期/组合框箭头可见、保存提示位于窗口内部；当前保留 `popups.silkui` 供继续测试。实际保存生成的 `popups.silk.go` 与既有场景共同编译通过，`go test -count=1 -run TestScreenSaved -v ./ui` 通过，日志 `bin/qa-artifacts/popup-saved-generation-tests.log`。运行日志为 `bin/qa-artifacts/native-owner-release-gui.log`，既有 gopls/property 资源提示未在本轮扩展处理。

## 2026-09-11：SDK 保存链、原生文件弹窗与跨平台收尾

继续使用隔离 `bin/qa-home` / `bin/qa-workspace`，新建 `ui/sdk-platforms.silkui`，
不覆盖原有 floating/popups 场景。完整变更与验证边界见 `SDK_COMPATIBILITY.md`。

1. 父 VBox 从 X=10mm 拖至 X=15mm（网格吸附），日期、组合框、按钮一起移动；
   Cmd+Z 后父控件与全部子控件恢复原位，未见残影或独立悬空。
2. 第一轮真实 Cmd+S 暴露 Combo 的 items 丢失及连字符文件名生成失败。
   保留失败文件 `bin/qa-artifacts/sdk-platforms-first-save-loss.silkui`；修复
   设计器属性捕获/恢复和共享生成器后，最终 app 再次 Cmd+S 成功。
   Output 显示 `silkgen: wrote .../ui/sdk-platforms.silk.go`；磁盘仍有
   `["Windows" "Linux" "macOS"]`、selectedIndex=2、日期及 Format。
3. 原外部 osascript 选择器曾无可见 owner，导致主窗一直禁用。最终版本中
   Cmd+O 显示 owner 的原生 open-panel sheet，Cancel 后主窗恢复；重新打开，
   使用 Go to Folder 定位 QA `main.go` 并打开，编辑模式正常显示内容。
4. 导出显示 save-panel sheet；Cancel/重开都成功。实际保存中文文件名
   `bin/qa-workspace/sdk-原生保存.svg`，文件 29342 bytes，XML 根元素为 svg，
   viewBox=`0 0 120 90`。保存后主窗口正常恢复；没有 osascript 子进程。
5. 最终预览显示 `29/02/2024`、`macOS` 与 `SDK nested control`；打开组合框，
   Home/Return 后实际显示 `Windows`。关闭预览返回已保存设计场景，文件仍为
   macOS 选中值。独立 module 运行三个 `TestScreenSaved*` 测试全部通过，
   包括这份屏幕直接生成的源码、嵌套归属/局部坐标和原布局场景。

最终主窗已留在设计模式 `sdk-platforms.silkui`，无调试描边。
运行日志：`bin/qa-artifacts/sdk-final-macos-gui.log`；测试、签名及平台证据：
`bin/qa-artifacts/sdk-compatibility/`。运行进程的 cwd 实测仍是 QA 工作区，
没有被 Cocoa 改到应用包 Resources。

最终未签名 staging SHA-256：
`975e837d6c43b68c66e200de6d5b59441329b8a92b1235d02deb98ec070dd9fd`。
最终 ad-hoc 签名 bundle 可执行文件 SHA-256：
`74bf928121c7c35996696d749e14fda5620a103c6b7e4d980811dbb08882077a`。
`codesign --verify --deep --strict --verbose=2 bin/SilkIDE.app` 通过；旧包完整备份
为 `bin/qa-artifacts/SilkIDE.before-sdk-signing.app`，没有修权限或删除旧运行数据。

Linux 已有真实构建、无显示 SDK、Xvfb 测试与非 root 原生窗口启动证据；
Windows 仍是类型/ABI 层检查，未冒充 Cairo/GUI 真机验收。先前“外部文件
选择器 owner 未验收”边界，本轮仅对 macOS 原生 Open/Save 更新为已实测。

## 2026-09-11：菜单动作与默认关于布局收尾

本轮完整修复、逐项菜单验收和未完成边界见 [MENU_QA.md](MENU_QA.md)。

- 真实复现并修复 Cocoa 按钮点击使用旧鼠标位置的问题；关闭关于后同点重开
  菜单成功。Escape 依据实际接收窗口取消 popup / Dialog，不被旧焦点吞掉。
- 主菜单 9 个入口逐项屏幕操作；导出文件 XML 实际验证；源码右键缺 LSP 明确
  提示；画布属性/生成代码确实激活目标面板，布局菜单补齐 Grid/Form 及禁用判断。
- 关于区分 IDE/SDK 版本并展示真实构建和运行时信息；正文换行/滚动、默认尺寸
  和右对齐关闭按钮经过主题/字号/语言绘制测试。项目设置绑定实际工程并持久化。
- Stop 在实际长任务中终止 go run 及子进程，不退出 IDE；隔离 main.go 已恢复。
- 11 包测试、vet、3 个屏幕保存产物测试通过；用户新增 Rating 保留且加强断言。
  最新 Linux 四包/构建/Xvfb 启动通过；Windows 仅类型编译，不是原生运行验收。
- 当前 `.app` / `bin/silkide` SHA 为
  `b2f8e550d96f7e4488202483d132ee0a8e1dc4143ab3f48463efd08d07a7a099`。
  签名验证通过，以 `SILK_DEBUG=false` 重启并留在 `sdk-platforms.silkui` 设计页。
  先前章节的二进制哈希只对应当时产物，不是当前版本。

证据集中于 `bin/qa-artifacts/menu-round/`；当前运行日志 `macos-release-gui.log`。
真实 LSP/dlv、Windows GUI、Linux 桌面交互和所有次级面板按钮仍需专门验收，
不把单测或编译结果冒充这些能力已可用。
