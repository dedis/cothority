# MiniNet-implementation at EPFL

## How to run it

0. Get your public ssh-key added by the sysadmins 
1. Ask for the machines at icsil1-sysadmins
	between 10 and 100 machines
2. Connect to icsil1-conodes as mininet
	iscisl1-conodes-exp - already on the simulated network
3. update which servers are up
    ./icsil1_search_server.py icsil1.servers.json
3. cd conodes; python dispatched.py
4. cd conodes/sites/icsil1; python cli.py
	sync - copy all configuration to all machines
	findip - searches which IP is on which server
	pingservers - contacts all servers
	ping nodes - pings all nodes using nmap - perhaps not all nodes are recognized 
	console - goes to the virtual machine - like ssh
	mininet - goes to the mininet console
		net - shows which processes are up
		pingall - contacts all hosts
		ipperf - tests the bandwith
		<ctrl-b>d - detaches
	start - launches the mininet and starts the experiment
5. on the head, type
	tail -f /tmp/stdout_all_nodes.txt
	dsh -g sites/icsil1/servers — uname -a
6. on the experiment, use
	dsh -g sites/icsil1/nodes — uname -a
	also look into .dsh/sites/icsil1
		.dsh/dsh.conf - forklift = 8 - sends only to 8 machines at once, not more
7. on head in ˜
	./icsil1_reboot_server.py icsil1.servers.json

Mininet

Namespace - for network, processes and disk

Default namespace which binds the process. Processes can be in any namespace, even different for different parts (network, processes and disk)

When adding a new namespace, it’s just like a switch, but with a routing-table.

MiniNet helps to build the setup. It is usually only done for one machine. The extension only works for about 100 to 1000 machines.

MA wrote a Python-script that sets up MiniNet for all machines which will run it on all machines.

You cannot use containers, because the traffic from the emulation still blocks part of the kernel, so the machines would have to be rebooted.

A Virtual machine will be run on the Physical machines, and one machine will be called the ‘head’, who has to check which machines are up.

Each machine will have two IP-addresses - one for internal communication and one for the simulation.

LBF - Lab Bryan Ford

Machines are Linux/64-bit/Ubuntu

In the conodes-directory, there is
sites/ - a collection of clusters, where each machine has direct access to each other
	icsil1/ - has all available machines
		*tmpl are the template-files that are used, written in Cheetah (http://www.cheetahtemplate.org/)
			default-mininet.tmpl - holds the configuration for each virtual machine
				Experiment
					expStart / expStop - starts and stops the experiment
				run - starts the experiment
				start_logging_server - listens for all incoming experiments
			cli.tmpl - command-line interface to launch the experiments
			cli.py - handle all machines

./dispatched.py - generates all configuration-files, depending on the virtual machine configurations
	Dispatcher.nb_of_nodes returns the number of nodes for each machine
		nb_of_nodes_per_GB - 
	Experiment.genConodeConfig - tells the different nodes on how to set up

1 Conode-image which is deployed to all machines

Bandwidth-Restrictions, delays and loss use a lot of resources.

There is no swapping enabled. But there must be some kind of boost.
