.PPHONY: 	proto
proto: 	proto/bevm.pb.go

proto/bevm.pb.go: 	bevm.proto
	mkdir -p ./proto
	protoc --go_out=./proto/ --go_opt=paths=source_relative $<
