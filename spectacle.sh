#!/bin/sh
go get -u golang.org/x/vgo &> goget.log
$GOPATH/bin/vgo build -o bin/valkyr &> build.log

pkill valkyr
cp bin/valkyr $HOME/services/
cd $HOME/services
nohup ./valkyr &> valkyr.log &
