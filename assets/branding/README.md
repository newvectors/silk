# SilkIDE 品牌资源

蓝青渐变的折叠丝带 **S**：呼应 Silk 的名字，以及可视化设计器的组合与布局。
标志不含文字，避免在任务栏和高 DPI 小图标中出现不可读字样。

## 文件

| 文件 | 用途 |
| --- | --- |
| `silkide-logo.png` | 1254×1254、真实透明 RGBA 高清原图，保留生成时的原始字节与元数据 |
| `silkide-{16,24,32,48,64,128,256,512,1024}.png` | 保留 alpha、带明确 sRGB 色彩标记的多尺寸派生图 |
| `silkide.ico` | Windows 16/24/32/48/64/128/256 多尺寸图标 |
| `silkide.icns` | macOS 8 个无损 PNG 图层，覆盖 Retina 至 1024px；非 Retina 小尺寸由系统缩放 |
| `silkide.rc` | Windows EXE 图标资源定义 |
| `branding.go` | 内嵌运行时 256px PNG，避免依赖工作目录或外部资源路径 |
| `PROMPT.txt` | 最终采用的图片生成提示词 |

使用内置 **image_gen** 生成；这是位图 Logo，不是可编辑 SVG。
原图未携带 ICC，派生阶段按标准 sRGB ICC 解释，由编码器写入明确的 sRGB 色彩标记。
渐变颜色不是逐像素固定色板；不同显示器/系统的色彩管理仍可能有差异。

## 重新生成图标

在安装了 ImageMagick 7 的 macOS 上运行：

```sh
bash scripts/generate-branding-icons.sh
```

脚本通过 `iconutil` 打包 ICNS，默认使用系统的 `sRGB Profile.icc`；
可通过 `SILKIDE_SRGB_PROFILE` 显式指定其他标准 sRGB ICC 路径。
它不修改高清原图；所有派生图生成成功后才复制回本目录。
ICNS 特意不写 16/32px 的旧式 ARGB 图层：原生转换器往返测试发现半透明边缘的
RGB 会改变；其余 8 个 PNG 图层已验证 RGBA 逐像素无损。
应用用户不需要安装 ImageMagick，构建直接消费已经生成的资源。

## 应用接入

- SilkIDE 欢迎页、关于窗口使用同一份内嵌 Logo；独立 SDK 应用不自动采用 IDE 品牌。
- Windows 运行时设置窗口图标；`make ide`（MSYS2）和 release 工作流使用
  `scripts/build-windows-icon.sh` / `windres` 嵌入 EXE 资源。
  直接 `go build` 不会自动执行资源生成脚本。
- macOS 使用 `make bundle-ide` 生成 `bin/SilkIDE.app`，通过 `CFBundleIconFile`
  指定 ICNS。裸二进制不等同于带 Dock 图标的应用包。
- Linux 使用 GLFW 设置窗口图标；`make bundle-ide` 或 release 包提供
  `install-desktop-entry.sh`，由用户显式运行以安装自己的应用菜单入口。

应用包仍需要 Cairo 等平台运行库，Logo 打包不代表独立分发、签名或公证已完成。
Windows/Linux 的真实桌面图标显示仍需在对应系统验收。

## 本轮验证（2026-09-12）

- macOS 原生窗口实测：欢迎页、自有关于窗口和系统关于面板均显示 Logo，透明背景正常，文字布局不重叠。
- 真实 Cairo 绘制测试覆盖 PNG → RGBA → Pixmap → 欢迎页像素，避免“图片可解码但画面不可见”的假通过。
- PNG/ICO/ICNS 格式、色彩标记、alpha、逐像素往返及隔离重新生成均通过；Windows 交叉构建检查了 EXE 图标资源表。
- 与甘特图合并后的五包完整回归通过：3678 个通过（含子测试）、4 个平台或 opt-in 跳过。

本地详细证据位于被 Git 忽略的 `bin/qa-artifacts/logo/` 与 `bin/qa-artifacts/branding/`。
