# Git 配置脚本 - 切换到新账户
# 使用方法：在 PowerShell 中运行: .\setup_git.ps1

Write-Host "=== Git 仓库配置工具 ===" -ForegroundColor Cyan
Write-Host ""

# 获取新账户信息
$newUsername = Read-Host "请输入新的 Git 用户名"
$newEmail = Read-Host "请输入新的 Git 邮箱"
$newRepoUrl = Read-Host "请输入新的仓库地址 (例如: https://github.com/username/repo.git)"

if ([string]::IsNullOrWhiteSpace($newUsername) -or 
    [string]::IsNullOrWhiteSpace($newEmail) -or 
    [string]::IsNullOrWhiteSpace($newRepoUrl)) {
    Write-Host "错误: 所有字段都必须填写！" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "即将更新以下配置:" -ForegroundColor Yellow
Write-Host "  用户名: $newUsername"
Write-Host "  邮箱: $newEmail"
Write-Host "  仓库地址: $newRepoUrl"
Write-Host ""

$confirm = Read-Host "确认更新? (y/n)"
if ($confirm -ne "y" -and $confirm -ne "Y") {
    Write-Host "已取消操作" -ForegroundColor Yellow
    exit 0
}

# 更新 Git 配置
Write-Host ""
Write-Host "正在更新 Git 配置..." -ForegroundColor Green

# 设置本地仓库的用户名和邮箱
git config user.name "$newUsername"
git config user.email "$newEmail"

# 更新远程仓库地址
git remote set-url origin "$newRepoUrl"

# 验证配置
Write-Host ""
Write-Host "=== 配置完成 ===" -ForegroundColor Green
Write-Host ""
Write-Host "当前配置:" -ForegroundColor Cyan
Write-Host "  用户名: $(git config user.name)"
Write-Host "  邮箱: $(git config user.email)"
Write-Host "  远程仓库: $(git remote get-url origin)"
Write-Host ""

Write-Host "提示:" -ForegroundColor Yellow
Write-Host "  1. 如果使用 HTTPS，推送时可能需要输入新账户的凭据"
Write-Host "  2. 如果使用 SSH，请确保已配置新账户的 SSH 密钥"
Write-Host "  3. 可以使用以下命令测试连接:"
Write-Host "     git fetch origin"
Write-Host ""

