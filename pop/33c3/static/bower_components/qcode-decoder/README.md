# qcode-decoder

> Decodes QRCode in the browser

[![Sauce Test Status](https://saucelabs.com/browser-matrix/cirocosta_github.svg)](https://saucelabs.com/u/cirocosta_github)

## Using in your project

Download it as a dependency

```sh
$ bower install qcode-decoder
```

attach it to your `.html` file

```html
<script src="../bower_components/qcode-decoder/build/qcode-decoder.min.js"></script>
```

and use it!

**For a full example, see `/examples` or [this plunkr](http://plnkr.co/aWikiL)**

The API is Pretty simple:

### QCodeDecoder()
Constructor. No args. Might be create with or without `new`.

```javascript
var qr = new QCodeDecoder();
// or
var qr = QCodeDecoder();
```

This construction lets us be able to chain some methods (although not very necessary - the API is **really** simple).

#### ::decodeFromImage(img)

Decodes an image from a source provided or an `<img>` element with a `src` attribute set.

```javascript
qr.decodeFromImage(img, function (err, result) {
  if (err) throw err;

  alert(result);
});
```

#### ::decodeFromVideo(videoElem, cb, [,once])
Decodes directly from a video with a well specified `src` attribute

```javascript
QCodeDecoder()
  .decodeFromVideo(document.querySelector('video'), function (err, result) {
    if (err) throw err;

    alert(result);
  }, true);
```


#### ::decodeFromCamera(videoElem, cb, [,once])
Decodes from a videoElement. The optional argument **once** makes the *QCodeDecoder* to only find a QRCode once.

```javascript
qr.decodeFromCamera(videoElem, function (err) {
  if (err) throw err;

  alert(result);
});
```

#### ::stop()

Stops the current `qr` from searching for a QRCode.


## Messing around

The proper use of camera APIs and, then, the use of this module, the developer needs to first initiate a webserver for running the examples. I suggest going with [http-server](https://github.com/nodeapps/http-server).

## Credits

The main decoder methods are from [Lazar Laszlo](http://www.
lazarsoft.info/), who ported ZXing lib (Apache V2) to JavaScript.

## LICENSE

The MIT License (MIT)

Copyright (c) <2014> <Ciro S. Costa>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
