#!/usr/bin/python

"""
This will run a number of hosts on the server and do all
the routing to being able to connect to the other mininets.

You have to give it a list of server/net/nbr for each server
that has mininet installed and what subnet should be run
on it.
"""

import sys, time, threading

from mininet.topo import Topo
from mininet.net import Mininet
from mininet.cli import CLI
from mininet.log import lg
from mininet.node import Node, Host
from mininet.util import netParse, ipAdd
from subprocess import Popen, PIPE, call

# The port used for socat
socatPort = 5000
# If this is true, only a dummy-function will be started on each mininet-node
cothorityDummy = False
# What debugging-level to use
debugging = 2

class BaseRouter(Node):
    """"A Node with IP forwarding enabled."""
    def config( self, rootLog=None, **params ):
        super(BaseRouter, self).config(**params)
        self.cmd( 'sysctl net.ipv4.ip_forward=1' )
        socat = "socat /tmp/stdout.gw udp4-listen:%d,reuseaddr,fork" % socatPort
        self.cmd( '%s &' % socat )
        if rootLog:
            self.cmd('tail -f /tmp/stdout.gw | socat - udp-sendto:%s:%d &' % (rootLog, socatPort))

    def terminate( self ):
        self.cmd( 'sysctl net.ipv4.ip_forward=0' )
        self.cmd( 'killall socat' )
        super(BaseRouter, self).terminate()


class Cothority(Host):
    """A cothority running in a host"""
    def config(self, gw=None, simul="", **params):
        super(Cothority, self).config(**params)
        self.cmd('cd ~/mininet_run')
        socat="socat - udp-sendto:%s:%d" % (gw, socatPort)
        if cothorityDummy:
            self.cmd('while (ip a | grep "inet 10" ); do sleep 1; done | %s &' % socat)
        else:
            self.cmd('( pwd; ./cothority -debug %s -address %s:2000 -simul %s ) | %s &' % 
		( debugging, self.IP(), simul, socat ))

    def terminate(self):
        if cothorityDummy:
            self.cmd('killall while')

        self.cmd('killall socat')
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
				    simul="CoSimul",
                                    gw=gw)
                self.addLink(host, switch)

def RunNet():
    """RunNet will start the mininet and add the routes to the other
    mininet-services"""
    rootLog = None
    if myNet[1] > 0:
        i, p = netParse(otherNets[0][1])
        rootLog = ipAdd(1, p, i)
    topo = InternetTopo(myNet=myNet, rootLog=rootLog)
    print "Starting on", myNet
    net = Mininet(topo=topo)
    net.start()
    for (gw, n, i) in otherNets:
        net['h0'].cmd( 'route add -net %s gw %s' % (n, gw) )
    #CLI(net)
    time.sleep(5)
    for (gw, n, i) in otherNets:
        net['h0'].cmd( 'route del -net %s gw %s' % (n, gw) )

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

# The only argument given to the script is the server-list. Everything
# else will be read from that and searched in the computer-configuration.
if __name__ == '__main__':
    lg.setLogLevel( 'critical')
    if len(sys.argv) < 2:
        print "please give list-name"
        sys.exit(-1)

    file = sys.argv[1]
    myNet, otherNets = GetNetworks(file)

    if myNet:
        t1 = threading.Thread(target=RunNet)
        t1.start()

    if len(sys.argv) > 2:
        for (server, mn, nbr) in otherNets:
            print "Going to copy things to %s and run %s hosts in net %s" % (server, nbr, mn)
            call("scp -q start.py %s %s:" % (file, server), shell=True)
            # print("going to clean mininet")
            # call("ssh %s /usr/local/bin/mn -c" % server, shell=True)
            call("ssh -q %s sudo python -u start.py %s &" % (server, file), shell=True)
            time.sleep(0.1)

    if myNet:
        t1.join()
        time.sleep(1)
