# Version management

For the latest version of the conodes, please use
https://github.com/c4dt/byzcoin, which provides a continuous release of valid
 conodes for all use cases apart from e-voting.

For e-voting, we provide a tagged release with built binaries for Linux, Windows
MacOS and FreeBSD.

For the `@dedis/kyber` and `@dedis/cothority` npm libraries, we provide a
 development and a somewhat stable build.
The development build is always up-to-date with the latest commit and can be
 used with a reference to `@dedis/cothority@dev`:
```bash
npm i -s @dedis/cothority@dev
```
This pins the library to the latest available development version.

## Releasing a new version for e-voting conode

Tag the release using

```bash
git tag -s vX.Y.Z -m "Cothority Release vX.Y.Z"
```

and push it to `main` with `git push origin vX.Y.Z`. A tag push triggers
the `release` action workflow which releases new binaries in
`https://github.com/dedis/cothority/releases/tag/vX.Y.Z`. Please make sure to
add the release description by editing that page.

## Releasing a new version for npm @dedis/*

The npm-packages should follow [semantic versioning](https://semver.org).
As we don't plan to have any big changes, the major-version is probably stuck
 at `3`, except if we go the linux path and reach 3.9, a 4.0 could follow
 , but more to avoid 3.10 than because anything else - or not.
A minor-version increase should be done for a new stable functionality that
 will be kept backward-compatible.
A patch-version is mostly an irregular release whenever somebody thinks it's
 important to have the latest code available (use `npm version patch`).

It is good to announce a release on the DEDIS/engineer slack channel.
This allows others to know that a new release is about to happen and propose
eventual changes.

`@dedis/kyber` and `@dedis/cothority` can evolve independently, and they
consequently each have their own version and are published separately. However,
since cothority depends on kyber, kyber has to be published first if they are
both to be updated.

To publish a new release of `@dedis/kyber`:
1. update the kyber-version in `kyber/package.json`, commit and push to master
1. publish the new kyber-npm using `kyber/publish.sh`
1. create a signed and annotated tag on the latest commit (adjust with the
   updated version):
```
git tag -s kyber-js-v3.4.6
git push --tags origin
```

To publish a new release of `@dedis/cothority`:
1. update the cothority-version and (if needed) the kyber dependency version in
   `cothority/package.json`, commit and push to `main`
1. publish the new cothority-npm using `cothority/publish.sh`
1. create a signed and annotated tag on the latest commit (adjust with the
   updated version):
```
git tag -s cothority-js-v3.4.6
git push --tags origin
```

## Development releases

Every merged PR will create development releases, which are named:

```
@dedis/kyber-major.minor.patch+1-pYYMM.DDHH.MMSS.0
```
and:
```
@dedis/cothority-major.minor.patch+1-pYYMM.DDHH.MMSS.0
```

## Releasing a binary

Up to 3.4.5, we released binaries for the conodes.
This was the last binary release of a conode.
For byzcoin-nodes, please use https://github.com/c4dt/byzcoin
