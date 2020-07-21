# Version management

For the latest version of the conodes, please use 
https://github.com/c4dt/byzcoin, which provides a continuous release of valid
 conodes.

For the `@dedis/kyber` and `@dedis/cothority` npm libraries, we provide a
 development and a somewhat stable build.
The development build is always up-to-date with the latest commit and can be
 used with a reference to `@dedis/cothority@dev`:
```bash
npm i -s @dedis/cothority@dev
```
This pins the library to the latest available development version.

## Releasing a new version for npm @dedis/*

The npm-packages should follow semantic versioning.
As we don't plan to have any big changes, the major-version is probably stuck
 at `3`, except if we go the linux path and reach 3.9, a 4.0 could follow
 , but more to avoid 3.10 than because anything else - or not.
A minor-version increase should be done for a new stable functionality that
 will be kept backward-compatible.
A patch-version is mostly an irregular release whenever somebody thinks it's
 important to have the latest code available.
 
It is good to announce a release on the DEDIS/engineer slack channel.
This allows others to know that a new release is about to happen and propose
eventual changes.

As the cothority depends on the kyber package, it's currently a bit akward to
 update the package-version:
1. update the kyber-version in `kyber/package.json` and push to master
2. publish the new kyber-npm using `kyber/publish.sh`
3. update the kyber-dependency in `cothority/package.json` and the cothority
-version, which has to be the same as the kyber-version, push to master
4. publish the new cothority-npm using `cothority/publish.sh`
5. tag the latest commit using 
```
git tag v3.4.6
git push origin v3.4.6
```

## Development releases

Every merged PR will create a development release, which is named:

```
@dedis/cothority-major.minor.patch+1-pYYMM.DDHH.MMSS.0
```

## Releasing a binary

Up to 3.4.5, we released binaries for the conodes.
This was the last binary release of a conode.
For byzcoin-nodes, please use https://github.com/c4dt/byzcoin
