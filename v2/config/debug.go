package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	SJ "github.com/sagernet/sing/common/json"
)

func SaveCurrentConfig(path string, options option.Options) error {
	json, err := ToJson(options)
	if err != nil {
		return err
	}
	p, err := filepath.Abs(path)
	os.MkdirAll(filepath.Dir(p), 0o755)
	fmt.Printf("Saving config to %v %+v\n", p, err)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(json), 0o644)
}

func ToJson(options option.Options) (string, error) {
	content, err := marshalOptionsJSON(options)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, content, "", "  "); err != nil {
		fmt.Printf("ERROR in coding:%+v\n", err)
		return "", err
	}
	buffer.WriteByte('\n')
	return buffer.String(), nil
}

func marshalOptionsJSON(options option.Options) ([]byte, error) {
	ctx := include.Context(context.Background())
	content, err := SJ.MarshalContext(ctx, options)
	if err != nil {
		fmt.Printf("ERROR in coding:%+v\n", err)
		return nil, err
	}
	return content, nil
}

func DeferPanicToError(name string, err func(error)) {
	if r := recover(); r != nil {
		s := fmt.Errorf("%s panic: %s\n%s", name, r, string(debug.Stack()))
		err(s)
		<-time.After(5 * time.Second)
	}
}
