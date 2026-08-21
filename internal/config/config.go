package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Generator GeneratorConfig         `yaml:"generator"`
	Scalars   map[string]ScalarConfig `yaml:"scalars"` // 新增：标量映射配置
}

type ScalarConfig struct {
	Model  string `yaml:"model"`  // Go 类型路径，如 "time.Time" 或 "pkg.IntTime"
	Target string `yaml:"target"` // 目标业务 Go 类型路径，如 "time.Time"
}

// ContentTypeSpec 协议规格全元数据模型
type ContentTypeSpec struct {
	MIME        string `yaml:"mime"`         // MIME 类型: application/json
	Tag         string `yaml:"tag"`          // 结构体 tag 名称: json, form, xml 等 (默认同 key)
	Case        string `yaml:"case"`         // 命名风格: snake, camel, lower, keep (缺省取 default_case)
	RawType     string `yaml:"raw_type"`     // 影子多态流类型: json.RawMessage, yaml.Node 等
	ImportPkg   string `yaml:"import_pkg"`   // 额外导包: encoding/json 等
	UnmarshalFn string `yaml:"unmarshal_fn"` // 反序列化函数: json.Unmarshal 等
}

// UnmarshalYAML 兼容器：支持简单字符串简写 (如 text: "text/plain") 和完整对象配置
func (s *ContentTypeSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		s.MIME = value.Value
		return nil
	}
	type rawSpec ContentTypeSpec
	var raw rawSpec
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*s = ContentTypeSpec(raw)
	return nil
}

type GeneratorConfig struct {
	Package            string                     `yaml:"package"`
	DefaultContentType string                     `yaml:"default_content_type"`
	DefaultCase        string                     `yaml:"default_case"` // snake (默认), camel, lower, keep
	ContentTypes       map[string]ContentTypeSpec `yaml:"content_types"`
	// 保留向后兼容字段（如有旧配置传入）
	ContentTypeAliases map[string]string          `yaml:"content_type_aliases"`
	StructTags         []TagConfig                `yaml:"struct_tags"`
	EnableApiDocs      bool                       `yaml:"enable_api_docs"`
	DocCase            string                     `yaml:"doc_case"` // snake, camel, lower, keep
	BaseURL            string                     `yaml:"base_url"`
	DefaultWrap        string                     `yaml:"default_wrap"`
	DefaultOkStatus    int                        `yaml:"default_ok_status"`
	ScalarStyle        string                     `yaml:"scalar_style"` // isolation (默认) | direct
	AuthDecorator      string                     `yaml:"auth_decorator"`
	AuthParamName      string                     `yaml:"auth_param_name"`
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
			DefaultCase:        "snake",
			ContentTypes: map[string]ContentTypeSpec{
				"json": {
					MIME:        "application/json",
					Tag:         "json",
					Case:        "snake",
					RawType:     "json.RawMessage",
					ImportPkg:   "encoding/json",
					UnmarshalFn: "json.Unmarshal",
				},
				"form": {
					MIME: "application/x-www-form-urlencoded",
					Tag:  "form",
					Case: "snake",
				},
				"multipart": {
					MIME: "multipart/form-data",
					Tag:  "form",
					Case: "snake",
				},
				"xml": {
					MIME:        "application/xml",
					Tag:         "xml",
					Case:        "camel",
					RawType:     "runtime.XMLRawNode",
					ImportPkg:   "github.com/xslasd/resgen/runtime",
					UnmarshalFn: "xml.Unmarshal",
				},
				"yaml": {
					MIME:        "application/x-yaml",
					Tag:         "yaml",
					Case:        "snake",
					RawType:     "yaml.Node",
					ImportPkg:   "gopkg.in/yaml.v3",
					UnmarshalFn: "yaml.Unmarshal",
				},
				"toml": {
					MIME:        "application/toml",
					Tag:         "toml",
					Case:        "snake",
					RawType:     "toml.Primitive",
					ImportPkg:   "github.com/BurntSushi/toml",
					UnmarshalFn: "toml.Unmarshal",
				},
				"text": {
					MIME: "text/plain",
				},
				"html": {
					MIME: "text/html",
				},
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

	// 自动补齐与兼容逻辑
	if conf.Generator.DefaultCase == "" {
		conf.Generator.DefaultCase = "snake"
	}
	if conf.Generator.ContentTypes == nil {
		conf.Generator.ContentTypes = make(map[string]ContentTypeSpec)
	}
	// 将旧版 ContentTypeAliases 自动合并进 ContentTypes
	for alias, mime := range conf.Generator.ContentTypeAliases {
		if spec, exists := conf.Generator.ContentTypes[alias]; exists {
			if spec.MIME == "" {
				spec.MIME = mime
				conf.Generator.ContentTypes[alias] = spec
			}
		} else {
			conf.Generator.ContentTypes[alias] = ContentTypeSpec{
				MIME: mime,
				Tag:  alias,
				Case: conf.Generator.DefaultCase,
			}
		}
	}
	// 补全各个 ContentTypeSpec 的默认 case
	for key, spec := range conf.Generator.ContentTypes {
		if spec.Case == "" {
			spec.Case = conf.Generator.DefaultCase
		}
		// 仅对结构体序列化协议或有编解码器声明的类型自动赋予 Tag
		if spec.Tag == "" {
			switch key {
			case "json", "form", "multipart", "xml", "yaml", "toml", "msgpack":
				spec.Tag = key
				if key == "multipart" {
					spec.Tag = "form"
				}
			}
		}
		conf.Generator.ContentTypes[key] = spec
	}

	return conf, nil
}
