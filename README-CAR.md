# MyGo Music 车机版

基于 [MyGo Music](https://github.com/egoist/mygo-music) 制作，感谢原作者 egoist 与所有贡献者。

本项目面向 Android ARM64 车机，包含本地音乐和音源插件、媒体会话与车机界面适配。不同车机的系统权限、媒体按键和语音接口有所差异，实际效果以设备为准。请在车辆停稳时进行屏幕操作。

## 构建

安装 Go、Bun、JDK 17、Android SDK 与 NDK，并设置 `JAVA_HOME`、`ANDROID_HOME`、`ANDROID_NDK_HOME`，然后在项目根目录运行：

```sh
go tool mygo build -platform android/arm64
```

APK 输出在 `build/android/`。未配置发布密钥时使用 Android 调试签名，不适合公开商店分发。

## 许可证

本项目保留上游的 AGPL-3.0 许可证。`third_party/mygo` 及内置插件各自的许可证和第三方声明随源码保留。

账号登录信息、密钥和个人数据不得提交到仓库。
