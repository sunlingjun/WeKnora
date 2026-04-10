# 合并前检查脚本
# 创建日期: 2026-04-01
# 用途: 在合并upstream/main前执行环境检查和备份

param(
    [switch]$CreateBackup,
    [switch]$GeneratePatches,
    [switch]$CheckStatus,
    [switch]$All
)

$ErrorActionPreference = "Stop"
$dateStamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupBranch = "backup/merge-before-$dateStamp"
$patchDir = "patches/merge-$dateStamp"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "    代码合并前检查脚本" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查是否在git仓库中
function Test-GitRepository {
    try {
        $null = git rev-parse --git-dir 2>&1
        return $true
    } catch {
        Write-Error "当前目录不是Git仓库"
        return $false
    }
}

# 检查远程upstream是否存在
function Test-UpstreamRemote {
    $remotes = git remote
    if ($remotes -contains "upstream") {
        $upstreamUrl = git remote get-url upstream
        Write-Host "✓ upstream远程已配置: $upstreamUrl" -ForegroundColor Green
        return $true
    } else {
        Write-Warning "upstream远程未配置"
        Write-Host "  建议执行: git remote add upstream <upstream-url>" -ForegroundColor Yellow
        return $false
    }
}

# 获取提交信息
function Get-CommitInfo {
    Write-Host "`n--- 提交信息 ---" -ForegroundColor Cyan

    $localCommit = git rev-parse --short HEAD
    $localMsg = git log -1 --pretty=format:"%s" HEAD
    Write-Host "本地最新提交: $localCommit - $localMsg" -ForegroundColor White

    git fetch upstream --quiet
    $upstreamCommit = git rev-parse --short upstream/main
    $upstreamMsg = git log -1 --pretty=format:"%s" upstream/main
    Write-Host "远程最新提交: $upstreamCommit - $upstreamMsg" -ForegroundColor White

    $commitDiff = git rev-list --count HEAD..upstream/main
    Write-Host "远程领先本地: $commitDiff 个提交" -ForegroundColor $(if ($commitDiff -gt 50) { "Red" } else { "Yellow" })
}

# 检查本地修改
function Test-LocalChanges {
    Write-Host "`n--- 本地修改检查 ---" -ForegroundColor Cyan

    $status = git status --porcelain
    if ($status) {
        $modifiedFiles = $status | Where-Object { $_ -match "^\s*M" } | Measure-Object
        $addedFiles = $status | Where-Object { $_ -match "^\s*A" } | Measure-Object
        $deletedFiles = $status | Where-Object { $_ -match "^\s*D" } | Measure-Object

        Write-Host "本地有未提交的修改:" -ForegroundColor Yellow
        Write-Host "  修改: $($modifiedFiles.Count) 个文件" -ForegroundColor White
        Write-Host "  新增: $($addedFiles.Count) 个文件" -ForegroundColor White
        Write-Host "  删除: $($deletedFiles.Count) 个文件" -ForegroundColor White

        $untracked = git ls-files --others --exclude-standard | Measure-Object
        Write-Host "  未跟踪: $($untracked.Count) 个文件" -ForegroundColor White

        return $false
    } else {
        Write-Host "✓ 工作区干净" -ForegroundColor Green
        return $true
    }
}

# 创建备份分支
function New-BackupBranch {
    Write-Host "`n--- 创建备份分支 ---" -ForegroundColor Cyan

    try {
        git branch $backupBranch
        Write-Host "✓ 备份分支已创建: $backupBranch" -ForegroundColor Green

        # 导出当前状态信息
        $infoFile = "merge-info-$dateStamp.txt"
        @"
备份时间: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
本地分支: $(git branch --show-current)
本地提交: $(git rev-parse HEAD)
远程提交: $(git rev-parse upstream/main)
提交差异: $(git rev-list --count HEAD..upstream/main)

本地核心功能文件:
$(git diff upstream/main...HEAD --name-only | Select-String -Pattern "cas|shared|theme|icons" | ForEach-Object { "  - $_" })

潜在冲突文件:
$(git diff --name-only upstream/main...HEAD | ForEach-Object { "  - $_" })
"@ | Out-File -FilePath $infoFile -Encoding UTF8

        Write-Host "✓ 备份信息已保存到: $infoFile" -ForegroundColor Green
    } catch {
        Write-Error "创建备份分支失败: $_"
    }
}

# 生成补丁文件
function New-PatchFiles {
    Write-Host "`n--- 生成保护补丁 ---" -ForegroundColor Cyan

    if (!(Test-Path $patchDir)) {
        New-Item -ItemType Directory -Path $patchDir -Force | Out-Null
    }

    $protectedFiles = @(
        @{Path="internal/handler/cas_auth.go"; Name="cas_auth"},
        @{Path="internal/application/service/cas_auth.go"; Name="cas_service_auth"},
        @{Path="internal/application/service/cas_client.go"; Name="cas_client"},
        @{Path="internal/types/cas.go"; Name="cas_types"},
        @{Path="frontend/src/stores/cas.ts"; Name="cas_store"},
        @{Path="internal/application/service/shared_kb.go"; Name="shared_kb_service"},
        @{Path="frontend/src/assets/theme/theme.css"; Name="theme_css"},
        @{Path="frontend/src/components/icons/SvgIcon.vue"; Name="svg_icon"},
        @{Path="frontend/src/components/icons/registry.ts"; Name="svg_registry"}
    )

    $generatedCount = 0
    foreach ($file in $protectedFiles) {
        $patchFile = "$patchDir/$($file.Name).patch"
        try {
            # 检查文件是否有本地修改
            $hasChanges = git diff upstream/main...HEAD --name-only | Select-String -Pattern $file.Path -SimpleMatch
            if ($hasChanges) {
                git diff upstream/main...HEAD -- $file.Path > $patchFile 2>$null
                if ((Get-Content $patchFile -ErrorAction SilentlyContinue | Measure-Object).Count -gt 0) {
                    Write-Host "✓ 已生成补丁: $patchFile" -ForegroundColor Green
                    $generatedCount++
                }
            } else {
                Write-Host "  跳过（无本地修改）: $($file.Path)" -ForegroundColor Gray
            }
        } catch {
            Write-Warning "生成补丁失败 ($($file.Path)): $_"
        }
    }

    Write-Host "`n共生成 $generatedCount 个保护补丁" -ForegroundColor Cyan
    Write-Host "补丁目录: $patchDir" -ForegroundColor White
}

# 分析潜在冲突
function Get-ConflictAnalysis {
    Write-Host "`n--- 潜在冲突分析 ---" -ForegroundColor Cyan

    # 获取双方修改的文件
    $localFiles = git diff upstream/main...HEAD --name-only
    $remoteFiles = git diff HEAD...upstream/main --name-only

    # 找出双方都有修改的文件（潜在冲突）
    $conflictFiles = $localFiles | Where-Object { $remoteFiles -contains $_ }

    if ($conflictFiles) {
        Write-Host "发现以下文件在本地和远程都有修改（潜在冲突）：" -ForegroundColor Red
        $conflictFiles | ForEach-Object {
            $localLines = (git diff upstream/main...HEAD -- $_ | Measure-Object).Count
            $remoteLines = (git diff HEAD...upstream/main -- $_ | Measure-Object).Count
            Write-Host "  ⚠ $_ (本地: $localLines 行, 远程: $remoteLines 行)" -ForegroundColor Yellow
        }

        Write-Host "`n冲突解决优先级建议:" -ForegroundColor Cyan
        $priorities = @{
            "frontend/src/assets/theme/theme.css" = "⭐⭐⭐⭐⭐ 完全保留本地";
            "internal/handler/router.go" = "⭐⭐⭐⭐ 合并双方路由";
            "cmd/server/main.go" = "⭐⭐⭐⭐ 接受远程改进";
            "client/knowledge.go" = "⭐⭐⭐ 合并Channel参数";
            "config/config.yaml" = "⭐⭐⭐ 合并配置";
        }

        foreach ($file in $conflictFiles) {
            if ($priorities.ContainsKey($file)) {
                Write-Host "  $priorities[$file] - $file" -ForegroundColor White
            } else {
                Write-Host "  ⭐⭐ 视情况解决 - $file" -ForegroundColor Gray
            }
        }
    } else {
        Write-Host "✓ 未发现双方同时修改的文件（无冲突风险）" -ForegroundColor Green
    }
}

# 检查核心功能文件状态
function Test-CoreFiles {
    Write-Host "`n--- 核心功能文件检查 ---" -ForegroundColor Cyan

    $coreFiles = @(
        @{Path="internal/handler/cas_auth.go"; Feature="CAS认证"},
        @{Path="internal/application/service/shared_kb.go"; Feature="共享知识库"},
        @{Path="frontend/src/assets/theme/theme.css"; Feature="CSS主题"},
        @{Path="frontend/src/components/icons/SvgIcon.vue"; Feature="SVG组件"}
    )

    foreach ($file in $coreFiles) {
        if (Test-Path $file.Path) {
            $hasLocalChanges = git diff upstream/main...HEAD --name-only | Select-String -Pattern $file.Path -SimpleMatch
            $hasRemoteChanges = git diff HEAD...upstream/main --name-only | Select-String -Pattern $file.Path -SimpleMatch

            $status = if ($hasLocalChanges -and $hasRemoteChanges) {
                "⚠ 双方修改（需合并）"
            } elseif ($hasLocalChanges) {
                "✓ 仅本地修改（安全）"
            } elseif ($hasRemoteChanges) {
                "⚠ 仅远程修改（需验证）"
            } else {
                "✓ 无修改"
            }

            $color = if ($status -like "*安全*" -or $status -like "*无修改*") { "Green" } else { "Yellow" }
            Write-Host "$($file.Feature): $status" -ForegroundColor $color
        } else {
            Write-Host "$($file.Feature): ✗ 文件不存在" -ForegroundColor Red
        }
    }
}

# 生成合并建议
function Write-MergeAdvice {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "         合并执行建议" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan

    Write-Host "`n执行步骤:" -ForegroundColor White
    Write-Host "1. 确保当前工作区干净（提交或暂存所有修改）" -ForegroundColor Gray
    Write-Host "2. 执行: git merge upstream/main --no-commit --no-ff" -ForegroundColor Gray
    Write-Host "3. 如有冲突，按优先级解决（theme.css → router → main.go → 其他）" -ForegroundColor Gray
    Write-Host "4. 验证: go build ./cmd/server && cd frontend && npm run build" -ForegroundColor Gray
    Write-Host "5. 提交合并: git commit -m 'Merge upstream/main with local features preserved'" -ForegroundColor Gray

    Write-Host "`n冲突解决原则:" -ForegroundColor White
    Write-Host "- theme.css: 完全保留本地版本（CSS变量主题）" -ForegroundColor Yellow
    Write-Host "- CAS相关文件: 完全保留本地实现" -ForegroundColor Yellow
    Write-Host "- shared_kb.go: 完全保留本地实现" -ForegroundColor Yellow
    Write-Host "- router.go: 合并双方路由定义" -ForegroundColor Yellow
    Write-Host "- main.go: 接受远程改进，验证CAS初始化" -ForegroundColor Yellow
}

# 主函数
function Start-MergeCheck {
    if (!(Test-GitRepository)) { return }

    if ($All -or $CheckStatus) {
        Test-UpstreamRemote
        Get-CommitInfo
        $clean = Test-LocalChanges
        Get-ConflictAnalysis
        Test-CoreFiles
        Write-MergeAdvice
    }

    if ($All -or $CreateBackup) {
        New-BackupBranch
    }

    if ($All -or $GeneratePatches) {
        New-PatchFiles
    }

    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "         检查完成" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
}

# 执行
Start-MergeCheck
