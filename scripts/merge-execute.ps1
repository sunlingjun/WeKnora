# 代码合并执行脚本
# 创建日期: 2026-04-01
# 用途: 执行upstream/main合并，自动处理已知冲突模式

param(
    [switch]$DryRun,      # 仅预览，不实际执行
    [switch]$AutoResolve, # 自动解决已知模式的冲突
    [switch]$SkipBuild,   # 跳过构建验证
    [string]$PatchDir = ""  # 补丁文件目录（用于恢复）
)

$ErrorActionPreference = "Stop"
$dateStamp = Get-Date -Format "yyyyMMdd-HHmmss"
$logFile = "merge-log-$dateStamp.txt"

# 颜色定义
$colors = @{
    Info = "Cyan"
    Success = "Green"
    Warning = "Yellow"
    Error = "Red"
    Title = "Magenta"
}

function Write-Log {
    param($Message, $Level = "Info")
    $timestamp = Get-Date -Format "HH:mm:ss"
    $color = $colors[$Level]
    Write-Host "[$timestamp] $Message" -ForegroundColor $color
    "[$timestamp] [$Level] $Message" | Out-File -FilePath $logFile -Append
}

function Start-MergeProcess {
    Write-Log "========================================" "Title"
    Write-Log "开始执行代码合并" "Title"
    Write-Log "========================================" "Title"

    # 1. 前置检查
    Write-Log "`n--- 步骤1: 前置检查 ---" "Info"

    # 检查工作区状态
    $status = git status --porcelain
    if ($status) {
        Write-Log "工作区有未提交修改，请先提交或暂存" "Error"
        git status
        return
    }
    Write-Log "✓ 工作区干净" "Success"

    # 检查upstream
    $remotes = git remote
    if (!($remotes -contains "upstream")) {
        Write-Log "upstream远程未配置" "Error"
        return
    }
    Write-Log "✓ upstream远程已配置" "Success"

    # 2. 创建备份
    Write-Log "`n--- 步骤2: 创建备份分支 ---" "Info"
    $backupBranch = "backup/merge-$dateStamp"
    git branch $backupBranch
    Write-Log "✓ 备份分支已创建: $backupBranch" "Success"

    # 3. 获取远程更新
    Write-Log "`n--- 步骤3: 获取远程更新 ---" "Info"
    if (!$DryRun) {
        git fetch upstream
    }
    $localCommit = git rev-parse --short HEAD
    $remoteCommit = git rev-parse --short upstream/main
    $commitCount = git rev-list --count HEAD..upstream/main
    Write-Log "本地: $localCommit" "Info"
    Write-Log "远程: $remoteCommit ($commitCount 个新提交)" "Info"

    # 4. 开始合并
    Write-Log "`n--- 步骤4: 开始合并 ---" "Info"
    if ($DryRun) {
        Write-Log "[DRY RUN] 将执行: git merge upstream/main --no-commit --no-ff" "Warning"
        return
    }

    # 尝试合并
    $mergeOutput = git merge upstream/main --no-commit --no-ff 2>&1
    $exitCode = $LASTEXITCODE

    if ($exitCode -eq 0 -and !($mergeOutput -match "conflict")) {
        Write-Log "✓ 合并成功，无冲突" "Success"
        Step-PostMerge
        return
    }

    # 有冲突，显示冲突状态
    Write-Log "发现冲突，需要解决" "Warning"
    Write-Log "冲突状态:" "Info"
    git status --short | ForEach-Object { Write-Log "  $_" "Warning" }

    # 5. 自动解决已知冲突
    if ($AutoResolve) {
        Write-Log "`n--- 步骤5: 自动解决已知冲突 ---" "Info"
        Resolve-KnownConflicts
    }

    # 6. 显示剩余冲突
    $remainingConflicts = git diff --name-only --diff-filter=U
    if ($remainingConflicts) {
        Write-Log "`n--- 剩余冲突文件（需手动解决）---" "Error"
        $remainingConflicts | ForEach-Object { Write-Log "  ⚠ $_" "Error" }
        Write-Log "`n解决冲突后执行:" "Info"
        Write-Log "  git add ." "Info"
        Write-Log "  git commit -m 'Merge upstream/main with local features preserved'" "Info"
    } else {
        Write-Log "✓ 所有冲突已解决" "Success"
        Step-PostMerge
    }
}

function Resolve-KnownConflicts {
    # 获取冲突文件列表
    $conflictFiles = git diff --name-only --diff-filter=U

    foreach ($file in $conflictFiles) {
        Write-Log "处理冲突: $file" "Info"

        switch -Wildcard ($file) {
            "*theme.css" {
                # theme.css: 完全保留本地
                Write-Log "  策略: 保留本地CSS主题" "Info"
                git checkout HEAD -- $file
                git add $file
                Write-Log "  ✓ 已解决" "Success"
            }

            "*cas_auth.go" {
                # CAS文件: 保留本地
                Write-Log "  策略: 保留本地CAS实现" "Info"
                git checkout HEAD -- $file
                git add $file
                Write-Log "  ✓ 已解决" "Success"
            }

            "*shared_kb.go" {
                # 共享知识库: 保留本地
                Write-Log "  策略: 保留本地共享知识库实现" "Info"
                git checkout HEAD -- $file
                git add $file
                Write-Log "  ✓ 已解决" "Success"
            }

            "*SvgIcon.vue" {
                # SVG组件: 保留本地
                Write-Log "  策略: 保留本地SVG组件" "Info"
                git checkout HEAD -- $file
                git add $file
                Write-Log "  ✓ 已解决" "Success"
            }

            "*registry.ts" {
                # 图标注册表: 保留本地
                Write-Log "  策略: 保留本地图标注册表" "Info"
                git checkout HEAD -- $file
                git add $file
                Write-Log "  ✓ 已解决" "Success"
            }

            "*main.go" {
                # main.go: 需要手动合并（标记为复杂）
                Write-Log "  策略: 需手动合并（服务器启动逻辑）" "Warning"
                Write-Log "  提示: 接受远程改进，保留CAS初始化" "Warning"
            }

            "*router.go" {
                # router: 需要手动合并
                Write-Log "  策略: 需手动合并（路由定义）" "Warning"
                Write-Log "  提示: 合并双方路由，保留CAS路由" "Warning"
            }

            "*knowledge.go" {
                # knowledge.go: 需要手动合并
                Write-Log "  策略: 需手动合并（Channel参数）" "Warning"
                Write-Log "  提示: 合并远程Channel参数，保留共享知识库扩展" "Warning"
            }

            default {
                Write-Log "  策略: 未知冲突，需手动检查" "Warning"
            }
        }
    }
}

function Step-PostMerge {
    Write-Log "`n--- 步骤6: 合并后验证 ---" "Info"

    # 检查是否还有未解决的冲突标记
    $conflictMarkers = git grep -l "<<<<<<<" -- "*.go" "*.vue" "*.ts" "*.yaml" "*.yml" "*.css" 2>$null
    if ($conflictMarkers) {
        Write-Log "⚠ 发现未解决的冲突标记:" "Error"
        $conflictMarkers | ForEach-Object { Write-Log "  $_" "Error" }
        return
    }
    Write-Log "✓ 无冲突标记残留" "Success"

    # 构建验证
    if (!$SkipBuild) {
        Write-Log "`n--- 后端构建验证 ---" "Info"
        $buildOutput = go build ./cmd/server 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log "✓ 后端构建成功" "Success"
        } else {
            Write-Log "✗ 后端构建失败" "Error"
            Write-Log $buildOutput "Error"
            return
        }

        Write-Log "`n--- 前端构建验证 ---" "Info"
        Set-Location frontend
        $npmInstall = npm install 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Log "✗ npm install 失败" "Error"
            Set-Location ..
            return
        }

        $npmBuild = npm run build 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log "✓ 前端构建成功" "Success"
        } else {
            Write-Log "✗ 前端构建失败" "Error"
            Write-Log $npmBuild "Error"
            Set-Location ..
            return
        }
        Set-Location ..
    }

    # 建议提交
    Write-Log "`n========================================" "Title"
    Write-Log "合并验证通过" "Success"
    Write-Log "========================================" "Title"
    Write-Log "执行以下命令完成合并:" "Info"
    Write-Log "  git add ." "Info"
    Write-Log "  git commit -m 'Merge upstream/main (preserve CAS, shared KB, theme, SVG)'" "Info"
}

function Restore-FromPatches {
    param([string]$PatchDirectory)

    Write-Log "========================================" "Title"
    Write-Log "从补丁恢复本地功能" "Title"
    Write-Log "========================================" "Title"

    if (!(Test-Path $PatchDirectory)) {
        Write-Log "补丁目录不存在: $PatchDirectory" "Error"
        return
    }

    $patches = Get-ChildItem $PatchDirectory -Filter "*.patch"
    if (!$patches) {
        Write-Log "补丁目录为空" "Error"
        return
    }

    Write-Log "发现 $($patches.Count) 个补丁文件" "Info"

    foreach ($patch in $patches) {
        Write-Log "应用补丁: $($patch.Name)" "Info"
        $result = git apply $patch.FullName 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log "  ✓ 应用成功" "Success"
        } else {
            Write-Log "  ✗ 应用失败: $result" "Error"
        }
    }
}

# 主逻辑
if ($PatchDir) {
    Restore-FromPatches -PatchDirectory $PatchDir
} else {
    Start-MergeProcess
}

Write-Log "`n日志已保存到: $logFile" "Info"
