(function (root, factory) {
  if (typeof define === 'function' && define.amd)
    define(['qrcode'], factory);
  else if (typeof exports === 'object')
    module.exports = factory(require('../build/qrcode'));
  else root.QCodeDecoder = factory(qrcode);
}(this, function (qrcode) {

'use strict';

/**
 * Constructor for QCodeDecoder
 */
function QCodeDecoder () {
  if (!(this instanceof QCodeDecoder))
    return new QCodeDecoder();

  this.timerCapture = null;
  this.canvasElem = null;
  this.stream = undefined;	
}

/**
 * Verifies if canvas element is supported.
 */
QCodeDecoder.prototype.isCanvasSupported = function () {
  var elem = document.createElement('canvas');

  return !!(elem.getContext && elem.getContext('2d'));
};


/**
 * Normalizes and Verifies if the user has
 * getUserMedia enabled in the browser.
 */
QCodeDecoder.prototype.hasGetUserMedia = function () {
  navigator.getUserMedia = navigator.getUserMedia ||
                           navigator.webkitGetUserMedia ||
                           navigator.mozGetUserMedia ||
                           navigator.msGetUserMedia;

  return !!(navigator.getUserMedia);
};

/**
 * Prepares the canvas element (which will
 * receive the image from the camera and provide
 * what the algorithm needs for checking for a
 * QRCode and then decoding it.)
 *
 *
 * @param  {DOMElement} canvasElem the canvas
 *                                 element
 * @param  {number} width      The width that
 *                             the canvas element
 *                             should have
 * @param  {number} height     The height that
 *                             the canvas element
 *                             should have
 * @return {DOMElement}            the canvas
 * after the resize if width and height
 * provided.
 */
QCodeDecoder.prototype._prepareCanvas = function (videoElem) {
  if (!this.canvasElem) {
    this.canvasElem = document.createElement('canvas');
    this.canvasElem.style.width = videoElem.videoWidth + "px";
    this.canvasElem.style.height = videoElem.videoHeight + "px";
    this.canvasElem.width = videoElem.videoWidth;
    this.canvasElem.height = videoElem.videoHeight;
  }

  qrcode.setCanvasElement(this.canvasElem);

  return this;
};

/**
 * Based on the video dimensions and the canvas
 * that was previously generated captures the
 * video/image source and then paints into the
 * canvas so that the decoder is able to work as
 * it expects.
 * @param  {Function} cb
 * @return {Object}      this
 */
QCodeDecoder.prototype._captureToCanvas = function (videoElem, cb, once) {
  if (this.timerCapture)
    clearTimeout(this.timerCapture);

  if (videoElem.videoWidth && videoElem.videoHeight) {
    if (!this.canvasElem)
      this._prepareCanvas(videoElem);

    var gCtx = this.canvasElem.getContext("2d");
    gCtx.clearRect(0, 0, videoElem.videoWidth,
                         videoElem.videoHeight);
    gCtx.drawImage(videoElem, 0, 0,
                   videoElem.videoWidth,
                   videoElem.videoHeight);
    try {
      cb(null, qrcode.decode());
      if (once) return;
    } catch (err){
      if (err !== "Couldn't find enough finder patterns")
        cb(new Error(err));
    }
  }

  this.timerCapture = setTimeout(function () {
    this._captureToCanvas.call(this, videoElem, cb, once);
  }.bind(this), 500);
};

/**
 * Prepares the video element for receiving
 * camera's input. Releases a stream if there
 * was any (resets).
 *
 * @param  {DOMElement} videoElem <video> dom
 *                                element
 * @param  {Function} errcb     callback
 *                              function to be
 *                              called in case of
 *                              error
 */
QCodeDecoder.prototype.decodeFromCamera = function (videoElem, cb, once) {
  var scope = (this.stop(), this);

  /*if (!this.hasGetUserMedia())*/
    //cb(new Error('Couldn\'t get video from camera'));

  var options =  true;
  if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
      alert("enumerateDevices() not supported.");
      return;
  }

    // first request user access to video
    options = { audio: false, video: { facingMode: { exact: "environment" } } }
    navigator.mediaDevices.getUserMedia(options).then(function (stream) {

        return navigator.mediaDevices.enumerateDevices();
    }).then(function(devices) {
            var found = false;
            devices.forEach(function(device) { 
                if (device.kind === 'videoinput') { 
                        if(device.label.toLowerCase().search("back") != -1) {
                            options={video: { 'deviceId': {'exact': device.deviceId } 
                                            , facingMode:'environment'},
                                     audio: false }; 
                            found = true;
            } } }); 
        // when it's right, launch the user media
        if (found) {
            return navigator.mediaDevices.getUserMedia(options)
        } else {
            return new Promise(function(resolve,reject) {
                reject("Did not found any environement camera");
            });
        }
    }).then(function (stream) {
        videoElem.src = window.URL.createObjectURL(stream);
        scope.videoElem = videoElem;
        scope.stream = stream;
        scope.videoDimensions = false;

        setTimeout(function () {
          scope._captureToCanvas.call(scope, videoElem, cb, once);
    }, 500);
  }).catch(function(err) {
      alert("error getusermedia:" + err);
      cb(err);
  });
 /* }).catch(function(err) {*/
      //alert("error enumerate:" + err);
 /*});*/
  return this;
};

QCodeDecoder.prototype.decodeFromVideo = function (videoElem, cb, once) {
  setTimeout(function () {
    this._captureToCanvas.call(this, videoElem, cb, once);
  }.bind(this), 500);

  return this;
};

/**
 * Decodes an image from its src.
 * @param  {DOMNode}   imageElemvideoElem
 * @param  {Function} cb        callback
 * @return {Object}             this
 */
QCodeDecoder.prototype.decodeFromImage = function (img, cb) {
  if (+img.nodeType > 0 && !img.src)
    throw new Error('The ImageElement must contain a src');

  img = img.src ? img.src : img;

  return (qrcode.decode(img, cb), this);
};



/**
 * Releases a video stream that was being
 * captured by prepareToVideo
 */
QCodeDecoder.prototype.stop = function() {
  if (this.stream) {
    this.stream.stop();
    this.stream = undefined;
  }

  if (this.timerCapture) {
    clearTimeout(this.timerCapture);
    this.timerCapture = undefined;
  }

  return this;
};

/**
 * Sets the sourceId for the camera to use.
 *
 * The sourceId can be found using the
 * getVideoSources function on a browser that
 * supports it (currently only Chrome).
 *
 * @param {String} sourceId     The id of the
 * video source you want to use (or false to use
 * the current default)
 */
QCodeDecoder.prototype.setSourceId = function (sourceId) {
  if (sourceId)
    this.videoConstraints.video = { optional: [{ sourceId: sourceId }]};
  else
    this.videoConstraints.video = true;

  return this;
};


/**
 * Gets a list of all available video sources on
 * the device
 */
QCodeDecoder.prototype.getVideoSources = function (cb) {
  var sources = [];

  if (MediaStreamTrack && MediaStreamTrack.getSources) {
    MediaStreamTrack.getSources(function (sourceInfos) {
      sourceInfos.forEach(function(sourceInfo) {
        if (sourceInfo.kind === 'video')
          sources.push(sourceInfo);
      });
      cb(null, sources);
    });
  } else {
    cb(new Error('Current browser doest not support MediaStreamTrack.getSources'));
  }

  return this;
};


return QCodeDecoder; }));
