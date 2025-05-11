package gw1h

import (
	"os"
)

func WineEnv() []string {
	wineArch, ok := os.LookupEnv("GW1H_WINE_ARCH")
	if !ok {
		wineArch = "win64"
	}

	winePrefix, ok := os.LookupEnv("GW1H_WINE_PREFIX")
	if !ok {
		winePrefix = os.Getenv("PWD")
	}

	env := os.Environ()
	env = append(env, "WINEARCH="+wineArch)
	env = append(env, "WINEPREFIX="+winePrefix)
	return env
}
