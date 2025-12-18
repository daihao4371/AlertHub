package global

import (
	"alertHub/config"

	"github.com/spf13/viper"
)

var (
	Layout  = "2006-01-02 15:04:05"
	Config  config.App
	Version string
	// StSignKey 签发的秘钥
	StSignKey = []byte(viper.GetString("jwt.AlertHub"))
)
