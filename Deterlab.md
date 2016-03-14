## DeterLab

If you use the `-platform deterlab` option, then you are prompted to 
enter the name of the DeterLab installation, your username, and the 
names of project and experiment. 
There are some flags which make your life as a cothority developer 
simpler when deploying to DeterLab:

* `-nobuild`: don't build any of the helpers which is useful if you're 
working on the main code
* `-build "helper1,helper2"`: only build the helpers, separated by a 
",", which speeds up recompiling
* `-range start:end`: runs only the simulation-lines including `start` 
and `end`. 
Counts from 0, start and end can be omitted and represent beginning and 
end of lines, respectively.

### SSH-keys
For convenience, we recommend that you upload a public SSH-key to the 
DeterLab site. 
If your SSH-key is protected through a passphrase (which should be the 
case for security reasons!) we further recommend that you add your 
private key to your SSH-agent / keychain. 
Afterwards you only need to unlock your SSH-agent / keychain once (per 
session) and can access all your stored keys without typing the 
passphrase each time.

**OSX:**

You can store your SSH-key directly in the OSX-keychain by executing:

```
$ /usr/bin/ssh-add -K ~/.ssh/<your private ssh key>
```

Make sure that you actually use the `ssh-add` program that comes with 
your OSX installation, since those installed through 
[homebrew](http://brew.sh/), [MacPorts](https://www.macports.org/) etc. 
**do not support** the `-K` flag per default.

**Linux:**

Make sure that the `ssh-agent` is running. Afterwards you can add your 
SSH-key via:

```
$ ssh-add ~/.ssh/<your private ssh key>
```

