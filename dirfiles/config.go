package dirfiles

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	ErrUnsupportFormat = errors.New("unsupported format")
)

type CRegexp struct {
	*regexp.Regexp
}

func (c *CRegexp) UnmarshalText(text []byte) error {
	re, err := regexp.Compile(string(text))
	if err != nil {
		return err
	}
	c.Regexp = re
	return nil
}

type DirFileConfig struct {
	KeyId  int      `json:"keyId" yaml:"keyId"`
	NameId int      `json:"nameId" yaml:"nameId"`
	Regex  *CRegexp `json:"regex" yaml:"regex"`
	Paths  []string `json:"paths" yaml:"paths"`
}

func (c *DirFileConfig) check() error {
	if c.KeyId <= 0 {
		c.KeyId = 1
	}
	if c.NameId <= 0 {
		return errors.New("nameId 不符合规则 nameId>0")
	}
	if len(c.Paths) == 0 {
		return errors.New("paths 必须存在")
	}
	return nil
}

type DirFileConfigs []*DirFileConfig

func ReadConfig(filename string) (confs DirFileConfigs, err error) {
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = DirFileConfigs{}
		if err = yaml.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	} else if strings.HasSuffix(filename, ".json") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = DirFileConfigs{}
		if err = json.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	}
	if confs != nil {
		for _, conf := range confs {
			if err := conf.check(); err != nil {
				return nil, err
			}
		}
		return confs, nil
	}
	return nil, errors.Wrap(ErrUnsupportFormat, filename)
}
