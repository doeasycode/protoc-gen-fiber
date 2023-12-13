package generator

import (
	"encoding/json"
	"fmt"
	"github.com/doeasycode/protoc-gen-fiber/generator/helper"
	"google.golang.org/genproto/googleapis/api/annotations"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/go-kratos/kratos/tool/protobuf/pkg/generator"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/naming"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/tag"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/typemap"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/utils"
)

type bm struct {
	generator.Base
	filesHandled int
}

// BmGenerator BM generator.
func BmGenerator() *bm {
	t := &bm{}
	return t
}

// Generate ...
func (t *bm) Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
	t.Setup(in)

	// Showtime! Generate the response.
	resp := new(plugin.CodeGeneratorResponse)
	for _, f := range t.GenFiles {
		respFile := t.generateForFile(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}
	return resp
}

func (t *bm) generateForFile(file *descriptor.FileDescriptorProto) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)

	validateComment := make(map[string]string)
	for _, i2 := range file.MessageType {
		for _, i3 := range i2.Field {
			comment := getValidateComment(i3)
			validateComment[i2.GetName()] = comment
		}
	}

	t.generateFileHeader(file, t.GenPkgName)
	t.generateImports(file)
	t.generatePathConstants(file)
	count := 0
	for i, service := range file.Service {
		count += t.generateBMInterface(file, service)
		t.generateBMRoute(file, service, i, validateComment)
	}

	resp.Name = proto.String(naming.GenFileName(file, ".fiber.go"))
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *bm) generatePathConstants(file *descriptor.FileDescriptorProto) {
	t.P()

	httpMaps := make(map[string]*HTTPInfo)
	for _, service := range file.Service {
		//name := naming.ServiceName(service)
		t.P("var (")
		for _, method := range service.Method {
			if !t.ShouldGenForMethod(file, service, method) {
				continue
			}

			httpInfo := t.getHTTPInfo(file, service, method)
			httpMaps[method.GetName()] = httpInfo

			apiInfo := t.GetHttpInfoCached(file, service, method)
			name := helper.Camelize(strings.Replace(apiInfo.Path, "/", "_", -1))
			t.P(`	Path`, name, ` = "`, apiInfo.Path, `"`)
		}
		t.P(")")
		t.P()
	}
	marshal, _ := json.Marshal(httpMaps)
	log.Println("httpMaps marshal:", string(marshal))
	//os.Exit(-1)
}

func (t *bm) generateFileHeader(file *descriptor.FileDescriptorProto, pkgName string) {
	t.P("// Code generated by protoc-gen-fiber ", generator.Version, ", DO NOT EDIT.")
	t.P("// source: ", file.GetName())
	t.P()
	if t.filesHandled == 0 {
		comment, err := t.Reg.FileComments(file)
		if err == nil && comment.Leading != "" {
			// doc for the first file
			t.P("/*")
			t.P("Package ", t.GenPkgName, " is a generated blademaster stub package.")
			t.P("This code was generated with protoc-gen-fiber ", generator.Version, ".")
			t.P()
			for _, line := range strings.Split(comment.Leading, "\n") {
				line = strings.TrimPrefix(line, " ")
				// ensure we don't escape from the block comment
				line = strings.Replace(line, "*/", "* /", -1)
				t.P(line)
			}
			t.P()
			t.P("It is generated from these files:")
			for _, f := range t.GenFiles {
				t.P("\t", f.GetName())
			}
			t.P("*/")
		}
	}
	t.P(`package `, pkgName)
	t.P()
}

func (t *bm) generateImports(file *descriptor.FileDescriptorProto) {
	//if len(file.Service) == 0 {
	//	return
	//}
	t.P(`import (`)
	//t.P(`	`,t.pkgs["context"], ` "context"`)
	t.P(`	"context"`)
	t.P()
	t.P(`	"github.com/gogo/protobuf/proto"`)
	t.P(`	"github.com/gofiber/fiber/v2"`)

	t.P(`)`)
	// It's legal to import a message and use it as an input or output for a
	// method. Make sure to import the package of any such message. First, dedupe
	// them.
	deps := t.DeduceDeps(file)
	for pkg, importPath := range deps {
		t.P(`import `, pkg, ` `, importPath)
	}
	t.P()
}

// Big header comments to makes it easier to visually parse a generated file.
func (t *bm) sectionComment(sectionTitle string) {
	t.P()
	t.P(`// `, strings.Repeat("=", len(sectionTitle)))
	t.P(`// `, sectionTitle)
	t.P(`// `, strings.Repeat("=", len(sectionTitle)))
	t.P()
}

func (t *bm) generateBMRoute(
	file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto,
	index int, validateComment map[string]string) {
	// old mode is generate xx.route.go in the http pkg
	// new mode is generate route code in the same .bm.go
	// route rule /x{department}/{project-name}/{path_prefix}/method_name
	// generate each route method
	servName := naming.ServiceName(service)
	versionPrefix := naming.GetVersionPrefix(t.GenPkgName)
	svcName := utils.LcFirst(utils.CamelCase(versionPrefix)) + servName + "Svc"

	t.P(`var (`)
	t.P(`	`, svcName, ` `, servName, `FiberServer`)
	t.P(`	`, servName, `Writer`, ` func(c *fiber.Ctx ,message proto.Message) error`)
	t.P(`	`, servName, `Validater`, ` func(message proto.Message) error`)
	t.P(`)`)

	type methodInfo struct {
		midwares      []string
		routeFuncName string
		apiInfo       *generator.HTTPInfo
		methodName    string
	}
	var methList []methodInfo
	var allMidwareMap = make(map[string]bool)
	var isLegacyPkg = false
	for _, method := range service.Method {
		if !t.ShouldGenForMethod(file, service, method) {
			continue
		}
		var midwares []string
		comments, _ := t.Reg.MethodComments(file, service, method)
		tags := tag.GetTagsInComment(comments.Leading)
		if tag.GetTagValue("dynamic", tags) == "true" {
			continue
		}
		apiInfo := t.GetHttpInfoCached(file, service, method)
		isLegacyPkg = apiInfo.IsLegacyPath
		//httpMethod, legacyPath, path := getHttpInfo(file, service, method, t.reg)
		//if legacyPath != "" {
		//	isLegacyPkg = true
		//}

		midStr := tag.GetTagValue("midware", tags)
		if midStr != "" {
			midwares = strings.Split(midStr, ",")
			for _, m := range midwares {
				allMidwareMap[m] = true
			}
		}

		methName := naming.MethodName(method)
		inputType := t.GoTypeName(method.GetInputType())

		needVaild := false

		if v, ok := validateComment[inputType]; ok {
			if v == "required" {
				needVaild = true
			}
		}

		routeName := utils.LcFirst(utils.CamelCase(servName) +
			utils.CamelCase(methName))

		methList = append(methList, methodInfo{
			apiInfo:       apiInfo,
			midwares:      midwares,
			routeFuncName: routeName,
			methodName:    method.GetName(),
		})

		t.P(fmt.Sprintf("func %s (c *fiber.Ctx) error {", routeName))
		t.P(`	p := new(`, inputType, `)`)
		t.P("	if err := c.BodyParser(p); err != nil {")
		t.P("		return err")
		t.P("	}")

		if needVaild {
			t.P(fmt.Sprintf("	if err := %sValidater(p); err != nil {", utils.CamelCase(servName)))
			t.P("		return err")
			t.P("	}")
		}

		t.P(fmt.Sprintf("	resp, err := %sSvc.%s(c.UserContext(), p)", utils.CamelCase(servName), methName))
		t.P("	if err != nil {")
		t.P("		return err")
		t.P("	}")
		t.P(fmt.Sprintf("	return %sWriter(c,resp)", utils.CamelCase(servName)))
		t.P("}")
	}

	// 注册老的路由的方法
	if isLegacyPkg {
		funcName := `Register` + utils.CamelCase(versionPrefix) + servName + `Service`
		t.P(`// `, funcName, ` Register the blademaster route with middleware map`)
		t.P(`// midMap is the middleware map, the key is defined in proto`)
		t.P(`func `, funcName, `(e *bm.Engine, svc `, servName, "BMServer, midMap map[string]bm.HandlerFunc)", ` {`)
		var keys []string
		for m := range allMidwareMap {
			keys = append(keys, m)
		}
		// to keep generated code consistent
		sort.Strings(keys)
		for _, m := range keys {
			t.P(m, ` := midMap["`, m, `"]`)
		}

		t.P(svcName, ` = svc`)
		for _, methInfo := range methList {
			var midArgStr string
			if len(methInfo.midwares) == 0 {
				midArgStr = ""
			} else {
				midArgStr = strings.Join(methInfo.midwares, ", ") + ", "
			}
			t.P(`e.`, helper.UcFirst(strings.ToLower(methInfo.apiInfo.HttpMethod)), `("`, methInfo.apiInfo.LegacyPath, `", `, midArgStr, methInfo.routeFuncName, `)`)
		}
		t.P(`	}`)
	} else {
		// 新的注册路由的方法
		var bmFuncName = fmt.Sprintf("Register%sFiberServer", servName)
		t.P(`// `, bmFuncName, ` Register the fiber route`)
		t.P(`func `, bmFuncName, fmt.Sprintf(`(e *fiber.App, server %sFiberServer, w func(c *fiber.Ctx, message proto.Message) error , v func(message proto.Message) error) {`, utils.CamelCase(servName)))
		t.P(svcName, ` = server`)
		t.P(fmt.Sprintf(`	%sWriter = w`, utils.CamelCase(servName)))
		t.P(fmt.Sprintf(`	%sValidater = v`, utils.CamelCase(servName)))
		for _, methInfo := range methList {
			t.P(`e.`, helper.UcFirst(strings.ToLower(methInfo.apiInfo.HttpMethod)), `("`, methInfo.apiInfo.NewPath, `",`, methInfo.routeFuncName, ` )`)
		}
		t.P(`	}`)
	}
}

func (t *bm) hasHeaderTag(md *typemap.MessageDefinition) bool {
	if md.Descriptor.Field == nil {
		return false
	}
	for _, f := range md.Descriptor.Field {
		t := tag.GetMoreTags(f)
		if t != nil {
			st := reflect.StructTag(*t)
			if st.Get("request") != "" {
				return true
			}
			if st.Get("header") != "" {
				return true
			}
		}
	}
	return false
}

func (t *bm) generateBMInterface(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) int {
	count := 0
	servName := naming.ServiceName(service)
	t.P("// " + servName + "FiberServer is the server API for " + servName + " service.")

	comments, err := t.Reg.ServiceComments(file, service)
	if err == nil {
		t.PrintComments(comments)
	}
	t.P(`type `, servName, `FiberServer interface {`)
	for _, method := range service.Method {
		if !t.ShouldGenForMethod(file, service, method) {
			continue
		}
		count++
		t.generateInterfaceMethod(file, service, method, comments)
		//t.P()
	}
	t.P(`}`)
	return count
}

func (t *bm) generateInterfaceMethod(file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto,
	method *descriptor.MethodDescriptorProto,
	comments typemap.DefinitionComments) {
	comments, err := t.Reg.MethodComments(file, service, method)

	methName := naming.MethodName(method)
	outputType := t.GoTypeName(method.GetOutputType())
	inputType := t.GoTypeName(method.GetInputType())
	tags := tag.GetTagsInComment(comments.Leading)
	if tag.GetTagValue("dynamic", tags) == "true" {
		return
	}

	if err == nil {
		t.PrintComments(comments)
	}

	respDynamic := tag.GetTagValue("dynamic_resp", tags) == "true"
	if respDynamic {
		t.P(fmt.Sprintf(`	%s(ctx context.Context, req *%s) (resp interface{}, err error)`,
			methName, inputType))
	} else {
		t.P(fmt.Sprintf(`	%s(ctx context.Context, req *%s) (resp *%s, err error)`,
			methName, inputType, outputType))
	}
}

func getValidateComment(field *descriptor.FieldDescriptorProto) string {
	var (
		tags []reflect.StructTag
	)
	//get required info from gogoproto.moretags
	moretags := tag.GetMoreTags(field)
	if moretags != nil {
		tags = []reflect.StructTag{reflect.StructTag(*moretags)}
	}
	validateTag := tag.GetTagValue("validate", tags)

	//// trim
	//regStr := []string{
	//	"required *,*",
	//	"omitempty *,*",
	//}
	//for _, v := range regStr {
	//	re, _ := regexp.Compile(v)
	//	validateTag = re.ReplaceAllString(validateTag, "")
	//}
	return validateTag
}

func (t *bm) getHTTPInfo(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto, method *descriptor.MethodDescriptorProto) *HTTPInfo {
	var (
		title            string
		desc             string
		httpMethod       string
		newPath          string
		explicitHTTPPath bool
	)
	comment, _ := t.Reg.MethodComments(file, service, method)
	tags := tag.GetTagsInComment(comment.Leading)
	cleanComments := tag.GetCommentWithoutTag(comment.Leading)
	if len(cleanComments) > 0 {
		title = strings.Trim(cleanComments[0], "\n\r ")
		if len(cleanComments) > 1 {
			descLines := cleanComments[1:]
			desc = strings.Trim(strings.Join(descLines, "\n"), "\r\n ")
		} else {
			desc = ""
		}
	} else {
		title = ""
	}
	googleOptionInfo, err := generator.ParseBMMethod(method)
	if err == nil {
		httpMethod = strings.ToUpper(googleOptionInfo.Method)
		p := googleOptionInfo.PathPattern
		if p != "" {
			explicitHTTPPath = true
			newPath = p
			goto END
		}
	}

	if httpMethod == "" {
		// resolve http method
		httpMethod = tag.GetTagValue("method", tags)
		if httpMethod == "" {
			httpMethod = "GET"
		} else {
			httpMethod = strings.ToUpper(httpMethod)
		}
	}

	newPath = "/" + file.GetPackage() + "." + service.GetName() + "/" + method.GetName()
END:
	var p = newPath
	param := &HTTPInfo{HttpMethod: httpMethod,
		Path:                p,
		NewPath:             newPath,
		IsLegacyPath:        false,
		Title:               title,
		Description:         desc,
		HasExplicitHTTPPath: explicitHTTPPath,
		GoogleOptionInfo: GoogleMethodOptionInfo{
			Method:      googleOptionInfo.Method,
			PathPattern: googleOptionInfo.PathPattern,
			HTTPRule:    googleOptionInfo.HTTPRule,
		},
	}
	if title == "" {
		param.Title = param.Path
	}
	return param
}

// HTTPInfo http info for method
type HTTPInfo struct {
	HttpMethod   string
	Path         string
	LegacyPath   string
	NewPath      string
	IsLegacyPath bool
	Title        string
	Description  string
	// is http path added in the google.api.http option ?
	HasExplicitHTTPPath bool
	GoogleOptionInfo    GoogleMethodOptionInfo
}

type GoogleMethodOptionInfo struct {
	Method      string
	PathPattern string
	HTTPRule    *annotations.HttpRule
}
