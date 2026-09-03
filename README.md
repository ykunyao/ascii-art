# ascii-art

把图片渲染成终端 ASCII 字符画的小工具。纯 Go 标准库实现，零第三方依赖。

> Render images (PNG / JPEG / GIF) as ASCII art in your terminal — truecolor, half-block mode, animated GIF playback. Zero dependencies.

支持本地文件、URL 下载和 stdin 管道输入；GIF 加 `-play` 可在终端里循环播放动画；`-mode half` 用 Unicode 半块字符渲染出高分辨率"高清终端图"。

## 安装

```bash
go install github.com/ykunyao/ascii-art@latest
```

## 用法

```bash
# 基本用法：渲染 PNG / JPEG / GIF
ascii-art photo.png

# 不指定宽度自动适配终端窗口（重定向输出时默认 100）
ascii-art photo.jpg

# 下载并渲染网络图片
ascii-art https://example.com/photo.png

# 从管道读取（配合 curl 等工具）
curl -s https://example.com/photo.png | ascii-art -

# half 模式：Unicode 半块字符 + 真彩色，纵向分辨率翻倍
ascii-art -mode half photo.png

# GIF 动画循环播放（Ctrl+C 退出，自动清屏复原）
ascii-art -play -width 80 animation.gif

# 强制彩色 / 关闭彩色
ascii-art -color=always photo.png
ascii-art -color=never photo.png

# 浅色背景的终端上反转明暗
ascii-art -invert photo.png

# 自定义明暗字符梯度
ascii-art -ramp " .oO@" photo.png
```

## 效果

对 `testdata/sample.png`（渐变 + 亮色圆）的灰度渲染结果（面积平均采样，边缘平滑）：

```
  .................................:::::::::::::::::::::::::::::
........................::::::::::::::::::::::::::::::::::------
..............::::::::::::::::::::::::::::::::::----------------
...::::::::::::::::::::::::::::::::::---------------------------
::::::::::::::::::::::::::-------------=+++++==------------=====
:::::::::::::::--------------------=*#%%%%%%%%%%#+==============
:::::----------------------------=*%%%%%%%%%%%%%%%#+============
---------------------------======%%%%%%%%%%%%%%%%%%%*========+++
```

## 选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `-width` | 0 | 输出宽度（字符数），0 = 自动适配终端 |
| `-mode` | ascii | 渲染模式: ascii / half（半块字符，需彩色） |
| `-ramp` | `" .:-=+*#%@"` | 明暗字符梯度，从暗到亮 |
| `-invert` | false | 反转明暗映射 |
| `-color` | auto | 彩色输出: auto / always / never |
| `-play` | false | 循环播放 GIF 动画 |

## 说明

- 采样采用面积平均（box filter），缩小大图时无摩尔纹、边缘平滑
- ASCII 模式按终端字符纵横比（约 1:2）校正；half 模式逐像素无需校正
- 终端中运行时输出自动限制在窗口尺寸内；重定向到文件/管道时不受限
- `-color=auto` 依据终端能力（Windows Terminal、TERM 等）自动开启真彩色
- GIF 播放按"帧间不清除"合成（覆盖大多数常见 GIF）

## LICENSE

[MIT](LICENSE)
