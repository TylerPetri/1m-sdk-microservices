package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
)

// CurrentEnv reads APP_ENV with default "dev".
func CurrentEnv() AppEnv {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if v == "" {
		return EnvDev
	}
	switch AppEnv(v) {
	case EnvDev, EnvStaging, EnvProd:
		return AppEnv(v)
	default:
		// Preserve unknown values so service validation rejects them.
		return AppEnv(v)
	}
}

// Load decodes environment variables into a config struct T using struct tags.
//
// Supported tags on struct fields:
//   - env:"NAME"           (env var name)
//   - default:"VALUE"      (default if unset/empty)
//   - required:"true"      (must be set in all envs)
//   - required_in:"a,b,c"  (must be set when APP_ENV is one of these)
//   - redact:"true"        (LogEffective will redact this value)
//
// Types supported:
//   - string, bool, int, int64, float64
//   - time.Duration
//
// Nested structs are supported.
// If T implements interface{ Validate() error }, Validate() is called after decoding.
func Load[T any]() (T, error) {
	var zero T

	// We require a struct or pointer-to-struct for T.
	rv := reflect.ValueOf(&zero).Elem()
	rt := rv.Type()
	if rt.Kind() == reflect.Pointer {
		// If user uses *Config as T, allocate it.
		if rt.Elem().Kind() != reflect.Struct {
			return zero, fmt.Errorf("%w: T must be a struct or pointer-to-struct", ErrInvalidConfig)
		}
		rv.Set(reflect.New(rt.Elem()))
		rt = rt.Elem()
		rv = rv.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return zero, fmt.Errorf("%w: T must be a struct or pointer-to-struct", ErrInvalidConfig)
	}

	env := CurrentEnv()
	if err := decodeStruct(rv, env); err != nil {
		return zero, err
	}

	// Run Validate() if present.
	if v, ok := any(rv.Interface()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return zero, err
		}
	} else if v, ok := any(zero).(interface{ Validate() error }); ok {
		// For non-pointer T
		if err := v.Validate(); err != nil {
			return zero, err
		}
	}

	return rv.Interface().(T), nil
}

func MustLoad[T any]() T {
	cfg, err := Load[T]()
	if err != nil {
		panic(err)
	}
	return cfg
}

func decodeStruct(rv reflect.Value, env AppEnv) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		fv := rv.Field(i)

		// Recurse into nested structs (except time.Duration which is a struct).
		if fv.Kind() == reflect.Struct && sf.Type != reflect.TypeOf(time.Duration(0)) {
			if err := decodeStruct(fv, env); err != nil {
				return err
			}
			continue
		}

		envKey := strings.TrimSpace(sf.Tag.Get("env"))
		if envKey == "" {
			// No env tag; leave default zero value (or field initializer if any).
			continue
		}

		raw, isSet := os.LookupEnv(envKey)
		raw = strings.TrimSpace(raw)

		def := sf.Tag.Get("default")
		required := strings.EqualFold(sf.Tag.Get("required"), "true")
		requiredIn := parseCSVSet(sf.Tag.Get("required_in"))

		need := required || requiredIn[string(env)]
		if need && (!isSet || raw == "") {
			return fmt.Errorf("%w: %s is required", ErrInvalidConfig, envKey)
		}

		val := raw
		if !isSet || val == "" {
			if def != "" {
				val = def
			} else if need {
				return fmt.Errorf("%w: %s is required", ErrInvalidConfig, envKey)
			} else {
				continue
			}
		}

		if err := setFromString(fv, sf.Type, val, envKey); err != nil {
			return err
		}
	}
	return nil
}

func parseCSVSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		m[p] = true
	}
	return m
}

func setFromString(fv reflect.Value, ft reflect.Type, raw string, envKey string) error {
	switch ft {
	case reflect.TypeOf(time.Duration(0)):
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("%w: %s invalid duration %q: %v", ErrInvalidConfig, envKey, raw, err)
		}
		fv.SetInt(int64(d))
		return nil
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("%w: %s invalid bool %q: %v", ErrInvalidConfig, envKey, raw, err)
		}
		fv.SetBool(b)
		return nil
	case reflect.Int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%w: %s invalid int %q: %v", ErrInvalidConfig, envKey, raw, err)
		}
		fv.SetInt(int64(n))
		return nil
	case reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s invalid int64 %q: %v", ErrInvalidConfig, envKey, raw, err)
		}
		fv.SetInt(n)
		return nil
	case reflect.Float64:
		x, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("%w: %s invalid float %q: %v", ErrInvalidConfig, envKey, raw, err)
		}
		fv.SetFloat(x)
		return nil
	default:
		return fmt.Errorf("%w: %s unsupported type %s", ErrInvalidConfig, envKey, ft.String())
	}
}

// LogEffective logs effective config as dotted keys, applying redact:"true" tags.
func LogEffective(log *zap.Logger, cfg any) {
	if log == nil {
		return
	}
	m := RedactedMap(cfg)
	// stable order for readability
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make([]zap.Field, 0, len(keys))
	for _, k := range keys {
		fields = append(fields, zap.String(k, m[k]))
	}
	log.Info("effective_config", fields...)
}

// RedactedMap returns a flattened dotted-key map of cfg with redact:"true" applied.
func RedactedMap(cfg any) map[string]string {
	out := map[string]string{}
	if cfg == nil {
		return out
	}
	rv := reflect.ValueOf(cfg)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return out
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return out
	}
	walkStruct(out, "", rv)
	return out
}

func walkStruct(out map[string]string, prefix string, rv reflect.Value) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		fv := rv.Field(i)
		name := sf.Name
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}

		// Nested structs (except time.Duration)
		if fv.Kind() == reflect.Struct && sf.Type != reflect.TypeOf(time.Duration(0)) {
			walkStruct(out, key, fv)
			continue
		}

		if strings.EqualFold(sf.Tag.Get("redact"), "true") {
			out[key] = "<redacted>"
			continue
		}

		out[key] = fmt.Sprint(fv.Interface())
	}
}
