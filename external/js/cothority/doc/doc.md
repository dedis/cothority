## Modules

<dl>
<dt><a href="#module_misc">misc</a></dt>
<dd></dd>
</dl>

## Functions

<dl>
<dt><a href="#Socket">Socket(string, protobufjs)</a></dt>
<dd><p>Socket is a WebSocket object instance through which protobuf messages are
sent to conodes.</p>
</dd>
<dt><a href="#send">send(request, response, data)</a> ⇒ <code>object</code></dt>
<dd><p>Send transmits data to a given url and parses the response.</p>
</dd>
</dl>

<a name="module_misc"></a>

## misc

* [misc](#module_misc)
    * [~uint8ArrayToHex(buffer)](#module_misc..uint8ArrayToHex) ⇒ <code>string</code>
    * [~hexToUint8Array(hex)](#module_misc..hexToUint8Array) ⇒ <code>Uint8Array</code>
    * [~reverseHex(hex)](#module_misc..reverseHex) ⇒ <code>string</code>

<a name="module_misc..uint8ArrayToHex"></a>

### misc~uint8ArrayToHex(buffer) ⇒ <code>string</code>
Convert a byte buffer to a hexadecimal string.

**Kind**: inner method of [<code>misc</code>](#module_misc)  
**Returns**: <code>string</code> - hexadecimal representation  
**Throws**:

- <code>TypeError</code> when buffer is not Uint8Array


| Param | Type |
| --- | --- |
| buffer | <code>Uint8Array</code> | 

<a name="module_misc..hexToUint8Array"></a>

### misc~hexToUint8Array(hex) ⇒ <code>Uint8Array</code>
Convert a hexadecimal string to a Uint8Array.

**Kind**: inner method of [<code>misc</code>](#module_misc)  
**Returns**: <code>Uint8Array</code> - byte buffer  
**Throws**:

- <code>TypeError</code> when hex is not a string


| Param | Type |
| --- | --- |
| hex | <code>string</code> | 

<a name="module_misc..reverseHex"></a>

### misc~reverseHex(hex) ⇒ <code>string</code>
Reverse a hexadecimal string.

**Kind**: inner method of [<code>misc</code>](#module_misc)  
**Returns**: <code>string</code> - reversed hex string  
**Throws**:

- <code>TypeError</code> when hex is not a string


| Param | Type |
| --- | --- |
| hex | <code>string</code> | 

<a name="Socket"></a>

## Socket(string, protobufjs)
Socket is a WebSocket object instance through which protobuf messages are
sent to conodes.

**Kind**: global function  
**Throws**:

- <code>TypeError</code> when url is not a string or protobuf is not an object


| Param | Type | Description |
| --- | --- | --- |
| string | <code>path</code> | websocket path. Composed from a node's address with the              websocket's service name |
| protobufjs | <code>object</code> | root messages. Usually just               use `require("cothority.protobuf").root` |

<a name="send"></a>

## send(request, response, data) ⇒ <code>object</code>
Send transmits data to a given url and parses the response.

**Kind**: global function  
**Returns**: <code>object</code> - Promise with response message on success, and an error on failure  

| Param | Type | Description |
| --- | --- | --- |
| request | <code>string</code> | name of registered protobuf message |
| response | <code>string</code> | name of registered protobuf message |
| data | <code>object</code> | to be sent |

