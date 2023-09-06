/* SPDX-License-Identifier: MIT */
package lib

type Request struct {
	Method      string `mapstructure:"method" validate:"omitempty,oneof=GET POST PUT DELETE OPTION"`
	Url         string `mapstructure:"url" validate:"required,http_url"`
	ContentType string `mapstructure:"content-type" validate:"required_with=ContentType"`
	Payload     string `mapstructure:"payload" validate:"required_with=ContentType"`
}

type Credentials struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}
type RequestGroup struct {
	Credentials Credentials `mapstructure:"credentials"`
	Requests    []Request   `mapstructure:"requests" validate:"required,dive"`
}

type Configuration struct {
	Once       bool
	ForceColor bool                    `mapstructure:"force-color"`
	Config     string                  `mapstructure:"config"`
	PortFile   string                  `mapstructure:"port-file" validate:"required,file"`
	Requests   map[string]RequestGroup `mapstructure:"requests" validate:"dive,required"`
}
