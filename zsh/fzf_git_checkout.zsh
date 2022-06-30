gch() {
    git checkout --track $(git branch -r | fzf)
}
