function sub_help
{
    echo "docker inspect for $Docker"
    echo
    echo "Example:"
    echo "  # inspect default for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # inspect by dir"
    echo "  $Command $dockerfile"
    echo
    echo "  # inspect all"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-a, --all" "inspect all ( $dockerfile)"
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
            my_DockerInspect "$arg"
        done
    elif [[ ${#@} == 0 ]];then
        my_DockerInspect "$DefaultTag"
    elif [[ ${#@} == 1 ]];then
        my_DockerInspect "$1"
    else
        for arg in "$@"; do
            my_DockerInspect "$arg"
        done
    fi
}