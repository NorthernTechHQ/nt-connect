// Copyright 2023 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package config

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/northerntechhq/nt-connect/api"
	cryptoutils "github.com/northerntechhq/nt-connect/utils/crypto"
	"github.com/northerntechhq/nt-connect/utils/types"
)

type TerminalConfig struct {
	Width  uint16
	Height uint16
	// Disable remote terminal
	Disable bool
}

type MenderClientConfig struct {
	// Disable mender-client websocket bindings.
	Disable bool
}

type FileTransferConfig struct {
	// Disable file transfer features
	Disable bool
}

type PortForwardConfig struct {
	// Disable port forwarding feature
	Disable bool
}

type SessionsConfig struct {
	// Whether to stop expired sessions
	StopExpired bool
	// Seconds after startup of a sessions that will make it expire
	ExpireAfter uint32
	// Seconds after last activity of a sessions that will make it expire
	ExpireAfterIdle uint32
	// Max sessions per user
	MaxPerUser uint32
}

// Counter for the limits  and restrictions for the File Transfer
// on and off the device(MEN-4325)
type RateLimits struct {
	// Maximum bytes count allowed to transfer per minute
	// this is per device global limit, which is consulted
	// every time there is a transfer starting. if above
	// the limit, we answer with error message indicating
	// limit reached.
	MaxBytesTxPerMinute uint64
	MaxBytesRxPerMinute uint64
}

// Limits and restrictions for the File Transfer on and off the device(MEN-4325)
type FileTransferLimits struct {
	// No way to escape Chroot, even if this one is set the Chroot setting will
	// be checked for the target of any link and restricted accordingly
	FollowSymLinks bool
	// Allow overwrite files
	AllowOverwrite bool
	// set the owner of new files to OwnerPut
	OwnerPut string
	// set the owner of new files to OwnerPut
	GroupPut string
	// allow to get only files owned by OwnerGet
	OwnerGet []string
	// allow to get only files owned by OwnerGet
	GroupGet []string
	// umask for new files
	Umask string
	// Maximum allowed file size
	MaxFileSize uint64
	// File transfer rate limits
	Counters RateLimits
	// If true it is allowed to upload files with set user id on execute bit set
	AllowSuid bool
	// By default we only allow to send/put regular files
	RegularFilesOnly bool
	// By default we preserve the file modes but set one according to
	//the current umask or configured Umask above
	PreserveMode bool
	// By default we preserve the owner of the file uploaded
	PreserveOwner bool
}

type Limits struct {
	Enabled      bool               `json:"Enabled"`
	FileTransfer FileTransferLimits `json:"FileTransfer"`
}

// NTConnectConfigFromFile holds the configuration settings read from the config file
type NTConnectConfigFromFile struct {
	// The command to run as shell
	ShellCommand string `json:",omitempty"`
	// ShellArguments is the arguments the shell is launched with. Defaults
	// to '--login'.
	ShellArguments []string `json:",omitempty"`
	// Name of the user who owns the shell process
	User string `json:",omitempty"`
	// Terminal settings
	Terminal TerminalConfig `json:"Terminal,omitempty"`
	// User sessions settings
	Sessions SessionsConfig `json:"Sessions,omitempty"`
	// Limits and restrictions
	Limits Limits `json:"Limits,omitempty"`
	// Reconnect interval
	ReconnectIntervalSeconds int `json:",omitempty"`
	// FileTransfer config
	FileTransfer FileTransferConfig `json:",omitempty"`
	// PortForward config
	PortForward PortForwardConfig `json:",omitempty"`
	// TLS configures how the client manages tls sessions.
	TLS TLSConfig `json:"TLS,omitempty"`
	// APIConfig
	APIConfig APIConfig `json:"API,omitempty"`
	// MenderClient config
	MenderClient MenderClientConfig
	Chroot       string `json:"Chroot,omitempty"`
}

type TLSConfig struct {
	CACertificate      string `json:"CACertificate,omitempty"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify,omitempty"`
}

func (cfg TLSConfig) ToStdConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if cfg.CACertificate != "" {
		certs, err := cryptoutils.LoadCertificates(cfg.CACertificate)
		if err != nil {
			return nil, fmt.Errorf("config: failed to load CACertificates: %w", err)
		}
		tlsConfig.RootCAs = certs
	}
	return tlsConfig, nil
}

type APIType string

const (
	APITypeHTTP = "http"
	APITypeDBus = "dbus"
)

func (t APIType) Validate() error {
	switch t {
	case APITypeHTTP, APITypeDBus:
		return nil
	default:
	}
	return fmt.Errorf("invalid auth type %q", t)
}

type APIConfig struct {
	APIType        `json:"Type"`
	ServerURL      string `json:"ServerURL"`
	PrivateKeyPath string `json:"PrivateKeyPath"`
	IdentityPath   string `json:"IdentityPath"`
	TenantToken    string `json:"TenantToken"`
	ExternalID     string `json:"ExternalID"`

	InventoryExecutable string         `json:"InventoryExecutable"`
	InventoryInterval   types.Duration `json:"InventoryInterval"`

	privateKey crypto.Signer
	identity   *api.Identity
}

const (
	envTenantToken = "CONNECT_TENANT_TOKEN"
	envServerURL   = "CONNECT_SERVER_URL"
	envChroot      = "CONNECT_CHROOT"
)

const maxIdentityFileSize = 512 * 1024

func (cfg *APIConfig) loadIdentity(buf *bytes.Buffer) error {
	buf.Reset()
	fd, err := os.Open(cfg.IdentityPath)
	if err != nil {
		return fmt.Errorf("failed to open identity file: %w", err)
	}
	_, err = buf.ReadFrom(io.LimitReader(fd, maxIdentityFileSize))
	_ = fd.Close()
	if err != nil {
		if os.IsNotExist(err) {
			progName := "nt-connect"
			if len(os.Args) > 0 {
				progName = os.Args[0]
			}
			log.Errorf("identity not bootstrapped: please run command: %s bootstrap",
				progName)
		}
		return fmt.Errorf("failed to read identity file: %w", err)
	}
	err = json.Unmarshal(buf.Bytes(), &cfg.identity)
	if err != nil {
		return fmt.Errorf("failed to deserialize device identity: %w", err)
	}
	if cfg.TenantToken != "" {
		cfg.identity.TenantToken = cfg.TenantToken
	} else {
		cfg.TenantToken = cfg.identity.TenantToken
	}
	return nil
}

func (cfg *APIConfig) load() error {

	if cfg.APIType != APITypeHTTP {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	if err := cfg.loadIdentity(buf); err != nil {
		return err
	}
	buf.Reset()

	fd, err := os.Open(cfg.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to open private key file: %w", err)
	}
	r := io.LimitReader(fd, maxIdentityFileSize)
	_, err = buf.ReadFrom(r)
	_ = fd.Close()
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}
	pkey, err := cryptoutils.LoadPrivateKey(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}
	cfg.privateKey = pkey
	buf.Reset()
	return nil
}

const magicTenantToken = "REPLACE_THIS_WITH_YOUR_TOKEN"

func (cfg *APIConfig) Validate() error {
	err := cfg.load()
	if err != nil {
		return err
	}
	if cfg.APIType == APITypeHTTP {
		if cfg.ServerURL == "" {
			err = fmt.Errorf("empty value")
		} else {
			_, err = url.Parse(cfg.ServerURL)
		}
		if err != nil {
			return fmt.Errorf("invalid ServerURL: %w", err)
		}
		if cfg.TenantToken == magicTenantToken {
			if strings.HasPrefix(cfg.ExternalID, "iot-hub") {
				return fmt.Errorf(
					"Default tenant token found in env var %s: "+
						"please customize tenant token in "+
						"Azure IoT Edge module, or where you set "+
						"environment variables", envTenantToken)
			} else {
				return fmt.Errorf("TenantToken (env: %s) invalid: "+
					"please copy the token from your account settings",
					envTenantToken)
			}
		}
		if cfg.TenantToken == "" {
			return fmt.Errorf("TenantToken (env: %s) cannot be blank", envTenantToken)
		}
	}
	return nil
}

func (cfg *APIConfig) GetPrivateKey() crypto.Signer {
	return cfg.privateKey
}

func (cfg *APIConfig) GetIdentity() *api.Identity {
	return cfg.identity
}

// NTConnectConfig holds the configuration settings for the Mender shell client
type NTConnectConfig struct {
	NTConnectConfigFromFile
	Debug bool
	Trace bool
}

// NewNTConnectConfig initializes a new NTConnectConfig struct
func NewNTConnectConfig() *NTConnectConfig {
	return &NTConnectConfig{
		NTConnectConfigFromFile: NTConnectConfigFromFile{
			APIConfig: APIConfig{
				PrivateKeyPath: path.Join(DefaultDataStore, "private.pem"),
				IdentityPath:   path.Join(DefaultDataStore, "identity.json"),

				InventoryInterval:   types.Duration(time.Hour),
				InventoryExecutable: path.Join(DefaultPathDataDir, "inventory.sh"),
			},
		},
	}
}

// LoadConfig parses the mender configuration json-files
// (/etc/mender/mender-connect.conf and /var/lib/mender/mender-connect.conf)
// and loads the values into the NTConnectConfig structure defining high level
// client configurations.
func LoadConfig(mainConfigFile string, fallbackConfigFile string) (*NTConnectConfig, error) {
	// Load fallback configuration first, then main configuration.
	// It is OK if either file does not exist, so long as the other one does exist.
	// It is also OK if both files exist.
	// Because the main configuration is loaded last, its option values
	// override those from the fallback file, for options present in both files.
	var filesLoadedCount int
	config := NewNTConnectConfig()

	if loadErr := loadConfigFile(fallbackConfigFile, config, &filesLoadedCount); loadErr != nil {
		return nil, loadErr
	}

	if loadErr := loadConfigFile(mainConfigFile, config, &filesLoadedCount); loadErr != nil {
		return nil, loadErr
	}

	log.Debugf("Loaded %d configuration file(s)", filesLoadedCount)
	if filesLoadedCount == 0 {
		log.Info("No configuration files present. Using defaults")
		return config, nil
	}

	log.Debugf("Loaded configuration = %#v", config)

	if token, ok := os.LookupEnv(envTenantToken); ok {
		config.APIConfig.TenantToken = token
	}
	if url, ok := os.LookupEnv(envServerURL); ok {
		config.APIConfig.ServerURL = url
	}
	if root, ok := os.LookupEnv(envChroot); ok {
		config.Chroot = root
	}
	return config, nil
}

func isExecutable(path string) bool {
	info, _ := os.Stat(path)
	if info == nil {
		return false
	}
	mode := info.Mode()
	return (mode & 0111) != 0
}

func isInShells(path string) bool {
	file, err := os.Open("/etc/shells")
	if err != nil {
		// if no /etc/shell is found, DefaultShellCommand is accepted
		if path == DefaultShellCommand {
			return true
		}
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	found := false
	for scanner.Scan() {
		if scanner.Text() == path {
			found = true
			break
		}
	}
	return found
}

func validateUser(c *NTConnectConfig) (err error) {
	if c.User == "" {
		return errors.New("please provide a user to run the shell as")
	}
	u, err := user.Lookup(c.User)
	if err == nil && u == nil {
		return errors.New("unknown error while getting a user id")
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *NTConnectConfig) applyDefaults() error {
	//check if shell is given, if not, defaulting to /bin/sh
	if c.ShellCommand == "" {
		log.Warnf("ShellCommand is empty, defaulting to %s", DefaultShellCommand)
		c.ShellCommand = DefaultShellCommand
	}

	if c.ShellArguments == nil {
		log.Warnf("ShellArguments is empty, defaulting to %s", DefaultShellArguments)
		c.ShellArguments = DefaultShellArguments
	}

	if c.Terminal.Width == 0 {
		c.Terminal.Width = DefaultTerminalWidth
	}

	if c.Terminal.Height == 0 {
		c.Terminal.Height = DefaultTerminalHeight
	}

	if !c.Sessions.StopExpired {
		c.Sessions.ExpireAfter = 0
		c.Sessions.ExpireAfterIdle = 0
	} else {
		if c.Sessions.ExpireAfter > 0 && c.Sessions.ExpireAfterIdle > 0 {
			log.Warnf("both ExpireAfter and ExpireAfterIdle specified.")
		}
	}

	if c.ReconnectIntervalSeconds == 0 {
		c.ReconnectIntervalSeconds = DefaultReconnectIntervalsSeconds
	}

	// permit by default, probably will be changed after integration test is modified
	c.Limits.FileTransfer.PreserveMode = true
	c.Limits.FileTransfer.PreserveOwner = true

	return nil
}

// Validate verifies the Servers fields in the configuration
func (c *NTConnectConfig) Validate() (err error) {
	if err = c.applyDefaults(); err != nil {
		return err
	}

	if !filepath.IsAbs(c.ShellCommand) {
		return errors.New("given shell (" + c.ShellCommand + ") is not an absolute path")
	}

	if !isExecutable(c.ShellCommand) {
		return errors.New("given shell (" + c.ShellCommand + ") is not executable")
	}

	err = validateUser(c)
	if err != nil {
		return err
	}

	if !isInShells(c.ShellCommand) {
		log.Errorf("ShellCommand %s is not present in /etc/shells", c.ShellCommand)
		return errors.New("ShellCommand " + c.ShellCommand + " is not present in /etc/shells")
	}

	if err := c.APIConfig.Validate(); err != nil {
		return fmt.Errorf("invalid API configuration: %w", err)
	}

	log.Debugf("Verified configuration = %#v", c)
	return nil
}

func loadConfigFile(configFile string, config *NTConnectConfig, filesLoadedCount *int) error {
	// Do not treat a single config file not existing as an error here.
	// It is up to the caller to fail when both config files don't exist.
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Debug("Configuration file does not exist: ", configFile)
		return nil
	}

	if err := readConfigFile(&config.NTConnectConfigFromFile, configFile); err != nil {
		log.Errorf("Error loading configuration from file: %s (%s)", configFile, err.Error())
		return err
	}

	*filesLoadedCount++
	log.Info("Loaded configuration file: ", configFile)
	return nil
}

func readConfigFile(config interface{}, fileName string) error {
	// Reads mender configuration (JSON) file.
	log.Debug("Reading nt-connect configuration from file " + fileName)
	conf, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(conf, &config); err != nil {
		switch err.(type) {
		case *json.SyntaxError:
			return errors.New("Error parsing nt-connect configuration file: " + err.Error())
		}
		return errors.New("Error parsing config file: " + err.Error())
	}

	return nil
}
