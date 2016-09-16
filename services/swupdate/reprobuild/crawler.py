#!/usr/bin/env python3.5

try:
    from bs4 import BeautifulSoup
    from urllib.request import urlopen
    import random
    import subprocess
    import templates
    from datetime import datetime
    import psutil
    import csv, sys, os, time
except:
    raise


packages_required = ['attr', 'base-files', 'base-passwd', 'debconf', 'debianutils', 'diffutils',
                     'dpkg', 'findutils', 'grep', 'gzip', 'init-system-helpers', 'libselinux', 'libsepol',
                     'lsb', 'mawk', 'sed', 'sysvinit', 'pcre3', 'perl', 'util-linux', 'zlib']


packages_essential = ['debianutils', 'diffutils', 'e2fsprogs', 'findutils', 'perl', 'sysvinit', 'tar']

packages_popular = ['hostname', 'netbase', 'adduser', 'tzdata', 'bsdmainutils', 'cpio', 'logrotate',
                    'debian-archive-keyring', 'liblocale-gettext-perl', 'net-tools', 'ucf', 'popularity-contest',
                    'cron', 'manpages', 'libtext-wrapi18n-perl', 'iptables', 'ifupdown', 'man-db', 'mime-support',
                    'pciutils', 'libxml2', 'initramfs-tools', 'libcap2', 'dmidecode', 'busybox', 'file', 'less',
                    'ca-certificates', 'psmisc', 'nano', 'tasksel', 'insserv', 'installation-report', 'laptop-detect',
                    'linux-base', 'xml-core', 'aptitude', 'bzip2', 'os-prober', 'acpid', 'discover-data',
                    'bash-completion', 'dictionaries-common', 'eject', 'kmod', 'whois', 'iso-codes', 'geoip-database',
                    'bc', 'acpi']

packages_random = ['golang-github-hlandau-xlog', 'cal', 'libpath-dispatcher-declarative-perl', 'lunar-date', 'pmailq',
                   'aolserver4-nsxml', 'node-tilelive-vector', 'golang-github-hashicorp-go-getter', 'yacpi',
                   'libdata-stag-perl', 'libnet-oauth2-perl', 'libjs-jquery-dotdotdot', 'libclass-c3-adopt-next-perl',
                   'libobject-remote-perl', 'libxml-rsslite-perl', 'python-click-log', 'cl-salza2',
                   'globus-ftp-control', 'childsplay-alphabet-sounds-sl', 'fgetty', 'xmlextras', 'node-superagent',
                   'django-memoize', 'libtemplate-plugin-stash-perl', 'systraq', 'libtpl',
                   'libdist-zilla-plugin-config-git-perl', 'php-doctrine-cache-bundle', 'tz-converter', 'hackrf',
                   'slice', 'xfce4-taskmanager', 'sshfs-fuse', 'node-simplesmtp', 'visionegg',
                   'haskell-mutable-containers', 'gvfs', 'qdacco', 'haskell-ghc-events', 'ply', 'dymo-cups-drivers',
                   'ruby-bacon', 'liblinux-usermod-perl', 'puppet-module-puppetlabs-postgresql', 'jalview', 'masscan',
                   'octave-gsl', 'geronimo-ejb-3.2-spec', 'haskell-pcap', 'exuberant-ctags']


# Modifier for a dependency line
def parse_dpnd(li):
    li = li.replace(' ', '')
    li = li.replace(',', ' ')
    li = li.replace(')', '')
    li = li.replace('(', '')

    return li


# Build a container from the docker file and retrieve hash of the binary
def compile_bin(name, bina):
    comhash = ''
    wall_time = cpu_user = cpu_system = -1.0
    with open(name + '.log', 'w') as flog:
        wall_start_time = time.perf_counter()
        cpu_user_start, cpu_system_start = psutil.cpu_times().user, psutil.cpu_times().system
        tag = "reprod:" + name + "-" + str(os.getpid())
        subprocess.run(['docker', 'build', '--tag=' + tag, '--force-rm', '.'], stdout=flog,
                                 universal_newlines=True)

    try:
        comhash = subprocess.check_output(['docker', 'run', '--rm', tag, 'sha256sum',
                                                 bina]).decode('ascii', 'ignore').partition(' ')[0]
        cpu_user, cpu_system = psutil.cpu_times().user - cpu_user_start, psutil.cpu_times().system - cpu_system_start
        wall_time = time.perf_counter() - wall_start_time
        subprocess.run(['docker', 'rmi', tag], stdout='/dev/null')

    except:
        print("Some error while building", name)
        id = subprocess.check_output(['docker', 'images']).decode('ascii').split('\n')[1].split()[2]
        print(id)
        # subprocess.run(['docker', 'rmi', id], stdout='/dev/null')

    return comhash, round(wall_time, 3), round(cpu_user, 3), round(cpu_system, 3)


# Find and add two Debian snapshots preceding the build time
def find_snapshots(btime, f):
    snappage = urlopen(
        'http://snapshot.debian.org/archive/debian/?year=' + datetime.strftime(btime, '%Y') + ';month='
        + datetime.strftime(build_time, '%m')).read()
    snapsoup = BeautifulSoup(snappage, 'html.parser')
    snapbuf = []
    for snapshot in snapsoup.body.p.find_all('a'):
        snaptime = datetime.strptime(snapshot.string, '%Y-%m-%d %H:%M:%S')
        if snaptime < build_time:
            snapbuf.append(snapshot)  # snapbuf is a list of snapshots before the build
        else:
            break

    # Adding retrieved snapshots as sources to Dockerfile
    snap_url = "http://snapshot.debian.org"
    # snap_url = "http://icsil1-conode1.epfl.ch:3142/snapshot.debian.org"
    # snap_url = "http://icsil1-conode1.epfl.ch:3128/"
    if len(snapbuf) > 1:
        f.write('&& echo \'deb ' + snap_url + '/archive/debian/' + snapbuf[-2][
            'href'] + ' stretch main\' >> /etc/apt/sources.list \\ \n')
        f.write(' && echo \'deb-src ' + snap_url + '/archive/debian/' + snapbuf[-2][
            'href'] + ' stretch main\' >> /etc/apt/sources.list \\ \n')
        f.write(' && echo \'deb ' + snap_url + '/archive/debian/' + snapbuf[-1][
            'href'] + ' stretch main\' >> /etc/apt/sources.list \\ \n')
        f.write(' && echo \'deb-src ' + snap_url + '/archive/debian/' + snapbuf[-1][
            'href'] + ' stretch main\' >> /etc/apt/sources.list \n\n')
    elif len(snapbuf) == 1:
        f.write(' && echo \'deb ' + snap_url + '/archive/debian/' + snapbuf[-1][
            'href'] + ' stretch main\' >> /etc/apt/sources.list \\ \n')
        f.write(' && echo \'deb ' + snap_url + '/archive/debian/' + snapbuf[-1][
            'href'] + ' sid main\' >> /etc/apt/sources.list \n\n')
    else:
        print("The build is done before the first snapshot of the month!")


# Save build results into csv file
def save_results(yes, no, fail):
    with open('reprotest.csv', 'w') as f:
        csvf = csv.writer(f)
        csvf.writerow(['package', 'binary', 'size', 'wall_time', 'cpu_user_time', 'cpu_system_time', 'outcome'])
        for group in yes, no, fail:
            for pckg in group:
                csvf.writerow(pckg)


# Returns a set of packages to be built
def get_packages(option):
    packs = []

    if option == 'required':
        packs = packages_required

    elif option == 'essential':
        packs = packages_essential

    elif option == 'popular':
        packs = packages_popular

    elif option == 'random':
        packs = packages_random

    elif option == 'random_fresh':
        SET_SIZE = 3
        allpacks = []
        url = 'https://tests.reproducible-builds.org/debian/testing/amd64/index_reproducible.html'
        content = urlopen(url).read()
        soup = BeautifulSoup(content, 'html.parser')
        for p in soup.body.div.find('code').find_all('a', class_='package'):
            allpacks.append(p.string)
        packs = random.sample(allpacks, SET_SIZE)

    elif option == 'cli':
        packs = [sys.argv[2]]

    return packs


packages = get_packages(sys.argv[1])
baseurl = 'https://tests.reproducible-builds.org'
url = 'https://tests.reproducible-builds.org/debian/rb-pkg/testing/amd64/'
hash_match, hash_differ, failed = [], [], []

for p in packages:
    page = urlopen(url + p + '.html').read()
    soup = BeautifulSoup(page, 'html.parser')

    # Retrieve build time
    timestr = soup.body.header.find('span', {'class': 'build-time'}).string.split()[1:3]
    build_time = datetime.strptime(' '.join(timestr), '%Y-%m-%d %H:%M')

    f = open('Dockerfile', 'w')
    f.write(templates.Header1)
    find_snapshots(build_time, f)
    f.write(templates.Header2)

    # Find all the dependencies
    page = urlopen(baseurl + soup.body.header.find('a', {'title': 'Show: build info'})['href']).read()
    soup = BeautifulSoup(page, 'html.parser')

    lines = str(soup).split('\n')
    shaflag = dflag = False
    version = name = sha = binary = dir = short_version = size = ''
    first = True
    i = 0
    for line in lines:  # Extracting describing data
        if not dflag:  # Parse data about the package at first
            words = line.split()
            if len(words):
                if words[0] == 'Version:':
                    if ':' in words[1]:
                        version = words[1]
                        short_version = words[1].split(':')[1]
                    else:
                        version = short_version = words[1]
                elif words[0] == 'Binary:':  # To know binary name for the verified package
                    if p in words:
                        name = p
                    elif (p + '1') in words:
                        name = p + '1'
                    else:
                        name = words[1]
                elif words[0] == 'Source:':  # To know a name of the folder where to compile a package
                    dir = words[1]
            if shaflag == True:
                if (len(words) > 2) and (name + '_' + short_version in words[2] and '.deb' in words[2]):
                    binary = words[2]
                    sha = words[0]
                    size = words[1]
                    shaflag = False
            if line == 'Checksums-Sha256:':
                shaflag = True
            if line == 'Installed-Build-Depends:':  # All the describing data is found, move to dependencies
                dflag = True
        else:
            # Parsing dependencies
            i += 1
            if not first:
                if i % 3 == 1 and line != "":
                    f.write(' \\ \n')
                else:
                    f.write(' ')
            else:
                first = False
            f.write(parse_dpnd(line))

    print(binary)
    f.write('\n\n' + templates.Closer + '\n')
    f.write('\nWORKDIR /project')
    f.write('\nRUN ${HOSTS}; apt-get source ' + name + '=' + version)
    f.write('\nRUN ${HOSTS}; apt-get build-dep -y --force-yes ' + name + '=' + version)
    f.write('\nWORKDIR /project/' + dir + '-' + short_version.partition('-')[0] + '/')
    f.write('\nRUN ${HOSTS}; dpkg-buildpackage -us -uc -tc')
    f.write('\nWORKDIR /project')
    f.close()

    computed_hash, wtime, utime, stime = compile_bin(name, binary)

    if sha != '' and computed_hash == sha:
        hash_match.append((p, binary, size, wtime, utime, stime, 'y'))
        print("Hashes match for", name, computed_hash)
    else:
        if computed_hash == '':
            failed.append((p, binary, size, wtime, utime, stime, 'f'))
            print('Fail in the build process')
        else:
            hash_differ.append((p, binary, size, wtime, utime, stime, 'n'))
            print("Hashes differ for", name, computed_hash, "and", sha)

save_results(hash_match, hash_differ, failed)

print('\nBuilt packages with matching hash: ', hash_match)
print('Failed to build:', failed)
print('Built packages with differed hash:', hash_differ)
