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

type RegexpLabel struct {
	Regex *regexp.Regexp `json:"regex" yaml:"regex"`
	Repl  string         `json:"repl" yaml:"repl"`
}

func (l *RegexpLabel) GetString(val string) string {
	var repl string
	if len(l.Repl) == 0 {
		repl = "$1"
	} else {
		repl = l.Repl
	}
	indexes := l.Regex.FindStringSubmatchIndex(val)
	if indexes == nil {
		return ""
	}
	res := l.Regex.ExpandString([]byte{}, repl, val, indexes)
	if len(res) == 0 {
		return ""
	}
	return string(res)
}

func (l *RegexpLabel) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		l.Regex, err = regexp.Compile(str)
		l.Repl = "$1"
		return err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		l.Regex, err = regexp.Compile(m["regex"])
		l.Repl = m["repl"]
		return err
	}
	return errors.Wrap(ErrUnsupportFormat, "regexp label")
}

func (l *RegexpLabel) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err == nil {
		l.Regex, err = regexp.Compile(str)
		l.Repl = "$1"
		return err
	}
	var m map[string]string
	if err := value.Decode(&m); err == nil {
		l.Regex, err = regexp.Compile(m["regex"])
		l.Repl = m["repl"]
		return err
	}
	return errors.Wrap(ErrUnsupportFormat, "regexp label")
}

type ConfigFile struct {
	Paths  []string                `json:"paths" yaml:"paths"`
	Labels map[string]*RegexpLabel `json:"labels" yaml:"labels"`
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
	for key, conf := range c.Labels {
		val := conf.GetString(fp)
		if key == NameKey {
			name = val
		} else {
			labels[key] = val
		}
	}
	return
}
