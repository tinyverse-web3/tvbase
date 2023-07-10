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
    sysctl -w net.core.rmem_max=2500000
else
    echo "unknow system"
    exit -1
fi

# ./tvnode
echo "Start tvnode..."

user_dir="~"
user_dir=$(eval echo "$user_dir")
pid_file="$user_dir/.tvnode/tvnode.pid"

isKillOldPid=0
if [ -f "$pid_file" ]; then
    pid=$(cat "$pid_file")
    if [ -z "$pid" ]; then
        echo "PID is empty"
    else
        echo "$pid_file is exist, killing process with PID: $pid"
        kill -9 "$pid"
        kill_result=$?
        if [ $kill_result -eq 0 ]; then
            echo "Process killed successfully"
            isKillOldPid=1
        else
            echo "Failed to kill process for tvnode (pid:$pid, error code: $kill_result)"
        fi
    fi
fi

if [ "$isKillOldPid" -eq 0 ]; then
    echo "Try to killall for tvnode"
    killall tvnode
    kill_result=$?
    if [ $kill_result -eq 0 ]; then
        echo "Process killall successfully"
    else
        echo "Failed to killall for tvnode (error code: $kill_result)"
    fi
fi


log_dir="$user_dir/.tvnode"
log_prefix="tvnode"
log_filename="$log_dir/$(date +"%Y-%m-%d_%H-%M-%S")_$log_prefix.log"
nohup tvnode > "$log_filename" 2>&1 &
pid=$!
if [ $? -eq 0 ]; then
    echo "tvnode is started, write to $pid_file, pid: $pid."
    echo "$pid" > $pid_file
else
    cat $log_filename
    echo "tvnode execution failed. check $log_filename for details."
fi
