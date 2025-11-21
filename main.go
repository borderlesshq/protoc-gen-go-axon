// main.go - Entry point for protoc-gen-go-axon plugin
package main

import (
	"bytes"
	"go/format"
	"log"

	"github.com/borderlesshq/protoc-gen-go-axon/src"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	opts := protogen.Options{}

	opts.Run(func(gen *protogen.Plugin) error {
		// Declare feature support in the response
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f)
		}
		return nil
	})
}

func generateFile(gen *protogen.Plugin, file *protogen.File) {
	if len(file.Services) == 0 {
		return
	}

	filename := file.GeneratedFilenamePrefix + "_axon.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)

	// Parse template data
	data := &src.TemplateData{
		PackageName: string(file.GoPackageName),
		SourceFile:  file.Desc.Path(),
		Services:    make([]*src.ServiceData, 0, len(file.Services)),
	}

	for _, service := range file.Services {
		serviceData := &src.ServiceData{
			Name:        service.GoName,
			Methods:     make([]*src.MethodData, 0, len(service.Methods)),
			ServiceName: service.GoName,
		}

		for _, method := range service.Methods {
			// Use QualifiedGoIdent so types from other packages get the proper package alias.
			inQualified := g.QualifiedGoIdent(method.Input.GoIdent)
			outQualified := g.QualifiedGoIdent(method.Output.GoIdent)

			methodData := &src.MethodData{
				Name:              method.GoName,
				ServiceName:       service.GoName,
				InputType:         "*" + inQualified,
				OutputType:        "*" + outQualified,
				InputTypeName:     inQualified,
				OutputTypeName:    outQualified,
				IsClientStreaming: method.Desc.IsStreamingClient(),
				IsServerStreaming: method.Desc.IsStreamingServer(),
				Topic:             service.GoName + "." + method.GoName,
			}
			serviceData.Methods = append(serviceData.Methods, methodData)
		}

		data.Services = append(data.Services, serviceData)
	}

	// Execute template
	var buf bytes.Buffer
	if err := src.FileTemplate.Execute(&buf, data); err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Failed to format generated code: %v", err)
		formatted = buf.Bytes()
	}

	g.P(string(formatted))
}
