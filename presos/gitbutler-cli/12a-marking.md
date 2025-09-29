# Marking

## Enable Jujutsu style workflows by marking commits

The way that JJ works is to always have new work be absorbed into the current changeset.

We can emulate that with "rules". Our first rule is "marking", which marks a commit for auto-absorbsion.

But we can even mark commits deeper in a branch or even stacked branches.

```commands
but mark
but unmark
but restore 02d110ecaa15
```
