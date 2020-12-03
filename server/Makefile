.PHONY = all protos

all:
	protos mongodb

protos:
	protoc --go_out=plugins=grpc,paths=source_relative:. ./server/protos/blog/blog.proto

mongodb:
	go get go.mongodb.org/mongo-driver