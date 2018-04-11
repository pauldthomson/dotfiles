#!/bin/bash

PROJECT=$1

tmux new -d -s $PROJECT

DIRECTORY=`find ~/code -maxdepth 2 -type d -name $PROJECT`
if [ -z $DIRECTORY ]; then
		echo Project not found, you may need to clone it 
else
		tmux send-keys -t $PROJECT "cd $DIRECTORY" ENTER
fi
