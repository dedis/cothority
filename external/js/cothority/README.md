# Cothority client library in Javascript

This library offers methods to talk to a cothority node. At this point, it
offers a socket interface that marshals and unmarshals automatically protobuf
messages.

# Usage

```html
<html>
  <head>
    <meta charset="UTF-8">
    <script src="dist/bundle.min.js" type="text/javascript"></script>
    <script type="text/javascript">
        const net = cothority.net; // the network module 
        const proto = cothority.proto; // the protobuf module
        const serverAddress = "ws://127.0.0.1:8000"; 
        const socket = net.Socket(serverAddress,proto.root); // socket to talk to a conode
        
        // the data that we want to send, as a JS object
        const deviceMessage = { 
            point: new Uint8Array([1,2,3,4]);
        }
        // the name of the protobuf structure we are sending
        const sendingMessageName = "Device";
        // the name of the protobuf structure we expect to receive
        const expectedMessageName = "ID";
        socket.send(sendingMessageName, expectedMessageName, deviceMessage)
            .then((data) => {
                // data is a JS object
                console.log(data.id);
            }).catch((err) =>  {
                console.log("error: " + err);
            });
    </script>
  </head>
  <body>
  </body>
</html>

``` 

# Documentation

You can find the markdown generated documentation in `doc/doc.md`.

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
