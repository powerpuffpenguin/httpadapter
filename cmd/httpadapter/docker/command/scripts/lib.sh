# * $1 flag
# * $2 message
function my_PrintFlag
{
    printf "  %-20s %s\n" "$1" "$2"
}

# * $1 dirname
function my_CheckDockerfile
{
    local file
    for file in ${dockerfile[@]}
    do
        if [[ "$1" == "$file" ]];then
            return 0
        fi
    done
    echo "not found version $1"
    return 1
}
TEST=0
# * $1 dirname
function my_LoadENV {
    TAG="$1"
    
    # docker run name
    DockerVarName=""
    # docker run shell
    DockerVarShell="bash"
    # docker exec user
    DockerVarUser="root"
    if [[ -f "$Dockerfile/$1/env.sh" ]];then
        source "$Dockerfile/$1/env.sh"
    fi

    if [[ "$DockerVarName" == "" ]];then
        DockerVarName="test-$Docker"
    fi
}
function before_build
{
    _s="before_build"
}
function after_build {
    _s="after_build"
}

# * $1 dirname
function my_DockerBuild
{
    my_CheckDockerfile "$1"

    echo build "$1"
    my_LoadENV "$1"

    cd "$Dockerfile/$1"
    before_build
    echo  ' ! sudo docker build \'
    echo '      --network host \'
    echo "      -t '$Docker:$TAG' \\"
    echo "      '$Dockerfile/$1' \\"

    if [[ $TEST == 0 ]];then
        sudo docker build \
            --network host \
            -t "$Docker:$TAG" \
            .
    fi
    after_build
}

function my_DockerImages
{
    if [[ "$1" == "" ]];then
        echo "sudo docker images | grep -w \"$Docker\""
        if [[ $TEST == 0 ]];then
            sudo docker images | grep -w "$Docker "
        fi
    else
        echo "sudo docker images | grep -w \"$Docker \" | egrep -w \"$1\""
        if [[ $TEST == 0 ]];then
            echo 'REPOSITORY                                       TAG             IMAGE ID       CREATED         SIZE'
           sudo docker images | grep -w "$Docker " | egrep -w "$1"
        fi
    fi
}
function my_DockerPS
{
    if [[ "$1" == "" ]];then
        echo "sudo docker ps -a | grep -w \"$Docker\""
        if [[ $TEST == 0 ]];then
            sudo docker ps -a | grep -w "$Docker "
        fi
    else
        echo "sudo docker ps -a | grep -w \"$Docker \" | egrep -w \"$1\""
        if [[ $TEST == 0 ]];then
            echo 'CONTAINER ID   IMAGE                   COMMAND                  CREATED         STATUS        PORTS'
            sudo docker ps -a | grep -w "$Docker " | egrep -w "$1"
        fi
    fi
}
# * $1 images
# * $2 name
function docker_run
{
    sudo docker run -it --rm \
        --name "$2" \
        -e TZ=Asia/Shanghai \
        -e LANG=C.UTF-8 \
        -e LC_ALL=C.UTF-8 \
        "$1"
}

function my_DockerLogs
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker logs \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker logs "$DockerVarName"
    fi
}
function my_DockerLogs
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker logs \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker logs "$DockerVarName"
    fi
}
function my_DockerStart
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker start \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker start "$DockerVarName"
    fi
}
function my_DockerStop
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker stop \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker stop "$DockerVarName"
    fi
}
function my_DockerRestart
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker restart \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker restart "$DockerVarName"
    fi
}
function my_DockerInspect
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    echo "sudo docker inspect \"$DockerVarName\""
    if [[ $TEST == 0 ]];then
        sudo docker inspect "$DockerVarName"
    fi
}
function my_DockerRM
{
    my_CheckDockerfile "$1"

    my_LoadENV "$1"

    if [[ $2 == 1 ]];then
        echo "sudo docker rm -f \"$DockerVarName\""
        if [[ $TEST == 0 ]];then
            sudo docker rm -f "$DockerVarName"
        fi
    else
        echo "sudo docker rm \"$DockerVarName\""
        if [[ $TEST == 0 ]];then
            sudo docker rm "$DockerVarName"
        fi
    fi
}

# * $1 dirname
function my_DockerPush
{
    my_CheckDockerfile "$1"
    my_LoadENV "$1"

    echo "sudo docker push \"$Docker:$TAG\""

    if [[ $TEST == 0 ]];then
        sudo docker push "$Docker:$TAG"
    fi
}
# * $1 dirname
function my_DockerPull
{
    my_CheckDockerfile "$1"
    my_LoadENV "$1"

    echo "sudo docker pull \"$Docker:$TAG\""

    if [[ $TEST == 0 ]];then
        sudo docker pull "$Docker:$TAG"
    fi
}