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
        -c $DIRECTORY \
        \; \
    send-keys \
        -t $PROJECT \
        'tmux split-window -v -p20' 'C-m' \
    \; \
    send-keys \
        -t $PROJECT \
        'vim' 'C-m'
fi

if [[ "$TMUX" ]]; then
    tmux switch -t $PROJECT
else
    tmux a -t $PROJECT
fi
