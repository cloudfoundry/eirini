rm $GOPATH/bin/protoc-gen-gogoslick
go install -v github.com/gogo/protobuf/protoc-gen-gogoslick
echo "Generating pb.go files"
protoc --proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf/:$GOPATH/src/github.com/golang/protobuf/ptypes/duration/:. --gogoslick_out=plugins=grpc:. *.proto
