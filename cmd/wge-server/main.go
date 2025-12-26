package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"wg-exchange/cmd"
	dbusclient "wg-exchange/cmd/wge-server/dbus_client"
	"wg-exchange/cmd/wge-server/processor"
	"wg-exchange/cmd/wge-server/server"
	"wg-exchange/cmd/wge-server/terminator"
	"wg-exchange/models"

	"github.com/BurntSushi/toml"
)

func main() {
	confFile := flag.String("conf", cmd.DefaultServerTomlName, "server toml conf file")
	enableDbus := flag.Bool("dbus", false, "enable dbus systemd management")
	version := flag.Bool("version", false, "version")
	flag.Parse()

	if *version {
		fmt.Fprint(os.Stderr, cmd.BuildVersionOutput("Wg-Exchange Server"))
		return
	}

	var wgeConf models.WGEServerConf

	if _, err := toml.DecodeFile(*confFile, &wgeConf); err != nil {
		log.Fatalln("invalid toml conf file", err)
	}

	dbusclient.DefaultSystemdManager.SetDbusSystemdManager(*enableDbus)
	store, err := processor.NewStore(wgeConf)
	if err != nil {
		log.Println("processor init failure...", err)
		return
	}

	if _, err := server.NewServer(wgeConf.Server, store); err != nil {
		log.Println("server init failure...", err)
		return
	}

	terminator.StartTerminator(wgeConf.Server.Ttl)
}
