ASK Terminal AI 设计文档

## 1. 简介
`ASK Terminal AI 是一个强大的命令行工具，它允许用户在忘记命令时快速获取并执行命令，或者直接在命令行与 AI 进行交互。提供了兼容模式让用户可以灵活指定模型。
`

## 2. 核心功能
### 2.1 虚拟 Terminal 模式
- 触发方式：输入 ask 不带 -i 参数进入虚拟 Terminal 模式
- 当查询命令时，AI 将返回列表格式的命令选项
- 用户可通过上下键选择合适的命令（若有多个）
- 按 e 进入命令修改模式，enter 结束修改
- 再次 enter 执行所选命令
- 按 q 或 Ctrl+C 退出虚拟 Terminal 模式
示例: 
```shell
$ ask
> 如何查看当前目录下的所有文件，包括隐藏文件？

1. ls -la
  ls 是可以查看当前目录的文件或文件夹, l 是参数展示更详细信息 a 是显示隐藏文件
2. ls -a
  describe
3. find . -maxdepth 1
  describe

```

### 2.2 对话模式
- 触发方式：在正常 Terminal 中输入 ask -i "your question" 进行对话
- 在查询内容末尾的引号未输入前，可通过 Shift + Enter 实现多行输入
- 输入完成后，等待 AI 返回结果
示例
```shell
$ ask "解释 awk 命令中 NR 和 FNR 的区别"

awk 中的 NR 和 FNR 都是内置变量，但有不同用途：

NR (Number of Records): 表示已处理的总行数，跨所有文件累计
FNR (File Number of Records): 表示当前文件中的行号，每处理新文件时重置

例如处理多个文件时：

awk '{print "文件行号:" FNR, "总行号:" NR, $0}' file1.txt file2.txt
```


### 2.3 参数覆盖机制
- 两种模式下均支持通过命令行参数临时覆盖配置文件中的设置
- 例如：ask --model gpt-4 "如何优化 Docker 容器?"
- 适用于临时需要使用其他模型或配置的场景

注：当前版本暂不支持多轮对话，这将在后续版本中添加。


## 3. 模型选型
### 3.1 支持的模型提供商
- OpenAI：GPT-3.5、GPT-4 系列
- Anthropic：Claude 系列模型
- Google：Gemini 系列模型
- 兼容模式：支持任何符合 OpenAI API 格式的模型服务

### 3.2 模型能力与选择建议

gpt-4o-mini (性价比高，响应快)
Gemini flash 系列, 主打生成快



## 4. 配置系统
### 4.1 配置文件位置
- 默认位置：/etc/askta/config.yaml
- 自定义位置：通过 -c 或 --config 选项指定配置文件路径

### 4.2 配置项详情

```YAML
# AI 服务配置
base_url: ""  # API基础URL
api_key: ""   # API密钥（将被加密存储）
model_name: "" # 使用的模型名称

# 功能配置
private_mode: false  # 隐私模式，不发送目录结构等环境信息
sys_prompt: "当前运行环境为 Linux"  # 默认系统提示词

# 提供商配置
provider: "openai-compatible "  # 可选值: 仅支持  "openai-compatible"  暂不支持 "openai", "anthropic", "google",

# 示例基础 URL:
# https://api.openai.com
# https://generativelanguage.googleapis.com
# https://api.anthropic.com
```

### 4.3 配置优先级
- 命令行参数（最高优先级）
- 配置文件
系统默认值: 没有默认值, 不配置会报错

```shell
# 使用不同模型
ask --model gpt-4 "如何查找大于 1GB 的文件?"

# 指定系统提示词
ask --sys_prompt "我使用 Ubuntu 22.04" "安装 Node.js" 
```
## 5. 安全设计
### 5.1 API 密钥加密存储
```
加密机制：用户首次输入 API 密钥后， 系统自动对密钥进行加密: 检测是否为 encty_ 开头, 是代表已加密, 否代表需要加密后写入
存储格式：encry_{加密串}
解密时机：仅在发送 API 请求前进行解密，最大限度保护密钥安全
安全保障：即使共享同一台机器，也能确保 API 密钥不被明文泄露
```

加密实现细节
```
1. 获取设备唯一标识（CPU ID、主板序列号等硬件信息的哈希）
2. 使用 Argon2id 算法从设备唯一标识派生加密密钥

```

### 5.2 隐私保护
- 私有模式：当 private_mode=true 时，不发送目录结构等环境信息
- 敏感信息处理：不保存用户的查询历史到磁盘
- 本地处理：关键信息在本地处理，减少网络传输风险

## 6. 技术实现
### 6.1 开发语言与框架
- 使用 Go 语言开发，确保跨平台兼容性和高性能
- 使用 termbox 或类似库实现交互式终端界面

```
核心依赖库
github.com/spf13/cobra - 命令行框架
github.com/spf13/viper - 配置管理
github.com/charmbracelet/bubbletea - 终端 UI 处理
github.com/fatih/color - 终端颜色支持
github.com/sashabaranov/go-openai - OpenAI API 客户端
```

### 6.2 代码架构
模块化设计：
- AI 服务适配器模块
- 终端交互模块
- 配置管理模块
- 密钥安全模块
- 命令执行模块


## 7. Prompt 设计
### 7.1 基础上下文构建
`当 private_mode=false 时，自动收集并附加用户当前目录结构`
`在系统提示中加入操作系统环境信息`

### 7.2 命令请求格式化
```Code

{
  "model": "gpt-3.5-turbo",
  "messages": [
    {"role": "system", "content": "你是一名命令行专家。当前环境: Linux, Bash Shell"},
    {"role": "user", "content": "如何找出占用空间最大的5个文件？"}
  ],
  "temperature": 0.1,
  "max_tokens": 500,
  "response_format": {"type": "text"}
}
```

### 7.3 命令输出格式控制(Terminal 模式 prompt )
```
当前环境:
- 操作系统: {os_info}
- 工作目录: {current_dir}
目录结构:
{dir_structure}
user's system prompt: {sys_prompt}
用户请求命令: {user_query}

严格按照下列格式提供命令:
```

```json
[
  {
    "1": {
      "ls -la": "ls 是可以查看当前目录的文件或文件夹, l 是参数展示更详细信息 a 是显示隐藏文件"
    }
  },
  {
    "2": {
      "command": "describe"
    }
  }
]

```

### 7.4 命令输出格式控制(常规模式 prompt )
```
当前环境:
- 操作系统: {os_info}
- 工作目录: {current_dir}
目录结构:
{dir_structure}
user's system prompt: {sys_prompt}
```

## 8. 安装与部署
### 8.1 一键安装脚本
```
curl -sSL https://raw.githubusercontent.com/username/ask-terminal/main/install.sh | bash
```
或者直接下载二进制文件使用
```bash

wget 
```

### 8.2 手动安装
```bash
git clone https://github.com/username/ask-terminal.git
cd ask-terminal
go build -o ask main.go
sudo mv ask /usr/local/bin/

chmod +x askta-linux-amd64
sudo mv askta-linux-amd64 /usr/local/bin/ask

# 创建配置目录
sudo mkdir -p /etc/askta
sudo touch /etc/askta/config.yaml
```

## 9.日志
### 9.1 记录用户查询日志
`记录最近用户在虚拟 Terminal 模式下生成的命令记录, key 为生成的日志, value 为所有命令,  当用户通过 ask -show 时显示最近 1000 条 Terminal 模式下的命令记录 (只包含命令, 不包含 describe), 日志文件放在 /tmp/askta_Chistory.log`

### 整体日志
```
/tmp/askta_run.log
包含了用户在常规模式下的对胡记录, 以及程序运行所产生的记录, 一些 GO 的报错可以记录在这里方便排查日志

```
## 10. 后续规划
### 10.1 短期计划
- 添加多轮对话支持
- 实现会话历史保存功能
- 增加更多 AI 提供商支持
### 10.2 中长期计划
- 添加插件系统，支持社区扩展功能
- 实现本地模型运行选项
- 提供 TUI (终端用户界面) 优化体验
- 添加命令执行结果反馈给 AI 的功能
- 支持 openai, clade, gemini 等 provider
### 10.3 性能优化计划
- 实现响应流式输出
- 本地缓存常见命令查询结果
- 优化大型输出的显示处理




## 选项卡:
`用法: ask [选项] ["查询内容"]`

```shell
  -c, --config FILE       指定配置文件位置
  -m, --model NAME        临时指定使用的模型
  -p, --provider NAME     临时指定 AI 提供商-暂不支持
  -u, --url URL           临时指定 API 基础 URL
  -k, --key KEY           临时指定 API 密钥
  -s, --sys-prompt TEXT   临时指定系统提示词
  --private-mode          启用隐私模式
  -v, --version           显示版本信息
  -h, --help              显示帮助信息
  -show                   显示最近 1000 条 Terminal 模式下的命令记录 (只包含命令, 不包含 describe)

```