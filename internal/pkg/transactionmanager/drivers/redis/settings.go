package redistx

type Settings struct {
	watchKeys []string
}

func NewSettings(opts ...Option) *Settings {
	s := Settings{}

	for _, opt := range opts {
		opt(&s)
	}

	return &s
}

func (s *Settings) WatchingKeys() []string {
	return s.watchKeys
}

type Option func(s *Settings) error

func WithWatchKeys(keys ...string) Option {
	return func(s *Settings) error {
		s.watchKeys = keys

		return nil
	}
}
