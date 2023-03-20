function sub_help
{
    echo "docker push for $Docker"
    echo
    echo "Example:"
    echo "  # push default for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # push dir"
    echo "  $Command $dockerfile"
    echo
    echo "  # push all dir"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-a, --all" "push all ( $dockerfile)"
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
            -a|--all)
                shift
                all=1
            ;;
            -t|--test)
                shift
                TEST=1
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
    local dir
    if [[ $all == 1 ]];then
        for dir in ${dockerfile[@]}
        do
            my_DockerPush "$dir"
        done
    elif [[ ${#@} == 0 ]];then
        my_DockerPush "$DefaultTag"
    else
        for dir in "$@"
        do
            my_DockerPush "$dir"
        done
    fi
}