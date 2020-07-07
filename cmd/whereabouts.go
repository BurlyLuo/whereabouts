package main

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/dougbtv/whereabouts/pkg/allocate"
	"github.com/dougbtv/whereabouts/pkg/config"
	"github.com/dougbtv/whereabouts/pkg/logging"
	"github.com/dougbtv/whereabouts/pkg/storage"
	"github.com/dougbtv/whereabouts/pkg/types"
	"net"
)

func main() {
	// TODO: implement plugin version
	skel.PluginMain(cmdAdd, cmdGet, cmdDel, version.All, "TODO")
}

func cmdGet(args *skel.CmdArgs) error {
	// TODO
	return fmt.Errorf("CNI GET method is not implemented")
}

func cmdAdd(args *skel.CmdArgs) error {
	ipamConf, confVersion, err := config.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		logging.Errorf("IPAM configuration load failed: %s", err)
		return err
	}
	logging.Debugf("ADD - IPAM configuration successfully read: %+v", filterConf(*ipamConf))

	logging.Debugf("validateIfName: %s, %s", args.Netns, args.IfName)
	podNs, err := ns.GetNS(args.Netns)
	if err != nil {
		return logging.Errorf("validateIfName: no net namespace %s found: %v", args.Netns, err)
	}

	var hardwareaddr string

	_ = podNs.Do(func(_ ns.NetNS) error {
		netInterface, err := net.InterfaceByName(args.IfName)
		if err != nil {
			logging.Errorf("error getting interface: %s", err)
			return err
		}
		hardwareaddr = fmt.Sprintf("%s", netInterface.HardwareAddr)
		return nil
	})

	logging.Debugf("Using mac address: %s", hardwareaddr)

	// Initialize our result, and assign DNS & routing.
	result := &current.Result{}
	result.DNS = ipamConf.DNS
	result.Routes = ipamConf.Routes

	logging.Debugf("Beginning IPAM for ContainerID: %v", args.ContainerID)
	newip, err := storage.IPManagement(types.Allocate, *ipamConf, args.ContainerID)
	if err != nil {
		logging.Errorf("Error assigning IP: %s", err)
		return fmt.Errorf("Error assigning IP: %w", err)
	}

	// Determine if v4 or v6.
	var useVersion string
	if allocate.IsIPv4(newip.IP) {
		useVersion = "4"
	} else {
		useVersion = "6"
	}

	result.IPs = append(result.IPs, &current.IPConfig{
		Version: useVersion,
		Address: newip,
		Gateway: ipamConf.Gateway})

	// Assign all the static IP elements.
	for _, v := range ipamConf.Addresses {
		result.IPs = append(result.IPs, &current.IPConfig{
			Version: v.Version,
			Address: v.Address,
			Gateway: v.Gateway})
	}

	return cnitypes.PrintResult(result, confVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	ipamConf, _, err := config.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		logging.Errorf("IPAM configuration load failed: %s", err)
		return err
	}
	logging.Debugf("DEL - IPAM configuration successfully read: %+v", filterConf(*ipamConf))
	logging.Debugf("ContainerID: %v", args.ContainerID)

	_, err = storage.IPManagement(types.Deallocate, *ipamConf, args.ContainerID)
	if err != nil {
		logging.Errorf("Error deallocating IP: %s", err)
		return fmt.Errorf("Error deallocating IP: %s", err)
	}

	return nil
}

func filterConf(conf types.IPAMConfig) types.IPAMConfig {
	new := conf
	new.EtcdPassword = "*********"
	return new
}
