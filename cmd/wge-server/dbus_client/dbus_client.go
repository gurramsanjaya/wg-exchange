// extremely volatile, will keep the service enabled if there is a failure somewhere
package dbusclient

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	wireguardServiceFormat = "wg-quick@%s.service"
	modeReplace            = "replace"
)

var (
	dbusManager *SystemdManager
)

func init() {
	dbusManager = &SystemdManager{}
}

// refer: https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html

/**
RestartUnit(in  s name,
			in  s mode,
			out o job);
*/

/**
EnableUnitFiles(in  as files,
                in  b runtime,
                in  b force,
                out b carries_install_info,
                out a(sss) changes);
*/

type Changes struct {
	TypeOfChange string
	FileName     string
	Destination  string
}

type CarriesInstallInfo bool

type SystemdManager struct {
	m          sync.Mutex
	conn       *dbus.Conn
	obj        dbus.BusObject
	enableDbus bool
}

func (d *SystemdManager) connect() (err error) {
	d.conn, err = dbus.ConnectSystemBus()
	d.obj = d.conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	return
}

func (d *SystemdManager) disconnect() error {
	return d.conn.Close()
}

func GetManager() *SystemdManager {
	return dbusManager
}

func (d *SystemdManager) SetDbusSystemdManager(enableInd bool) {
	d.m.Lock()
	defer d.m.Unlock()
	d.enableDbus = enableInd
}

func (d *SystemdManager) EnableAndStartService(intrfc string) error {
	d.m.Lock()
	defer d.m.Unlock()

	service := fmt.Sprintf(wireguardServiceFormat, intrfc)

	if d.enableDbus {
		if err := d.connect(); err != nil {
			return err
		}
		defer d.disconnect()

		var carriesInstallInfo CarriesInstallInfo
		var changes []Changes

		call := d.obj.Call("org.freedesktop.systemd1.Manager.EnableUnitFiles", 0, []string{service}, false, false)
		if call.Err != nil {
			return call.Err
		}
		log.Println(len(call.Body))

		if err := call.Store(&carriesInstallInfo, &changes); err != nil {
			return err
		}

		if len(changes) == 0 {
			log.Println("service is already previously enabled")
		} else {
			log.Println("service enabled - file:", changes[0].FileName, ", dest:", changes[0].Destination)

		}

		// wait for 2 secs,
		// TODO: refactor this using dbus systemd1 signals
		<-time.NewTimer(2 * time.Second).C
	} else {
		log.Println("simulating enable service:", service)
	}

	return nil
}

func (d *SystemdManager) RestartService(intrfc string) error {
	d.m.Lock()
	defer d.m.Unlock()

	service := fmt.Sprintf(wireguardServiceFormat, intrfc)

	if d.enableDbus {
		if err := d.connect(); err != nil {
			return err
		}
		defer d.disconnect()

		call := d.obj.Call("org.freedesktop.systemd1.Manager.RestartUnit", 0, service, modeReplace)
		if call.Err != nil {
			return call.Err
		}
		log.Println("successfully dispatched restart job")

		// wait for 2 secs,
		// TODO: refactor this using dbus systemd1 signals
		<-time.NewTimer(2 * time.Second).C
	} else {
		log.Println("simulating restart service:", service)
	}

	return nil

}
