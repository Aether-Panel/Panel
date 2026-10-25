package curseforge

import "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"

// InstallViaJar runs a Forge or NeoForge installer that sits in the server folder with
// --installServer, removes it afterwards and, for pre-1.17 Forge, renames the produced
// server jar to server.jar. The modpack installer uses it for Modrinth packs.
func InstallViaJar(server skypanel.DaemonServer, env *skypanel.Environment, jarFile, javaBinary string) error {
	return installViaJar(server, env, jarFile, javaBinary)
}
