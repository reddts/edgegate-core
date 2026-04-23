package mobile

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	cfg "github.com/reddts/edgegate-core/v2/config"

	_ "github.com/sagernet/gomobile"
	"github.com/sagernet/sing-box/experimental/libbox"
)

var (
	setupLock         sync.Mutex
	server            *libbox.CommandServer
	serverHandlerInst = &mobileCommandServerHandler{}
	lastConfigContent string
)

type noopPlatformInterface struct{}

func (n *noopPlatformInterface) LocalDNSTransport() libbox.LocalDNSTransport { return nil }
func (n *noopPlatformInterface) UsePlatformAutoDetectInterfaceControl() bool { return false }
func (n *noopPlatformInterface) AutoDetectInterfaceControl(fd int32) error   { return nil }
func (n *noopPlatformInterface) OpenTun(options libbox.TunOptions) (int32, error) {
	return 0, fmt.Errorf("OpenTun not available without platform interface")
}
func (n *noopPlatformInterface) UseProcFS() bool { return false }
func (n *noopPlatformInterface) FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (*libbox.ConnectionOwner, error) {
	return nil, fmt.Errorf("not supported")
}
func (n *noopPlatformInterface) StartDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	return nil
}
func (n *noopPlatformInterface) CloseDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	return nil
}
func (n *noopPlatformInterface) GetInterfaces() (libbox.NetworkInterfaceIterator, error) {
	return nil, nil
}
func (n *noopPlatformInterface) UnderNetworkExtension() bool                              { return false }
func (n *noopPlatformInterface) IncludeAllNetworks() bool                                 { return false }
func (n *noopPlatformInterface) ReadWIFIState() *libbox.WIFIState                         { return nil }
func (n *noopPlatformInterface) SystemCertificates() libbox.StringIterator                { return nil }
func (n *noopPlatformInterface) ClearDNSCache()                                           {}
func (n *noopPlatformInterface) SendNotification(notification *libbox.Notification) error { return nil }

type mobileCommandServerHandler struct{}

func (h *mobileCommandServerHandler) ServiceStop() error {
	if server == nil {
		return nil
	}
	return server.CloseService()
}

func (h *mobileCommandServerHandler) ServiceReload() error {
	if server == nil {
		return fmt.Errorf("command server not initialized")
	}
	if strings.TrimSpace(lastConfigContent) == "" {
		return fmt.Errorf("empty config content")
	}
	return server.StartOrReloadService(lastConfigContent, &libbox.OverrideOptions{})
}

func (h *mobileCommandServerHandler) GetSystemProxyStatus() (*libbox.SystemProxyStatus, error) {
	return &libbox.SystemProxyStatus{Enabled: false, Available: false}, nil
}

func (h *mobileCommandServerHandler) SetSystemProxyEnabled(enabled bool) error {
	return nil
}

func (h *mobileCommandServerHandler) WriteDebugMessage(message string) {}

func ensureSetup(baseDir string, workingDir string, tempDir string, listen string, secret string, debug bool, platformInterface libbox.PlatformInterface) error {
	setupLock.Lock()
	defer setupLock.Unlock()

	if platformInterface == nil {
		platformInterface = &noopPlatformInterface{}
	}

	listenPort := int32(0)
	if listen != "" {
		if host, port, err := net.SplitHostPort(listen); err == nil {
			_ = host
			if p, err := strconv.Atoi(port); err == nil && p > 0 && p <= 65535 {
				listenPort = int32(p)
			}
		}
	}

	err := libbox.Setup(&libbox.SetupOptions{
		BasePath:                baseDir,
		WorkingPath:             workingDir,
		TempPath:                tempDir,
		FixAndroidStack:         true,
		CommandServerListenPort: listenPort,
		CommandServerSecret:     secret,
		LogMaxLines:             800,
		Debug:                   debug,
	})
	if err != nil {
		return err
	}

	if server == nil {
		server, err = libbox.NewCommandServer(serverHandlerInst, platformInterface)
		if err != nil {
			return err
		}
		if err = server.Start(); err != nil {
			server = nil
			return err
		}
	}

	return nil
}

func Setup(baseDir string, workingDir string, tempDir string, mode int, listen string, secret string, debug bool, platformInterface libbox.PlatformInterface) error {
	_ = mode
	return ensureSetup(baseDir, workingDir, tempDir, listen, secret, debug, platformInterface)
}

func decodeCoreOptions(optionsJSON string) (*cfg.CoreOptions, error) {
	coreOptions := cfg.DefaultCoreOptions()
	if strings.TrimSpace(optionsJSON) == "" {
		return coreOptions, nil
	}

	if err := json.Unmarshal([]byte(optionsJSON), coreOptions); err != nil {
		return nil, err
	}
	if coreOptions.Warp.WireguardConfigStr != "" {
		if err := json.Unmarshal([]byte(coreOptions.Warp.WireguardConfigStr), &coreOptions.Warp.WireguardConfig); err != nil {
			return nil, err
		}
	}
	if coreOptions.Warp2.WireguardConfigStr != "" {
		if err := json.Unmarshal([]byte(coreOptions.Warp2.WireguardConfigStr), &coreOptions.Warp2.WireguardConfig); err != nil {
			return nil, err
		}
	}
	return coreOptions, nil
}

func BuildConfig(configPath string, optionsJSON string) (string, error) {
	coreOptions, err := decodeCoreOptions(optionsJSON)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	if dir := filepath.Dir(configPath); dir != "" {
		_ = os.Chdir(dir)
	}
	if coreOptions.ExecuteConfigAsIs {
		parsedOptions, err := cfg.ParseOfficialOptions(string(content))
		if err != nil {
			return "", err
		}
		cfg.ApplyOfficialAndroidRuntimeDefaults(parsedOptions)
		cfg.ApplyOfficialResolveDestinationPolicy(parsedOptions, coreOptions)
		cfg.ApplyOfficialRouteRegionPolicy(parsedOptions, coreOptions)
		finalContent, err := cfg.ToJson(*parsedOptions)
		if err != nil {
			return "", err
		}
		return finalContent, nil
	}

	parsedOptions, err := cfg.ParseConfigContentToOptions(string(content), false, coreOptions, false)
	if err != nil {
		return "", err
	}

	finalContent, err := cfg.BuildConfigJson(*coreOptions, *parsedOptions)
	if err != nil {
		return "", err
	}
	return finalContent, nil
}

func ValidateConfig(configPath string, optionsJSON string) error {
	coreOptions, err := decodeCoreOptions(optionsJSON)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(configPath); dir != "" {
		_ = os.Chdir(dir)
	}
	if coreOptions.ExecuteConfigAsIs {
		parsedOptions, parseErr := cfg.ParseOfficialOptions(string(content))
		if parseErr != nil {
			return parseErr
		}
		cfg.ApplyOfficialResolveDestinationPolicy(parsedOptions, coreOptions)
		_, err = cfg.ToJson(*parsedOptions)
		return err
	}
	_, err = cfg.ParseConfigContentToOptions(string(content), false, coreOptions, false)
	return err
}

func Start(configPath string, configContent string) error {
	if server == nil {
		return fmt.Errorf("command server not initialized")
	}
	if strings.TrimSpace(configContent) == "" {
		generated, err := BuildConfig(configPath, "")
		if err != nil {
			return err
		}
		configContent = generated
	}
	lastConfigContent = configContent
	return server.StartOrReloadService(configContent, &libbox.OverrideOptions{})
}

func Stop() error {
	if server == nil {
		return nil
	}
	return server.CloseService()
}

func GetServerPublicKey() []byte {
	return nil
}

func AddGrpcClientPublicKey(clientPublicKey []byte) error {
	_ = clientPublicKey
	return nil
}

func Close(mode int) {
	_ = mode
	setupLock.Lock()
	defer setupLock.Unlock()
	if server != nil {
		server.Close()
		server = nil
	}
	lastConfigContent = ""
}

func Test() string {
	return "Hello from mobile"
}
