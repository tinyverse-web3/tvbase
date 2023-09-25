#!/bin/sh
# See https://github.com/quic-go/quic-go/wiki/UDP-Receive-Buffer-Size for details 
if [ "$(uname)" = "Darwin" ]; then
    echo "BSD system"
    sysctl -w kern.ipc.maxsockbuf=3014656
    #Add the following lines to the /etc/sysctl.conf file to keep this setting across reboots:
    #kern.ipc.maxsockbuf=3014656
elif [ "$(uname)" = "Linux" ]; then
    echo "Linux sysytem"
    sysctl -w net.core.rmem_max=2500000
    sysctl -w net.core.wmem_max=2500000
else
    echo "unknow system"
    exit -1
fi


user_dir="~"
user_dir=$(eval echo "$user_dir")

conf_path="$user_dir/.tvnode"
if [ ! -d "$conf_path" ]; then
    mkdir "$conf_path"
    echo "tvnode Folder created successfully, it is $conf_path"
fi

pid_file="$user_dir/.tvnode/tvnode.pid"

isKillOldPid=0
if [ -f "$pid_file" ]; then
    pid=$(cat "$pid_file")
    if [ -z "$pid" ]; then
        echo "pid is empty"
    else
        echo "$pid_file is exist, start terminite tvnode process($pid)"
        kill_output=$(kill -9 "$pid" 2>&1)
        kill_result=$?
        if [ $kill_result -eq 0 ]; then
            echo "success to terminite tvnode"
            isKillOldPid=1
        elif [ $kill_result -eq 1 ]; then
            echo "tvnode process($pid) isn't exist"
        else 
            echo "failed to terminite tvnode($pid) process (errorCode: $kill_result, output: $kill_output)"
        fi
    fi
fi

# if [ "$isKillOldPid" -eq 0 ]; then
#     echo "Try to killall for tvnode"
#     killall tvnode
#     kill_result=$?
#     if [ $kill_result -eq 0 ]; then
#         echo "process killall successfully"
#     else
#         echo "failed to killall for tvnode (error code: $kill_result)"
#     fi
# fi



root_dir="$user_dir/.tvnode"
log_back_dir="$root_dir/log"
log_filename="$root_dir/$(date +"%Y-%m-%d_%H-%M-%S").log"

if [ ! -d "$log_back_dir" ]; then
    mkdir "$log_back_dir"
fi
mv "$root_dir/"*.log "$log_back_dir"

echo "start tvnode..."
nohup tvnode > "$log_filename" 2>&1 &

if [ $? -eq 0 ]; then
    pid=$!
    echo "tvnode is started, write to $pid_file, pid: $pid, logfile is $log_filename"
    echo "$pid" > $pid_file
else
    echo "fail to exec tvnode."
fi
