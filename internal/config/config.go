package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Generator GeneratorConfig         `yaml:"generator"`
	Scalars   map[string]ScalarConfig `yaml:"scalars"`  // 新增：标量映射配置
}

type ScalarConfig struct {
	Model  string `yaml:"model"`  // Go 类型路径，如 "time.Time" 或 "pkg.IntTime"
	Target string `yaml:"target"` // 目标业务 Go 类型路径，如 "time.Time"
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
	ScalarStyle        string            `yaml:"scalar_style"` // isolation (默认) | direct
	AuthDecorator      string            `yaml:"auth_decorator"`
	AuthParamName      string            `yaml:"auth_param_name"`
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
			ScalarStyle:     "isolation",
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
