# Cothority client library in Javascript

This library offers methods to talk to a cothority node. At this point, it
offers a socket interface that marshals and unmarshals automatically protobuf
messages.

# Usage

Import the library using
```js
import Cothority from "@dedis/cothority"
```

Check out the example in index.html for a browser-based usage

# Documentation

Execute `npm run doc` to generate the documentation and browse doc/index.html

# Development

You need to have `npm` installed. Then
```go
git clone https://github.com/dedis/cothority"
cd cothority/external/js/cothority
npm install
```

You should be able to run the tests with 
```
npm run test
```

To run the tests, be sure to have docker installed and `make docker` executed from the root of this repo.

## Protobuf generation

To add a new protobuf file to the library, simply place your `*.proto` file
somewhere in `lib/protobuf/build/models` and then run 
```
npm run protobuf
```

That would compile all protobuf definitions into a single JSON file
(`models.json`). This json file is then embedded in the library automatically.

### Message classes

You can write a class that will be used when decoding protobuf messages by using
this template:
```javascript
class MyMessage extends Message<MyMessage> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("abc.MyMessage", MyMessage, MyMessageDependency);
    }

    readonly myField: MyMessageDependency;

    constructor(props?: Properties<MyMessage>) {
        super(props);

        // whatever you need to do for initialization
    }
}

MyMessage.register();
```

Note that protobuf will instantiate with an empty object and then fill the fields
so this happens after the constructor has been called.
The _register_ is used to register the dependencies of the message but you also
have to use it as a side effect of the package so that as soon as the class is
imported, the message will be known by protobuf and used during decoding.

### Side note on Buffer

Protobuf definition and classes implemented expect a _Buffer_ for _bytes_ but
as you should know, in a browser environment _bytes_ are instantiated with
Uint8Array. You should then be aware that the actual type will be Uint8Array
when using the library in a browser environment *but* the buffer interface
will be provided thanks to the [buffer package](https://www.npmjs.com/package/buffer).

As this is a [polyfill](https://remysharp.com/2010/10/08/what-is-a-polyfill), please
check that what you need is implemented or you will need to use a different approach. Of
course for NodeJS, you will always get a [Buffer](https://nodejs.org/api/buffer.html).

## Use of a development version from an external app

The simplest way to use a cothority development version in an app and being able to 
add debug-lines and change the code is to add the following to your
`tsconfig.json`:

```json
{
  "compilerOptions": {
    "paths": {
      "@dedis/cothority": [
        "../cothority/external/js/cothority/src",
        "node_modules/@dedis/cothority/*"
      ],
      "@dedis/cothority/*": [
        "../cothority/external/js/cothority/src/*",
        "node_modules/@dedis/cothority/*"
      ]
    }
  }
}
```

This will look for the cothority-sources in the parent directory of your app and
include those. If it doesn't find them, it will use the sources found in the `node_modules`
directory.

It is important that the cothority-repository is in the parent directory, else
typescript will try to include it in the compilation.

Also, the cothority-sources need to have all the libraries installed with
`npm ci`, else the compilation will fail.

### Old way

Steps to use `js/cothority` as a local module for a local external app:

1) Build a version

```bash
(cothority/external/js/cothority) $ npm run build
```

2) Create a link

```bash
(cothority/external/js/cothority) $ npm run link
```

3) From the root folder of the external app, link the new package

```bash
(external_app) $ npm link @dedis/cothority
```

4) To unlink

```bash
(external_app) $ npm unlink @dedis/cothority
(cothority/external/js/cothority) $ npm unlink
```

# Releases

`@dedis/cothority` releases are done irregularly and on a 'as-needed' basis.
It is good to announce a release on the DEDIS/engineer slack channel.
This allows others to know that a new release is about to happen and propose
eventual changes.
Then a PR with the new version can be made and will be merged.
Once the PR is merged, please go on as described in [Publishing](#Publishing). 

## Publishing

You must use the given script instead of `npm publish` because we need to publish
the _dist_ folder instead. If you try to use the official command, you will get
an error on purpose.

## Development releases

Every merged PR will create a development release, which is named:

```
@dedis/cothority-major.minor.patch+1-pYYMM.DDHH.MMSS.0

```
