# 🤝 WeKnora 贡献指南

欢迎参与 WeKnora 开源项目！本文档将指导你如何为项目贡献代码。

## 📋 目录

- [贡献方式](#贡献方式)
- [准备工作](#准备工作)
- [贡献流程](#贡献流程)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 指南](#pull-request-指南)
- [常见问题](#常见问题)

## 🎯 贡献方式

你可以通过以下方式参与贡献：

- 🐛 **Bug 修复**: 发现并修复系统缺陷
- ✨ **新功能**: 提出并实现新特性
- 📚 **文档改进**: 完善项目文档
- 🧪 **测试用例**: 编写单元测试和集成测试
- 🎨 **UI/UX 优化**: 改进用户界面和体验
- 🌐 **国际化**: 翻译和本地化支持

## 🛠 准备工作

### 1. 安装必要工具

确保你的开发环境已安装：

- [Git](https://git-scm.com/) (版本 >= 2.0)
- [Docker](https://www.docker.com/) 和 [Docker Compose](https://docs.docker.com/compose/)
- [Go](https://go.dev/) (版本 >= 1.21) - 用于后端开发
- [Node.js](https://nodejs.org/) (版本 >= 18) - 用于前端开发
- 代码编辑器（推荐 VS Code 或 GoLand）

### 2. 配置 Git

```bash
# 设置你的 Git 用户名和邮箱
git config --global user.name "你的名字"
git config --global user.email "你的邮箱@example.com"
```

### 3. Fork 项目

1. 访问 [WeKnora GitHub 仓库](https://github.com/Tencent/WeKnora)
2. 点击右上角的 **Fork** 按钮
3. 等待 Fork 完成，你会得到一个属于你的仓库副本：`https://github.com/你的用户名/WeKnora`

## 📝 贡献流程

### 步骤 1: 克隆你的 Fork

```bash
# 克隆你 Fork 的仓库
git clone https://github.com/你的用户名/WeKnora.git
cd WeKnora
```

### 步骤 2: 添加上游仓库

```bash
# 添加上游仓库（原始仓库）作为 remote
git remote add upstream https://github.com/Tencent/WeKnora.git

# 验证 remote 配置
git remote -v
# 应该看到：
# origin    https://github.com/你的用户名/WeKnora.git (fetch)
# origin    https://github.com/你的用户名/WeKnora.git (push)
# upstream  https://github.com/Tencent/WeKnora.git (fetch)
# upstream  https://github.com/Tencent/WeKnora.git (push)
```

### 步骤 3: 保持代码同步

在开始工作前，确保你的代码是最新的：

```bash
# 切换到主分支
git checkout main

# 从上游仓库拉取最新代码
git fetch upstream

# 合并上游的更改
git merge upstream/main

# 推送到你的 Fork
git push origin main
```

### 步骤 4: 创建功能分支

**重要**: 永远不要直接在 `main` 分支上工作！

```bash
# 创建并切换到新分支
git checkout -b feature/你的功能名称

# 或者修复 Bug
git checkout -b fix/问题描述

# 或者改进文档
git checkout -b docs/文档主题
```

**分支命名规范**:
- `feature/功能名称` - 新功能
- `fix/问题描述` - Bug 修复
- `docs/文档主题` - 文档改进
- `refactor/重构内容` - 代码重构
- `test/测试内容` - 测试相关
- `style/样式内容` - UI/样式改进

### 步骤 5: 进行开发

#### 后端开发（Go）

```bash
# 进入项目根目录
cd WeKnora

# 启动开发环境（快速开发模式）
make dev-start      # 启动基础设施
make dev-app        # 启动后端（新终端）

# 或者使用脚本
./scripts/dev.sh start
./scripts/dev.sh app
```

#### 前端开发（Vue.js）

```bash
# 进入前端目录
cd frontend

# 安装依赖（首次）
npm install

# 启动开发服务器
npm run dev

# 或者使用 Make 命令
make dev-frontend
```

### 步骤 6: 提交更改

#### 6.1 查看更改

```bash
# 查看所有更改
git status

# 查看具体改动
git diff
```

#### 6.2 暂存更改

```bash
# 暂存所有更改
git add .

# 或者暂存特定文件
git add path/to/file1.go path/to/file2.vue

# 查看暂存的文件
git status
```

#### 6.3 提交更改

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```bash
# 提交格式：<type>(<scope>): <subject>
git commit -m "feat(frontend): 添加颜色统一化功能"

# 或者更详细的提交信息
git commit -m "feat(frontend): 添加颜色统一化功能

- 替换硬编码颜色为 CSS 变量
- 支持主题色动态切换
- 更新相关组件样式

Closes #123"
```

**提交类型**:
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更改
- `style`: 代码格式（不影响功能）
- `refactor`: 代码重构
- `test`: 添加测试
- `chore`: 构建过程或辅助工具的变动

### 步骤 7: 推送分支

```bash
# 推送你的分支到你的 Fork
git push origin feature/你的功能名称

# 如果是第一次推送，设置上游分支
git push -u origin feature/你的功能名称
```

### 步骤 8: 创建 Pull Request

1. **访问你的 GitHub Fork**: `https://github.com/你的用户名/WeKnora`

2. **你会看到提示创建 Pull Request**，点击 "Compare & pull request"

3. **填写 PR 信息**:
   - **标题**: 清晰描述你的更改
     ```
     feat: 添加颜色统一化功能
     ```
   - **描述**: 详细说明更改内容
     ```markdown
     ## 变更说明
     
     - 替换硬编码颜色为 CSS 变量
     - 支持主题色动态切换
     - 更新相关组件样式
     
     ## 相关 Issue
     Closes #123
     
     ## 测试说明
     - [ ] 已测试主题色切换功能
     - [ ] 已测试深色/浅色模式
     - [ ] 已检查所有相关组件
     
     ## 截图（如适用）
     [添加截图]
     ```

4. **选择目标分支**: 通常是 `main`

5. **点击 "Create pull request"**

### 步骤 9: 代码审查

- 维护者会审查你的代码
- 根据反馈进行修改
- 如果有需要修改的地方，继续提交到同一分支：

```bash
# 修改代码后
git add .
git commit -m "fix: 修复代码审查反馈的问题"
git push origin feature/你的功能名称
```

PR 会自动更新，无需创建新的 PR。

### 步骤 10: 合并后清理

PR 合并后，清理本地分支：

```bash
# 切换回主分支
git checkout main

# 拉取最新代码（包含你的更改）
git pull upstream main

# 删除已合并的功能分支
git branch -d feature/你的功能名称

# 删除远程分支（可选）
git push origin --delete feature/你的功能名称
```

## 🎨 代码规范

### Go 代码规范

1. **遵循官方规范**: [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

2. **格式化代码**:
   ```bash
   gofmt -w .
   # 或使用 goimports
   goimports -w .
   ```

3. **运行 linter**:
   ```bash
   golangci-lint run
   ```

4. **添加测试**:
   ```bash
   go test ./...
   ```

### 前端代码规范

1. **格式化代码**:
   ```bash
   cd frontend
   npm run lint
   npm run format
   ```

2. **TypeScript 类型检查**:
   ```bash
   npm run type-check
   ```

3. **遵循 Vue 3 最佳实践**:
   - 使用 Composition API
   - 使用 `<script setup>` 语法
   - 遵循组件命名规范

### 通用规范

- **代码注释**: 为复杂逻辑添加注释
- **变量命名**: 使用有意义的变量名
- **函数长度**: 保持函数简洁，单一职责
- **错误处理**: 妥善处理错误情况

## 📝 提交规范

### Conventional Commits 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 示例

```bash
# 简单提交
git commit -m "feat: 添加用户认证功能"

# 带范围的提交
git commit -m "feat(api): 添加用户登录接口"

# 详细提交
git commit -m "fix(frontend): 修复菜单 SVG 图标颜色问题

- 将 SVG 图标改为内联方式
- 使用 currentColor 支持主题色
- 更新相关组件样式

Closes #456"
```

### 提交类型说明

| 类型 | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat: 添加知识库搜索功能` |
| `fix` | Bug 修复 | `fix: 修复向量检索精度问题` |
| `docs` | 文档更改 | `docs: 更新 API 文档` |
| `style` | 代码格式 | `style: 格式化代码` |
| `refactor` | 代码重构 | `refactor: 重构文档解析模块` |
| `test` | 测试相关 | `test: 添加检索引擎测试用例` |
| `chore` | 构建/工具 | `chore: 更新依赖版本` |
| `perf` | 性能优化 | `perf: 优化向量检索性能` |
| `ci` | CI 配置 | `ci: 添加 GitHub Actions` |

## 🔍 Pull Request 指南

### PR 标题规范

- 使用 Conventional Commits 格式
- 简洁明了，不超过 72 个字符
- 使用中文或英文（保持一致）

### PR 描述模板

```markdown
## 变更说明
<!-- 描述你的更改内容 -->

## 相关 Issue
<!-- 关联的 Issue，如：Closes #123 -->

## 变更类型
<!-- 勾选适用的类型 -->
- [ ] Bug 修复
- [ ] 新功能
- [ ] 文档更新
- [ ] 代码重构
- [ ] 性能优化
- [ ] 测试相关

## 测试说明
<!-- 描述如何测试你的更改 -->
- [ ] 已测试功能 A
- [ ] 已测试功能 B
- [ ] 已通过所有单元测试

## 截图（如适用）
<!-- 添加相关截图 -->

## 检查清单
- [ ] 代码已通过 lint 检查
- [ ] 已添加必要的测试
- [ ] 已更新相关文档
- [ ] 提交信息符合规范
```

### PR 审查流程

1. **自动检查**: CI 会自动运行测试和 lint
2. **代码审查**: 至少需要一名维护者审查
3. **反馈处理**: 根据审查意见修改代码
4. **合并**: 审查通过后由维护者合并

## ❓ 常见问题

### Q: 如何同步上游仓库的最新代码？

```bash
git checkout main
git fetch upstream
git merge upstream/main
git push origin main
```

### Q: 我的 PR 需要多长时间才能合并？

- 这取决于 PR 的复杂性和维护者的时间安排
- 通常会在 1-3 个工作日内得到回复
- 如果一周内没有回复，可以友好地提醒维护者

### Q: 如何撤销一次提交？

```bash
# 撤销最后一次提交（保留更改）
git reset --soft HEAD~1

# 完全撤销最后一次提交（丢弃更改）
git reset --hard HEAD~1
```

### Q: 如何修改最后一次提交？

```bash
# 修改提交信息
git commit --amend -m "新的提交信息"

# 添加文件到上次提交
git add forgotten-file.go
git commit --amend --no-edit
```

### Q: 如何处理合并冲突？

```bash
# 拉取最新代码
git fetch upstream
git merge upstream/main

# 如果有冲突，解决冲突后
git add .
git commit -m "fix: 解决合并冲突"
git push origin feature/你的功能名称
```

### Q: 我可以提交多个不相关的更改吗？

**不建议**。每个 PR 应该专注于一个功能或修复。如果有多项更改，请分别创建多个 PR。

### Q: 如何贡献文档？

文档贡献同样重要！你可以：
- 修复文档中的错误
- 添加缺失的说明
- 改进文档的可读性
- 翻译文档到其他语言

## 📞 获取帮助

如果你在贡献过程中遇到问题：

1. **查看文档**: 先查看项目文档和 README
2. **搜索 Issue**: 在 GitHub Issues 中搜索相关问题
3. **创建 Issue**: 如果问题未解决，创建新的 Issue
4. **社区讨论**: 参与项目讨论区

## 🙏 致谢

感谢你为 WeKnora 项目做出的贡献！每一个贡献，无论大小，都让项目变得更好。

---

**Happy Contributing! 🎉**
