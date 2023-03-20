#!/bin/bash

if [[ $MY_DOCKER_TOOLS_BASH_COMPLETION == 1 ]];then
    echo "base-completion already exists"
    return 0
fi

function __my_docker_tools_basic_
{
    local opts="$1 -h --help -t --test $dockerfile"
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
}
function __my_docker_tools_
{
    local dockerfile=`$1 -l 2> /dev/null`
    local cur=${COMP_WORDS[COMP_CWORD]}
    local previous=${COMP_WORDS[COMP_CWORD-1]}

    # 第一個參數
    if [ 1 == $COMP_CWORD ];then
        local opts="build exec images inspect logs ps pull push restart rm run start stop -h --help -l --list"
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    else
        case ${COMP_WORDS[1]} in
            build|images|inspect|logs|ps|pull|push|restart|start|stop)
                __my_docker_tools_basic_ "-a --all"
            ;;
            rm)
                __my_docker_tools_basic_ "-a --all -f --force"
            ;;
            exec)
                __my_docker_tools_basic_ "-n --name -s --shell"
            ;;
            run)
                __my_docker_tools_basic_ "-n --name -s --shell -d --detach"
            ;;
        esac
    fi
}
complete -F __my_docker_tools_ base_docker.sh
complete -F __my_docker_tools_ tools_docker.sh

echo load base-completion success
export MY_DOCKER_TOOLS_BASH_COMPLETION=1