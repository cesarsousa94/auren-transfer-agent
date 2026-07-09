// Package viper is a small local compatibility layer used by the v0.1.x
// foundation line so the project remains self-contained and compilable in
// offline build environments.
//
// It intentionally implements only the Viper surface used by the Agent today.
// Later deliveries can replace this local module with the upstream Viper module
// without changing the internal/config package contract.
package viper

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// ConfigFileNotFoundError mirrors the upstream Viper error used by callers to
// distinguish optional config discovery misses from hard read errors.
type ConfigFileNotFoundError struct {
	ConfigFile string
}

func (err ConfigFileNotFoundError) Error() string {
	if strings.TrimSpace(err.ConfigFile) == "" {
		return "config file not found"
	}
	return fmt.Sprintf("config file not found: %s", err.ConfigFile)
}

// Viper stores defaults, discovery rules and decoded configuration values.
type Viper struct {
	configType   string
	configFile   string
	configName   string
	paths        []string
	values       map[string]any
	envPrefix    string
	envReplacer  *strings.Replacer
	automaticEnv bool
}

// New returns an isolated configuration reader.
func New() *Viper {
	return &Viper{
		configType: "yaml",
		values:     make(map[string]any),
	}
}

// SetConfigType stores the expected config format. The v0.1.x layer supports
// yaml/yml because that is the official Agent configuration format.
func (v *Viper) SetConfigType(configType string) {
	v.configType = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(configType)), ".")
}

// SetConfigFile defines an explicit file path.
func (v *Viper) SetConfigFile(path string) {
	v.configFile = strings.TrimSpace(path)
}

// SetConfigName defines the base name searched in registered config paths.
func (v *Viper) SetConfigName(name string) {
	v.configName = strings.TrimSpace(name)
}

// AddConfigPath appends a directory to the search list.
func (v *Viper) AddConfigPath(path string) {
	if trimmed := strings.TrimSpace(path); trimmed != "" {
		v.paths = append(v.paths, trimmed)
	}
}

// SetEnvPrefix defines the prefix used when reading environment overrides.
func (v *Viper) SetEnvPrefix(prefix string) {
	v.envPrefix = strings.ToUpper(strings.TrimSpace(prefix))
}

// SetEnvKeyReplacer defines how dotted config keys are converted to env names.
func (v *Viper) SetEnvKeyReplacer(replacer *strings.Replacer) {
	v.envReplacer = replacer
}

// AutomaticEnv enables environment override lookup during Unmarshal.
func (v *Viper) AutomaticEnv() {
	v.automaticEnv = true
}

// SetDefault stores a dotted-key default value.
func (v *Viper) SetDefault(key string, value any) {
	setDotted(v.values, key, value)
}

// ReadInConfig discovers and reads the selected YAML configuration file.
func (v *Viper) ReadInConfig() error {
	path, err := v.resolvePath()
	if err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parsed, err := parseSimpleYAML(string(content))
	if err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}

	merge(v.values, parsed)
	return nil
}

// Unmarshal copies the loaded configuration into a struct using mapstructure
// tags when present, falling back to lower-cased field names.
func (v *Viper) Unmarshal(target any) error {
	if v.automaticEnv {
		v.applyEnvironmentOverrides(v.values, nil)
	}

	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	return fillStruct(value.Elem(), v.values)
}

func (v *Viper) resolvePath() (string, error) {
	if v.configFile != "" {
		if fileExists(v.configFile) {
			return v.configFile, nil
		}
		return "", ConfigFileNotFoundError{ConfigFile: v.configFile}
	}

	extensions := []string{v.configType}
	if v.configType == "yaml" {
		extensions = []string{"yaml", "yml"}
	}

	for _, dir := range v.paths {
		for _, ext := range extensions {
			candidate := filepath.Join(dir, v.configName+"."+ext)
			if fileExists(candidate) {
				return candidate, nil
			}
		}
	}

	return "", ConfigFileNotFoundError{ConfigFile: v.configName}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func setDotted(values map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	current := values
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if index == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
}

func merge(dst, src map[string]any) {
	for key, value := range src {
		if srcMap, ok := value.(map[string]any); ok {
			dstMap, ok := dst[key].(map[string]any)
			if !ok {
				dstMap = make(map[string]any)
				dst[key] = dstMap
			}
			merge(dstMap, srcMap)
			continue
		}
		dst[key] = value
	}
}

func parseSimpleYAML(content string) (map[string]any, error) {
	root := make(map[string]any)
	sections := map[int]map[string]any{-1: root}

	for lineNumber, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("line %d: lists are not supported in the v0.1.x config subset", lineNumber+1)
		}

		indent := countLeadingSpaces(line)
		if indent%2 != 0 {
			return nil, fmt.Errorf("line %d: indentation must use two-space levels", lineNumber+1)
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected key: value", lineNumber+1)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("line %d: key cannot be empty", lineNumber+1)
		}

		parentIndent := indent - 2
		if parentIndent < 0 {
			parentIndent = -1
		}
		parent, ok := sections[parentIndent]
		if !ok {
			return nil, fmt.Errorf("line %d: invalid indentation", lineNumber+1)
		}

		value := strings.TrimSpace(parts[1])
		if value == "" {
			child := make(map[string]any)
			parent[key] = child
			sections[indent] = child
			continue
		}

		parent[key] = parseScalar(value)
		deleteDeeperSections(sections, indent)
	}

	return root, nil
}

func countLeadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}

func deleteDeeperSections(sections map[int]map[string]any, indent int) {
	for level := range sections {
		if level > indent {
			delete(sections, level)
		}
	}
}

func parseScalar(value string) any {
	value = strings.Trim(value, `"'`)
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	return value
}

func (v *Viper) applyEnvironmentOverrides(values map[string]any, path []string) {
	for key, existing := range values {
		currentPath := append(append([]string{}, path...), key)
		if nested, ok := existing.(map[string]any); ok {
			v.applyEnvironmentOverrides(nested, currentPath)
			continue
		}

		if raw, ok := os.LookupEnv(v.envName(currentPath)); ok && raw != "" {
			values[key] = raw
		}
	}
}

func (v *Viper) envName(path []string) string {
	name := strings.Join(path, ".")
	if v.envReplacer != nil {
		name = v.envReplacer.Replace(name)
	} else {
		name = strings.ReplaceAll(name, ".", "_")
	}
	name = strings.ToUpper(name)
	if v.envPrefix == "" {
		return name
	}
	return v.envPrefix + "_" + name
}

func fillStruct(target reflect.Value, values map[string]any) error {
	if target.Kind() != reflect.Struct {
		return fmt.Errorf("target must point to a struct")
	}

	typeInfo := target.Type()
	for index := 0; index < target.NumField(); index++ {
		fieldValue := target.Field(index)
		fieldType := typeInfo.Field(index)
		if !fieldValue.CanSet() {
			continue
		}

		key := fieldType.Tag.Get("mapstructure")
		if key == "" {
			key = strings.ToLower(fieldType.Name)
		}

		raw, ok := values[key]
		if !ok {
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			nested, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("field %s expects object", fieldType.Name)
			}
			if err := fillStruct(fieldValue, nested); err != nil {
				return err
			}
			continue
		}

		if err := setField(fieldValue, raw); err != nil {
			return fmt.Errorf("field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

func setField(field reflect.Value, value any) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprint(value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch typed := value.(type) {
		case int:
			field.SetInt(int64(typed))
		case int64:
			field.SetInt(typed)
		case string:
			parsed, err := strconv.ParseInt(typed, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(parsed)
		default:
			return fmt.Errorf("unsupported integer value %T", value)
		}
	case reflect.Bool:
		switch typed := value.(type) {
		case bool:
			field.SetBool(typed)
		case string:
			parsed, err := strconv.ParseBool(typed)
			if err != nil {
				return err
			}
			field.SetBool(parsed)
		default:
			return fmt.Errorf("unsupported boolean value %T", value)
		}
	default:
		return fmt.Errorf("unsupported field kind %s", field.Kind())
	}

	return nil
}
