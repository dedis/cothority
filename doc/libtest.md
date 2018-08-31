# libtest.sh

For integration support to test the CLI-apps, `libtest.sh` supports a range
of tests to make sure that the binary behaves as expected.

This is a specialized bash-library to handle the following parts of the test:

* compiling of a conode with the required services
* setting up and removing the directory structure to run the tests
* different test primitives to test failure or success of running the binary

One goal of `libtest.sh` is to test the binary in a way as close to what a user
would do when running the binary. This leads to a better understanding whether
the binary has a logical setup of the commands and command line options.

## Command Line Options

The default for `libtest.sh`-enabled test is to:

* compile the binary
* run all the tests in a temporary directory
* delete the temporary directory after the test.

Two command line options are available to infulence how the test is run:

* `-nt` - NoTemp - sets up the test-environment in `./build` and doesn't delete
it after the test is done. This is useful if you want to inspect the environment
after the test run. *CAREFUL* because it will *NOT* recompile the binary. You
will use this flag mostly while developing the `test.sh` file, so that you can
quickly test if your new tests are correct or not.
* `-b` - Build - only useful with `-nt` to force a build of the binary. While
developing the tests, in a first time you might want to use `-nt` until your
`test.sh` file is OK, then you will work on your go-code. To recompile the go-code,
use this flag. Once the go-code is OK, you can remove this flag again and work
on the `test.sh` file.

## test.sh setup

The common name for using `libtest.sh` is a file called `test.sh` in the
same directory as the CLI-binary. The usual structure for this file is:

* Global variables and inclusion of `libtest.sh`
* main-method setting up the environment and starting the tests
* a list of test-methods
* helper methods
* a call to main, so that the main method is at the top and all necessary
methods are defined before main is called.

### Global Variables and inclusion of `libtest.sh`

| Variable | Default | Explanation |
| -------- | ------- | ----------- |
| `DBG_TEST` | `0` | Show the output of the commands: 0=none, 1=test-names, 2=all |
| `DBG_SRV` | `0` | Debug level for server, passed as `-debug` to the `conode` command |
| `NBR` | `3` | Highest number of servers and clients |
| `NBR_SERVERS` | `NBR` | How many servers to configure |
| `NBR_SERVERS_GROUP` | `NBR_SERVERS - 1` | How many servers to write to the group-file - per default, keep one server out of the group, to test what happens if a server gets added later |
| `APPDIR` | `pwd` | where the `test.sh`-script is located |
| `APP` | `$(basename $APPDIR)` | The name of the builddir |

All variables can be overriden by defining them _before_ the inclusion of
`libtest.sh`. The most common setup of a `test.sh` only has the first two
global variables, `DBG_TEST` and `DBG_SRV`:

```bash
# Show the output of the commands: 0=none, 1=test-names, 2=all
DBG_TEST=1
# DBG-level for call to the app - this is not handled by libtest.sh, but will
# be used in the helper methods.
DBG_APP=2
# DBG-level for server
DBG_SRV=0

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh
```

If a test fails, it is common to change those variables to:

```bash
DBG_TEST=2
DBG_SRV=2
```

So that `libtest.sh` shows the output of the CLI command and the `conode` prints
at a debug-level of 2.

### `main`

To have the most important method at the top (for easier editing), `main`
is defined at the beginning. So any edits to tests can be done there.

A typical main looks like this:

```bash
main(){
    startTest
    buildConode
    run testBuild
    tun testNetwork
    stopTest
}
```

* `startTest` cleans the test-directory and builds the CLI binary
* `buildConode` creates a new conode and automatically includes the
service defined in the `./service` or `../service` directory
* `run testBuild` makes sure all conodes are stopped and removes all databases
from the previous test, then calls `testBuild`
* `run Network` makes sure all conodes are stopped and removes all databases
from the previous test, then calls `testNetwork`
* `stopTest` stops all remaining conodes and cleans up the test-directory, if it
is the temporary directory

One common use-case of the `main` method in the case of a failing test is to
comment out all tests that run successfully up to the failing test, so that
a subsequent `./test.sh` run only runs the failing test. Calling `./test.sh -nt`
allows for changing the failing `testName` method to find out what is wrong.

### Test-methods

Each test-method usually starts the required conodes before running the tests
necessary to verify the correct running of the service.

```bash
testNetwork(){
    runCoBG 1 2
    testOut "Running network"
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl -g public.toml
}
```

* `runCoBG 1 2` starts conode number `1` and `2` (the index starts at 1) in
the background. There is some code to make sure the conodes are actually up
and running and listening on the websocket-ports.
* `testOut` prints the first argument if `DBG_TEST` is greater or equal to `1`
* `testGrep` searches the first argument in the output of the given command

The whole list of all `test`-commands can be found [here](#test-commands).

### Helper methods

Each `test.sh` will have its own helper methods, but the most common one is
to write something to run your CLI-app:

```bash
runCl(){
    dbgRun ./$APP -d $DBG_APP $@
}
```

This will call your app with the given debugging-option, referenced from the top
of your `test.sh` for easy change in case you need to debug your test.

Sometimes you might want to give more option, most often the configuration-directory
to be used:

```bash
runCl(){
	local CFG=cl$1
	shift
	dbgRun ./$APP -d $DBG_APP -c $CFG $@
}
```

This passes the configuration-directory to the app, supposing the app has a
`-c` argument for passing it.

### A call to main

This is very simple:

```bash
main
```

As bash is an interpreter, it needs to run through all your methods before
being able to call them. And because it is nice to have all the important
methods at the top, we propose this setup.

## Test-commands

Here is an overview of the currently supported test-commands in `libtest.sh`:

| Command | Arguments | Description |
| ------- | --------- | ----------- |
| `run` | `Name` | Prints `Name`, cleans up build-directory, deletes all databases from services in previous run, and calls the function named `Name`. |
| `testOut` | `$@` | Outputs all arguments if `DBT_TEST -ge 1` |
| `dbgOut` | `$@` | Outputs all arguments if `DBT_TEST -ge 2` |
| `dbgRun` | `$@` | Runs `$@` and outputs the result of `$@` if `DBG_TEST -ge 2`. Redirects the output in all cases if `OUTFILE` is set. |
| `testOK` | `$@` | Asserts that the exit-code of  running `$@` using `dbgRun` is `0`. |
| `testFail` | `$@` | Asserts that the exit-code of  running `$@` using `dbgRun` is NOT `0`. |
| `testFile` | `File` | Asserts `File` exists and is a file. |
| `testNFile` | `File` | Asserts that `File` DOES NOT exist. |
| `testFileGrep` | `String` `File` | Asserts that `String` exists in `File`. |
| `testGrep` | `String` `$@[1..]` | Asserts that `String` is in the output of the command being run by `dbgRun` and all but the first input argument. Ignores the exit-code of the command. |
| `testNGrep` | `String` `$@[1..]` | Asserts that `String` is NOT in the output of the command being run by `dbgRun` and all but the first input argument. Ignores the exit-code of the command. |
| `testReGrep` | `String` | Asserts `String` is part of the last command being run by `testGrep` or `testNGrep`. |
| `testReNGrep` | `String` | Asserts `String` is NOT part of the last command being run by `testGrep` or `testNGrep`. |
| `testCount` | `Count` `String` `$@[2..]` | Asserts that `String` exists exactly `Count` times in the output of the command being run by `dbgRun` and all but the first two arguments. |
