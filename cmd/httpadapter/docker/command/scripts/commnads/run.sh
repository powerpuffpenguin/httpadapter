function sub_help
{
    echo "docker run for $Docker"
    echo
    echo "Example:"
    echo "  # docker run shell for $Docker:$DefaultTag"
    echo "  $Command $DefaultTag"
    echo 
    echo "  # docker run shell for $Docker:$DefaultTag"
    echo "  $Command"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-n, --name" "docker name"
    my_PrintFlag "-s, --shell" "shell name"
    my_PrintFlag "-u, --user" "user name"
    my_PrintFlag "-d, --detach" "run container in background and print container ID"
    my_PrintFlag "-t, --test" "print the executed command, but don't actually execute it"
}
function sub_command
{
    local args=`getopt -o hn:s:u:dt --long help,name:,shell:,user:,detach,test -n "$Command" -- "$@"`
    eval set -- "${args}"
    TEST=0
    local name=""
    local shell=""
    local user=""
    local detach=0
    while true
    do
        case "$1" in
            -h|--help)
                sub_help
                return $?
            ;;
            -t|--test)
                shift
                TEST=1
            ;;
            -n|--name)
                name="$2"
                shift 2
            ;; 
            -s|--shell)
                shell="$2"
                shift 2
            ;; 
            -u|--user)
                user="$2"
                shift 2
            ;; 
            -d|--detach)                
                shift
                detach=1
            ;;
            --)
                shift
                break
            ;;
            *)
                echo Error: unknown flag "'$1'" for "$Command"
                echo "Run '$Command --help' for usage."
                return 1
            ;;
        esac
    done
    local tag="$DefaultTag"
    if [[ "$1" != "" ]];then
        tag="$1"
    fi

    my_LoadENV "$tag"
    if [[ "$name" == "" ]];then
        name="$DockerVarName"
    fi
    if [[ "$shell" == "" ]];then
        shell="$DockerVarShell"
    fi
    if [[ "$user" == "" ]];then
        user="$DockerVarUser"
    fi

    echo 'sudo docker run \'
    if [[ $detach == 0 ]];then
       echo '  -it \'
    fi
    echo "  --rm --name '$name' \\"
    if [[ $detach == 0 ]];then
        if [[ "$user" != root ]];then
            echo "  -u '$user' \\"
        fi
        echo "  $Docker:$tag '$shell'"
    else
        echo "  $Docker:$tag -d"
    fi


    if [[ $TEST == 0 ]];then
        if [[ $detach == 0 ]];then
            sudo docker run -it --rm \
                -u "$user" \
                --name "$name" \
                -e TZ=Asia/Shanghai \
                -e LANG=C.UTF-8 \
                -e LC_ALL=C.UTF-8 \
                $Docker:$tag "$shell"
        else
            docker_run "$Docker:$tag" "$name"
        fi
    fi
}