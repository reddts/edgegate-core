package sdk

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"

	"github.com/reddts/edgegate-core/v2/config"
	hcore "github.com/reddts/edgegate-core/v2/hcore"
	"github.com/sagernet/sing-box/option"
)

func RunInstance(coreSettings *config.CoreOptions, singconfig *option.Options) (*hcore.InstanceService, error) {
	return hcore.RunInstance(coreSettings, singconfig)
}

func ParseConfig(coreSettings *config.CoreOptions, configStr string) (*option.Options, error) {
	if coreSettings == nil {
		coreSettings = config.DefaultCoreOptions()
	}
	if strings.HasPrefix(configStr, "http://") || strings.HasPrefix(configStr, "https://") {
		client := &http.Client{}
		configPath := strings.Split(configStr, "\n")[0]
		// Create a new request
		req, err := http.NewRequest("GET", configPath, nil)
		if err != nil {
			fmt.Println("Error creating request:", err)
			return nil, err
		}
		req.Header.Set("User-Agent", "edgegate/2.2.0 ("+runtime.GOOS+") like ClashMeta v2ray sing-box")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error making GET request:", err)
			return nil, err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read config body: %w", err)
		}
		configStr = string(body)
	}
	return config.ParseConfigContentToOptions(configStr, true, coreSettings, false)
}
