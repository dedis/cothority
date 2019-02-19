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

## Protobuf generation

To add a new protobuf file to the library, simply place your `*.proto` file
somewhere in `lib/protobuf/build/models` and then run 
```
npm run protobuf
```

That would compile all protobuf definitions into a single JSON file
(`models.json`). This json file is then embedded in the library automatically.

Publishing
----------

You must use the given script instead of `npm publish` because we need to publish
the _dist_ folder instead. If you try to use the official command, you will get
an error on purpose.
