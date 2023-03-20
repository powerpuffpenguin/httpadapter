#!/usr/bin/env bash
set -e

# 設置顯示的命令名稱
CommandName="build.sh"

# 項目根路徑
cd `dirname $BASH_SOURCE`
ProjectDir=`pwd`

# 設置環境
Docker="king011/httpadapter" # 發佈名稱
DefaultTag="1.0" # 默認版本
Dockerfile="`pwd`/dockerfile" # 設置 dockerfile 檔案夾

# 加載腳本
source ./command/tools.sh
# 調用腳本
main "${@}"
