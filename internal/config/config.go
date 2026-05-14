package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Generator GeneratorConfig         `yaml:"generator"`
	Models    map[string]ModelMapping `yaml:"models"`
	Overrides map[string]string      `yaml:"overrides"` // 新增：字段级覆盖，如 "User.id": "github.com/.../uuid.UUID"
}

type ModelMapping struct {
	Model string `yaml:"model"` // 对应用户提供的路径，如 "github.com/.../graphql.Int64"
}

type GeneratorConfig struct {
	Package            string            `yaml:"package"`
	DefaultContentType string            `yaml:"default_content_type"`
	ContentTypeAliases map[string]string `yaml:"content_type_aliases"`
	StructTags         []TagConfig       `yaml:"struct_tags"`
	EnableApiDocs      bool              `yaml:"enable_api_docs"`
	DocCase            string            `yaml:"doc_case"` // snake, camel, lower, keep
	BaseURL            string            `yaml:"base_url"`
	DefaultWrap        string            `yaml:"default_wrap"`
	DefaultOkStatus    int               `yaml:"default_ok_status"`
}

type TagConfig struct {
	Name string `yaml:"name"`
	Case string `yaml:"case"` // snake, camel, lower, keep
}

func LoadConfig(path string) (*Config, error) {
	// 默认配置
	conf := &Config{
		Generator: GeneratorConfig{
			Package:            "resolver",
			DefaultContentType: "json",
			ContentTypeAliases: map[string]string{
				"json":      "application/json",
				"form":      "application/x-www-form-urlencoded",
				"multipart": "multipart/form-data",
			},
			StructTags: []TagConfig{
				{Name: "json", Case: "lower"},
				{Name: "form", Case: "lower"},
			},
			DefaultOkStatus: 200,
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// 如果文件不存在，返回默认配置
		if os.IsNotExist(err) {
			return conf, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, err
	}

	return conf, nil
}
