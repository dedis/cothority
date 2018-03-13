# Coding

Scripts for coding and best practices for the DeDiS-workgroup.

## GitHub usage

On top of github, we rely on the [ZenHub](http://ZenHub.io) for project
management. We use its board to put each task into a 'pipeline':

- Ready4Merge: pull-requests we think are ready to be merged. Please
have a regular look into this pipeline and comment. If two engineers
other than the owner of the pull-request-owner agree to merge, it
should be done.
- WIP: Work-In-Progress - what people are currently working on. It is
OK to move things around between WIP and the following pipeline
- TODO: Open issues that should be treated next.
- BUG: Issues that make the project behave in a not-wanted way.
- NeedDetails: Issues that are not detailed enough to work on them
and need more explanation.

### Branches

All work has to be done in branches. Per default, branches go off from
`master`, which should always be in a functional state.

The branch-name should be one to three words, concatenated using underscores,
followed by the number of the issue it solves.
If you work on a foreign branch, include the start of the foreign branch in
your branch.

Let's say user 1 made a branch `add_platform_lxc_345` and you want to
participate, you'll create a branch that goes off it with the name
`add_platform_lxc_macosxfix_345` and do a pull request to the first branch,
`add_platform_lxc_345`.

There are two scripts, [`gic`](#gid) and [`gid`](#gid), in the `bin`
directory which make it a lot easier to handle a lot of long branch names
and can save you a lot of typing.

### Pull Requests and Issues

We now follow the github practice of having separate issues and pull
requests. Ideally this allows to have general discussions in the
issues and more implementation-specific discussions in the pull request.
If a pull request is deleted, the general discussion is still available.

Using Zenhub, you can connect pull-requests and issues, so they show
up as being together in the zenhub-board.

### Assignees

An issue/pull-request with an assignee belongs to this person - he is
 responsible for it. Specially for a pull-request, this means:

- only the assignee may add commits to this pull-request
- only somebody else than the assignee may merge the pull-request

If somebody else wants to participate on a given pull-request, he can make a
 new branch off from this pull-request and continue the work therein:

```
PR1 with assignee1
+- PR2 with assignee2
```

Now the assignee1 has to merge the PR2 into his PR1, but only somebody else
 than assignee1 can merge the PR1 back into the development-branch.

### Commits and push

The general rule is that for each commit, all tests should pass. This is not
  a hard rule, but it should be used whenever possible.

### Merge to master

Before merging into master, all tests MUST pass. This can be enforced by
using the ```.git/hooks/pre-push``` hook provided in this repository.
Then you have to pass code-review by one or two other developers, which will
comment your code and ask for changes. Only once at least one other
developer is happy with your branch can he merge it.

It is good practice to do a `squash` when merging and adding all important
information about the merge in the squash-comment.

### Travis

A travis-script checks the go-formatting and all tests. Before a merge is done,
Travis must be OK.

### Coveralls

In every PR the code coverage shall not decrease (+/-0.5% is OK though).
We aim for ~100% and have 80% as lower boundary. Code containing only `func main`
and not much more is OK if it is tested by integration tests and manually instead
of unit tests (for these few packages may have lower code coverage).

### Go-imports and git pre-push

If you have troubles using goimports in your editor, please use the pre-push hook
in this directory for git. If you alread installed the 'bin'-directory with
`add_path_to_mac`, you can just call `add_hooks`. Now everytime before your
changes get pushed, `goimport` renices all your files.

## Comments

Two important links regarding comments:
- [Godoc: documenting Go code](http://blog.golang.org/godoc-documenting-go-code)
- [Effective Go](https://golang.org/doc/effective_go.html)

Some important notes about what to comment:

- every function should be commented
- every package needs a comment in the `packagename.go`-file (arbitrarily
 set by myself)

Commenting-language is English, if you're not sure, don't hesitate to take
some time off in Google or wherever to increase your knowledge of English!

Please turn your auto-correction on and fix words that are marked as wrong,
except function- and variable-names that aren't English words.

## Line-width

The standard line-width is 80 characters and this is a hard limit.

## Scripts

Two scripts are provided for more easy switching and cleaning up between
branches.

### gic

The bash script [`gic`](bin/gic) stands for git-checkout. If you call it
without any arguments, it will output a list of all branches that are checked
out, together with a number in front. This lets you easily change between
branches.

New branches that have never been checked out will come in the second part of
the list and can be checked out the same way.

### gid

The script [`gid`](bin/gic) will delete a local branch to make place for new
branches. Called without arguments, it will show a list of all branches
available for deletion, called with a number, it will try to delete that
branch. This can fail if the branch hasn't been pushed.

## Debug-levels

We're using the `cothority/lib/dbg`-library for debug-output which offers a
numerical debug-level. The debug-levels represent:

  * 1 - important information to follow the correct working of a simulation
  * 2 - additional information which doesn't spam the screen when running with
     more than 20 hosts
  * 3 - debugging information for following the code-path, only useful for up to
     20 hosts
  * 4 - information for verbose output in testing
  * 5 - not really used

### Evolution of debug-levels

While writing fresh code, the new functions will have lower debug-levels, as they
will most probably influence a lot of what is being coded and where bugs reside.
As the functions mature, the debug-levels can be increased, as most often they
don't indicate anything interesting anymore.

### Debugging with LLvl and Print

If a given output is interesting for debugging regardless of the level, the
`dbg.Lvl` can be changed to `dbg.LLvl` which will always print the information.

This is useful if you are debugging something and want to follow a certain path
that has only high debug-levels.

For fast dumping of variables one can also use `log.Print` which is easy to find
and remove once the debugging-session is done.

### Format-functions in debug

Every debug-function has also a -*f*-function: `Lvl1` and `Lvlf1`, `Lvl2` and
`Lvlf2`..., `Print` and `Printf`, `Fatal` and `Fatalf`, `Error` and `Errorf`.

The format-functions work like `fmt.Printf`.
