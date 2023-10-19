package config

type CustomProtocolConfig struct {
	IpfsSyncFile IpfsSyncFileConfig
}

type IpfsSyncFileConfig struct {
	IpfsURL string
}

type TvNodeConfig struct {
	Tvbase         *TvbaseConfig
	CustomProtocol *CustomProtocolConfig
	Log            *LogConfig
}

type LogConfig struct {
	ModuleLevels map[string]string
}

func NewDefaultTvNodeConfig() *TvNodeConfig {
	ret := TvNodeConfig{
		CustomProtocol: &CustomProtocolConfig{
			IpfsSyncFile: IpfsSyncFileConfig{
				IpfsURL: "/ip4/127.0.0.1/tcp/5001",
			},
		},
		Log: &LogConfig{
			ModuleLevels: map[string]string{
				"tvbase":         "debug",
				"dkvs":           "debug",
				"dmsg":           "debug",
				"customProtocol": "debug",
				"tvnode":         "debug",
				"tvipfs":         "debug",
				"core_http":      "debug",
			},
		},
	}
	ret.Tvbase = NewDefaultTvbaseConfig()
	return &ret
}

func (cfg *TvNodeConfig) SetSyncFileIpfsUrl(ip string, port string) {
	cfg.CustomProtocol.IpfsSyncFile.IpfsURL = "/ip4/" + ip + "/tcp/" + port
}
