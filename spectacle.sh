#!/bin/sh
go get -u golang.org/x/vgo &> goget.log
$GOPATH/bin/vgo build -o bin/runestone &> build.log

pkill runestone
cp bin/runestone $HOME/services/
cd $HOME/services
nohup ./runestone &> runestone.log &
