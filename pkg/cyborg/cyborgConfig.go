package cyborg

import (
	"bytes"
	"errors"
	"github.com/spf13/viper"
	"io/ioutil"
)

type AssignRoleOnGame struct {
	GuildName   string
	GameName    string
	RoleName    string
	ExcludeUIDs []string
}

type CyborgBotDiscordConfig struct {
	Token     string
	Operators []string
	AutoRoles []AssignRoleOnGame
}

type CyborgBotConfig struct {
	DiscordConf CyborgBotDiscordConfig
}

func (c *CyborgBotConfig) check() error {
	if len(c.DiscordConf.Token) == 0 {
		return errors.New("Empty discord bot token")
	}
	return nil
}

func LoadConfig(path string) (*CyborgBotConfig, error) {
	var cfg CyborgBotConfig

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	viper.SetConfigType("yaml")

	err = viper.ReadConfig(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	viper.Unmarshal(&cfg)
	err = cfg.check()
	return &cfg, err
}
