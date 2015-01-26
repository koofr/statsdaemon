#!/bin/bash -e

REV=${1:-master}

pushd `dirname $BASH_SOURCE`
WDIR=`pwd`

if [[ `git status --porcelain | wc -l` > 0 ]]; then
  echo "Uncommited files, commit them first!"
  exit 1
fi

git checkout $REV

NAME=`basename $WDIR`-$REV
BUILDIR=`mktemp -d`/$NAME
mkdir -p $BUILDIR

go test
go build -o $BUILDIR/statsdaemon
cp statsdaemon.ini $BUILDIR/statsdaemon.example.ini

pushd $BUILDIR/..
tar cvzf $WDIR/${NAME}.tar.gz $NAME/
popd
popd

