package viperx

import (
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var lock sync.RWMutex

func IsSet(key string) bool {
	return rCall(key, viper.IsSet)
}

func Get(key string) interface{} {
	return rCall(key, viper.Get)
}

func GetBool(key string) bool {
	return rCall(key, viper.GetBool)
}

func GetInt(key string) int {
	return rCall(key, viper.GetInt)
}

func GetInt32(key string) int32 {
	return rCall(key, viper.GetInt32)
}

func GetString(key string) string {
	return rCall(key, viper.GetString)
}

func GetStringSlice(key string) []string {
	return rCall(key, viper.GetStringSlice)
}

func GetDuration(key string) time.Duration {
	return rCall(key, viper.GetDuration)
}

func AllSettings() map[string]interface{} {
	lock.RLock()
	defer lock.RUnlock()
	return viper.AllSettings()
}

func rCall[T interface{}](key string, f func(string) T) T {
	lock.RLock()
	defer lock.RUnlock()
	return f(key)
}

func Set(key string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	viper.Set(key, value)
}

func SetDefault(key string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	viper.SetDefault(key, value)
}

func MergeConfigMap(cfg map[string]interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	return viper.MergeConfigMap(cfg)
}

func AutomaticEnv() {
	viper.AutomaticEnv()
}

func BindEnv(input ...string) error {
	return viper.BindEnv(input...)
}

func SetEnvPrefix(in string) {
	viper.SetEnvPrefix(in)
}

func SetEnvKeyReplacer(r *strings.Replacer) {
	viper.SetEnvKeyReplacer(r)
}

func SetConfigName(in string) {
	viper.SetConfigName(in)
}

func SetConfigType(in string) {
	viper.SetConfigType(in)
}

func AddConfigPath(in string) {
	viper.AddConfigPath(in)
}

func SetConfigFile(in string) {
	viper.SetConfigFile(in)
}

func BindPFlags(flags *pflag.FlagSet) error {
	return viper.BindPFlags(flags)
}

func ReadInConfig() error {
	return viper.ReadInConfig()
}

func ConfigFileUsed() string {
	return viper.ConfigFileUsed()
}

func OnConfigChange(run func(in fsnotify.Event)) {
	viper.OnConfigChange(run)
}

func GetViper() *viper.Viper {
	return viper.GetViper()
}

func WatchConfig() {
	viper.WatchConfig()
}

func Reset() {
	viper.Reset()
}
