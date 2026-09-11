# 浮层与独立窗口审计

日期：2026-09-11。范围：保存提示、菜单/子菜单、下拉选择、属性颜色面板、tooltip、日期日历和模态对话框。

**状态：底层修复、自动测试和 macOS 关键流程复验完成。** 不等于全部控件、全部平台验收。原生浮层的截图工具只截取主窗口，故使用实际 UI 操作＋原生 owner/几何日志验证浮层生命周期；主题颜色另有 Cairo 像素回归。复现记录见 `WORKBENCH_QA.md`。

## 最终底层补强与实测

- **真正的原生归属**：macOS 不再对 popup 设置全局 `Floating`，新增 `native_popup_owner_darwin.go/.m`，使用 Cocoa child-window 关系、普通窗口层级及失活隐藏；每次显示重新绑定、隐藏/销毁解绑。Apple 文档说明附属窗口相对排序及 `orderOut` 解除关系：[child window](https://developer.apple.com/documentation/appkit/nswindow/addchildwindow(_:ordered:))、[parent](https://developer.apple.com/documentation/appkit/nswindow/parent)。
- **事件一致性**：每次无捕获的鼠标按下按本次坐标刷新命中与 hover；模态销毁前清理自身 capture/hover/focus/手势状态，清空捕获数组尾部引用；同一 owner 的重复失焦合并为一次延迟检查，避免正常主窗→菜单→子菜单切换时提前关闭。
- **实际菜单验收**：同一点连续打开/`Escape`/重开三次成功；右键菜单正常建立 owner；主窗口缩放、最小化、移动均触发所属菜单 `dismiss/hide`，重新打开使用更新后的 owner 坐标。关于对话框打开/关闭后主窗口保存、鼠标交互正常。
- **最终二进制原生验收**：菜单、日期日历、组合框均记录 `popup native-owner: bound=true`；日期日历随预览缩放收起，组合框在预览关闭时同步清理，主窗口继续运行。日志：`bin/qa-artifacts/native-owner-gui.log`。
- **颜色与图标**：DatePicker/Calendar 修复深色白底白字，ComboBox 补齐预留区内的矢量箭头；深浅主题 1×/2× 真绘制像素与文字对比度回归通过，深色预览已截图确认。
- **多屏代码**：新增按 anchor 所在显示器 work area 选屏的公共几何逻辑，支持负坐标、上/右屏、边界翻转；GLFW/Win32 接点已接入。仅纯几何和编译验证，尚未做真实双屏 GUI 验收。

下面保留底层补强前的分组件排查快照；其“待 Root”列是当时状态，最终验收以上述实测记录为准。tooltip 的逐项屏幕验证及跨平台场景仍不计入已实测结论。

## 中间排查快照：根因与修复清单

| 对象 | 根因 / 原行为 | 本轮修复与代码 | 自动验证 | 最终 GUI 验收 |
| --- | --- | --- | --- | --- |
| 保存等 toast | 普通反馈使用独立 `WtPopup`，混用全局定位与 owner 坐标，能够浮到主窗外 | 改为 owner 本地 overlay，不创建原生窗口，不加入布局/序列化子树；内容之后绘制、优先命中关闭按钮；按 owner 分组，小窗口裁限，隐藏/销毁清理计时器。`gui/toast.go`、`gui/toast_overlay.go`、`gui/widget.go`、两个窗口后端 | `toast_overlay_test.go`：无 Window/布局子项、owner 隔离、resize、关闭命中、小窗口、真实绘制像素与主题色 | 待 Root 最终确认保存提示位置、主窗移动/缩放、关闭及多预览窗口 |
| 公共 owner 获取 | `syncWindow` 安装自己的 Window 后再找 owner，会找到新窗口自身 | 创建前保存真实 owner，再传给后端。`gui/widget.go: syncWindow` | 源码路径检查；popup/modal 模型测试间接覆盖，headless 不执行原生 create | 待确认原生创建后的行为，不宣称所有平台 OS owner 绑定完整 |
| 菜单、子菜单、ComboBox/颜色等下拉 | 缺少稳定 owner 链；主窗变化后弹层、capture 或 focus 仍留在原处；临时菜单部分未设 parent | GLFW 记录/刷新 `popupOwner`；owner 移动、resize、隐藏、关闭、最小化时收起所属 popup 树；失焦延迟检查，保留 owner→菜单→子菜单的内部焦点切换；恢复输入状态。临时 context menu / dropdown 补 parent。`gui/window_glfw.go`、`gui/contextmenu.go`、`gui/dropdown_button.go` | `popup_lifecycle_glfw_test.go`：事件回调、嵌套链、独立 Form 隔离、重复几何、capture/focus、重新挂接及可复用 Hide | 待 Root 最终确认真实拖窗、切应用、菜单/下拉再次打开 |
| tooltip | 归属不清，先 attach 后尺寸/定位可能提前显示 | 独立 owner 引用但不进入布局子树；隐藏后设置尺寸、attach、全局定位再显示；收起清理引用。`gui/tooltip.go` | `TestOwnerDismissClearsTooltipReferenceWithoutLayoutChild`；首帧顺序另经源码检查 | 待确认首次显示、主窗移动和切应用后无残留 |
| 日期日历 | 构造时默认 visible，attach 早于尺寸/定位，可能闪出默认窗口 | 默认隐藏；隐藏→最终尺寸→attach→定位→显示→capture；创建失败保持隐藏。`gui/datepicker.go` | `datepicker_popup_test.go` 两项红转绿：默认 hidden/no-native/owner、真实日期点击处理器关闭后下一日历读取新月份 | 待确认无首次闪窗；headless 未运行真实 ShowAsPopup |
| Dialog/InputBox/MessageBox | 销毁后控件保留失效 Window；仅清 Window 又可能让 visible 对话框作为普通 child 留下残影 | 销毁时隐藏仍属于该 Window 的控件并清引用；保留逻辑 owner 以便复用；不误隐藏已主动转回 inline 的控件。`gui/window_glfw.go: destroy` | `dialog_lifecycle_test.go`：三类控件关闭、owner/sibling 隔离、二次 attachment、重复销毁、inline 复用 | 待确认真实模态关闭/再次打开无残影；未在测试中运行原生模态循环 |
| 普通控件 / `WtChild` | 普通控件应在所属窗口中 inline 绘制，不能把所有“子控件”都当作独立窗口 | 保持 `WtInherit` 默认；本轮图谱/源码审计未发现 `WtChild` 业务消费者，只有类型定义、后端分支与 guard。属性颜色编辑走 `SetSubPopup`，不是额外 dialog 实现 | 静态审计；不宣称每个控件均做了 GUI 验收 | 普通画布控件的布局/几何验收另见工作台记录 |

## 已执行的自动验证

```sh
go test ./gui
git diff --check -- gui/datepicker.go gui/datepicker_popup_test.go gui/dialog_lifecycle_test.go
```

- 最近一次 `go test ./gui` 通过，日志 `/tmp/dc-popup-audit-gui.log`；部分结果使用 Go 测试缓存。
- 日历缺口保留红/绿日志：`/tmp/dc-datepicker-popup-red.log`、`/tmp/dc-datepicker-popup-green.log`。
- popup/modal 生命周期测试使用无原生句柄的 Window fixture，不打开真实桌面窗口；toast 像素测试验证软件绘制，不代表显示器校准。
- GLFW 专属测试不等于 Windows 验收；两个后端的 toast 绘制接点已接入，但 Windows/Linux 原生行为仍需目标平台复验。

## 明确保留的边界

1. **外部文件选择器 owner**：`gui/dialog_glfw.go` 的 `OpenFileDialog()` / `SaveFileDialog()` 无 parent 参数，macOS 使用 `osascript`、Linux 使用 `zenity`。继续处理绘制并禁用 IDE 输入，不等于原生 owner/sheet 关系完整；本轮未替换为 owner-aware 原生选择器。
2. **多屏与缩放**：按显示器 work area 定位已实现并测试；真实负坐标屏、跨屏移动、混合 DPI 仍未 GUI 验收。macOS 原生 owner 已绑定并实测；Linux 原生 transient-for、Windows 全 GUI 尚未验收。
3. **一次性隐藏菜单引用**：可复用菜单 Hide 时故意保留对象/窗口；临时 context menu、dropdown 等关闭后可能仍通过 parent 保存引用。后续应区分一次性销毁和可复用关闭，不能简单给全部 Hide 加 Detach。
4. **对话框显式尺寸**：`gui/dialog.go: dialogSize` 根据默认值和 SizeHints 重新计算，忽略调用方 `SetSize`；新建项目、命令面板等请求尺寸可能被覆盖。本轮未修改，不能将它与 toast 悬空混为一谈。

没有以本文宣称完整 Qt Creator/Qt Widgets Designer 功能或全平台窗口行为对等。
