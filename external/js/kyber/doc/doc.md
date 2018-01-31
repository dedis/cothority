## Modules

<dl>
<dt><a href="#module_group/nist">group/nist</a></dt>
<dd></dd>
<dt><a href="#module_group/nist">group/nist</a></dt>
<dd></dd>
</dl>

## Classes

<dl>
<dt><a href="#Weierstrass">Weierstrass</a></dt>
<dd><p>Class Weierstrass defines the weierstrass form of
elliptic curves</p>
</dd>
</dl>

<a name="module_group/nist"></a>

## group/nist

* [group/nist](#module_group/nist)
    * [~Point](#module_group/nist..Point)
        * [new Point(curve, x, y)](#new_module_group/nist..Point_new)
        * [.toString()](#module_group/nist..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_group/nist..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_group/nist..Point+set) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.clone()](#module_group/nist..Point+clone) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.null()](#module_group/nist..Point+null) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.base()](#module_group/nist..Point+base) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.embedLen()](#module_group/nist..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, [callback])](#module_group/nist..Point+embed) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.data()](#module_group/nist..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_group/nist..Point+add) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.sub(p1, p2)](#module_group/nist..Point+sub) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.neg(p)](#module_group/nist..Point+neg) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.mul(s, [p])](#module_group/nist..Point+mul) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.pick([callback])](#module_group/nist..Point+pick) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.marshalBinary()](#module_group/nist..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist..Point+unmarshalBinary)
    * [~Scalar](#module_group/nist..Scalar)
        * [new Scalar(curve, red)](#new_module_group/nist..Scalar_new)
        * [.equal(s2)](#module_group/nist..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_group/nist..Scalar+set) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.clone()](#module_group/nist..Scalar+clone) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.zero()](#module_group/nist..Scalar+zero) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.add(s1, s2)](#module_group/nist..Scalar+add) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.sub(s1, s2)](#module_group/nist..Scalar+sub) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.neg(a)](#module_group/nist..Scalar+neg) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.one()](#module_group/nist..Scalar+one) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.mul(s1, s2)](#module_group/nist..Scalar+mul) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.div(s1, s2)](#module_group/nist..Scalar+div) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.inv(a)](#module_group/nist..Scalar+inv) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.setBytes(b)](#module_group/nist..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.bytes()](#module_group/nist..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick()](#module_group/nist..Scalar+pick) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.marshalBinary()](#module_group/nist..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="module_group/nist..Point"></a>

### group/nist~Point
**Kind**: inner class of [<code>group/nist</code>](#module_group/nist)  

* [~Point](#module_group/nist..Point)
    * [new Point(curve, x, y)](#new_module_group/nist..Point_new)
    * [.toString()](#module_group/nist..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_group/nist..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_group/nist..Point+set) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.clone()](#module_group/nist..Point+clone) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.null()](#module_group/nist..Point+null) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.base()](#module_group/nist..Point+base) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.embedLen()](#module_group/nist..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, [callback])](#module_group/nist..Point+embed) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.data()](#module_group/nist..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_group/nist..Point+add) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.sub(p1, p2)](#module_group/nist..Point+sub) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.neg(p)](#module_group/nist..Point+neg) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.mul(s, [p])](#module_group/nist..Point+mul) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.pick([callback])](#module_group/nist..Point+pick) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.marshalBinary()](#module_group/nist..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist..Point+unmarshalBinary)

<a name="new_module_group/nist..Point_new"></a>

#### new Point(curve, x, y)
Represents a Point on the twisted nist curve
(X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT

The value of the parameters is expected in little endian form if being
passed as a Uint8Array


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> | Weierstrass curve |
| x | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |
| y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |

<a name="module_group/nist..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object to compare |

<a name="module_group/nist..Point+set"></a>

#### point.set(p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object |

<a name="module_group/nist..Point+clone"></a>

#### point.clone() ⇒ [<code>Point</code>](#module_group/nist..Point)
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - new Point object  
<a name="module_group/nist..Point+null"></a>

#### point.null() ⇒ [<code>Point</code>](#module_group/nist..Point)
Set to the neutral element for the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+base"></a>

#### point.base() ⇒ [<code>Point</code>](#module_group/nist..Point)
Set to the standard base point for this curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+embed"></a>

#### point.embed(data, [callback]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | data to embed with length <= embedLen |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_group/nist..Point+add"></a>

#### point.add(p1, p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
Returns the sum of two points on the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_group/nist..Point) | Point object, addend |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object, addend |

<a name="module_group/nist..Point+sub"></a>

#### point.sub(p1, p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
Subtract two points
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_group/nist..Point) | Point object |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object |

<a name="module_group/nist..Point+neg"></a>

#### point.neg(p) ⇒ [<code>Point</code>](#module_group/nist..Point)
Finds the negative of a point p
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | [<code>Point</code>](#module_group/nist..Point) | Point to negate |

<a name="module_group/nist..Point+mul"></a>

#### point.mul(s, [p]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Multiply point p by scalar s.
If p is not passed then multiplies the base point of the curve with
scalar s
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| s | [<code>Scalar</code>](#module_group/nist..Scalar) |  | Scalar |
| [p] | [<code>Point</code>](#module_group/nist..Point) | <code></code> | Point |

<a name="module_group/nist..Point+pick"></a>

#### point.pick([callback]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
converts a point into the form specified in section 4.3.6 of ANSI X9.62.

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_group/nist..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes)
Convert a Uint8Array back to a curve point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_group/nist..Scalar"></a>

### group/nist~Scalar
**Kind**: inner class of [<code>group/nist</code>](#module_group/nist)  

* [~Scalar](#module_group/nist..Scalar)
    * [new Scalar(curve, red)](#new_module_group/nist..Scalar_new)
    * [.equal(s2)](#module_group/nist..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_group/nist..Scalar+set) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.clone()](#module_group/nist..Scalar+clone) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.zero()](#module_group/nist..Scalar+zero) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.add(s1, s2)](#module_group/nist..Scalar+add) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.sub(s1, s2)](#module_group/nist..Scalar+sub) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.neg(a)](#module_group/nist..Scalar+neg) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.one()](#module_group/nist..Scalar+one) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.mul(s1, s2)](#module_group/nist..Scalar+mul) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.div(s1, s2)](#module_group/nist..Scalar+div) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.inv(a)](#module_group/nist..Scalar+inv) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.setBytes(b)](#module_group/nist..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.bytes()](#module_group/nist..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick()](#module_group/nist..Scalar+pick) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.marshalBinary()](#module_group/nist..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="new_module_group/nist..Scalar_new"></a>

#### new Scalar(curve, red)
Scalar


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> |  |
| red | <code>BN.Red</code> | BN.js Reduction context |

<a name="module_group/nist..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+set"></a>

#### scalar.set(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+clone"></a>

#### scalar.clone() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+zero"></a>

#### scalar.zero() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Returns**: [<code>Scalar</code>](#module_group/nist..Scalar) - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Returns**: [<code>Scalar</code>](#module_group/nist..Scalar) - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+neg"></a>

#### scalar.neg(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+one"></a>

#### scalar.one() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | 
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | 
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+inv"></a>

#### scalar.inv(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Sets the scalar from big-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type |
| --- | --- |
| b | <code>Uint8Array</code> | 

<a name="module_group/nist..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+pick"></a>

#### scalar.pick() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to a random scalar

param {function} [callback] - to generate randomBytes of given length

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (big endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes) ⇒ <code>undefined</code>
Reads the binary representation (big endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_group/nist"></a>

## group/nist

* [group/nist](#module_group/nist)
    * [~Point](#module_group/nist..Point)
        * [new Point(curve, x, y)](#new_module_group/nist..Point_new)
        * [.toString()](#module_group/nist..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_group/nist..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_group/nist..Point+set) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.clone()](#module_group/nist..Point+clone) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.null()](#module_group/nist..Point+null) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.base()](#module_group/nist..Point+base) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.embedLen()](#module_group/nist..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, [callback])](#module_group/nist..Point+embed) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.data()](#module_group/nist..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_group/nist..Point+add) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.sub(p1, p2)](#module_group/nist..Point+sub) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.neg(p)](#module_group/nist..Point+neg) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.mul(s, [p])](#module_group/nist..Point+mul) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.pick([callback])](#module_group/nist..Point+pick) ⇒ [<code>Point</code>](#module_group/nist..Point)
        * [.marshalBinary()](#module_group/nist..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist..Point+unmarshalBinary)
    * [~Scalar](#module_group/nist..Scalar)
        * [new Scalar(curve, red)](#new_module_group/nist..Scalar_new)
        * [.equal(s2)](#module_group/nist..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_group/nist..Scalar+set) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.clone()](#module_group/nist..Scalar+clone) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.zero()](#module_group/nist..Scalar+zero) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.add(s1, s2)](#module_group/nist..Scalar+add) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.sub(s1, s2)](#module_group/nist..Scalar+sub) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.neg(a)](#module_group/nist..Scalar+neg) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.one()](#module_group/nist..Scalar+one) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.mul(s1, s2)](#module_group/nist..Scalar+mul) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.div(s1, s2)](#module_group/nist..Scalar+div) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.inv(a)](#module_group/nist..Scalar+inv) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.setBytes(b)](#module_group/nist..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.bytes()](#module_group/nist..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick()](#module_group/nist..Scalar+pick) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
        * [.marshalBinary()](#module_group/nist..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="module_group/nist..Point"></a>

### group/nist~Point
**Kind**: inner class of [<code>group/nist</code>](#module_group/nist)  

* [~Point](#module_group/nist..Point)
    * [new Point(curve, x, y)](#new_module_group/nist..Point_new)
    * [.toString()](#module_group/nist..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_group/nist..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_group/nist..Point+set) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.clone()](#module_group/nist..Point+clone) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.null()](#module_group/nist..Point+null) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.base()](#module_group/nist..Point+base) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.embedLen()](#module_group/nist..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, [callback])](#module_group/nist..Point+embed) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.data()](#module_group/nist..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_group/nist..Point+add) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.sub(p1, p2)](#module_group/nist..Point+sub) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.neg(p)](#module_group/nist..Point+neg) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.mul(s, [p])](#module_group/nist..Point+mul) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.pick([callback])](#module_group/nist..Point+pick) ⇒ [<code>Point</code>](#module_group/nist..Point)
    * [.marshalBinary()](#module_group/nist..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist..Point+unmarshalBinary)

<a name="new_module_group/nist..Point_new"></a>

#### new Point(curve, x, y)
Represents a Point on the twisted nist curve
(X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT

The value of the parameters is expected in little endian form if being
passed as a Uint8Array


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> | Weierstrass curve |
| x | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |
| y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |

<a name="module_group/nist..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object to compare |

<a name="module_group/nist..Point+set"></a>

#### point.set(p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object |

<a name="module_group/nist..Point+clone"></a>

#### point.clone() ⇒ [<code>Point</code>](#module_group/nist..Point)
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - new Point object  
<a name="module_group/nist..Point+null"></a>

#### point.null() ⇒ [<code>Point</code>](#module_group/nist..Point)
Set to the neutral element for the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+base"></a>

#### point.base() ⇒ [<code>Point</code>](#module_group/nist..Point)
Set to the standard base point for this curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
<a name="module_group/nist..Point+embed"></a>

#### point.embed(data, [callback]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | data to embed with length <= embedLen |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_group/nist..Point+add"></a>

#### point.add(p1, p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
Returns the sum of two points on the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_group/nist..Point) | Point object, addend |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object, addend |

<a name="module_group/nist..Point+sub"></a>

#### point.sub(p1, p2) ⇒ [<code>Point</code>](#module_group/nist..Point)
Subtract two points
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_group/nist..Point) | Point object |
| p2 | [<code>Point</code>](#module_group/nist..Point) | Point object |

<a name="module_group/nist..Point+neg"></a>

#### point.neg(p) ⇒ [<code>Point</code>](#module_group/nist..Point)
Finds the negative of a point p
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: [<code>Point</code>](#module_group/nist..Point) - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | [<code>Point</code>](#module_group/nist..Point) | Point to negate |

<a name="module_group/nist..Point+mul"></a>

#### point.mul(s, [p]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Multiply point p by scalar s.
If p is not passed then multiplies the base point of the curve with
scalar s
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| s | [<code>Scalar</code>](#module_group/nist..Scalar) |  | Scalar |
| [p] | [<code>Point</code>](#module_group/nist..Point) | <code></code> | Point |

<a name="module_group/nist..Point+pick"></a>

#### point.pick([callback]) ⇒ [<code>Point</code>](#module_group/nist..Point)
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  

| Param | Type | Description |
| --- | --- | --- |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
converts a point into the form specified in section 4.3.6 of ANSI X9.62.

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_group/nist..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes)
Convert a Uint8Array back to a curve point

**Kind**: instance method of [<code>Point</code>](#module_group/nist..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_group/nist..Scalar"></a>

### group/nist~Scalar
**Kind**: inner class of [<code>group/nist</code>](#module_group/nist)  

* [~Scalar](#module_group/nist..Scalar)
    * [new Scalar(curve, red)](#new_module_group/nist..Scalar_new)
    * [.equal(s2)](#module_group/nist..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_group/nist..Scalar+set) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.clone()](#module_group/nist..Scalar+clone) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.zero()](#module_group/nist..Scalar+zero) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.add(s1, s2)](#module_group/nist..Scalar+add) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.sub(s1, s2)](#module_group/nist..Scalar+sub) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.neg(a)](#module_group/nist..Scalar+neg) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.one()](#module_group/nist..Scalar+one) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.mul(s1, s2)](#module_group/nist..Scalar+mul) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.div(s1, s2)](#module_group/nist..Scalar+div) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.inv(a)](#module_group/nist..Scalar+inv) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.setBytes(b)](#module_group/nist..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.bytes()](#module_group/nist..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick()](#module_group/nist..Scalar+pick) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
    * [.marshalBinary()](#module_group/nist..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="new_module_group/nist..Scalar_new"></a>

#### new Scalar(curve, red)
Scalar


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> |  |
| red | <code>BN.Red</code> | BN.js Reduction context |

<a name="module_group/nist..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+set"></a>

#### scalar.set(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+clone"></a>

#### scalar.clone() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+zero"></a>

#### scalar.zero() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Returns**: [<code>Scalar</code>](#module_group/nist..Scalar) - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Returns**: [<code>Scalar</code>](#module_group/nist..Scalar) - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+neg"></a>

#### scalar.neg(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | Scalar |

<a name="module_group/nist..Scalar+one"></a>

#### scalar.one() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | 
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_group/nist..Scalar) | 
| s2 | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+inv"></a>

#### scalar.inv(a) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| a | [<code>Scalar</code>](#module_group/nist..Scalar) | 

<a name="module_group/nist..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Sets the scalar from big-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type |
| --- | --- |
| b | <code>Uint8Array</code> | 

<a name="module_group/nist..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+pick"></a>

#### scalar.pick() ⇒ [<code>Scalar</code>](#module_group/nist..Scalar)
Set to a random scalar

param {function} [callback] - to generate randomBytes of given length

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (big endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  
<a name="module_group/nist..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes) ⇒ <code>undefined</code>
Reads the binary representation (big endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist..Scalar)  

| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="Weierstrass"></a>

## Weierstrass
Class Weierstrass defines the weierstrass form of
elliptic curves

**Kind**: global class  
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

