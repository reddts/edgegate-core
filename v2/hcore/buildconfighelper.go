package hcore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/reddts/edgegate-core/v2/config"
	"github.com/reddts/edgegate-core/v2/db"
	hcommon "github.com/reddts/edgegate-core/v2/hcommon"
	hutils "github.com/reddts/edgegate-core/v2/hutils"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func BuildConfigJson(in *StartRequest) (string, error) {
	Log(LogLevel_DEBUG, LogType_CORE, "Stating Service ")

	parsedContent, err := BuildConfig(in)
	if err != nil {
		return "", err
	}
	return config.ToJson(*parsedContent)
}

func BuildConfig(in *StartRequest) (*option.Options, error) {
	Log(LogLevel_DEBUG, LogType_CORE, "Building Config...")
	content := in.ConfigContent

	if content == "" {
		fileContent, err := os.ReadFile(in.ConfigPath)
		if err != nil {
			return nil, err
		}
		content = string(fileContent)
	}
	// Log(LogLevel_DEBUG, LogType_CORE, "Parsing Config... ", in.ConfigPath, " content:", content, "-")
	Log(LogLevel_DEBUG, LogType_CORE, "Parsing Config... ")

	options, err := buildRuntimeOptionsFromContent(
		content,
		in.EnableRawConfig || (static.CoreOptions != nil && static.CoreOptions.ExecuteConfigAsIs),
		static.CoreOptions,
	)
	Log(LogLevel_DEBUG, LogType_CORE, "Parsed")
	if err != nil {
		return nil, err
	}
	return options, nil
}

func (s *CoreRPCServer) Parse(ctx context.Context, in *ParseRequest) (*ParseResponse, error) {
	return Parse(in)
}

func Parse(in *ParseRequest) (*ParseResponse, error) {
	defer config.DeferPanicToError("parse", func(err error) {
		Log(LogLevel_FATAL, LogType_CONFIG, err.Error())
		StopAndAlert(MessageType_UNEXPECTED_ERROR, err.Error())
	})

	content := in.Content
	if content == "" {
		path := in.TempPath
		if path == "" {
			path = in.ConfigPath
		}
		contentBytes, err := os.ReadFile(path)
		content = string(contentBytes)
		// os.Chdir(filepath.Dir(in.ConfigPath))
		if err != nil {
			return nil, err
		}

	}

	var err error
	var configContent []byte
	if static.CoreOptions != nil && static.CoreOptions.ExecuteConfigAsIs {
		_, err = readOfficialOptions(content)
		if err != nil {
			return &ParseResponse{
				ResponseCode: hcommon.ResponseCode_FAILED,
				Message:      err.Error(),
			}, err
		}
		configContent = []byte(content)
	} else {
		var parsed []byte
		parsed, err = config.ParseConfigContent(content, true, static.CoreOptions, false)
		if err != nil {
			return &ParseResponse{
				ResponseCode: hcommon.ResponseCode_FAILED,
				Message:      err.Error(),
			}, err
		}
		configContent = parsed
	}
	if in.ConfigPath != "" {
		err = os.WriteFile(in.ConfigPath, configContent, 0o644)
		if err != nil {
			return &ParseResponse{
				ResponseCode: hcommon.ResponseCode_FAILED,
				Message:      err.Error(),
			}, err
		}
	}
	return &ParseResponse{
		ResponseCode: hcommon.ResponseCode_OK,
		Content:      string(configContent),
		Message:      "",
	}, err
}

func (s *CoreRPCServer) ChangeCoreSettings(ctx context.Context, in *ChangeCoreSettingsRequest) (*CoreInfoResponse, error) {
	return ChangeCoreSettings(in)
}

func ChangeCoreSettings(in *ChangeCoreSettingsRequest) (*CoreInfoResponse, error) {
	static.CoreOptions = config.DefaultCoreOptions()
	if in.CoreSettingsJson == "" {
		return &CoreInfoResponse{}, nil
	}
	settings := db.GetTable[hcommon.AppSettings]()
	settings.UpdateInsert(&hcommon.AppSettings{
		Id:    "CoreSettingsJson",
		Value: in.CoreSettingsJson,
	})
	err := json.Unmarshal([]byte(in.CoreSettingsJson), static.CoreOptions)
	if err != nil {
		return nil, err
	}
	if static.CoreOptions.Warp.WireguardConfigStr != "" {
		err := json.Unmarshal([]byte(static.CoreOptions.Warp.WireguardConfigStr), &static.CoreOptions.Warp.WireguardConfig)
		if err != nil {
			return nil, err
		}
	}
	if static.CoreOptions.Warp2.WireguardConfigStr != "" {
		err := json.Unmarshal([]byte(static.CoreOptions.Warp2.WireguardConfigStr), &static.CoreOptions.Warp2.WireguardConfig)
		if err != nil {
			return nil, err
		}
	}
	return &CoreInfoResponse{}, nil
}

func (s *CoreRPCServer) GenerateConfig(ctx context.Context, in *GenerateConfigRequest) (*GenerateConfigResponse, error) {
	return GenerateConfig(in)
}

func GenerateConfig(in *GenerateConfigRequest) (*GenerateConfigResponse, error) {
	defer config.DeferPanicToError("generateConfig", func(err error) {
		Log(LogLevel_FATAL, LogType_CONFIG, err.Error())
		StopAndAlert(MessageType_UNEXPECTED_ERROR, err.Error())
	})
	if static.CoreOptions == nil {
		static.CoreOptions = config.DefaultCoreOptions()
	}
	config, err := generateConfigFromFile(in.Path, *static.CoreOptions)
	if err != nil {
		return nil, err
	}
	return &GenerateConfigResponse{
		ConfigContent: config,
	}, nil
}

func generateConfigFromFile(path string, configOpt config.CoreOptions) (string, error) {
	os.Chdir(filepath.Dir(path))
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	options, err := buildRuntimeOptionsFromContent(string(content), configOpt.ExecuteConfigAsIs, &configOpt)
	if err != nil {
		return "", err
	}
	generatedConfig, err := config.ToJson(*options)
	if err != nil {
		return "", err
	}
	return generatedConfig, nil
}

func removeTunnelIfNeeded(options *option.Options) (tuninb *option.TunInboundOptions) {
	if hutils.TunAllowed() {
		return nil
	}

	// Create a new slice to hold the remaining inbounds
	newInbounds := make([]option.Inbound, 0, len(options.Inbounds))

	for _, inb := range options.Inbounds {
		if inb.Type == C.TypeTun {
			switch tunOpts := inb.Options.(type) {
			case option.TunInboundOptions:
				copied := tunOpts
				tuninb = &copied
			case *option.TunInboundOptions:
				tuninb = tunOpts
			}
		} else {
			newInbounds = append(newInbounds, inb)
		}
	}

	options.Inbounds = newInbounds
	return tuninb
}

func buildRuntimeOptionsFromContent(content string, rawConfig bool, coreOptions *config.CoreOptions) (*option.Options, error) {
	if rawConfig {
		return config.ParseOfficialRuntimeOptions(content, coreOptions)
	}
	return config.ConvertLegacyContentToRuntimeOptions(content, true, coreOptions)
}
