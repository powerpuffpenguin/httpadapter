function sub_help
{
    echo "docker logs for $Docker"
    echo
    echo "Example:"
    echo "  # print default logs for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # print logs by dir"
    echo "  $Command $dockerfile"
    echo
    echo "  # print all logs"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-a, --all" "print all logs ( $dockerfile)"
    my_PrintFlag "-t, --test" "print the executed command, but don't actually execute it"
}
function sub_command
{
    local args=`getopt -o hat --long help,all,test -n "$Command" -- "$@"`
    eval set -- "${args}"
    local all=0
    TEST=0
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
            -a|--all)
                shift
                all=1
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
    if [[ $all == 1 ]];then
        for arg in ${dockerfile[@]}; do
            my_DockerLogs "$arg"
        done
    elif [[ ${#@} == 0 ]];then
        my_DockerLogs "$DefaultTag"
    elif [[ ${#@} == 1 ]];then
        my_DockerLogs "$1"
    else
        for arg in "$@"; do
            my_DockerLogs "$arg"
        done
    fi
}