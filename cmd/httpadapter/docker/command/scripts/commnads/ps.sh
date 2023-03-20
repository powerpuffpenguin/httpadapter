function sub_help
{
    echo "docker ps for $Docker"
    echo
    echo "Example:"
    echo "  # ps default for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # ps by tag"
    echo "  $Command $dockerfile"
    echo
    echo "  # ps all"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-a, --all" "ps all ( $dockerfile)"
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
    
    if [[ $all == 1 ]];then
        my_DockerPS
    elif [[ ${#@} == 0 ]];then
        my_DockerPS "$DefaultTag"
    elif [[ ${#@} == 1 ]];then
        my_DockerPS "$1"
    else
        for arg in "$@"
        do
            if [[ "$val" == "" ]];then
                val="($arg)"
            else
                val="$val|($arg)"
            fi
        done
        my_DockerPS "$val"
    fi
}