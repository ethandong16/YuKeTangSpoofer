# YuKeTangSpoofer

一个用于读取雨课堂课程结构、查询视频完成状态并批量发送学习心跳的 Go 命令行工具。

## 免责声明

- 本项目仅用于学习 HTTP 接口分析与 Go 网络编程实践。  
- 请遵守学校与平台的使用规范、课程要求和相关法律法规。  
- 因使用本项目造成的任何后果由使用者自行承担。

## 功能概览

- 读取课程列表 JSON（`courseListJson.txt`）并在终端展示可选课程
- 根据课程 `courseID/classroomID` 拉取章节与视频列表
- 查询每个视频当前完成状态（`[DONE] / [TODO]`）
- 可选择对未完成视频自动发送心跳包
- 心跳过程中支持按 `s` 跳过当前视频

## 项目结构

- `main.go`：主入口，课程选择、章节拉取、完成状态查询、批量心跳流程
- `iteratevideo.go`：独立心跳发送逻辑（包含参数化版本 `RunHeartbeatTool`）
- `courseListJson.txt`：示例课程列表数据（运行时会读取）
- `cookie.txt`：可选本地 Cookie 文件（已在 `.gitignore` 中）

## 运行环境

- Go（版本以 `go.mod` 为准）
- 可访问 `www.yuketang.cn` 的网络环境
- 有效的雨课堂登录 Cookie（建议包含 `sessionid`、`csrftoken`）

## 使用说明

### 1. 准备课程列表

程序会读取仓库根目录下的 `courseListJson.txt`。  
请先准备你自己的课程列表 JSON 并覆盖该文件内容。

### 2. 准备 Cookie（两种方式）

1. 在仓库根目录创建 `cookie.txt`，写入完整 Cookie 字符串（推荐）  
2. 运行后按提示在终端粘贴 Cookie

Cookie 示例（单行）：

```txt
sessionid=xxx; csrftoken=xxx; university_id=xxx; ...
```

### 3. 启动

在仓库根目录执行：

```bash
go run .
```

### 4. 交互流程

1. 程序打印课程列表  
2. 输入课程序号  
3. 程序获取章节并显示每个视频状态  
4. 输入 `y` 确认后，对未完成视频发送心跳  
5. 心跳执行时可按 `s` 跳过当前视频

## 关键行为说明

- 只会自动处理标记为未完成的视频
- 查询完成状态依赖 Cookie 中的用户身份信息；缺失时会请求用户信息接口补全
- 章节接口请求失败或返回异常时会输出日志并终止当前流程

## 常见问题

### 1) 提示课程索引无效

确认输入的是打印列表中的数字序号，且 `courseListJson.txt` 结构正确。

### 2) 提示缺少关键 Cookie

请重新复制浏览器请求头中的完整 `Cookie` 值，确保包含 `sessionid` 和 `csrftoken`。

### 3) 一直显示请求失败或未完成

- 检查 Cookie 是否过期  
- 检查网络是否可访问雨课堂接口  
- 检查所选课程的 `courseID/classroomID` 是否匹配

## 开发提示

- 默认入口为 `main.go` 中的交互式流程  
- 若需使用 `iteratevideo.go` 中的参数化工具逻辑，可按需调整 `main` 调用路径

