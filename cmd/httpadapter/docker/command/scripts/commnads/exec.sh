function sub_help
{
    echo "docker exec for $Docker"
    echo
    echo "Example:"
    echo "  # docker exec shell for $Docker:$DefaultTag"
    echo "  $Command $DefaultTag"
    echo 
    echo "  # docker exec shell for $Docker:$DefaultTag"
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
    my_PrintFlag "-t, --test" "print the executed command, but don't actually execute it"
}
function sub_command
{
    local args=`getopt -o hs:n:u:t --long help,shell:,name:,user:,test -n "$Command" -- "$@"`
    eval set -- "${args}"
    TEST=0
    local name=""
    local shell=""
    local user=""
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

    echo "sudo docker exec -u \"$user\" -it \"$name\" \"$shell\""

    if [[ $TEST == 0 ]];then
        sudo docker exec -u "$user" -it "$name" "$shell"
    fi
}