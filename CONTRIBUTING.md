# Coding

Scripts for coding and best practices for the DeDiS-workgroup.

## GitHub usage

We use a [GitHub project](https://github.com/orgs/dedis/projects/1) for project
management. We use its board to put each task into a 'pipeline':

- Ready4Merge: pull-requests we think are ready to be merged. Please
have a regular look into this pipeline and comment. If two engineers
other than the owner of the pull-request-owner agree to merge, it
should be done.
- WIP: Work-In-Progress - what people are currently working on. It is
OK to move things around between WIP and the following pipeline
- TODO: Open issues that should be treated next.
- BUG: Issues that make the project behave in a not-wanted way.

### Branches

All work has to be done in branches. Per default, branches go off from
`master`, which should always be in a functional state.

The branch-name should be one to three words, concatenated using underscores,
followed by the number of the issue it solves.

### Pull Requests and Issues

We now follow the github practice of having separate issues and pull
requests. Ideally this allows to have general discussions in the
issues and more implementation-specific discussions in the pull request.
If a pull request is deleted, the general discussion is still available.

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

Before merging into master, all tests MUST pass.
Then you have to pass code-review by one or two other developers, which will
comment your code and ask for changes. Only once at least one other
developer is happy with your branch can he merge it.

### Travis

A travis-script checks the go-formatting and all tests. Before a merge is done,
Travis must be OK.

### Coveralls

In every PR the code coverage shall not decrease (+/-0.5% is OK though).
We aim for ~100% and have 80% as lower boundary. Code containing only `func main`
and not much more is OK if it is tested by integration tests and manually instead
of unit tests (for these few packages may have lower code coverage).

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

## Pre-commit

As of August 2020, we use the [pre-commit](https://pre-commit.com/) framework
 to make sure the code is up-to-date with regard to formatting and linting.
After installation, you can launch this locally with

```bash
pre-commit
```

This will make sure that all changed files pass our linting- and formatting
 preferences.

# Licenses

Cothority is an Open Source program with many contributors (listed in the AUTHORS
files), so we actively seek contributions (help on the Cothority users list,
documentation, source code, ideas, …).  All contributions help us make a
better product.

All contributions without an explicit copyright statement or a CLAI/CLAC (see below)
are assumed to be covered under a AGPL 2-Clause license as described in the
file LICENSE.

Developers who have contributed significant changes to the Cothority code must sign
a Contributor License Agreement (CLAI), eventually also a CLAC if they work for
a corporation. Together they guarantee them the right to use the code they have
developed, and also ensures that EPFL/DEDIS (and thus the Cothority project) has the
rights to the code.
By signing the CLAI/CLAC, your contributions are eligible to be integrated into the
Cothority source code.
Having the CLAI/CLAC signed is essential for the Cothority project to maintain a
clean copyright and guarantees that Cothority and your source code will always remain
Free Software (Open Source).
Providing that your contribution is accepted by the Cothority project, your signed
CLAI/CLAC also permits EPFL/DEDIS to submit your contribution for use in other
EPFL/DEDIS-projects.

The Contributor License Agreements are in the [CLAI] and [CLAC]-file.
