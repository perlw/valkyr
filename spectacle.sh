#!/bin/sh
APP=valkyr
APPBASE=$HOME/services/$APP

go get -u golang.org/x/vgo &> goget.log
$GOPATH/bin/vgo build -o bin/$APP &> build.log

pkill $APP
rm -rf $HOME/services/$APP
rm -rf $APPBASE
mkdir -p $APPBASE

cp bin/$APP $APPBASE
cp $APP.ini $APPBASE

cd $APPBASE
nohup ./$APP &> $APP.log &
