"use strict";


/**
 * Socket is a WebSocket object instance through which protobuf messages are
 * sent to conodes.
 * @param {url} string conode identity
 * @param {object} messages protobufjs messages
 *
 * @throws {TypeError} when url is not a string or protobuf is not an object
 */
function Socket(node, messages) {
    if (typeof protobuf !== 'object')
	throw new TypeError;

    this.url = convertServerIdentityToWebSocket(node, '/nevv');
    this.protobuf = protobuf.Root.fromJSON(messages);

   /**
    * Send transmits data to a given url and parses the response.
    * @param {string} request name of registered protobuf message
    * @param {string} response name of registered protobuf message
    * @param {object} data to be sent
    *
    * @returns {object} Promise with response message on success, and an error on failure
    */
   this.send = (request, response, data) => {
       return new Promise((resolve, reject) => {
	    const ws = new WebSocket(this.url + '/' + request);
	    ws.binaryType = 'arraybuffer';

	    const requestModel = this.protobuf.lookup(request);
	    if (requestModel === undefined)
		reject(new Error('Model ' + request + ' not found'));
	    const responseModel = this.protobuf.lookup(response);
	    if (responseModel === undefined)
		reject(new Error('Model ' + response + ' not found'));

	    ws.onopen = () => {
		const message = requestModel.create(data);
		const marshal = requestModel.encode(message).finish();
		ws.send(marshal);
	    };

	    ws.onmessage = (event) => {
		ws.close();
		const buffer = new Uint8Array(event.data);
		const unmarshal = responseModel.decode(buffer);
		resolve(unmarshal);
	    };

	    ws.onclose = (event) => {
		if (!event.wasClean)
		    reject(new Error(event.reason));
	    };

	    ws.onerror = (error) => {
		reject(new Error('Could not connect to ' + this.url));
	    };
	});
   };
}


