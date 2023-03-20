function sub_help
{
    echo "docker images for $Docker"
    echo
    echo "Example:"
    echo "  # print default images for $DefaultTag"
    echo "  $Command"
    echo
    echo "  # print images by tag"
    echo "  $Command $dockerfile"
    echo
    echo "  # print all images"
    echo "  $Command -a"
    echo 
    echo "Usage:"
    echo "  $Command [flags]"
    echo
    echo "Flags:"
    my_PrintFlag "-h, --help" "help for $Command"
    my_PrintFlag "-a, --all" "print all images ( $dockerfile)"
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
        my_DockerImages
    elif [[ ${#@} == 0 ]];then
        my_DockerImages "$DefaultTag"
    elif [[ ${#@} == 1 ]];then
        my_DockerImages "$1"
    else
        for arg in "$@"
        do
            if [[ "$val" == "" ]];then
                val="($arg)"
            else
                val="$val|($arg)"
            fi
        done
        my_DockerImages "$val"
    fi
}