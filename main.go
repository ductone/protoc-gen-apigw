package main

import (
	"bytes"
	"io"
	"os"
	"strconv"

	"github.com/ductone/protoc-gen-apigw/internal/apigw"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

func main() {
	options := []pgs.InitOption{
		pgs.DebugEnv("DEBUG_PROTOC_GEN_APIGW"),
	}

	if ok, _ := strconv.ParseBool(os.Getenv("DEBUG_PROTOC_INPUT")); ok {
		buf := &bytes.Buffer{}
		_, err := io.Copy(buf, os.Stdin)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile("input.data", buf.Bytes(), 0600)
		if err != nil {
			panic(err)
		}
		options = append(options,
			pgs.ProtocInput(bytes.NewReader(buf.Bytes())),
		)
	}

	if fname := os.Getenv("DEBUG_PROTOC_USE_FILE"); fname != "" {
		data, err := os.ReadFile(fname)
		if err != nil {
			panic(err)
		}
		options = append(options,
			pgs.ProtocInput(bytes.NewReader(data)),
		)
	}

	pgs.Init(options...).
		RegisterModule(apigw.New()).
		RegisterPostProcessor(pgsgo.GoImports()).
		RegisterPostProcessor(pgsgo.GoFmt()).
		Render()
}
