package servers

// PluginPlatform is what the server loads plugins or mods for: "paper", "fabric", "forge" or
// "neoforge", worked out the same way as for the world map (modpack loader, "modlauncher"
// variable, then the install steps). It is empty when the panel cannot tell, such as on a
// vanilla server, a proxy or an unknown launcher.
func (p *Server) PluginPlatform() string {
	platform, _, _ := p.mapPlatform()
	return platform
}

// ValidJarName is true for a plain .jar file name with no folder in it, the same check the
// world map applies to the names Modrinth gives its files
func ValidJarName(name string) bool {
	return validJarName(name)
}
