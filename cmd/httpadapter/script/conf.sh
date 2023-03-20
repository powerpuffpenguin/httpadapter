Target="httpadapter"
Docker="king011/httpadapter"
Dir=$(cd "$(dirname $BASH_SOURCE)/.." && pwd)
Platforms=(
    darwin/amd64
    windows/amd64
    linux/arm
    linux/amd64
)