package ruadan

import (
	"encoding"
	"errors"
	"flag"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"
)

// ErrInvalidConfig is the default error message if you don't pass the cfg argument as a struct pointer to
// GetConfigFlagSet
var ErrInvalidConfig = errors.New("cfg must be a struct pointer")

// ConfigurationOption is the extensible struct used to build up a struct field that will be returned as
// Configuration.Config
type ConfigurationOption struct {
	name         string
	envName      string
	cliName      string
	jsonName     string
	usage        string
	defaultValue interface{}
	useCLI       bool
}

// Decoder interface to decode a string
type Decoder interface {
	Decode(value string) error
}

// Setter interface to set a string value
type Setter interface {
	Set(value string) error
}

// ConfigurationOptions function used to build the individual ConfigurationOption field
type ConfigurationOptions func(*ConfigurationOption)

// Configuration is returned by BuildConfig as an unknown struct to read valued from after initial creation
type Configuration struct {
	Config interface{}
}

// GetBool gets a boolean value from the key that matches the provided name in the Configuration
func (c *Configuration) GetBool(name string) bool {
	return reflect.ValueOf(c.Config).Elem().FieldByName(name).Bool()
}

// GetString gets a string value from the key that matches the provided name in the Configuration
func (c *Configuration) GetString(name string) string {
	return reflect.ValueOf(c.Config).Elem().FieldByName(name).String()
}

// GetInt64 gets a int64 value from the key that matches the provided name in the Configuration
func (c *Configuration) GetInt64(name string) int64 {
	return reflect.ValueOf(c.Config).Elem().FieldByName(name).Int()
}

// GetFloat64 gets a float64 value from the key that matches the provided name in the Configuration
func (c *Configuration) GetFloat64(name string) float64 {
	return reflect.ValueOf(c.Config).Elem().FieldByName(name).Float()
}

// GetComplex gets an interface value from the key that matches the provided name in the Configuration.
// This assumes you know what you're asking for and how to cast the result
func (c *Configuration) GetComplex(name string) interface{} {
	return reflect.ValueOf(c.Config).Elem().FieldByName(name).Interface()
}

// OptionJSONName used to add a json: tag to a struct field
func OptionJSONName(name string) ConfigurationOptions {
	return func(o *ConfigurationOption) { o.jsonName = jsonify(name) }
}

// OptionENVName used to add a envconfig: tag to a struct field
func OptionENVName(name string) ConfigurationOptions {
	return func(o *ConfigurationOption) { o.envName = envify(name) }
}

// OptionCLIName used to add a envcli: tag to a struct field; will also assume that there is a cli flag
func OptionCLIName(name string) ConfigurationOptions {
	return func(o *ConfigurationOption) {
		o.cliName = snakify(name)
		o.useCLI = true
	}
}

// OptionCLIUsage used to add a clidesc: tag to a struct field; will also assume that there is a cli flag
func OptionCLIUsage(usage string) ConfigurationOptions {
	return func(o *ConfigurationOption) {
		o.usage = usage
		o.useCLI = true
	}
}

// NewOptionInt creates a new int64 struct field with the given name and options. When considering the name, remember
// Go's syntax of an upper-case first letter
func NewOptionInt(name string, options ...ConfigurationOptions) ConfigurationOption {
	return newOption(name, int64(0), options...)
}

// NewOptionString creates a new string struct field with the given name and options. When considering the name,
// remember Go's syntax of an upper-case first letter
func NewOptionString(name string, options ...ConfigurationOptions) ConfigurationOption {
	return newOption(name, "", options...)
}

// NewOptionBool creates a new bool struct field with the given name and options. When considering the name, remember
// Go's syntax of an upper-case first letter
func NewOptionBool(name string, options ...ConfigurationOptions) ConfigurationOption {
	return newOption(name, false, options...)
}

// NewOptionFloat creates a new float64 struct field with the given name and options. When considering the name,
// remember Go's syntax of an upper-case first letter
func NewOptionFloat(name string, options ...ConfigurationOptions) ConfigurationOption {
	return newOption(name, float64(0), options...)
}

// NewOptionComplex creates a new interface{} struct field with the given name and options. When considering the name,
// remember Go's syntax of an upper-case first letter
func NewOptionComplex(name string, defaultValue interface{}, options ...ConfigurationOptions) ConfigurationOption {
	return newOption(name, defaultValue, options...)
}

// GetConfigFlagSet takes in the args from the cli and a struct pointer to the struct it will parse. It will look at
// the tags to determine what keys and areas to look for. The base use case is that you can pass a struct pointer and
// it will use the envconfig: tag to find the matching environment variable and that can be overridden at launch with a
// command line flag. The flag will be the same as the envconfig: if not specified, or can be changed with the
// envcli: tag
func GetConfigFlagSet(args []string, cfg interface{}) (*flag.FlagSet, error) {
	metas, err := reflectConfig("", cfg)
	if err != nil {
		return nil, err
	}

	fs := flag.NewFlagSet("config", flag.ExitOnError)
	for _, meta := range metas {
		err = parseMeta(fs, meta)
		if err != nil {
			return nil, err
		}
	}

	err = fs.Parse(args)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// BuildConfig takes a variable amount of ConfigurationOption arguments and uses them to build a struct. This allows
// you to be very specific in how to build the struct if you don't want to have a struct at the top of your file and
// want to build it as you go
func BuildConfig(options ...ConfigurationOption) Configuration {
	fields := []reflect.StructField{}
	for _, o := range options {
		switch o.defaultValue.(type) {
		case bool:
			dv := lookupEnvOrBool(o.envName, o.defaultValue.(bool))
			if o.useCLI {
				flag.Bool(o.cliName, dv, o.usage)
			}
		case int64:
			dv := lookupEnvOrInt64(o.envName, o.defaultValue.(int64))
			if o.useCLI {
				flag.Int64(o.cliName, dv, o.usage)
			}
		case float64:
			dv := lookupEnvOrFloat64(o.envName, o.defaultValue.(float64))
			if o.useCLI {
				flag.Float64(o.cliName, dv, o.usage)
			}
		default:
			dv := lookupEnvOrString(o.envName, o.defaultValue.(string))
			if o.useCLI {
				flag.String(o.cliName, dv, o.usage)
			}
		}
		fields = append(fields, reflect.StructField{
			Name: o.name,
			Type: reflect.TypeOf(o.defaultValue),
			Tag:  tags(o),
		})
	}

	obj := reflect.StructOf(fields)
	return Configuration{Config: reflect.New(obj).Interface()}
}

func newOption(name string, dv interface{}, options ...ConfigurationOptions) ConfigurationOption {
	opt := &ConfigurationOption{
		name:     name,
		envName:  envify(name),
		jsonName: jsonify(name),
		useCLI:   true,
		cliName:  snakify(name),
	}

	if dv != nil {
		opt.defaultValue = dv
	}

	for _, o := range options {
		o(opt)
	}

	if opt.useCLI && opt.usage == "" {
		opt.usage = opt.name
	}

	return *opt
}

func parseMeta(fs *flag.FlagSet, meta fieldMeta) error {
	field := meta.Field
	if field.Type().Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	switch field.Kind() {
	case reflect.Bool:
		v := (*bool)(unsafe.Pointer(field.UnsafeAddr()))
		fs.BoolVar(v, tagCLI(meta), lookupEnvOrBool(tagENV(meta), false), tagDesc(meta))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := (*int64)(unsafe.Pointer(field.UnsafeAddr()))
		if meta.Field.Kind() == reflect.Int64 &&
			field.Type().PkgPath() == "time" &&
			field.Type().Name() == "Duration" {
			fs.Int64Var(v, tagCLI(meta), lookupEnvOrDuration(tagENV(meta), int64(0)), tagDesc(meta))
		} else {
			fs.Int64Var(v, tagCLI(meta), lookupEnvOrInt64(tagENV(meta), int64(0)), tagDesc(meta))
		}
	case reflect.Uint8:
		v := (*uint)(unsafe.Pointer(field.UnsafeAddr()))
		fs.UintVar(v, tagCLI(meta), lookupEnvOrUint8(tagENV(meta), uint8(0)), tagDesc(meta))
	case reflect.Uint16:
		v := (*uint)(unsafe.Pointer(field.UnsafeAddr()))
		fs.UintVar(v, tagCLI(meta), lookupEnvOrUint16(tagENV(meta), uint16(0)), tagDesc(meta))
	case reflect.Uint32:
		v := (*uint)(unsafe.Pointer(field.UnsafeAddr()))
		fs.UintVar(v, tagCLI(meta), lookupEnvOrUint32(tagENV(meta), uint32(0)), tagDesc(meta))
		field.SetUint(uint64(*v))
	case reflect.Uint64, reflect.Uint:
		v := (*uint)(unsafe.Pointer(field.UnsafeAddr()))
		fs.UintVar(v, tagCLI(meta), lookupEnvOrUint64(tagENV(meta), uint64(0)), tagDesc(meta))
	case reflect.Float32:
		v := (*float64)(unsafe.Pointer(field.UnsafeAddr()))
		fs.Float64Var(v, tagCLI(meta), lookupEnvOrFloat32(tagENV(meta), float32(0)), tagDesc(meta))
	case reflect.Float64:
		v := (*float64)(unsafe.Pointer(field.UnsafeAddr()))
		fs.Float64Var(v, tagCLI(meta), lookupEnvOrFloat64(tagENV(meta), float64(0)), tagDesc(meta))
	case reflect.String:
		v := (*string)(unsafe.Pointer(field.UnsafeAddr()))
		fs.StringVar(v, tagCLI(meta), lookupEnvOrString(tagENV(meta), ""), tagDesc(meta))
	case reflect.Slice:
		v := (*string)(unsafe.Pointer(field.UnsafeAddr()))
		fs.StringVar(v, tagCLI(meta), lookupEnvOrString(tagENV(meta), ""), tagDesc(meta))
		s := reflect.MakeSlice(field.Type(), 0, 0)
		switch {
		case field.Type().Kind() == reflect.Uint8:
			s = reflect.ValueOf([]byte(*v))
		case len(strings.TrimSpace(*v)) != 0:
			vs := strings.Split(*v, ",")
			s = reflect.MakeSlice(field.Type(), len(vs), len(vs))
			for i, val := range vs {
				err := parseValue(val, s.Index(i))
				if err != nil {
					return err
				}
			}
		}
		field.Set(s)
	}

	return nil
}

func parseValue(v string, field reflect.Value) error {
	decoder := parseDecoder(field)
	if decoder != nil {
		return decoder.Decode(v)
	}

	setter := parseSetter(field)
	if setter != nil {
		return setter.Set(v)
	}

	if t := textUnmarshaler(field); t != nil {
		return t.UnmarshalText([]byte(v))
	}

	if b := binaryUnmarshaler(field); b != nil {
		return b.UnmarshalBinary([]byte(v))
	}

	if field.Type().Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	switch field.Type().Kind() {
	case reflect.Bool:
		val, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		field.SetBool(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var val int64
		var err error
		if field.Kind() == reflect.Int64 &&
			field.Type().PkgPath() == "time" &&
			field.Type().Name() == "Duration" {
			var d time.Duration
			d, err = time.ParseDuration(v)
			val = int64(d)
		} else {
			val, err = strconv.ParseInt(v, 0, field.Type().Bits())
		}
		if err != nil {
			return err
		}

		field.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(v, 0, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(v, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(val)
	case reflect.String:
		field.SetString(v)
	}

	return nil
}

func tagCLI(meta fieldMeta) string {
	switch {
	case meta.AltCLI != "":
		return meta.AltCLI
	case meta.AltJSON != "":
		return meta.AltJSON
	case meta.AltENV != "":
		return meta.AltENV
	default:
		return meta.Key
	}
}

func tagENV(meta fieldMeta) string {
	switch {
	case meta.AltENV != "":
		return meta.AltENV
	case meta.AltCLI != "":
		return strings.ToUpper(meta.AltCLI)
	case meta.AltJSON != "":
		return strings.ToUpper(meta.AltJSON)
	default:
		return strings.ToUpper(meta.Key)
	}
}

func tagDesc(meta fieldMeta) string {
	switch {
	case meta.DescCLI != "":
		return meta.DescCLI
	default:
		return "flag: " + tagCLI(meta) + " or env: " + tagENV(meta)
	}
}

func lookupEnvOrString(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func lookupEnvOrInt64(key string, defaultVal int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return int64(0)
		}
		return v
	}
	return defaultVal
}

func lookupEnvOrUint8(key string, defaultVal uint8) uint {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseUint(val, 10, 8)
		if err != nil {
			return uint(0)
		}
		return uint(v)
	}
	return uint(defaultVal)
}

func lookupEnvOrUint16(key string, defaultVal uint16) uint {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return uint(0)
		}
		return uint(v)
	}
	return uint(defaultVal)
}

func lookupEnvOrUint32(key string, defaultVal uint32) uint {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return uint(0)
		}
		return uint(v)
	}
	return uint(defaultVal)
}

func lookupEnvOrUint64(key string, defaultVal uint64) uint {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return uint(0)
		}
		return uint(v)
	}
	return uint(defaultVal)
}

func lookupEnvOrDuration(key string, defaultVal int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		v, err := time.ParseDuration(val)
		if err != nil {
			return int64(0)
		}
		return int64(v)
	}
	return defaultVal
}

func lookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseBool(val)
		if err != nil {
			return false
		}
		return v
	}
	return defaultVal
}

func lookupEnvOrFloat32(key string, defaultVal float32) float64 {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return float64(0)
		}
		return float64(v)
	}
	return float64(defaultVal)
}

func lookupEnvOrFloat64(key string, defaultVal float64) float64 {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return float64(0)
		}
		return v
	}
	return defaultVal
}

type fieldMeta struct {
	Name    string
	AltENV  string
	AltCLI  string
	AltJSON string
	DescCLI string
	Key     string
	Field   reflect.Value
	Tags    reflect.StructTag
}

func parseInterface(v reflect.Value, fn func(interface{}, *bool)) {
	if !v.CanInterface() {
		return
	}

	var ok bool
	fn(v.Interface(), &ok)
	if !ok && v.CanAddr() {
		fn(v.Addr().Interface(), &ok)
	}
}

func parseDecoder(field reflect.Value) Decoder {
	var d Decoder
	parseInterface(field, func(v interface{}, ok *bool) { d, *ok = v.(Decoder) })
	return d
}

func parseSetter(field reflect.Value) Setter {
	var s Setter
	parseInterface(field, func(v interface{}, ok *bool) { s, *ok = v.(Setter) })
	return s
}

func textUnmarshaler(field reflect.Value) encoding.TextUnmarshaler {
	var t encoding.TextUnmarshaler
	parseInterface(field, func(v interface{}, ok *bool) { t, *ok = v.(encoding.TextUnmarshaler) })
	return t
}

func binaryUnmarshaler(field reflect.Value) encoding.BinaryUnmarshaler {
	var b encoding.BinaryUnmarshaler
	parseInterface(field, func(v interface{}, ok *bool) { b, *ok = v.(encoding.BinaryUnmarshaler) })
	return b
}

func reflectConfig(prefix string, cfg interface{}) ([]fieldMeta, error) {
	c := reflect.ValueOf(cfg)
	if c.Kind() != reflect.Ptr {
		return nil, ErrInvalidConfig
	}

	c = c.Elem()
	if c.Kind() != reflect.Struct {
		return nil, ErrInvalidConfig
	}

	ct := c.Type()
	metas := make([]fieldMeta, 0, c.NumField())
	for i := 0; i < c.NumField(); i++ {
		f := c.Field(i)
		ft := ct.Field(i)

		if !f.CanSet() {
			continue
		}

		for f.Kind() == reflect.Ptr {
			if f.IsNil() {
				if f.Type().Elem().Kind() != reflect.Struct {
					break
				}

				f.Set(reflect.New(f.Type().Elem()))
			}

			f = f.Elem()
		}

		meta := fieldMeta{
			Name:    ft.Name,
			Field:   f,
			Tags:    ft.Tag,
			AltCLI:  ft.Tag.Get("envcli"),
			AltENV:  strings.ToUpper(ft.Tag.Get("envconfig")),
			AltJSON: ft.Tag.Get("json"),
			DescCLI: ft.Tag.Get("clidesc"),
		}

		meta.Key = meta.Name

		if meta.AltENV != "" {
			meta.Key = meta.AltENV
		}
		meta.Key = strings.ToUpper(meta.Key)
		metas = append(metas, meta)

		if f.Kind() == reflect.Struct {
			if parseDecoder(f) == nil &&
				parseSetter(f) == nil &&
				textUnmarshaler(f) == nil &&
				binaryUnmarshaler(f) == nil {
				pre := ""
				if !ft.Anonymous {
					pre = meta.Key
				}

				embeddedPtr := f.Addr().Interface()
				embeddedMetas, err := reflectConfig(pre, embeddedPtr)
				if err != nil {
					return nil, err
				}
				metas = append(metas[:len(metas)-1], embeddedMetas...)
				continue
			}
		}
	}

	return metas, nil
}

func snakify(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}

func envify(s string) string {
	return strings.ToUpper(snakify(strings.TrimSpace(s)))
}

func jsonify(s string) string {
	str := strings.ToLower(snakify(strings.TrimSpace(s)))
	if !strings.ContainsAny(str, "_") {
		return str
	}

	formatted := []rune{}
	var pr rune
	for _, r := range str {
		switch {
		case pr == '_':
			formatted = append(formatted, unicode.ToUpper(r))
		case r != '_':
			formatted = append(formatted, r)
		}
		pr = r
	}
	return string(formatted)
}

func tags(o ConfigurationOption) reflect.StructTag {
	tag := ""
	if o.jsonName != "" {
		tag += ` json:"` + o.jsonName + `" `
	}

	if o.envName != "" {
		tag += ` envconfig:"` + o.envName + `" `
	}

	if o.useCLI {
		tag += ` envcli:"` + o.cliName + `" clidesc:"` + o.usage + `"`
	}

	return reflect.StructTag(strings.TrimSpace(tag))
}
