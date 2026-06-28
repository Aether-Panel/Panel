package skypanel

type EnvironmentFactory interface {
	Create() EnvironmentImpl

	Key() string
}
