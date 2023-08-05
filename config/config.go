package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Struct struct {
	WatchingFilePath string        `mapstructure:"watchingFilePath"`
	SoundFilePath    string        `mapstructure:"soundFilePath"`
	SoundFileFormat  string        `mapstructure:"soundFileFormat"`
	Volume           float64       `mapstructure:"volume"`
	DebounceDelay    time.Duration `mapstructure:"debounceDelay"`
}

var Config = &Struct{
	WatchingFilePath: "",
	SoundFilePath:    "",
	SoundFileFormat:  "",
	Volume:           0,
	DebounceDelay:    0 * time.Millisecond,
}

func getConfigPaths() (pathDirs []string, err error) {
	pathDirs = make([]string, 0)

	executable, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	pathDirs = append(pathDirs, filepath.Dir(executable))

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "os.UserHomeDir()")
	}
	pathDirs = append(pathDirs, home)

	return pathDirs, nil
}

func configureSources(v *viper.Viper, configFilePath string) error {
	if configFilePath != "" {
		v.SetConfigFile(configFilePath)
		return nil
	}

	v.SetConfigName("fs-changes-notifier")
	configPaths, err := getConfigPaths()
	if err != nil {
		return errors.Wrap(err, "getConfigPaths()")
	}

	for _, configPath := range configPaths {
		v.AddConfigPath(configPath)
	}

	return nil
}

func Init(configFilePath string) error {
	v := viper.New()

	err := configureSources(v, configFilePath)
	if err != nil {
		return errors.Wrap(err, "configureSources()")
	}

	replacer := strings.NewReplacer(".", "_")
	v.SetEnvKeyReplacer(replacer)
	v.AutomaticEnv()

	err = v.ReadInConfig()

	switch err.(type) {
	case nil:
	case viper.ConfigFileNotFoundError:
		break
	default:
		return errors.Wrap(err, "ReadInConfig()")
	}

	err = v.Unmarshal(Config)
	return errors.Wrap(err, "v.Unmarshal()")
}
