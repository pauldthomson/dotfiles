#!/bin/bash

PROJECT=$1

tmux new -d -s $PROJECT

DIRECTORY=`find ~/code -maxdepth 2 -type d -name $PROJECT`
echo $DIRECTORY
NUMBER=1
if [ "${#DIRECTORY[@]}" == 0 ]; then
		printf "Project not found, you may need to clone it" 
elif [ "${#DIRECTORY[@]}" > 1 ]; then
		printf "Which directory do you want?\n"
		for dir in "${DIRECTORY[@]}"
		do
				echo "$NUMBER. $dir"
				(( NUMBER++ ))
				echo $NUMBER
		done
else
		tmux send-keys -t $PROJECT "cd $DIRECTORY" ENTER
fi
