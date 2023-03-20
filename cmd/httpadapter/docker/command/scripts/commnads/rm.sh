function sub_help
{
    echo "docker rm for $Docker"
    echo
    echo "Example:"
    echo "  # rm default for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # rm by dir"
    echo "  $Command $dockerfile"
    echo
    echo "  # rm all"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-f, --force" "force rm"
    my_PrintFlag "-a, --all" "rm all ( $dockerfile)"
    my_PrintFlag "-t, --test" "print the executed command, but don't actually execute it"
}
function sub_command
{
    local args=`getopt -o hatf --long help,all,test,force -n "$Command" -- "$@"`
    eval set -- "${args}"
    local all=0
    local force=0
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
            -f|--force)
                shift
                force=1
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
            my_DockerRM "$arg" $force
        done
    elif [[ ${#@} == 0 ]];then
        my_DockerRM "$DefaultTag" $force
    elif [[ ${#@} == 1 ]];then
        my_DockerRM "$1" $force
    else
        for arg in "$@"; do
            my_DockerRM "$arg" $force
        done
    fi
}