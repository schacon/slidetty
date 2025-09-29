# but branch

## Create new branches, stacked or in parallel

- `but branch new new-branch` -> parallel 

Unlike git, we can have multiple parallel branches, so you can create a new one that is _also_ active and start committing stuff to it.

- `but branch new <new> <t>` -> stacked

Also, stacked branches are a main feature.

```commands
but branch new parallel-branch
but branch new -a sc-branch-26 stacked-branch
```
