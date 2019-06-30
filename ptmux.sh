#!/bin/bash

PROJECT=$1

DIRECTORY=$(find ~/repos -maxdepth 3 -type d -name $PROJECT)
if [[ -z "$DIRECTORY" ]]; then
    echo Project not found, you may need to clone it 
    tmux new -d -s $PROJECT
else
    tmux new  \
        -d \
        -s $PROJECT \
        -c $DIRECTORY
fi

tmux send-keys -t $PROJECT 'tmux split-window -v -p20' 'C-m'
tmux send-keys -t $PROJECT 'vim' 'C-m'

if [[ "$TMUX" ]]; then
    tmux switch -t $PROJECT
else
    tmux a -t $PROJECT
fi
