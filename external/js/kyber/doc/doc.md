## Modules

<dl>
<dt><a href="#module_constants">constants</a></dt>
<dd></dd>
<dt><a href="#module_group/nist/point">group/nist/point</a></dt>
<dd></dd>
<dt><a href="#module_group/nist/scalar">group/nist/scalar</a></dt>
<dd></dd>
</dl>

## Classes

<dl>
<dt><a href="#Edwards25519">Edwards25519</a></dt>
<dd><p>Represents an Ed25519 curve</p>
</dd>
<dt><a href="#Point">Point</a></dt>
<dd><p>Represents a Point on the twisted edwards curve
(X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT</p>
<p>The value of the parameters is expcurveted in little endian form if being
passed as a Uint8Array</p>
</dd>
<dt><a href="#Scalar">Scalar</a></dt>
<dd><p>Scalar represents a value in GF(2^252 + 27742317777372353535851937790883648493)</p>
</dd>
<dt><a href="#Weierstrass">Weierstrass</a></dt>
<dd><p>Class Weierstrass defines the weierstrass form of
elliptic curves</p>
</dd>
</dl>

## Functions

<dl>
<dt><a href="#bits">bits(bitlen, exact, callback)</a> ⇒ <code>Uint8Array</code></dt>
<dd><p>bits choses a random Uint8Array with a maximum bitlength
If exact is <code>true</code>, chose Uint8Array with <em>exactly</em> that bitlenght not less</p>
</dd>
<dt><a href="#int">int(mod, callback)</a> ⇒ <code>Uint8Array</code></dt>
<dd><p>int choses a random uniform Uint8Array less than given modulus</p>
</dd>
<dt><a href="#Sign">Sign(privateKey, message)</a> ⇒</dt>
<dd><p>Sign computes a Schnorr signature over the given message.</p>
</dd>
<dt><a href="#Verify">Verify()</a> ⇒</dt>
<dd><p>Verify verifies if the signature of the message is valid under the given public
key.</p>
</dd>
<dt><a href="#hashSchnorr">hashSchnorr()</a> ⇒</dt>
<dd><p>hashSchnorr returns a scalar out of hashing the given inputs.</p>
</dd>
</dl>

<a name="module_constants"></a>

## constants
<a name="module_constants..constants"></a>

### constants~constants : <code>Object</code>
Constants

**Kind**: inner typedef of [<code>constants</code>](#module_constants)  
**Properties**

| Name | Type | Description |
| --- | --- | --- |
| zeroBN | <code>BN.jsObject</code> | BN.js object representing 0 |

<a name="module_group/nist/point"></a>

## group/nist/point

* [group/nist/point](#module_group/nist/point)
    * [~Point](#module_group/nist/point..Point)
        * [new Point(curve, x, y)](#new_module_group/nist/point..Point_new)
        * [.toString()](#module_group/nist/point..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_group/nist/point..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_group/nist/point..Point+set) ⇒ <code>module:group/nist~Point</code>
        * [.clone()](#module_group/nist/point..Point+clone) ⇒ <code>module:group/nist~Point</code>
        * [.null()](#module_group/nist/point..Point+null) ⇒ <code>module:group/nist~Point</code>
        * [.base()](#module_group/nist/point..Point+base) ⇒ <code>module:group/nist~Point</code>
        * [.embedLen()](#module_group/nist/point..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, [callback])](#module_group/nist/point..Point+embed) ⇒ <code>module:group/nist~Point</code>
        * [.data()](#module_group/nist/point..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_group/nist/point..Point+add) ⇒ <code>module:group/nist~Point</code>
        * [.sub(p1, p2)](#module_group/nist/point..Point+sub) ⇒ <code>module:group/nist~Point</code>
        * [.neg(p)](#module_group/nist/point..Point+neg) ⇒ <code>module:group/nist~Point</code>
        * [.mul(s, [p])](#module_group/nist/point..Point+mul) ⇒ <code>module:group/nist~Point</code>
        * [.pick([callback])](#module_group/nist/point..Point+pick) ⇒ <code>module:group/nist~Point</code>
        * [.marshalBinary()](#module_group/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist/point..Point+unmarshalBinary)

<a name="module_group/nist/point..Point"></a>

### group/nist/point~Point
Represents a Point on the nist curve

The value of the parameters is expected in little endian form if being
passed as a Uint8Array

**Kind**: inner class of [<code>group/nist/point</code>](#module_group/nist/point)  

* [~Point](#module_group/nist/point..Point)
    * [new Point(curve, x, y)](#new_module_group/nist/point..Point_new)
    * [.toString()](#module_group/nist/point..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_group/nist/point..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_group/nist/point..Point+set) ⇒ <code>module:group/nist~Point</code>
    * [.clone()](#module_group/nist/point..Point+clone) ⇒ <code>module:group/nist~Point</code>
    * [.null()](#module_group/nist/point..Point+null) ⇒ <code>module:group/nist~Point</code>
    * [.base()](#module_group/nist/point..Point+base) ⇒ <code>module:group/nist~Point</code>
    * [.embedLen()](#module_group/nist/point..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, [callback])](#module_group/nist/point..Point+embed) ⇒ <code>module:group/nist~Point</code>
    * [.data()](#module_group/nist/point..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_group/nist/point..Point+add) ⇒ <code>module:group/nist~Point</code>
    * [.sub(p1, p2)](#module_group/nist/point..Point+sub) ⇒ <code>module:group/nist~Point</code>
    * [.neg(p)](#module_group/nist/point..Point+neg) ⇒ <code>module:group/nist~Point</code>
    * [.mul(s, [p])](#module_group/nist/point..Point+mul) ⇒ <code>module:group/nist~Point</code>
    * [.pick([callback])](#module_group/nist/point..Point+pick) ⇒ <code>module:group/nist~Point</code>
    * [.marshalBinary()](#module_group/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist/point..Point+unmarshalBinary)

<a name="new_module_group/nist/point..Point_new"></a>

#### new Point(curve, x, y)

| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> | Weierstrass curve |
| x | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |
| y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |

<a name="module_group/nist/point..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/nist~Point</code> | Point object to compare |

<a name="module_group/nist/point..Point+set"></a>

#### point.set(p2) ⇒ <code>module:group/nist~Point</code>
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/nist~Point</code> | Point object |

<a name="module_group/nist/point..Point+clone"></a>

#### point.clone() ⇒ <code>module:group/nist~Point</code>
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - new Point object  
<a name="module_group/nist/point..Point+null"></a>

#### point.null() ⇒ <code>module:group/nist~Point</code>
Set to the neutral element for the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+base"></a>

#### point.base() ⇒ <code>module:group/nist~Point</code>
Set to the standard base point for this curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+embed"></a>

#### point.embed(data, [callback]) ⇒ <code>module:group/nist~Point</code>
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | data to embed with length <= embedLen |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist/point..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_group/nist/point..Point+add"></a>

#### point.add(p1, p2) ⇒ <code>module:group/nist~Point</code>
Returns the sum of two points on the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/nist~Point</code> | Point object, addend |
| p2 | <code>module:group/nist~Point</code> | Point object, addend |

<a name="module_group/nist/point..Point+sub"></a>

#### point.sub(p1, p2) ⇒ <code>module:group/nist~Point</code>
Subtract two points
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/nist~Point</code> | Point object |
| p2 | <code>module:group/nist~Point</code> | Point object |

<a name="module_group/nist/point..Point+neg"></a>

#### point.neg(p) ⇒ <code>module:group/nist~Point</code>
Finds the negative of a point p
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | <code>module:group/nist~Point</code> | Point to negate |

<a name="module_group/nist/point..Point+mul"></a>

#### point.mul(s, [p]) ⇒ <code>module:group/nist~Point</code>
Multiply point p by scalar s.
If p is not passed then multiplies the base point of the curve with
scalar s
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| s | <code>module:group/nist~Scalar</code> |  | Scalar |
| [p] | <code>module:group/nist~Point</code> | <code></code> | Point |

<a name="module_group/nist/point..Point+pick"></a>

#### point.pick([callback]) ⇒ <code>module:group/nist~Point</code>
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist/point..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
converts a point into the form specified in section 4.3.6 of ANSI X9.62.

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_group/nist/point..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes)
Convert a Uint8Array back to a curve point.
Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_group/nist/scalar"></a>

## group/nist/scalar

* [group/nist/scalar](#module_group/nist/scalar)
    * [~Scalar](#module_group/nist/scalar..Scalar)
        * [new Scalar(curve, red)](#new_module_group/nist/scalar..Scalar_new)
        * [.equal(s2)](#module_group/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_group/nist/scalar..Scalar+set) ⇒ <code>module:group/nist~Scalar</code>
        * [.clone()](#module_group/nist/scalar..Scalar+clone) ⇒ <code>module:group/nist~Scalar</code>
        * [.zero()](#module_group/nist/scalar..Scalar+zero) ⇒ <code>module:group/nist~Scalar</code>
        * [.add(s1, s2)](#module_group/nist/scalar..Scalar+add) ⇒ <code>module:group/nist~Scalar</code>
        * [.sub(s1, s2)](#module_group/nist/scalar..Scalar+sub) ⇒ <code>module:group/nist~Scalar</code>
        * [.neg(a)](#module_group/nist/scalar..Scalar+neg) ⇒ <code>module:group/nist~Scalar</code>
        * [.one()](#module_group/nist/scalar..Scalar+one) ⇒ <code>module:group/nist~Scalar</code>
        * [.mul(s1, s2)](#module_group/nist/scalar..Scalar+mul) ⇒ <code>module:group/nist~Scalar</code>
        * [.div(s1, s2)](#module_group/nist/scalar..Scalar+div) ⇒ <code>module:group/nist~Scalar</code>
        * [.inv(a)](#module_group/nist/scalar..Scalar+inv) ⇒ <code>module:group/nist~Scalar</code>
        * [.setBytes(b)](#module_group/nist/scalar..Scalar+setBytes) ⇒ <code>module:group/nist~Scalar</code>
        * [.bytes()](#module_group/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick()](#module_group/nist/scalar..Scalar+pick) ⇒ <code>module:group/nist~Scalar</code>
        * [.marshalBinary()](#module_group/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="module_group/nist/scalar..Scalar"></a>

### group/nist/scalar~Scalar
Scalar

**Kind**: inner class of [<code>group/nist/scalar</code>](#module_group/nist/scalar)  

* [~Scalar](#module_group/nist/scalar..Scalar)
    * [new Scalar(curve, red)](#new_module_group/nist/scalar..Scalar_new)
    * [.equal(s2)](#module_group/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_group/nist/scalar..Scalar+set) ⇒ <code>module:group/nist~Scalar</code>
    * [.clone()](#module_group/nist/scalar..Scalar+clone) ⇒ <code>module:group/nist~Scalar</code>
    * [.zero()](#module_group/nist/scalar..Scalar+zero) ⇒ <code>module:group/nist~Scalar</code>
    * [.add(s1, s2)](#module_group/nist/scalar..Scalar+add) ⇒ <code>module:group/nist~Scalar</code>
    * [.sub(s1, s2)](#module_group/nist/scalar..Scalar+sub) ⇒ <code>module:group/nist~Scalar</code>
    * [.neg(a)](#module_group/nist/scalar..Scalar+neg) ⇒ <code>module:group/nist~Scalar</code>
    * [.one()](#module_group/nist/scalar..Scalar+one) ⇒ <code>module:group/nist~Scalar</code>
    * [.mul(s1, s2)](#module_group/nist/scalar..Scalar+mul) ⇒ <code>module:group/nist~Scalar</code>
    * [.div(s1, s2)](#module_group/nist/scalar..Scalar+div) ⇒ <code>module:group/nist~Scalar</code>
    * [.inv(a)](#module_group/nist/scalar..Scalar+inv) ⇒ <code>module:group/nist~Scalar</code>
    * [.setBytes(b)](#module_group/nist/scalar..Scalar+setBytes) ⇒ <code>module:group/nist~Scalar</code>
    * [.bytes()](#module_group/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick()](#module_group/nist/scalar..Scalar+pick) ⇒ <code>module:group/nist~Scalar</code>
    * [.marshalBinary()](#module_group/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="new_module_group/nist/scalar..Scalar_new"></a>

#### new Scalar(curve, red)

| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> |  |
| red | <code>BN.Red</code> | BN.js Reduction context |

<a name="module_group/nist/scalar..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+set"></a>

#### scalar.set(a) ⇒ <code>module:group/nist~Scalar</code>
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+clone"></a>

#### scalar.clone() ⇒ <code>module:group/nist~Scalar</code>
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+zero"></a>

#### scalar.zero() ⇒ <code>module:group/nist~Scalar</code>
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Returns**: <code>module:group/nist~Scalar</code> - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | Scalar |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Returns**: <code>module:group/nist~Scalar</code> - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | Scalar |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+neg"></a>

#### scalar.neg(a) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+one"></a>

#### scalar.one() ⇒ <code>module:group/nist~Scalar</code>
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | 
| s2 | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | 
| s2 | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+inv"></a>

#### scalar.inv(a) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| a | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ <code>module:group/nist~Scalar</code>
Sets the scalar from big-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type |
| --- | --- |
| b | <code>Uint8Array</code> | 

<a name="module_group/nist/scalar..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+pick"></a>

#### scalar.pick() ⇒ <code>module:group/nist~Scalar</code>
Set to a random scalar

param {function} [callback] - to generate randomBytes of given length

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (big endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes) ⇒ <code>undefined</code>
Reads the binary representation (big endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="Edwards25519"></a>

## Edwards25519
Represents an Ed25519 curve

**Kind**: global class  

* [Edwards25519](#Edwards25519)
    * [.string()](#Edwards25519+string) ⇒ <code>string</code>
    * [.scalarLen()](#Edwards25519+scalarLen) ⇒ <code>number</code>
    * [.scalar()](#Edwards25519+scalar) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.pointLen()](#Edwards25519+pointLen) ⇒ <code>number</code>
    * [.point()](#Edwards25519+point) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.newKey()](#Edwards25519+newKey) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>

<a name="Edwards25519+string"></a>

### edwards25519.string() ⇒ <code>string</code>
Return the name of the curve

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Edwards25519+scalarLen"></a>

### edwards25519.scalarLen() ⇒ <code>number</code>
Returns 32, the size in bytes of a Scalar on Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Edwards25519+scalar"></a>

### edwards25519.scalar() ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Returns a new Scalar for the prime-order subgroup of Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Edwards25519+pointLen"></a>

### edwards25519.pointLen() ⇒ <code>number</code>
Returns 32, the size of a Point on Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Edwards25519+point"></a>

### edwards25519.point() ⇒ <code>module:group/edwards25519/point~Point</code>
Creates a new point on the Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Edwards25519+newKey"></a>

### edwards25519.newKey() ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
it to be a multiple of 8).

**Kind**: instance method of [<code>Edwards25519</code>](#Edwards25519)  
<a name="Point"></a>

## Point
Represents a Point on the twisted edwards curve
(X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT

The value of the parameters is expcurveted in little endian form if being
passed as a Uint8Array

**Kind**: global class  

* [Point](#Point)
    * [new Point(curve, X, Y, Z, T)](#new_Point_new)
    * [.toString()](#Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#Point+set) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.clone()](#Point+clone) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.null()](#Point+null) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.base()](#Point+base) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.embedLen()](#Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, callback)](#Point+embed) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.data()](#Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#Point+add) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.sub(p1, p2)](#Point+sub) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.neg(p)](#Point+neg) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.mul(s, p)](#Point+mul) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.pick(callback)](#Point+pick) ⇒ <code>module:group/edwards25519/point~Point</code>
    * [.marshalBinary()](#Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#Point+unmarshalBinary) ⇒ <code>module:group/edwards25519/point~Point</code>

<a name="new_Point_new"></a>

### new Point(curve, X, Y, Z, T)

| Param | Type |
| --- | --- |
| curve | <code>module:group/edwards25519~Edwards25519</code> | 
| X | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| Y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| Z | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| T | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 

<a name="Point+toString"></a>

### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#Point)  
<a name="Point+equal"></a>

### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point to compare |

<a name="Point+set"></a>

### point.set(p2) ⇒ <code>module:group/edwards25519/point~Point</code>
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point |

<a name="Point+clone"></a>

### point.clone() ⇒ <code>module:group/edwards25519/point~Point</code>
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#Point)  
**Returns**: <code>module:group/edwards25519/point~Point</code> - new Point module:group/edwards25519/point~Point  
<a name="Point+null"></a>

### point.null() ⇒ <code>module:group/edwards25519/point~Point</code>
Set to the neutral element, which is (0, 1) for twisted Edwards
Curve

**Kind**: instance method of [<code>Point</code>](#Point)  
<a name="Point+base"></a>

### point.base() ⇒ <code>module:group/edwards25519/point~Point</code>
Set to the standard base point for this curve

**Kind**: instance method of [<code>Point</code>](#Point)  
<a name="Point+embedLen"></a>

### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#Point)  
<a name="Point+embed"></a>

### point.embed(data, callback) ⇒ <code>module:group/edwards25519/point~Point</code>
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | to embed with length <= embedLen |
| callback | <code>function</code> | to generate a random byte array of given length |

<a name="Point+data"></a>

### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="Point+add"></a>

### point.add(p1, p2) ⇒ <code>module:group/edwards25519/point~Point</code>
Returns the sum of two points on the curve

**Kind**: instance method of [<code>Point</code>](#Point)  
**Returns**: <code>module:group/edwards25519/point~Point</code> - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point, addend |
| p2 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point, addend |

<a name="Point+sub"></a>

### point.sub(p1, p2) ⇒ <code>module:group/edwards25519/point~Point</code>
Subtract two points

**Kind**: instance method of [<code>Point</code>](#Point)  
**Returns**: <code>module:group/edwards25519/point~Point</code> - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point |
| p2 | <code>module:group/edwards25519/point~Point</code> | Point module:group/edwards25519/point~Point |

<a name="Point+neg"></a>

### point.neg(p) ⇒ <code>module:group/edwards25519/point~Point</code>
Finds the negative of a point p
For Edwards Curves, the negative of (x, y) is (-x, y)

**Kind**: instance method of [<code>Point</code>](#Point)  
**Returns**: <code>module:group/edwards25519/point~Point</code> - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | <code>module:group/edwards25519/point~Point</code> | Point to negate |

<a name="Point+mul"></a>

### point.mul(s, p) ⇒ <code>module:group/edwards25519/point~Point</code>
Multiply point p by scalar s

**Kind**: instance method of [<code>Point</code>](#Point)  

| Param | Type | Description |
| --- | --- | --- |
| s | <code>module:group/edwards25519/point~Point</code> | Scalar |
| p | <code>module:group/edwards25519/point~Point</code> | Point |

<a name="Point+pick"></a>

### point.pick(callback) ⇒ <code>module:group/edwards25519/point~Point</code>
Selects a random point

**Kind**: instance method of [<code>Point</code>](#Point)  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | to generate a random byte array of given length |

<a name="Point+marshalBinary"></a>

### point.marshalBinary() ⇒ <code>Uint8Array</code>
Convert a ed25519 curve point into a byte representation

**Kind**: instance method of [<code>Point</code>](#Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="Point+unmarshalBinary"></a>

### point.unmarshalBinary(bytes) ⇒ <code>module:group/edwards25519/point~Point</code>
Convert a Uint8Array back to a ed25519 curve point
[tools.ietf.org/html/rfc8032#scurvetion-5.1.3](tools.ietf.org/html/rfc8032#scurvetion-5.1.3)

**Kind**: instance method of [<code>Point</code>](#Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="Scalar"></a>

## Scalar
Scalar represents a value in GF(2^252 + 27742317777372353535851937790883648493)

**Kind**: global class  

* [Scalar](#Scalar)
    * [.equal(s2)](#Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#Scalar+set) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.clone()](#Scalar+clone) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.zero()](#Scalar+zero) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.add(s1, s2)](#Scalar+add) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.sub(s1, s2)](#Scalar+sub) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.neg(a)](#Scalar+neg) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.one()](#Scalar+one) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.mul(s1, s2)](#Scalar+mul) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.div(s1, s2)](#Scalar+div) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.inv(a)](#Scalar+inv) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.setBytes(b)](#Scalar+setBytes) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.bytes()](#Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick(callback)](#Scalar+pick) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
    * [.marshalBinary()](#Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#Scalar+unmarshalBinary)

<a name="Scalar+equal"></a>

### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |

<a name="Scalar+set"></a>

### scalar.set(a) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |

<a name="Scalar+clone"></a>

### scalar.clone() ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
<a name="Scalar+zero"></a>

### scalar.zero() ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
<a name="Scalar+add"></a>

### scalar.add(s1, s2) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
**Returns**: <code>module:group/edwards25519/scalar~Scalar</code> - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |
| s2 | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |

<a name="Scalar+sub"></a>

### scalar.sub(s1, s2) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
**Returns**: <code>module:group/edwards25519/scalar~Scalar</code> - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |
| s2 | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |

<a name="Scalar+neg"></a>

### scalar.neg(a) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/edwards25519/scalar~Scalar</code> | Scalar |

<a name="Scalar+one"></a>

### scalar.one() ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
<a name="Scalar+mul"></a>

### scalar.mul(s1, s2) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param | Type |
| --- | --- |
| s1 | <code>module:group/edwards25519/scalar~Scalar</code> | 
| s2 | <code>module:group/edwards25519/scalar~Scalar</code> | 

<a name="Scalar+div"></a>

### scalar.div(s1, s2) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param |
| --- |
| s1 | 
| s2 | 

<a name="Scalar+inv"></a>

### scalar.inv(a) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param |
| --- |
| a | 

<a name="Scalar+setBytes"></a>

### scalar.setBytes(b) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Sets the scalar from little-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type | Description |
| --- | --- | --- |
| b | <code>Uint8Array</code> | bytes |

<a name="Scalar+bytes"></a>

### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
<a name="Scalar+pick"></a>

### scalar.pick(callback) ⇒ <code>module:group/edwards25519/scalar~Scalar</code>
Set to a random scalar

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | to generate random byte array of given length |

<a name="Scalar+marshalBinary"></a>

### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (little endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  
<a name="Scalar+unmarshalBinary"></a>

### scalar.unmarshalBinary(bytes)
Reads the binary representation (little endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#Scalar)  

| Param |
| --- |
| bytes | 

<a name="Weierstrass"></a>

## Weierstrass
Class Weierstrass defines the weierstrass form of
elliptic curves

**Kind**: global class  

* [Weierstrass](#Weierstrass)
    * [new Weierstrass(config)](#new_Weierstrass_new)
    * [.string()](#Weierstrass+string) ⇒ <code>string</code>
    * [.scalarLen()](#Weierstrass+scalarLen) ⇒ <code>number</code>
    * [.scalar()](#Weierstrass+scalar) ⇒ [<code>Scalar</code>](#module_group/nist/scalar..Scalar)
    * [.pointLen()](#Weierstrass+pointLen) ⇒ <code>number</code>
    * [.point()](#Weierstrass+point) ⇒ [<code>Point</code>](#module_group/nist/point..Point)

<a name="new_Weierstrass_new"></a>

### new Weierstrass(config)
Create a new Weierstrass Curve


| Param | Type | Description |
| --- | --- | --- |
| config | <code>object</code> | Curve configuration |
| config.name | <code>String</code> | Curve name |
| config.p | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Order of the underlying field. Little Endian if string or Uint8Array. |
| config.a | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Curve Parameter a. Little Endian if string or Uint8Array. |
| config.b | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Curve Parameter b. Little Endian if string or Uint8Array. |
| config.n | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Order of the base point. Little Endian if string or Uint8Array |
| config.gx | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | x coordinate of the base point. Little Endian if string or Uint8Array |
| config.gy | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | y coordinate of the base point. Little Endian if string or Uint8Array |
| config.bitSize | <code>number</code> | the size of the underlying field. |

<a name="Weierstrass+string"></a>

### weierstrass.string() ⇒ <code>string</code>
Returns the name of the curve

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+scalarLen"></a>

### weierstrass.scalarLen() ⇒ <code>number</code>
Returns the size in bytes of a scalar

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+scalar"></a>

### weierstrass.scalar() ⇒ [<code>Scalar</code>](#module_group/nist/scalar..Scalar)
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+pointLen"></a>

### weierstrass.pointLen() ⇒ <code>number</code>
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+point"></a>

### weierstrass.point() ⇒ [<code>Point</code>](#module_group/nist/point..Point)
Returns a new Point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="bits"></a>

## bits(bitlen, exact, callback) ⇒ <code>Uint8Array</code>
bits choses a random Uint8Array with a maximum bitlength
If exact is `true`, chose Uint8Array with *exactly* that bitlenght not less

**Kind**: global function  

| Param | Type | Description |
| --- | --- | --- |
| bitlen | <code>number</code> | Bitlength |
| exact | <code>boolean</code> |  |
| callback | <code>function</code> | to generate random Uint8Array of given length |

<a name="int"></a>

## int(mod, callback) ⇒ <code>Uint8Array</code>
int choses a random uniform Uint8Array less than given modulus

**Kind**: global function  

| Param | Type | Description |
| --- | --- | --- |
| mod | <code>BN.jsObject</code> | modulus |
| callback | <code>function</code> | to generate a random byte array |

<a name="Sign"></a>

## Sign(privateKey, message) ⇒
Sign computes a Schnorr signature over the given message.

**Kind**: global function  
**Returns**: signature as a Uint8Array  

| Param | Description |
| --- | --- |
| privateKey | private key scalar to sign with |
| message | message over which the signature is computed |

<a name="Verify"></a>

## Verify() ⇒
Verify verifies if the signature of the message is valid under the given public
key.

**Kind**: global function  
**Returns**: boolean true if signature is valid or false otherwise.  
**Params**: suite suite to use  
**Params**: publicKey public key under which to verify the signature  
**Params**: message message that is signed  
**Params**: signature signature made over the given message  
<a name="hashSchnorr"></a>

## hashSchnorr() ⇒
hashSchnorr returns a scalar out of hashing the given inputs.

**Kind**: global function  
**Returns**: a scalar  
**Params**: inputs a list of Uint8Array  
