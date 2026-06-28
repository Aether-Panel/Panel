package SkyPanel

type EnvironmentFactory interface {
	Create() EnvironmentImpl

	Key() string
}
