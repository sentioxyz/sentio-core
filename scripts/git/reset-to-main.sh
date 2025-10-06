COMMIT=$(git rev-parse HEAD)
git checkout main && git reset --hard $COMMIT
