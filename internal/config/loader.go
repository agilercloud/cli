package config

// Loader is the subset of config access the CLI depends on. Production
// uses the OS-backed osLoader; tests can substitute a fake.
type Loader interface {
	Load() (*Config, error)
	Save(*Config) error
	Get(key string) (string, error)
	Set(key, value string) error
	Path() string
}

// NewOSLoader returns a Loader that reads/writes the real filesystem.
func NewOSLoader(opts Options) Loader {
	return &osLoader{opts: opts}
}

type osLoader struct {
	opts Options
}

func (l *osLoader) Load() (*Config, error)            { return Load(l.opts) }
func (l *osLoader) Save(c *Config) error              { return Save(l.opts, c) }
func (l *osLoader) Get(key string) (string, error)    { return Get(l.opts, key) }
func (l *osLoader) Set(key, value string) error       { return Set(l.opts, key, value) }
func (l *osLoader) Path() string                      { return Path(l.opts) }
