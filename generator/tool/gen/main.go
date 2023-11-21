package gen

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"protoc-gen-fiber/generator/helper"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

// Generator ...
type Generator interface {
	Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse
}

// Main ...
func Main(g Generator) {
	req := readGenRequest()
	//req := readGenRequestTest()
	resp := g.Generate(req)
	//fmt.Println("resp:", resp)
	writeResponse(os.Stdout, resp)
}

// FilesToGenerate ...
func FilesToGenerate(req *plugin.CodeGeneratorRequest) []*descriptor.FileDescriptorProto {
	genFiles := make([]*descriptor.FileDescriptorProto, 0)
Outer:
	for _, name := range req.FileToGenerate {
		for _, f := range req.ProtoFile {
			if f.GetName() == name {
				genFiles = append(genFiles, f)
				continue Outer
			}
		}
		Fail("could not find file named", name)
	}

	return genFiles
}

func readGenRequest() *plugin.CodeGeneratorRequest {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		Error(err, "reading input")
	}

	req := new(plugin.CodeGeneratorRequest)
	if err = proto.Unmarshal(data, req); err != nil {
		Error(err, "parsing input proto")
	}

	if len(req.FileToGenerate) == 0 {
		Fail("no files to generate")
	}

	writeJson(req)
	return req
}

func writeJson(req *plugin.CodeGeneratorRequest) {
	marshal, err := json.Marshal(req)
	file, err := os.OpenFile("./Stdout.json", os.O_RDWR|os.O_CREATE, 0666) //打开文件
	if err != nil {
		Error(err, "OpenFile")
	}
	file.WriteString(string(marshal))
	file.Close()
}

func readGenRequestTest() *plugin.CodeGeneratorRequest {

	file, err := os.OpenFile("d:/www/go/src/golang_demo/Stdout.txt", os.O_RDWR|os.O_CREATE, 0766) //打开文件
	if err != nil {
		Error(err, "OpenFile")
	}

	data, err := io.ReadAll(file)

	if err != nil {
		Error(err, "ReadAll")
	}

	req := new(plugin.CodeGeneratorRequest)
	data = helper.BinaryStringToBytes(string(data))
	if err = proto.Unmarshal(data, req); err != nil {
		Error(err, "parsing input proto")
	}

	if len(req.FileToGenerate) == 0 {
		Fail("no files to generate")
	}

	return req
}

func writeResponse(w io.Writer, resp *plugin.CodeGeneratorResponse) {
	data, err := proto.Marshal(resp)
	if err != nil {
		Error(err, "marshaling response")
	}
	_, err = w.Write(data)
	if err != nil {
		Error(err, "writing response")
	}
}

// Fail log and exit
func Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("error:", s)
	os.Exit(1)
}

// Fail log and exit
func Info(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("info:", s)
	os.Exit(1)
}

// Error log and exit
func Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("error:", s)
	os.Exit(1)
}
