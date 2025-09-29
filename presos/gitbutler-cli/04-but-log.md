# but log

## The log command is not a graph walker, but a smartlog

I find `git log` sort of useless.

What is almost always more interesting is `git log origin/main..`

I honestly find _pretty annoying_ that users need to do complicated refspec work for it to do essentially anything useful.


```commands
but log
git log --oneline --graph origin/main~..
```

