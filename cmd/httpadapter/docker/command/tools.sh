# 檢查環變量設定設定
if [[ "$Docker" == "" ]];then
    echo "variable 'Docker' is not defined"
    exit 1
elif [[ "$DefaultTag" == "" ]];then
    echo "variable 'DefaultTag' is not defined"
    exit 1
elif [[ "$Dockerfile" == "" ]];then
    echo "variable 'Dockerfile' is not defined"
    exit 1
fi

# 根路徑
cd `dirname $BASH_SOURCE`
# 腳本所在根路徑
BashDir=`pwd`

Script="$BashDir/scripts"
# 當前指令
if [[ "$CommandName" == "" ]];then
   CommandName="tools.sh"
fi
Command="$CommandName"

# 加載公共代碼
file="$Script/lib.sh"
if [[ -f "$file" ]];then
   source "$file"
fi

# 加載子命令
file="$Script/commnads"
if [[ -d "$file" ]];then
   commnads=$(find "$file" -maxdepth 1 -name "*.sh" -type f | while read line
      do
         name=$(basename "$line")
         for str in $name
         do
               if [[ "$str" == "$name" ]];then
                  name=${name%.sh}
                  echo "$name "
               fi
               break
         done
      done
   )
else
   commnads=()
fi

# 加載項目
file="$Dockerfile"
if [[ -d "$file" ]];then
   dockerfile=$(find "$file" -maxdepth 1 -type d | while read line
      do
        name=$(basename "$line")
        if [[ "$name" == dockerfile ]];then
            continue
        fi
        if [[ -f "$line/Dockerfile" ]];then
            echo -n "$name "
        fi
      done
   )
else
   dockerfile=()
fi
# 顯示幫助
function display_help
{
   echo "$Command script"
   echo
   echo "Usage:"
   echo "  $Command [flags]"
   echo "  $Command [command]"
   echo
   echo "Available Commands:"

   my_PrintFlag "build"  "docker build for $Docker"
   my_PrintFlag "exec"  "docker exec for $Docker"
   my_PrintFlag "images"  "docker images for $Docker"
   my_PrintFlag "inspect"  "docker inspect for $Docker"
   my_PrintFlag "logs"  "docker logs for $Docker"
   my_PrintFlag "ps"  "docker ps for $Docker"
   my_PrintFlag "pull"  "docker pull for $Docker"
   my_PrintFlag "push"  "docker push for $Docker"
   my_PrintFlag "restart"  "docker restart for $Docker"
   my_PrintFlag "rm"  "docker rm for $Docker"
   my_PrintFlag "run"  "docker run for $Docker"
   my_PrintFlag "start"  "docker start for $Docker"
   my_PrintFlag "stop"  "docker stop for $Docker"

   echo
   echo "Flags:"
   my_PrintFlag "-h, --help" "help for $Command"
   my_PrintFlag "-l, --list" "list dockerfile dir"
}
# 實現主函數
function main
{
   case "$1" in
      -h|--help)
         display_help
         return $?
      ;;
      -l|--list)
         for file in ${dockerfile[@]}; do
            echo "$file"
         done
         return $?
      ;;
      *)
         local commnad
         for commnad in ${commnads[@]}
         do
            if [[ "$1" == "$commnad" ]];then
               shift
               Command="$Command $commnad"
               # 加載子命令
               source "$Script/commnads/$commnad.sh"
               sub_command "${@}"
               return $?
            fi
         done

         if [[ "$1" == "" ]];then
            display_help
         elif [[ "$1" == -* ]];then
            echo Error: unknown flag "'$1'" for "$Command"
            echo "Run '$Command --help' for usage."
         else
            echo Error: unknown command "'$1'" for "$Command"
            echo "Run '$Command --help' for usage."
         fi        
         return 1
      ;;
   esac
}