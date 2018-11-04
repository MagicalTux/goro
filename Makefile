#!/bin/make

all:
	goimports -w -l .
	go build -v
