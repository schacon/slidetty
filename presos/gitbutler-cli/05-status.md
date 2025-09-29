# but status

## Show the state of your active branches and uncommitted work

Use `but status` to show:

- active branches
- basic commit info
- uncommitted work

`but status` is not just modified files, it is also a sort of smartlog

like `git log origin/main..` for several active branches

```commands
git log --oneline --graph origin/main~..
but status --files
but restore 02d110ecaa15
```
