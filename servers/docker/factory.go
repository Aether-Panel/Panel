package docker

import (
	"github.com/SkyPanel/SkyPanel/v3"
)

type EnvironmentFactory struct {
	SkyPanel.EnvironmentFactory
}

func (ef EnvironmentFactory) Create() SkyPanel.EnvironmentImpl {
	return &Docker{
		// pufferpanel/generic es la imagen oficial válida (todo minúsculas requerido por Docker)
		// La imagen anterior "SkyPanel/generic" era inválida: Docker interpreta mayúsculas
		// como un registro privado y falla al resolver el DNS.
		ImageName: "pufferpanel/generic",
		// "bridge" permite el port-mapping correcto (0.0.0.0:PUERTO->PUERTO/tcp)
		// "host" comparte la red del host directamente, lo que rompe el mapping en VPS
		Network:   "bridge",
		Ports:     make([]string, 0),
		Binds:     make(map[string]string),
		Labels:    make(map[string]string),
	}
}

func (ef EnvironmentFactory) Key() string {
	return "docker"
}
