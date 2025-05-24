package dirfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	NameKey string = "__name__"
)

var (
	ErrUnsupportFormat = errors.New("unsupported format")
)

type ConfigFile struct {
	Paths  []string                  `json:"paths" yaml:"paths"`
	Labels map[string]*regexp.Regexp `json:"labels" yaml:"labels"`
}

type Config struct {
	Keys  []string      `json:"keys" yaml:"keys"`
	Files []*ConfigFile `json:"files" yaml:"files"`
}

func ReadConfig(filename string) (confs *Config, err error) {
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = &Config{}
		if err = yaml.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	} else if strings.HasSuffix(filename, ".json") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = &Config{}
		if err = json.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	}
	if confs != nil {
		return confs, nil
	}
	return nil, errors.Wrap(ErrUnsupportFormat, filename)
}

func (c *ConfigFile) GetKeyMap(fp string) (name string, labels map[string]string) {
	labels = make(map[string]string)
	name = filepath.Base(fp)
	for key, re := range c.Labels {
		var val string
		vals := re.FindStringSubmatch(fp)
		if len(vals) == 0 {
			continue
		} else if len(vals) == 1 {
			val = vals[0]
		} else {
			val = vals[1]
		}
		if key == NameKey {
			name = val
		} else {
			labels[key] = val
		}
	}
	return
}
