package config

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/databricks/databricks-sql-go/auth"
	"github.com/databricks/databricks-sql-go/auth/noop"
	"github.com/databricks/databricks-sql-go/auth/pat"
	"github.com/databricks/databricks-sql-go/internal/cli_service"
	"github.com/databricks/databricks-sql-go/logger"
	"github.com/pkg/errors"
)

// Driver Configurations.
// Only UserConfig are currently exposed to users
type Config struct {
	UserConfig
	TLSConfig                 *tls.Config // nil disables TLS
	RunAsync                  bool
	PollInterval              time.Duration
	ClientTimeout             time.Duration // max time the http request can last
	PingTimeout               time.Duration // max time allowed for ping
	CanUseMultipleCatalogs    bool
	DriverName                string
	DriverVersion             string
	ThriftProtocol            string
	ThriftTransport           string
	ThriftProtocolVersion     cli_service.TProtocolVersion
	ThriftDebugClientProtocol bool
}

// ToEndpointURL generates the endpoint URL from Config that a Thrift client will connect to
func (c *Config) ToEndpointURL() string {
	var userInfo string
	endpointUrl := fmt.Sprintf("%s://%s%s:%d%s", c.Protocol, userInfo, c.Host, c.Port, c.HTTPPath)
	return endpointUrl
}

// DeepCopy returns a true deep copy of Config
func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}

	return &Config{
		UserConfig:                c.UserConfig.DeepCopy(),
		TLSConfig:                 c.TLSConfig.Clone(),
		RunAsync:                  c.RunAsync,
		PollInterval:              c.PollInterval,
		ClientTimeout:             c.ClientTimeout,
		PingTimeout:               c.PingTimeout,
		CanUseMultipleCatalogs:    c.CanUseMultipleCatalogs,
		DriverName:                c.DriverName,
		DriverVersion:             c.DriverVersion,
		ThriftProtocol:            c.ThriftProtocol,
		ThriftTransport:           c.ThriftTransport,
		ThriftProtocolVersion:     c.ThriftProtocolVersion,
		ThriftDebugClientProtocol: c.ThriftDebugClientProtocol,
	}
}

// UserConfig is the set of configurations exposed to users
type UserConfig struct {
	Protocol       string
	Host           string // from databricks UI
	Port           int    // from databricks UI
	HTTPPath       string // from databricks UI
	Catalog        string
	Schema         string
	Authenticator  auth.Authenticator
	AccessToken    string        // from databricks UI
	MaxRows        int           // max rows per page
	QueryTimeout   time.Duration // Timeout passed to server for query processing
	UserAgentEntry string
	Location       *time.Location
	SessionParams  map[string]string
	RetryWaitMin   time.Duration
	RetryWaitMax   time.Duration
	RetryMax       int
}

// DeepCopy returns a true deep copy of UserConfig
func (ucfg UserConfig) DeepCopy() UserConfig {
	var sessionParams map[string]string
	if ucfg.SessionParams != nil {
		sessionParams = make(map[string]string)
		for k, v := range ucfg.SessionParams {
			sessionParams[k] = v
		}
	}
	var loccp *time.Location
	if ucfg.Location != nil {
		var err error
		loccp, err = time.LoadLocation(ucfg.Location.String())
		if err != nil {
			logger.Warn().Msg("could not copy location")
		}

	}

	return UserConfig{
		Protocol:       ucfg.Protocol,
		Host:           ucfg.Host,
		Port:           ucfg.Port,
		HTTPPath:       ucfg.HTTPPath,
		Catalog:        ucfg.Catalog,
		Schema:         ucfg.Schema,
		Authenticator:  ucfg.Authenticator,
		AccessToken:    ucfg.AccessToken,
		MaxRows:        ucfg.MaxRows,
		QueryTimeout:   ucfg.QueryTimeout,
		UserAgentEntry: ucfg.UserAgentEntry,
		Location:       loccp,
		SessionParams:  sessionParams,
		RetryWaitMin:   ucfg.RetryWaitMin,
		RetryWaitMax:   ucfg.RetryWaitMax,
		RetryMax:       ucfg.RetryMax,
	}
}

var defaultMaxRows = 100000

// WithDefaults provides default settings for optional fields in UserConfig
func (ucfg UserConfig) WithDefaults() UserConfig {
	if ucfg.MaxRows <= 0 {
		ucfg.MaxRows = defaultMaxRows
	}
	if ucfg.Protocol == "" {
		ucfg.Protocol = "https"
		ucfg.Port = 443
	}
	if ucfg.Port == 0 {
		ucfg.Port = 443
	}
	if ucfg.Authenticator == nil {
		ucfg.Authenticator = &noop.NoopAuth{}
	}
	if ucfg.SessionParams == nil {
		ucfg.SessionParams = make(map[string]string)
	}
	if ucfg.RetryMax == 0 {
		ucfg.RetryMax = 4
	}
	if ucfg.RetryWaitMin == 0 {
		ucfg.RetryWaitMin = 1 * time.Second
	}
	if ucfg.RetryWaitMax == 0 {
		ucfg.RetryWaitMax = 30 * time.Second
	}

	return ucfg
}

// WithDefaults provides default settings for Config
func WithDefaults() *Config {
	return &Config{
		UserConfig:                UserConfig{}.WithDefaults(),
		TLSConfig:                 &tls.Config{MinVersion: tls.VersionTLS12},
		RunAsync:                  true,
		PollInterval:              1 * time.Second,
		ClientTimeout:             900 * time.Second,
		PingTimeout:               60 * time.Second,
		CanUseMultipleCatalogs:    true,
		DriverName:                "godatabrickssqlconnector", // important. Do not change
		DriverVersion:             "0.9.0",
		ThriftProtocol:            "binary",
		ThriftTransport:           "http",
		ThriftProtocolVersion:     cli_service.TProtocolVersion_SPARK_CLI_SERVICE_PROTOCOL_V6,
		ThriftDebugClientProtocol: false,
	}

}

// ParseDSN constructs UserConfig by parsing DSN string supplied to `sql.Open()`
func ParseDSN(dsn string) (UserConfig, error) {
	fullDSN := dsn
	if !strings.HasPrefix(dsn, "https://") && !strings.HasPrefix(dsn, "http://") {
		fullDSN = "https://" + dsn
	}
	parsedURL, err := url.Parse(fullDSN)
	if err != nil {
		return UserConfig{}, errors.Wrap(err, "invalid DSN: invalid format")
	}
	ucfg := UserConfig{}.WithDefaults()
	ucfg.Protocol = parsedURL.Scheme
	ucfg.Host = parsedURL.Hostname()
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return UserConfig{}, errors.Wrap(err, "invalid DSN: invalid DSN port")
	}
	ucfg.Port = port
	name := parsedURL.User.Username()
	if name == "token" {
		pass, ok := parsedURL.User.Password()
		if pass == "" {
			return UserConfig{}, errors.New("invalid DSN: empty token")
		}
		if ok {
			ucfg.AccessToken = pass
			pat := &pat.PATAuth{
				AccessToken: pass,
			}
			ucfg.Authenticator = pat
		}
	} else {
		if name != "" {
			return UserConfig{}, errors.New("invalid DSN: basic auth not enabled")
		}
	}
	ucfg.HTTPPath = parsedURL.Path
	params := parsedURL.Query()
	maxRowsStr := params.Get("maxRows")
	if maxRowsStr != "" {
		maxRows, err := strconv.Atoi(maxRowsStr)
		if err != nil {
			return UserConfig{}, errors.Wrap(err, "invalid DSN: maxRows param is not an integer")
		}
		// we should always have at least some page size
		if maxRows != 0 {
			ucfg.MaxRows = maxRows
		}
	}
	params.Del("maxRows")

	timeoutStr := params.Get("timeout")
	if timeoutStr != "" {
		timeoutSeconds, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return UserConfig{}, errors.Wrap(err, "invalid DSN: timeout param is not an integer")
		}
		ucfg.QueryTimeout = time.Duration(timeoutSeconds) * time.Second
	}
	params.Del("timeout")
	if params.Has("catalog") {
		ucfg.Catalog = params.Get("catalog")
		params.Del("catalog")
	}
	if params.Has("userAgentEntry") {
		ucfg.UserAgentEntry = params.Get("userAgentEntry")
		params.Del("userAgentEntry")
	}
	if params.Has("schema") {
		ucfg.Schema = params.Get("schema")
		params.Del("schema")
	}
	for k := range params {
		if strings.ToLower(k) == "timezone" {
			ucfg.Location, err = time.LoadLocation(params.Get("timezone"))
		}
	}
	if len(params) > 0 {
		sessionParams := make(map[string]string)
		for k := range params {
			sessionParams[k] = params.Get(k)
		}
		ucfg.SessionParams = sessionParams
	}

	return ucfg, err
}
