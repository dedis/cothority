#!/usr/bin/python

"""
This will run a number of hosts on the server and do all
the routing to being able to connect to the other mininets.

You have to give it a list of server/net/nbr for each server
that has mininet installed and what subnet should be run
on it.

It will create nbr+1 entries for each net, where the ".1" is the
router for the net, and ".2"..".nbr+1" will be the nodes.
"""

import sys, time, threading, os, datetime, contextlib, errno

from mininet.topo import Topo
from mininet.net import Mininet
from mininet.cli import CLI
from mininet.log import lg, setLogLevel
from mininet.node import Node, Host
from mininet.util import netParse, ipAdd, irange
from mininet.nodelib import NAT
from subprocess import Popen, PIPE, call

# The port used for socat
socatPort = 5000
# If this is true, only a dummy-function will be started on each mininet-node
cothorityDummy = False
# What debugging-level to use
debugging = 1
# Logging-file
logfile = "/tmp/mininet.log"
logsocat = "/tmp/socat.log"
logdone = "/tmp/done.log"
# Whether a ssh-daemon should be launched
runSSHD = False

class BaseRouter(Node):
    """"A Node with IP forwarding enabled."""
    def config( self, rootLog=None, **params ):
        super(BaseRouter, self).config(**params)
        print "Starting router at", self.IP(), rootLog
        for (gw, n, i) in otherNets:
            print "Adding route for", n, gw
            self.cmd( 'route add -net %s gw %s' % (n, gw) )
        if runSSHD:
            self.cmd('/usr/sbin/sshd -D &')

        self.cmd( 'sysctl net.ipv4.ip_forward=1' )
        socat = "socat %s udp4-listen:%d,reuseaddr,fork" % (logfile, socatPort)
        self.cmd( '%s &' % socat )
        if rootLog:
            self.cmd('tail -f %s | socat - udp-sendto:%s:%d &' % (logfile, rootLog, socatPort))

    def terminate( self ):
        print "Stopping router"
        for (gw, n, i) in otherNets:
            print "Deleting route for", n, gw
            self.cmd( 'route del -net %s gw %s' % (n, gw) )

        self.cmd( 'sysctl net.ipv4.ip_forward=0' )
        self.cmd( 'killall socat' )
        super(BaseRouter, self).terminate()


class Cothority(Host):
    """A cothority running in a host"""
    def config(self, gw=None, simul="", **params):
        self.gw = gw
        self.simul = simul
        super(Cothority, self).config(**params)
        if runSSHD:
            self.cmd('/usr/sbin/sshd -D &')

    def startCothority(self):
        # TODO: this will fail on other nodes than the root.
        self.cmd('cd ~/mininet_run')
        socat="socat -v - udp-sendto:%s:%d 2>> %s" % (self.gw, socatPort, logsocat)
        # print "Socat is", socat, "on", self.IP()

        if cothorityDummy:
            # print "Starting dummy with gw", gw
            self.cmd('while (ip a | grep "inet 10" ); do sleep 1; done | %s &' % socat)
        else:
            ldone = ""
            if self.IP().endswith(".0.2"):
                ldone = "; date > " + logdone
            # print "Starting cothority on node", self.IP(), ldone
            self.cmd('( ./cothority -debug %s -address %s:2000 -simul %s -monitor %s 2>&1 %s ) | %s &' %
                 ( debugging, self.IP(), self.simul, "10.90.38.3:10000 ", ldone, socat ))

    def terminate(self):
        # print "Stopping cothority"
        if cothorityDummy:
            self.cmd('killall while')

        self.cmd('killall socat cothority')
        super(Cothority, self).terminate()


class InternetTopo(Topo):
        """Create one switch with all hosts connected to it and host
        .1 as router - all in subnet 10.x.0.0/16"""
        def __init__(self, myNet=None, rootLog=None, **opts):
            Topo.__init__(self, **opts)
            server, mn, n = myNet[0]
            switch = self.addSwitch('s0')
            baseIp, prefix = netParse(mn)
            gw = ipAdd(1, prefix, baseIp)
            # print "Gw", gw, "baseIp", baseIp, prefix
            hostgw = self.addNode('h0', cls=BaseRouter,
                                  ip='%s/%d' % (gw, prefix),
                                  inNamespace=False,
                                  rootLog=rootLog)
            self.addLink(switch, hostgw)

            for i in range(1, int(n) + 1):
                ipStr = ipAdd(i + 1, prefix, baseIp)
                host = self.addHost('h%d' % i, cls=Cothority,
                                    ip = '%s/%d' % (ipStr, prefix),
                                    defaultRoute='via %s' % gw,
			                	    simul="CoSimul", gw=gw)
                # print "Adding link", host, switch
                self.addLink(host, switch)

def RunNet():
    """RunNet will start the mininet and add the routes to the other
    mininet-services"""
    rootLog = None
    if myNet[1] > 0:
        i, p = netParse(otherNets[0][1])
        rootLog = ipAdd(1, p, i)
    print "Creating network", myNet
    topo = InternetTopo(myNet=myNet, rootLog=rootLog)
    print "Starting on", myNet
    net = Mininet(topo=topo)
    net.start()

    for host in net.hosts[1:]:
        host.startCothority()

    # CLI(net)
    while not os.path.exists(logdone):
        print "Waiting for cothority to finish"
        time.sleep(1)

    # print "cothority is finished"
    print "Stopping on", myNet
    net.stop()

def GetNetworks(filename):
    """GetServer will read the file and search if the current server
    is in it and return those. It will also return whether we're in the
    first line and thus the 'root'-server for logging."""

    process = Popen(["ip", "a"], stdout=PIPE)
    (ips, err) = process.communicate()
    process.wait()

    with open(filename) as f:
        content = f.readlines()
    list = []
    for line in content:
        list.append(line.rstrip().split(' '))

    otherNets = []
    myNet = None
    pos = 0
    for (server, net, count) in list:
        t = [server, net, count]
        if ips.find('inet %s/' % server) >= 0:
            myNet = [t, pos]
        else:
            otherNets.append(t)
        pos += 1

    return myNet, otherNets

def rm_file(file):
    try:
        os.remove(file)
    except OSError:
        pass

def call_other(server, list_file):
    call("ssh -q %s sudo python -u start.py %s" % (server, list_file), shell=True)

# The only argument given to the script is the server-list. Everything
# else will be read from that and searched in the computer-configuration.
if __name__ == '__main__':
    # setLogLevel('info')
    # Mininet doesn't set up correctly if we put this loglevel
    lg.setLogLevel( 'critical')
    if len(sys.argv) < 2:
        print "please give list-name"
        sys.exit(-1)

    list_file = sys.argv[1]
    myNet, otherNets = GetNetworks(list_file)

    if myNet:
        print "Cleaning up mininet and logfiles"
        rm_file(logfile)
        rm_file(logsocat)
        rm_file(logdone)
        call("mn -c > /dev/null 2>&1", shell=True)
        # print "Starting mininet for", myNet
        t1 = threading.Thread(target=RunNet)
        t1.start()

    threads = []
    if len(sys.argv) > 2:
        for (server, mn, nbr) in otherNets:
            call("ssh -q %s mn -c > /dev/null 2>&1" % server, shell=True)
            # print "Going to copy things %s to %s and run %s hosts in net %s" % \
            #       (list_file, server, nbr, mn)
            call("scp -q simulation.bin cothority start.py %s %s:" % (list_file, server), shell=True)
            # print("going to clean mininet")
            # call("ssh %s /usr/local/bin/mn -c" % server, shell=True)
            # print "Launching script on %s" % server
            threads.append(threading.Thread(target=call_other,
                                            args=[server, list_file]))
            threads[-1].start()

    if myNet:
        t1.join()

    for thr in threads:
        thr.join()

    call("echo Done with main in $( hostname )", shell=True)
