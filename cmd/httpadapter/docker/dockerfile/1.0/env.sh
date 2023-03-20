DockerVarName="test-httpadapter-1.0"
DockerVarShell="sh"
function before_build
{
    rm root -rf
    mkdir root/opt/httpadapter -p

    cp "$ProjectDir/../bin/httpadapter" root/opt/httpadapter
    cp "$ProjectDir/../bin/etc" root/opt/httpadapter/ -r
}
