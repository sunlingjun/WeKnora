# WeKnora 冲突解决辅助脚本
# 用途：帮助分析和解决 Git 合并冲突

param(
    [switch]$Analyze,      # 仅分析冲突，不解决
    [switch]$Backup,       # 创建备份
    [switch]$Stash,        # 暂存本地修改
    [switch]$Merge         # 执行合并
)

$ErrorActionPreference = "Stop"

Write-Host "=== WeKnora 冲突解决辅助脚本 ===" -ForegroundColor Cyan
Write-Host ""

# 检查 Git 状态
function Check-GitStatus {
    Write-Host "检查 Git 状态..." -ForegroundColor Yellow
    $status = git status --porcelain
    if ($LASTEXITCODE -ne 0) {
        Write-Host "错误: 不在 Git 仓库中" -ForegroundColor Red
        exit 1
    }
    return $status
}

# 分析冲突
function Analyze-Conflicts {
    Write-Host "`n=== 冲突分析 ===" -ForegroundColor Cyan
    
    # 检查是否落后于上游
    $behindOutput = git rev-list --count HEAD..origin/main 2>&1
    if ($LASTEXITCODE -eq 0 -and $behindOutput -match '^\d+$') {
        $behind = [int]$behindOutput
        if ($behind -gt 0) {
            Write-Host "警告: 本地分支落后于 origin/main $behind 个提交" -ForegroundColor Yellow
        }
    }
    
    # 统计已暂存文件
    $staged = (git diff --cached --name-only).Count
    Write-Host "已暂存文件: $staged 个" -ForegroundColor Green
    
    # 统计未暂存修改
    $unstaged = (git diff --name-only).Count
    Write-Host "未暂存修改: $unstaged 个" -ForegroundColor Yellow
    
    # 统计未跟踪文件
    $untracked = (git ls-files --others --exclude-standard).Count
    Write-Host "未跟踪文件: $untracked 个" -ForegroundColor Magenta
    
    # 找出冲突文件
    Write-Host "`n=== 潜在冲突文件 ===" -ForegroundColor Cyan
    $stagedFiles = git diff --cached --name-only
    $unstagedFiles = git diff --name-only
    
    $conflicts = Compare-Object $stagedFiles $unstagedFiles -IncludeEqual | Where-Object { $_.SideIndicator -eq "==" }
    
    if ($conflicts) {
        Write-Host "发现 $($conflicts.Count) 个潜在冲突文件:" -ForegroundColor Red
        foreach ($file in $conflicts.InputObject) {
            Write-Host "  [警告] $file" -ForegroundColor Yellow
        }
    } else {
        Write-Host "[完成] 未发现明显冲突文件" -ForegroundColor Green
    }
    
    # 高优先级冲突文件
    Write-Host "`n=== 高优先级冲突文件 ===" -ForegroundColor Cyan
    $highPriority = @(
        "frontend/src/components/Input-field.vue",
        "frontend/src/components/menu.vue",
        "frontend/src/components/AgentSelector.vue",
        "frontend/src/assets/theme/theme.css"
    )
    
    foreach ($file in $highPriority) {
        $inStaged = $stagedFiles -contains $file
        $inUnstaged = $unstagedFiles -contains $file
        
        if ($inStaged -and $inUnstaged) {
            Write-Host "  [冲突] $file (已暂存 + 未暂存)" -ForegroundColor Red
        } elseif ($inStaged) {
            Write-Host "  [已暂存] $file" -ForegroundColor Yellow
        } elseif ($inUnstaged) {
            Write-Host "  [未暂存] $file" -ForegroundColor Green
        }
    }
}

# 创建备份
function Create-Backup {
    Write-Host "`n=== 创建备份 ===" -ForegroundColor Cyan
    
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $backupBranch = "backup-$timestamp"
    
    Write-Host "创建备份分支: $backupBranch" -ForegroundColor Yellow
    git branch $backupBranch
    
    # 创建补丁文件
    $patchFile = "backup-local-changes-$timestamp.patch"
    Write-Host "创建补丁文件: $patchFile" -ForegroundColor Yellow
    git diff > $patchFile
    git diff --cached > "backup-staged-$timestamp.patch"
    
    Write-Host "[完成] 备份完成" -ForegroundColor Green
    Write-Host "  备份分支: $backupBranch" -ForegroundColor Gray
    Write-Host "  补丁文件: $patchFile" -ForegroundColor Gray
}

# 暂存本地修改
function Stash-LocalChanges {
    Write-Host "`n=== 暂存本地修改 ===" -ForegroundColor Cyan
    
    $message = Read-Host "请输入暂存消息 (默认: 本地修改)"
    if ([string]::IsNullOrWhiteSpace($message)) {
        $message = "本地修改"
    }
    
    Write-Host "执行 git stash..." -ForegroundColor Yellow
    git stash push -u -m $message
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[完成] 本地修改已暂存" -ForegroundColor Green
        Write-Host "使用 'git stash list' 查看暂存列表" -ForegroundColor Gray
        Write-Host "使用 'git stash pop' 恢复修改" -ForegroundColor Gray
    } else {
        Write-Host "❌ 暂存失败" -ForegroundColor Red
    }
}

# 执行合并
function Start-Merge {
    Write-Host "`n=== 执行合并 ===" -ForegroundColor Cyan
    
    Write-Host "警告: 这将开始合并过程" -ForegroundColor Yellow
    $confirm = Read-Host "是否继续? (y/N)"
    
    if ($confirm -ne "y" -and $confirm -ne "Y") {
        Write-Host "已取消" -ForegroundColor Yellow
        return
    }
    
    # 1. 提交已暂存的更改
    Write-Host "`n步骤 1: 提交已暂存的更改..." -ForegroundColor Yellow
    $stagedCount = (git diff --cached --name-only).Count
    if ($stagedCount -gt 0) {
        git commit -m "chore: 合并上游更新"
        Write-Host "[完成] 已提交 $stagedCount 个文件" -ForegroundColor Green
    } else {
        Write-Host "ℹ️  没有已暂存的文件" -ForegroundColor Gray
    }
    
    # 2. 恢复本地修改
    Write-Host "`n步骤 2: 恢复本地修改..." -ForegroundColor Yellow
    $stashList = git stash list
    if ($stashList) {
        Write-Host "发现暂存的修改:" -ForegroundColor Gray
        git stash list | Select-Object -First 5
        $confirm = Read-Host "`n是否恢复最新的暂存? (Y/n)"
        if ($confirm -ne "n" -and $confirm -ne "N") {
            git stash pop
            Write-Host "[完成] 已恢复本地修改" -ForegroundColor Green
        }
    } else {
        Write-Host "ℹ️  没有暂存的修改" -ForegroundColor Gray
    }
    
    # 3. 检查冲突
    Write-Host "`n步骤 3: 检查冲突..." -ForegroundColor Yellow
    $conflicts = git diff --name-only --diff-filter=U
    if ($conflicts) {
        Write-Host "发现冲突文件:" -ForegroundColor Red
        $conflicts | ForEach-Object { Write-Host "  [冲突] $_" -ForegroundColor Yellow }
        Write-Host "`n请使用以下命令解决冲突:" -ForegroundColor Cyan
        Write-Host "  git mergetool" -ForegroundColor White
        Write-Host "  或使用 VS Code 打开冲突文件" -ForegroundColor White
    } else {
        Write-Host "[完成] 未发现冲突" -ForegroundColor Green
    }
}

# 主流程
function Main {
    Check-GitStatus | Out-Null
    
    if ($Analyze) {
        Analyze-Conflicts
    }
    
    if ($Backup) {
        Create-Backup
    }
    
    if ($Stash) {
        Stash-LocalChanges
    }
    
    if ($Merge) {
        Start-Merge
    }
    
    # 如果没有指定任何操作，显示帮助
    if (-not ($Analyze -or $Backup -or $Stash -or $Merge)) {
        Write-Host "用法:" -ForegroundColor Cyan
        Write-Host "  .\resolve-conflicts.ps1 -Analyze    # 分析冲突"
        Write-Host "  .\resolve-conflicts.ps1 -Backup    # 创建备份"
        Write-Host "  .\resolve-conflicts.ps1 -Stash     # 暂存本地修改"
        Write-Host "  .\resolve-conflicts.ps1 -Merge     # 执行合并"
        Write-Host ""
        Write-Host "示例:" -ForegroundColor Cyan
        Write-Host "  .\resolve-conflicts.ps1 -Analyze -Backup"
        Write-Host ""
        
        # 默认执行分析
        Analyze-Conflicts
    }
}

Main
