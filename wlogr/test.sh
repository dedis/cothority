#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
DBG_SRV=2

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
    buildConode "github.com/dedis/logread/service"
	test Build
	test Create
	test RoleCreate
	test ManageJoin
	test Write
	test Read
    stopTest
}

testRead(){
	setupWlr
	runGrepSed "Stored file" "s/.* //" runCl 1 write bar2 public.toml
	FILE=$SED
	testOK runCl 2 manage join public.toml $SID $READER
	testFail runCl 2 read request bar3 $READER
	runGrepSed "Request-id" "s/.* //" runCl 2 read request bar3 $FILE
	READREQ=$SED
	tmp=$( mktemp )
	testOK runCl 2 read fetch $READREQ $tmp
	testOK [ $( md5 -q public.toml ) = $( md5 -q $tmp ) ]
	rm $tmp
}

testWrite(){
	setupWlr
	testFail runCl 1 write admin public.toml
	testOK runCl 1 write bar2 public.toml
	testOK runCl 2 manage join public.toml $SID $WRITER
	testOK runCl 2 write bar2 public.toml
}

testManageJoin(){
	setupWlr
	testFail runCl 2 manage join public.toml $SID a7d0124049e3829893cb2b264f93961bb7491032e0dfb56154777a0ca55a5400
	testGrep "Found admin" runCl 2 manage join public.toml $SID $ADMIN
	testFail runCl 2 manage join public.toml $SID $WRITER
	testGrep "Found writer" runCl 2 manage join -overwrite public.toml $SID $WRITER
	testGrep "Found reader" runCl 2 manage join -overwrite public.toml $SID $READER
}

setupWlr(){
	runCoBG 1 2
	runGrepSed skipchainid "s/.* //" runCl 1 manage create public.toml foo
	SID=$SED
	runGrepSed Private "s/.* //" runCl 1 manage role create admin:bar1
	ADMIN=$SED
	runGrepSed Private "s/.* //" runCl 1 manage role create writer:bar2
	WRITER=$SED
	runGrepSed Private "s/.* //" runCl 1 manage role create reader:bar3
	READER=$SED
}

testRoleCreate(){
	runCoBG 1 2
	runCl 1 manage create public.toml foo
	testFail runCl 1 manage role create admin:foo
	testFail runCl 1 manage role create unknown:bar
	testOK runCl 1 manage role create admin:bar
	testOK runCl 1 manage role create writer:foo2
	testOK runCl 1 manage role create reader:foo3
	testGrep foo runCl 1 manage role list
	testGrep bar runCl 1 manage role list
	testGrep foo2 runCl 1 manage role list
	testGrep foo3 runCl 1 manage role list
}

testCreate(){
       runCoBG 1 2
       testFail runCl 1 list
       testOK runCl 1 manage create public.toml foo
       testOK runCl 1 list
}

testBuild(){
    testOK dbgRun runCl 1 --help
}

runCl(){
    local D=cl$1
    shift
    dbgRun ./$APP -d $DBG_APP -c $D $@
}

main
